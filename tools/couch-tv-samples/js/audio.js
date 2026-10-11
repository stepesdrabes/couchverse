// Couch TV audio engine. Beds are decoded buffers in the graph (never <audio>), so every gain
// move is sample-accurate automation; the sonic logo and the whole SFX kit are synthesized.
// Every voice takes an absolute AudioContext time, so one Timeline drives picture and sound.
//
// Graph: piece buses (music / sfx / reverb send, one set per playing piece so the reel can
// crossfade pieces) -> music + sfx buses -> glue compressor -> brickwall-ish limiter -> trim.

import { rng } from './core.js';

// The Couch TV mnemonic: D up a fifth to A, down to F#, settling on E over a D major
// add9 chord. Short-short-long, open rather than resolved: "cou-ch-ver-se". The ident states
// it broadly on brass and bell; the promo tag restates it quicker.
const N = {
	D1: 36.71, Bb1: 58.27, D2: 73.42, F2: 87.31, A2: 110.0, Bb2: 116.54, D3: 146.83, E3: 164.81,
	F3: 174.61, Fs3: 185.0, A3: 220.0, D4: 293.66, E4: 329.63, F4: 349.23, Fs4: 369.99, A4: 440.0,
	D5: 587.33, E5: 659.25, Fs5: 739.99, A5: 880.0
};
export const LOGO = {
	notes: [N.D4, N.A4, N.Fs4, N.E4],
	rhythm: [0, 0.22, 0.44, 0.9],
	tagRhythm: [0, 0.19, 0.38, 0.76],
	sub: N.D2
};

// D major pentatonic, high: UI ticks and sparkles stay in the logo's key
export const SPARKLE = [1174.66, 1318.51, 1479.98, 1760.0, 1975.53, 2349.32, 2637.02, 2959.96];

export class AudioEngine {
	constructor() {
		this.ctx = null;
		this.buffers = new Map();
		this.onsetMap = new Map();
		this.muted = false;
		this.buses = new Set();
	}

	// --- loading and analysis ---------------------------------------------------------

	async load(beds) {
		// an OfflineAudioContext decodes without a user gesture (and without a console warning)
		const off = new OfflineAudioContext(2, 1, 44100);
		await Promise.all(
			Object.entries(beds).map(async ([id, url]) => {
				const res = await fetch(url);
				const buf = await off.decodeAudioData(await res.arrayBuffer());
				this.buffers.set(id, buf);
				this.onsetMap.set(id, detectOnsets(buf));
			})
		);
	}

	/** Detected onsets of a bed: [{t, s}] with s normalized so 1 is a strong hit. */
	onsets(id) {
		return this.onsetMap.get(id) || [];
	}

	bedDuration(id) {
		return this.buffers.get(id)?.duration || 0;
	}

	// --- context ------------------------------------------------------------------------

	get running() {
		return !!this.ctx && this.ctx.state === 'running';
	}

	async unlock() {
		if (!this.ctx) this._build();
		if (this.ctx.state !== 'running') await this.ctx.resume();
	}

	_build(offline = null, { raw = false } = {}) {
		const ctx = (this.ctx = offline || new AudioContext({ latencyHint: 'interactive' }));
		// noise start offsets come from a seeded generator, so an offline render is repeatable
		this.rand = rng(4242);
		const g = (v) => {
			const n = ctx.createGain();
			n.gain.value = v;
			return n;
		};
		// gentle glue, then a fast limiter; both apply the spec's automatic makeup gain,
		// which the trim takes back so beds land near their -16 LUFS
		const glue = ctx.createDynamicsCompressor();
		glue.threshold.value = -16;
		glue.knee.value = 10;
		glue.ratio.value = 2.2;
		glue.attack.value = 0.02;
		glue.release.value = 0.28;
		const limit = ctx.createDynamicsCompressor();
		limit.threshold.value = -2.5;
		limit.knee.value = 0;
		limit.ratio.value = 20;
		limit.attack.value = 0.002;
		limit.release.value = 0.12;
		this.trim = g(0.72);
		this.master = g(this.muted ? 0 : 1);
		this.sum = g(1);
		// raw: dynamics bypassed, for measuring the headroom going into them
		if (raw) this.sum.connect(this.master).connect(ctx.destination);
		else this.sum.connect(glue).connect(limit).connect(this.trim).connect(this.master).connect(ctx.destination);
		this.musicBus = g(0.92);
		this.sfxBus = g(1);
		this.musicBus.connect(this.sum);
		this.sfxBus.connect(this.sum);
		const verb = ctx.createConvolver();
		verb.buffer = makeImpulse(ctx, 3.4, 0.82);
		this.verbSend = g(1);
		const verbReturn = g(0.55);
		// keep mud out of the tail
		const verbHp = ctx.createBiquadFilter();
		verbHp.type = 'highpass';
		verbHp.frequency.value = 180;
		this.verbSend.connect(verbHp).connect(verb).connect(verbReturn).connect(this.sum);
		this.white = makeNoise(ctx, 'white');
		this.pink = makeNoise(ctx, 'pink');
		this.projectorBuf = makeProjector(ctx);
	}

	setMuted(m) {
		this.muted = m;
		if (!this.ctx) return;
		const t = this.ctx.currentTime;
		this.master.gain.cancelScheduledValues(t);
		this.master.gain.setTargetAtTime(m ? 0 : 1, t, 0.04);
	}

	/** Seconds-from-now to AudioContext time and the matching document time (ms), aligned
	 *  to what is actually leaving the speakers. */
	clockPair(lead = 0.12) {
		const ctx = this.ctx;
		const ctxStart = ctx.currentTime + lead;
		const ts = ctx.getOutputTimestamp ? ctx.getOutputTimestamp() : null;
		let docStart;
		if (ts && ts.performanceTime > 0) docStart = ts.performanceTime + (ctxStart - ts.contextTime) * 1000;
		else docStart = performance.now() + (lead + (ctx.outputLatency || ctx.baseLatency || 0)) * 1000;
		return { ctxStart, docStart };
	}

	// --- per-piece bus ---------------------------------------------------------------------

	bus() {
		const ctx = this.ctx;
		const mk = (dest) => {
			const n = ctx.createGain();
			n.connect(dest);
			return n;
		};
		const b = {
			music: mk(this.musicBus),
			sfx: mk(this.sfxBus),
			verb: mk(this.verbSend),
			sources: new Set()
		};
		const gains = [b.music, b.sfx, b.verb];
		b.fade = (to, at, dur) => {
			for (const n of gains) {
				n.gain.cancelScheduledValues(at);
				n.gain.setValueAtTime(n.gain.value, at);
				n.gain.linearRampToValueAtTime(to, at + dur);
			}
		};
		b.stop = (fade = 0.12) => {
			const t = ctx.currentTime;
			for (const n of gains) {
				n.gain.cancelScheduledValues(t);
				n.gain.setValueAtTime(n.gain.value, t);
				n.gain.linearRampToValueAtTime(0, t + fade);
			}
			for (const s of b.sources) {
				try {
					s.stop(t + fade + 0.02);
				} catch {
					/* never started */
				}
			}
			setTimeout(() => gains.forEach((n) => n.disconnect()), (fade + 0.3) * 1000);
			this.buses.delete(b);
		};
		this.buses.add(b);
		return b;
	}

	stopAll() {
		for (const b of [...this.buses]) b.stop();
	}

	// --- building blocks --------------------------------------------------------------------

	_track(b, src) {
		b.sources.add(src);
		src.onended = () => b.sources.delete(src);
		return src;
	}
	_gain(v = 1) {
		const n = this.ctx.createGain();
		n.gain.value = v;
		return n;
	}
	_filter(type, freq, q = 0.707) {
		const f = this.ctx.createBiquadFilter();
		f.type = type;
		f.frequency.value = freq;
		f.Q.value = q;
		return f;
	}
	_pan(v = 0) {
		const p = this.ctx.createStereoPanner();
		p.pan.value = v;
		return p;
	}
	_osc(b, type, freq, when, dur) {
		const o = this.ctx.createOscillator();
		o.type = type;
		o.frequency.value = freq;
		o.start(when);
		o.stop(when + dur);
		return this._track(b, o);
	}
	_noise(b, kind, when, dur, rate = 1) {
		const s = this.ctx.createBufferSource();
		s.buffer = kind === 'pink' ? this.pink : this.white;
		s.loop = true;
		s.playbackRate.value = rate;
		s.start(when, this.rand() * 1.5);
		s.stop(when + dur);
		return this._track(b, s);
	}
	_send(b, node, amount) {
		if (amount <= 0) return;
		const s = this._gain(amount);
		node.connect(s).connect(b.verb);
	}
	/** attack/decay envelope on a fresh gain */
	_ad(when, peak, attack, decay) {
		const g = this._gain(0);
		g.gain.setValueAtTime(0, when);
		g.gain.linearRampToValueAtTime(peak, when + attack);
		g.gain.exponentialRampToValueAtTime(Math.max(peak * 0.0005, 1e-5), when + attack + decay);
		return g;
	}

	// --- beds ------------------------------------------------------------------------------

	/**
	 * Play a bed. All option times are relative to the bed cue. `ducks` dip it under
	 * stingers and the logo tag; `tapeStop` winds it down like a stopped deck.
	 */
	bed(b, when, id, skip = 0, o = {}) {
		const buf = this.buffers.get(id);
		if (!buf) return;
		const { from = 0, gain = 1, fadeIn = 0.25, fadeOut = 1.2, ducks = [], tapeStop = null } = o;
		const end = Math.min(o.end ?? buf.duration - from, buf.duration - from);
		if (skip >= end) return;
		const ctx = this.ctx;
		const src = ctx.createBufferSource();
		src.buffer = buf;
		const fade = this._gain(0);
		const duck = this._gain(1);
		src.connect(fade).connect(duck).connect(b.music);
		const fadePts = [
			[0, 0],
			[fadeIn, gain],
			[end - fadeOut, gain],
			[end, 0]
		];
		applyEnv(fade.gain, fadePts, when, skip);
		const duckPts = [[0, 1]];
		for (const d of [...ducks].sort((a, c) => a.at - c.at)) {
			const att = d.attack ?? 0.15;
			const rel = d.release ?? 0.7;
			duckPts.push([d.at - att, 1], [d.at, d.depth], [d.at + (d.hold ?? 1), d.depth], [d.at + (d.hold ?? 1) + rel, 1]);
		}
		applyEnv(duck.gain, duckPts, when, skip);
		if (tapeStop && tapeStop.at > skip) {
			const t0 = when + tapeStop.at - skip;
			src.playbackRate.setValueAtTime(1, t0);
			src.playbackRate.exponentialRampToValueAtTime(0.08, t0 + tapeStop.dur);
			fade.gain.cancelScheduledValues(t0);
			fade.gain.setValueAtTime(gain, t0);
			fade.gain.linearRampToValueAtTime(gain * 0.8, t0 + tapeStop.dur * 0.6);
			fade.gain.linearRampToValueAtTime(0, t0 + tapeStop.dur);
		}
		src.start(when, from + skip);
		src.stop(when + (end - skip) + 0.05);
		this._track(b, src);
	}

	// --- the SFX kit ---------------------------------------------------------------------------

	/** Filtered-noise sweep, panned with the motion it accompanies. */
	whoosh(b, when, o = {}) {
		const { dur = 0.8, pan = [-0.5, 0.5], gain = 0.2, lo = 380, hi = 2800, q = 1.1, verb = 0.2, peak = 0.6, body = 0.5 } = o;
		const out = this._pan(pan[0]);
		out.pan.setValueAtTime(pan[0], when);
		out.pan.linearRampToValueAtTime(pan[1], when + dur);
		const layer = (l, h, g, rate) => {
			const src = this._noise(b, 'pink', when, dur + 0.05, rate);
			const hp = this._filter('highpass', 90);
			const bp = this._filter('bandpass', l, q);
			bp.frequency.setValueAtTime(l, when);
			bp.frequency.exponentialRampToValueAtTime(h, when + dur * peak);
			bp.frequency.exponentialRampToValueAtTime(l * 1.3, when + dur);
			const env = this._gain(0);
			env.gain.setValueCurveAtTime(bell(peak, g), when, dur);
			src.connect(hp).connect(bp).connect(env).connect(out);
		};
		layer(lo, hi, gain, 1);
		if (body > 0) layer(lo * 0.35, hi * 0.3, gain * body, 0.8);
		out.connect(b.sfx);
		this._send(b, out, verb);
	}

	/** Rising noise plus a gliding, trembling tone; peaks exactly at when + dur. */
	riser(b, when, o = {}) {
		const { dur = 1.4, gain = 0.12, lo = 400, hi = 7500, tone = 0.5, verb = 0.35, root = 110 } = o;
		const end = when + dur;
		const src = this._noise(b, 'white', when, dur + 0.05);
		const hp = this._filter('highpass', lo, 0.7);
		hp.frequency.setValueAtTime(lo, when);
		hp.frequency.exponentialRampToValueAtTime(hi, end);
		const bp = this._filter('bandpass', lo * 2, 0.9);
		bp.frequency.setValueAtTime(lo * 2, when);
		bp.frequency.exponentialRampToValueAtTime(Math.min(hi * 1.4, 16000), end);
		const env = this._gain(0);
		env.gain.setValueAtTime(0.0001, when);
		env.gain.exponentialRampToValueAtTime(gain, end - 0.02);
		env.gain.linearRampToValueAtTime(0, end + 0.04);
		const pan = this._pan(0);
		src.connect(hp).connect(bp).connect(env).connect(pan);
		if (tone > 0) {
			const lp = this._filter('lowpass', 500, 1.5);
			lp.frequency.setValueAtTime(500, when);
			lp.frequency.exponentialRampToValueAtTime(5000, end);
			const tg = this._gain(0);
			tg.gain.setValueAtTime(0.0001, when);
			tg.gain.exponentialRampToValueAtTime(gain * tone * 0.5, end - 0.02);
			tg.gain.linearRampToValueAtTime(0, end + 0.03);
			const trem = this._gain(1);
			const lfo = this._osc(b, 'sine', 5, when, dur + 0.1);
			lfo.frequency.setValueAtTime(5, when);
			lfo.frequency.exponentialRampToValueAtTime(19, end);
			const depth = this._gain(0.45);
			lfo.connect(depth).connect(trem.gain);
			for (const det of [-9, 9]) {
				const o2 = this._osc(b, 'sawtooth', root, when, dur + 0.1);
				o2.detune.value = det;
				o2.frequency.setValueAtTime(root, when);
				o2.frequency.exponentialRampToValueAtTime(root * 4, end);
				o2.connect(lp);
			}
			lp.connect(tg).connect(trem).connect(pan);
		}
		pan.connect(b.sfx);
		this._send(b, pan, verb);
	}

	/** Sub boom: pitch-dropping sine through a soft clipper, a click and a low rumble tail. */
	impact(b, when, o = {}) {
		const { gain = 0.6, size = 1, tone = 48, verb = 0.25, click = 1, pan = 0 } = o;
		const out = this._pan(pan);
		const sub = this._osc(b, 'sine', tone * 2.8, when, 1.6 * size + 0.1);
		sub.frequency.setValueAtTime(tone * 2.8, when);
		sub.frequency.exponentialRampToValueAtTime(tone, when + 0.11);
		sub.frequency.exponentialRampToValueAtTime(tone * 0.78, when + 1.4 * size);
		const shaper = this.ctx.createWaveShaper();
		shaper.curve = softClip(2.2);
		const lp = this._filter('lowpass', 900);
		sub.connect(shaper).connect(lp).connect(this._ad(when, gain, 0.004, 1.3 * size)).connect(out);
		if (click > 0) {
			const n = this._noise(b, 'white', when, 0.05);
			const bp = this._filter('bandpass', 2600, 0.8);
			n.connect(bp).connect(this._ad(when, gain * 0.45 * click, 0.001, 0.035)).connect(out);
		}
		const rum = this._noise(b, 'pink', when, 1.8 * size);
		const rlp = this._filter('lowpass', 170, 0.9);
		rum.connect(rlp).connect(this._ad(when, gain * 0.9, 0.01, 1.5 * size)).connect(out);
		out.connect(b.sfx);
		this._send(b, out, verb);
	}

	/** Grit hit: a short punch with a snare-like crack (for hard cuts). */
	punch(b, when, o = {}) {
		const { gain = 0.5, pan = 0, verb = 0.12 } = o;
		// punches sit under the music: the soundtrack carries the energy, the punch the edge
		this.impact(b, when, { gain: gain * 0.58, size: 0.45, tone: 56, click: 1.2, verb: 0.08, pan });
		const out = this._pan(pan);
		const n = this._noise(b, 'white', when, 0.3);
		const bp = this._filter('bandpass', 1700, 0.7);
		const hp = this._filter('highpass', 600);
		n.connect(hp).connect(bp).connect(this._ad(when, gain * 0.36, 0.002, 0.16)).connect(out);
		out.connect(b.sfx);
		this._send(b, out, verb);
	}

	/** Soft UI tick: a tiny pitched "tock" plus a filtered click. */
	tick(b, when, o = {}) {
		const { freq = 2200, gain = 0.06, pan = 0, decay = 0.06, type = 'sine', verb = 0.1 } = o;
		const out = this._pan(pan);
		const osc = this._osc(b, type, freq * 1.5, when, decay + 0.05);
		osc.frequency.setValueAtTime(freq * 1.5, when);
		osc.frequency.exponentialRampToValueAtTime(freq, when + 0.012);
		osc.connect(this._ad(when, gain, 0.0015, decay)).connect(out);
		const n = this._noise(b, 'white', when, 0.02);
		const hp = this._filter('highpass', 4500);
		n.connect(hp).connect(this._ad(when, gain * 0.35, 0.0008, 0.008)).connect(out);
		out.connect(b.sfx);
		this._send(b, out, verb);
	}

	/** Glow sweep: a cascade of high pentatonic pings over a bright air swell. */
	shimmer(b, when, o = {}) {
		const { dur = 1.1, gain = 0.05, count = 14, verb = 0.7, seed = 7, notes = SPARKLE } = o;
		const r = rng(seed);
		const out = this._gain(1);
		for (let i = 0; i < count; i++) {
			const t = when + dur * Math.pow(i / count, 0.85) + r() * 0.03;
			const f = notes[Math.floor(r() * notes.length)] * (r() < 0.3 ? 2 : 1);
			const p = this._pan(r() * 1.4 - 0.7);
			const o2 = this._osc(b, 'sine', f, t, 1);
			o2.connect(this._ad(t, gain * (0.5 + r() * 0.5), 0.003, 0.5 + r() * 0.5)).connect(p).connect(out);
		}
		const air = this._noise(b, 'white', when, dur + 0.1);
		const bp = this._filter('bandpass', 6000, 1.2);
		bp.frequency.setValueAtTime(5000, when);
		bp.frequency.exponentialRampToValueAtTime(11000, when + dur);
		const env = this._gain(0);
		env.gain.setValueCurveAtTime(bell(0.45, gain * 0.9), when, dur);
		air.connect(bp).connect(env).connect(out);
		out.connect(b.sfx);
		this._send(b, out, verb);
	}

	/** One FM tine/bell voice (carrier:modulator 1:1 plus an inharmonic 3.5 glint). */
	bell(b, when, freq, o = {}) {
		const { gain = 0.2, decay = 2.4, index = 2, pan = 0, verb = 0.45 } = o;
		const car = this._osc(b, 'sine', freq, when, decay + 0.2);
		const mod = this._osc(b, 'sine', freq, when, decay + 0.2);
		const mg = this._gain(0);
		mg.gain.setValueAtTime(freq * index, when);
		mg.gain.exponentialRampToValueAtTime(freq * 0.12, when + Math.min(1.2, decay * 0.5));
		mod.connect(mg).connect(car.frequency);
		const mod2 = this._osc(b, 'sine', freq * 3.51, when, 0.4);
		const mg2 = this._gain(0);
		mg2.gain.setValueAtTime(freq * 0.9, when);
		mg2.gain.exponentialRampToValueAtTime(freq * 0.005, when + 0.2);
		mod2.connect(mg2).connect(car.frequency);
		const p = this._pan(pan);
		car.connect(this._ad(when, gain, 0.003, decay)).connect(p);
		// the hammer: a short octave-and-a-fifth partial
		const ham = this._osc(b, 'sine', freq * 3, when, 0.2);
		ham.connect(this._ad(when, gain * 0.12, 0.001, 0.09)).connect(p);
		p.connect(b.sfx);
		this._send(b, p, verb);
	}

	/** Warm pad chord: detuned saws through a closing low-pass. */
	pad(b, when, freqs, o = {}) {
		const { gain = 0.05, attack = 0.08, hold = 0.4, release = 2.4, cutoff = [2400, 650], verb = 0.5, pan = 0.35 } = o;
		const total = attack + hold + release;
		const lp = this._filter('lowpass', cutoff[0], 0.6);
		lp.frequency.setValueAtTime(cutoff[0], when);
		lp.frequency.exponentialRampToValueAtTime(cutoff[1], when + total);
		const env = this._gain(0);
		env.gain.setValueAtTime(0, when);
		env.gain.linearRampToValueAtTime(gain, when + attack);
		env.gain.setValueAtTime(gain, when + attack + hold);
		env.gain.exponentialRampToValueAtTime(gain * 0.001, when + total);
		freqs.forEach((f, i) => {
			const p = this._pan(((i % 2) * 2 - 1) * pan * (i / freqs.length));
			for (const det of [-7, 6]) {
				const o2 = this._osc(b, i === 0 ? 'triangle' : 'sawtooth', f, when, total + 0.1);
				o2.detune.value = det;
				o2.connect(p);
			}
			p.connect(lp);
		});
		lp.connect(env).connect(b.sfx);
		this._send(b, env, verb);
	}

	sub(b, when, freq, o = {}) {
		const { gain = 0.3, decay = 1.6 } = o;
		const s = this._osc(b, 'sine', freq, when, decay + 0.2);
		s.connect(this._ad(when, gain, 0.01, decay)).connect(b.sfx);
	}

	// --- the Couch TV theme: an orchestral-hybrid kit -----------------------------------

	/** Low brass "braam": detuned saw stacks scooping into pitch, through an opening low-pass
	 *  and a soft clipper (which also bounds the level however many notes are stacked). */
	braam(b, when, notes, o = {}) {
		const { gain = 0.4, dur = 2.6, open = [180, 2400, 650], attack = 0.03, verb = 0.35, drive = 1.6, pan = 0.25 } = o;
		const lp = this._filter('lowpass', open[0], 1.1);
		lp.frequency.setValueAtTime(open[0], when);
		lp.frequency.exponentialRampToValueAtTime(open[1], when + attack + 0.11);
		lp.frequency.exponentialRampToValueAtTime(open[2], when + dur);
		const shaper = this.ctx.createWaveShaper();
		shaper.curve = softClip(drive);
		const env = this._gain(0);
		env.gain.setValueAtTime(0, when);
		env.gain.linearRampToValueAtTime(gain, when + attack);
		env.gain.exponentialRampToValueAtTime(gain * 0.55, when + attack + 0.6);
		env.gain.exponentialRampToValueAtTime(gain * 0.0008, when + dur);
		const voices = notes.length * 4;
		const pre = this._gain(1.9 / Math.sqrt(voices));
		notes.forEach((f, i) => {
			const p = this._pan(((i % 2) * 2 - 1) * pan * (i / notes.length));
			[-14, -5, 6, 15].forEach((det) => {
				const o2 = this._osc(b, 'sawtooth', f, when, dur + 0.1);
				o2.detune.setValueAtTime(det - 45, when);
				o2.detune.linearRampToValueAtTime(det, when + 0.09);
				o2.connect(p);
			});
			p.connect(pre);
		});
		pre.connect(lp).connect(shaper).connect(env).connect(b.sfx);
		this._send(b, env, verb);
	}

	/** Taiko-style drum: a pitch-dropping sine body, a skin thump and a noise transient. */
	taiko(b, when, o = {}) {
		const { gain = 0.5, pitch = 1, pan = 0, size = 1, verb = 0.35 } = o;
		const out = this._pan(pan);
		const body = this._osc(b, 'sine', 150 * pitch, when, 1.2 * size + 0.1);
		body.frequency.setValueAtTime(150 * pitch, when);
		body.frequency.exponentialRampToValueAtTime(50 * pitch, when + 0.13);
		body.connect(this._ad(when, gain, 0.002, 0.95 * size)).connect(out);
		const skin = this._osc(b, 'triangle', 240 * pitch, when, 0.25);
		skin.frequency.setValueAtTime(240 * pitch, when);
		skin.frequency.exponentialRampToValueAtTime(120 * pitch, when + 0.05);
		skin.connect(this._ad(when, gain * 0.28, 0.001, 0.14)).connect(out);
		const n = this._noise(b, 'white', when, 0.06);
		const bp = this._filter('bandpass', 1100, 0.8);
		n.connect(bp).connect(this._ad(when, gain * 0.24, 0.001, 0.03)).connect(out);
		out.connect(b.sfx);
		this._send(b, out, verb);
	}


	/** Sub drop: a long pitch fall under the hit, lightly saturated so it reads on small speakers. */
	subDrop(b, when, o = {}) {
		const { gain = 0.5, from = 110, to = 29, dur = 1.8 } = o;
		const s = this._osc(b, 'sine', from, when, dur + 0.2);
		s.frequency.setValueAtTime(from, when);
		s.frequency.exponentialRampToValueAtTime(to, when + dur * 0.8);
		const shaper = this.ctx.createWaveShaper();
		shaper.curve = softClip(1.8);
		const env = this._gain(0);
		env.gain.setValueAtTime(0, when);
		env.gain.linearRampToValueAtTime(gain, when + 0.012);
		env.gain.setValueAtTime(gain, when + 0.25);
		env.gain.exponentialRampToValueAtTime(gain * 0.001, when + dur);
		s.connect(shaper).connect(env).connect(b.sfx);
	}

	/** A clean sine sub swelling into a hit; ends a breath before it, so the hit lands on silence. */
	subSwell(b, when, dur, skip = 0, o = {}) {
		const { gain = 0.3 } = o;
		const left = dur - skip;
		if (left <= 0.05) return;
		const shape = (x) => 0.04 + 0.96 * Math.pow(x, 2.2);
		const env = this._gain(0);
		env.gain.setValueAtTime(gain * shape(skip / dur), when);
		const steps = 24;
		for (let i = 1; i <= steps; i++) {
			const x = skip / dur + (i / steps) * (left / dur);
			env.gain.linearRampToValueAtTime(gain * shape(Math.min(1, x)), when + (i / steps) * left);
		}
		env.gain.linearRampToValueAtTime(0, when + left + 0.03);
		for (const [f, g] of [[N.D1, 1], [N.D2, 0.55]]) {
			const o2 = this._osc(b, 'sine', f, when, left + 0.1);
			o2.connect(this._gain(g)).connect(env);
		}
		env.connect(b.sfx);
	}

	/** Shepard-style riser: octave-spaced voices gliding up under a bell-shaped spectral
	 *  window, so the pitch keeps climbing without arriving. */
	shepard(b, when, dur, o = {}) {
		const { gain = 0.07, base = N.D2, voices = 5, octaves = 1.2 } = o;
		const out = this._gain(1);
		const lp = this._filter('lowpass', 2600, 0.5);
		for (let i = 0; i < voices; i++) {
			const osc = this._osc(b, 'sawtooth', base * 2 ** i, when, dur + 0.05);
			osc.frequency.setValueAtTime(base * 2 ** i, when);
			osc.frequency.exponentialRampToValueAtTime(base * 2 ** (i + octaves), when + dur);
			const curve = new Float32Array(64);
			for (let k = 0; k < 64; k++) {
				const tau = k / 63;
				const pos = (i + octaves * tau) / voices;
				const window = Math.exp(-((pos - 0.55) ** 2) / (2 * 0.2 ** 2));
				curve[k] = gain * window * (0.12 + 0.88 * tau ** 1.6) * (k === 63 ? 0 : 1);
			}
			const g = this._gain(0);
			g.gain.setValueCurveAtTime(curve, when, dur);
			osc.connect(g).connect(this._pan(i % 2 ? 0.3 : -0.3)).connect(lp);
		}
		lp.connect(out).connect(b.sfx);
		this._send(b, out, 0.4);
	}



	/** Spiccato string pulse: short detuned saws through one low-pass that opens across the phrase. */
	ostinato(b, steps, o = {}) {
		const { cutoff = [700, 3200], verb = 0.25 } = o;
		if (!steps.length) return;
		const t0 = steps[0].when;
		const t1 = steps[steps.length - 1].when + 0.2;
		const lp = this._filter('lowpass', cutoff[0], 1.4);
		lp.frequency.setValueAtTime(cutoff[0], t0);
		lp.frequency.exponentialRampToValueAtTime(cutoff[1], t1);
		const out = this._gain(1);
		for (const s of steps) {
			const env = this._ad(s.when, s.gain, 0.004, 0.13);
			for (const det of [-6, 6]) {
				const o2 = this._osc(b, 'sawtooth', s.f, s.when, 0.2);
				o2.detune.value = det;
				o2.connect(env);
			}
			env.connect(this._pan(s.pan || 0)).connect(lp);
		}
		lp.connect(out).connect(b.sfx);
		this._send(b, out, verb);
	}


	/** A brass note for the mnemonic: saws with an octave-down layer, a bright attack that
	 *  settles, and vibrato that arrives late, as a player would. */
	brass(b, when, f, dur, o = {}) {
		const { gain = 0.2, pan = 0, bright = 1, verb = 0.45, release = 0.3 } = o;
		const lp = this._filter('lowpass', 300, 1.0);
		lp.frequency.setValueAtTime(300, when);
		lp.frequency.exponentialRampToValueAtTime(900 + 2000 * bright, when + 0.07);
		lp.frequency.exponentialRampToValueAtTime(700 + 900 * bright, when + 0.45);
		const lfo = this._osc(b, 'sine', 5.4, when, dur + release + 0.1);
		const depth = this._gain(0);
		depth.gain.setValueAtTime(0, when + Math.min(0.2, dur * 0.5));
		depth.gain.linearRampToValueAtTime(7, when + Math.min(0.6, dur));
		lfo.connect(depth);
		const pre = this._gain(0.45);
		for (const [ratio, det, lvl] of [[1, -7, 1], [1, 0, 1], [1, 7, 1], [0.5, 0, 0.6]]) {
			const o2 = this._osc(b, 'sawtooth', f * ratio, when, dur + release + 0.1);
			o2.detune.setValueAtTime(det - 30, when);
			o2.detune.linearRampToValueAtTime(det, when + 0.05);
			depth.connect(o2.detune);
			o2.connect(this._gain(lvl)).connect(pre);
		}
		const shaper = this.ctx.createWaveShaper();
		shaper.curve = softClip(1.3);
		const env = this._gain(0);
		env.gain.setValueAtTime(0, when);
		env.gain.linearRampToValueAtTime(gain, when + 0.035);
		env.gain.exponentialRampToValueAtTime(gain * 0.72, when + 0.3);
		env.gain.setValueAtTime(gain * 0.72, when + dur);
		env.gain.exponentialRampToValueAtTime(gain * 0.001, when + dur + release);
		const p = this._pan(pan);
		pre.connect(lp).connect(shaper).connect(env).connect(p).connect(b.sfx);
		this._send(b, p, verb);
	}

	/**
	 * The Couch TV theme as timeline events ({at, dur, fn(when, bus, skip)}), derived from
	 * the ident's cue table so picture and score cannot drift apart. Both cuts use it:
	 * the full cut builds (sub swell, two brass statements, the pulse) before the hit; the short
	 * cut keeps the riser, the roll, the hit and the mnemonic.
	 */
	themeScore(c) {
		const ev = [];
		const at = (t, fn, dur = 0) => t != null && ev.push({ at: t, fn, dur });
		const H = c.hit;
		const F = c.final;
		const D = c.D;
		const tail = Math.max(1.6, D - F - 0.4);

		// tension: a clean sine sub under the build
		at(c.swell, (w, b, skip) => this.subSwell(b, w, H - 0.06 - c.swell, skip, { gain: 0.24 }), H - c.swell);
		// two low brass statements, a fifth and octave (no thirds): D, then the B-flat lift
		at(c.b1, (w, b) => {
			this.braam(b, w, [N.D2, N.A2, N.D3], { gain: 0.3, dur: 2.4, drive: 1.2, open: [180, 2000, 600] });
			this.sub(b, w, N.D1, { gain: 0.22, decay: 1.8 });
			this.taiko(b, w, { gain: 0.3, size: 1.3 });
		});
		at(c.b2, (w, b) => {
			this.braam(b, w, [N.Bb1, N.F2, N.Bb2], { gain: 0.32, dur: 2.2, drive: 1.2, open: [200, 2400, 650] });
			this.sub(b, w, N.Bb1, { gain: 0.18, decay: 1.6 });
			this.taiko(b, w, { gain: 0.3, size: 1.2 });
		});
		// the pulse: 16ths at 120 bpm, D, the B-flat bar, then climbing octaves into the hit
		if (c.ost) {
			at(c.ost[0], (w, b) => {
				const steps = [];
				const n = Math.round((c.ost[1] - c.ost[0]) / 0.125);
				const pat = {
					d: [N.D3, N.D3, N.A3, N.D3, N.F3, N.D3, N.A3, N.D3],
					bb: [N.Bb2, N.Bb2, N.F3, N.Bb2, N.D3, N.Bb2, N.F3, N.Bb2],
					up: [N.D3, N.A3, N.D4, N.A3, N.D3, N.A3, N.D4, N.F4]
				};
				for (let i = 0; i < n; i++) {
					const t = c.ost[0] + i * 0.125;
					// the last bar before the hit drops out: the pulse stops, the riser carries it
					if (t >= H - 0.5) break;
					const key = c.b2 != null && t >= c.b2 && t < c.b2 + 1.0 ? 'bb' : t >= H - 1.0 ? 'up' : 'd';
					const p = i / n;
					steps.push({ when: w + i * 0.125, f: pat[key][i % 8], gain: (0.045 + 0.08 * p) * (i % 4 === 0 ? 1.25 : 1), pan: i % 2 ? 0.2 : -0.2 });
				}
				this.ostinato(b, steps, { cutoff: [600, 3600], verb: 0.18 });
			});
		}
		// drums leading in, accelerating into the hit
		for (const [t, g] of c.drums) at(t, (w, b) => this.taiko(b, w, { gain: g * 0.62, pitch: 1 + (t % 0.5) * 0.3, pan: ((t * 7) % 1) - 0.5, size: 0.8, verb: 0.25 }));
		at(c.riser, (w, b) => this.shepard(b, w, H - 0.06 - c.riser, { gain: 0.055 }));

		// the hit: D major, one big drum, a clean sub drop; then a single warm pad holds the chord
		at(H, (w, b) => {
			this.braam(b, w, [N.D2, N.A2, N.D3, N.Fs3], { gain: 0.28, dur: 2.4, open: [220, 2800, 700], drive: 1.4 });
			this.taiko(b, w, { gain: 0.42, size: 1.6, pitch: 0.9, verb: 0.3 });
			this.subDrop(b, w, { gain: 0.26, from: 90, to: 31, dur: 1.8 });
		});
		at(H + 0.05, (w, b) => this.pad(b, w, [N.D3, N.A3, N.D4, N.Fs4], { gain: 0.03, attack: 0.4, hold: Math.max(0.3, F - H - 0.5), release: 0.6, cutoff: [1800, 900], verb: 0.5 }));

		// the mnemonic, stated plainly: brass with an octave-up bell
		const R = LOGO.rhythm;
		const M = c.mn;
		[0, 1, 2].forEach((i) => {
			at(M + R[i], (w, b) => {
				this.brass(b, w, LOGO.notes[i], R[i + 1] - R[i] - 0.02, { gain: 0.17, pan: [-0.12, 0.12, -0.04][i], release: 0.16, verb: 0.35 });
				this.bell(b, w, LOGO.notes[i] * 2, { gain: 0.1, decay: 1.3, index: 1.5, pan: [-0.15, 0.15, 0][i], verb: 0.45 });
			});
		});
		// the final chord: E over D (add9), sub, one drum, a long clean tail
		at(F, (w, b) => {
			this.brass(b, w, N.E4, tail * 0.5, { gain: 0.18, release: tail * 0.45, bright: 1.1, verb: 0.5 });
			this.brass(b, w, N.E3, tail * 0.45, { gain: 0.08, release: tail * 0.4, bright: 0.6, pan: -0.2, verb: 0.5 });
			this.bell(b, w, N.E5, { gain: 0.12, decay: Math.min(4, tail), index: 1.3, verb: 0.65 });
			this.pad(b, w, [N.D3, N.A3, N.E4, N.Fs4], { gain: 0.034, attack: 0.25, hold: tail * 0.25, release: tail * 0.7, cutoff: [2000, 700], verb: 0.6 });
			this.sub(b, w, N.D1, { gain: 0.22, decay: tail * 0.7 });
			this.taiko(b, w, { gain: 0.24, size: 1.4, pitch: 0.95 });
		});
		return ev;
	}

	/** The promo tag: the same theme condensed (mnemonic on brass and bell, a small final chord). */
	themeTag(b, when, o = {}) {
		const { gain = 0.5 } = o;
		const g = gain / 0.5;
		const R = LOGO.tagRhythm;
		[0, 1, 2].forEach((i) => {
			this.brass(b, when + R[i], LOGO.notes[i], R[i + 1] - R[i] - 0.02, { gain: 0.13 * g, pan: [-0.12, 0.12, -0.04][i], release: 0.14, bright: 0.8, verb: 0.35 });
			this.bell(b, when + R[i], LOGO.notes[i] * 2, { gain: 0.08 * g, decay: 1.1, index: 1.5, verb: 0.45 });
		});
		const t = when + R[3];
		this.brass(b, t, N.E4, 0.9, { gain: 0.14 * g, release: 1.3, bright: 1, verb: 0.5 });
		this.bell(b, t, N.E5, { gain: 0.1 * g, decay: 2.2, index: 1.3, verb: 0.6 });
		this.pad(b, t, [N.D3, N.A3, N.E4, N.Fs4], { gain: 0.026 * g, attack: 0.15, hold: 0.4, release: 1.8, cutoff: [1800, 700], verb: 0.55 });
		this.sub(b, t, N.D1, { gain: 0.22 * g, decay: 1.5 });
		this.taiko(b, t, { gain: 0.24 * g, size: 1.2 });
	}


	/** Typewriter key: a bright clack and a low thunk. */
	key(b, when, o = {}) {
		const { gain = 0.08, pan = 0, seed = 1 } = o;
		const r = rng(seed);
		const out = this._pan(pan);
		const n = this._noise(b, 'white', when, 0.03);
		const bp = this._filter('bandpass', 2400 + r() * 1800, 2.2);
		n.connect(bp).connect(this._ad(when, gain, 0.0008, 0.018)).connect(out);
		const th = this._osc(b, 'sine', 150 + r() * 40, when, 0.06);
		th.connect(this._ad(when, gain * 0.6, 0.001, 0.035)).connect(out);
		out.connect(b.sfx);
		this._send(b, out, 0.06);
	}

	/** 24 fps projector rattle (looped procedural buffer). */
	projector(b, when, dur, skip = 0, o = {}) {
		const { gain = 0.05, fadeOut = 0.3 } = o;
		const left = dur - skip;
		if (left <= 0) return;
		const s = this.ctx.createBufferSource();
		s.buffer = this.projectorBuf;
		s.loop = true;
		const bp = this._filter('bandpass', 1500, 0.5);
		const env = this._gain(0);
		env.gain.setValueAtTime(skip > 0 ? gain : 0, when);
		env.gain.linearRampToValueAtTime(gain, when + 0.08);
		env.gain.setValueAtTime(gain, when + Math.max(0.1, left - fadeOut));
		env.gain.linearRampToValueAtTime(0, when + left);
		s.connect(bp).connect(env).connect(b.sfx);
		s.start(when);
		s.stop(when + left + 0.05);
		this._track(b, s);
	}

	/** Synthesized ambient bed (the lineup): two slow chords, sub and drifting air. */
	ambient(b, when, dur, skip = 0, o = {}) {
		const { gain = 0.05, chords = [[146.83, 220.0, 277.18, 329.63, 369.99], [123.47, 185.0, 220.0, 277.18, 293.66]] } = o;
		const left = dur - skip;
		if (left <= 0.2) return;
		const fadeIn = skip > 0 ? 0.3 : 1.6;
		const master = this._gain(0);
		master.gain.setValueAtTime(0, when);
		master.gain.linearRampToValueAtTime(gain, when + fadeIn);
		master.gain.setValueAtTime(gain, when + Math.max(fadeIn, left - 1.8));
		master.gain.linearRampToValueAtTime(0, when + left);
		const lp = this._filter('lowpass', 900, 0.5);
		const lfo = this._osc(b, 'sine', 0.13, when, left + 0.1);
		const lfoAmt = this._gain(380);
		lfo.connect(lfoAmt).connect(lp.frequency);
		lp.connect(master);
		const half = dur / 2;
		chords.forEach((chord, ci) => {
			const c0 = ci * half;
			const c1 = c0 + half;
			if (c1 <= skip) return;
			const start = when + Math.max(0, c0 - skip);
			const end = when + (c1 - skip) + 1.2;
			const cg = this._gain(0);
			cg.gain.setValueAtTime(0, start);
			cg.gain.linearRampToValueAtTime(1, start + (ci === 0 ? 0.05 : 1.6));
			cg.gain.setValueAtTime(1, Math.max(start + 1.7, end - 1.6));
			cg.gain.linearRampToValueAtTime(0, end);
			chord.forEach((f, i) => {
				const p = this._pan((i / (chord.length - 1)) * 1.2 - 0.6);
				for (const det of [-8, 7]) {
					const osc = this._osc(b, i === 0 ? 'triangle' : 'sawtooth', f, start, end - start + 0.1);
					osc.detune.value = det + i;
					osc.connect(p);
				}
				p.connect(cg);
			});
			cg.connect(lp);
		});
		const s = this._osc(b, 'sine', 73.42, when, left + 0.1);
		const sg = this._gain(0.55);
		s.connect(sg).connect(master);
		const air = this._noise(b, 'pink', when, left + 0.1);
		const ahp = this._filter('bandpass', 3800, 0.4);
		const ag = this._gain(0.08);
		air.connect(ahp).connect(ag).connect(master);
		master.connect(b.sfx);
		this._send(b, master, 0.5);
	}
}

/**
 * Debug: render a timeline's sound cues offline and measure them. `solo` keeps only the
 * music or the sfx bus (reverb returns follow sfx), so the mix balance can be checked.
 */
export async function renderMix(source, makeTimeline, { solo = null, raw = false, sampleRate = 44100 } = {}) {
	const probe = new AudioEngine();
	probe.buffers = source.buffers;
	probe.onsetMap = source.onsetMap;
	const tl = makeTimeline(probe);
	const len = Math.ceil((tl.duration + 1.5) * sampleRate);
	const ctx = new OfflineAudioContext(2, len, sampleRate);
	probe._build(ctx, { raw });
	if (solo === 'music') probe.sfxBus.gain.value = 0;
	if (solo === 'sfx') probe.musicBus.gain.value = 0;
	const bus = probe.bus();
	tl.scheduleSounds(0.0, 0, bus);
	const buf = await ctx.startRendering();
	return { tl, stats: measure(buf), buffer: buf };
}

function measure(buf) {
	const a = buf.getChannelData(0);
	const b = buf.getChannelData(1);
	let peak = 0;
	let sum = 0;
	const win = Math.floor(buf.sampleRate * 0.4);
	const hop = Math.floor(buf.sampleRate * 0.1);
	const st = [];
	for (let i = 0; i < a.length; i++) {
		peak = Math.max(peak, Math.abs(a[i]), Math.abs(b[i]));
		sum += a[i] * a[i] + b[i] * b[i];
	}
	for (let i = 0; i + win < a.length; i += hop) {
		let s = 0;
		for (let j = i; j < i + win; j++) s += a[j] * a[j] + b[j] * b[j];
		st.push({ t: +(i / buf.sampleRate).toFixed(1), db: +(10 * Math.log10(s / (2 * win) + 1e-12)).toFixed(1) });
	}
	const db = (x) => +(20 * Math.log10(x + 1e-12)).toFixed(1);
	const loud = st.filter((x) => x.db > -50);
	return {
		peakDb: db(peak),
		rmsDb: +(10 * Math.log10(sum / (2 * a.length) + 1e-12)).toFixed(1),
		maxShortTermDb: loud.length ? Math.max(...loud.map((x) => x.db)) : null,
		shortTerm: st.filter((_, i) => i % 5 === 0)
	};
}

// --- helpers ---------------------------------------------------------------------------------

/** Piecewise-linear envelope applied from `skip` seconds in (playback may start mid-way). */
function applyEnv(param, pts, when, skip) {
	pts = pts.filter((p) => isFinite(p[0])).sort((a, b) => a[0] - b[0]);
	const valueAt = (t) => {
		if (t <= pts[0][0]) return pts[0][1];
		for (let i = 1; i < pts.length; i++) {
			const [t1, v1] = pts[i];
			const [t0, v0] = pts[i - 1];
			if (t <= t1) return t1 === t0 ? v1 : v0 + ((v1 - v0) * (t - t0)) / (t1 - t0);
		}
		return pts[pts.length - 1][1];
	};
	param.cancelScheduledValues(when);
	param.setValueAtTime(valueAt(skip), when);
	for (const [t, v] of pts) if (t > skip) param.linearRampToValueAtTime(v, when + (t - skip));
}

function bell(peak, amp, n = 64) {
	const c = new Float32Array(n);
	for (let i = 0; i < n; i++) {
		const x = i / (n - 1);
		const y = x < peak ? Math.pow(x / peak, 2) : Math.pow((1 - x) / (1 - peak), 1.6);
		c[i] = y * amp;
	}
	c[n - 1] = 0;
	return c;
}

function softClip(drive) {
	const n = 1024;
	const c = new Float32Array(n);
	for (let i = 0; i < n; i++) {
		const x = (i / (n - 1)) * 2 - 1;
		c[i] = Math.tanh(x * drive) / Math.tanh(drive);
	}
	return c;
}

function makeNoise(ctx, kind) {
	const len = ctx.sampleRate * 2;
	const buf = ctx.createBuffer(2, len, ctx.sampleRate);
	const r = rng(kind === 'pink' ? 11 : 5);
	for (let ch = 0; ch < 2; ch++) {
		const d = buf.getChannelData(ch);
		let b0 = 0, b1 = 0, b2 = 0, b3 = 0, b4 = 0, b5 = 0, b6 = 0;
		for (let i = 0; i < len; i++) {
			const w = r() * 2 - 1;
			if (kind === 'pink') {
				// Paul Kellet's refined pink filter
				b0 = 0.99886 * b0 + w * 0.0555179;
				b1 = 0.99332 * b1 + w * 0.0750759;
				b2 = 0.969 * b2 + w * 0.153852;
				b3 = 0.8665 * b3 + w * 0.3104856;
				b4 = 0.55 * b4 + w * 0.5329522;
				b5 = -0.7616 * b5 - w * 0.016898;
				d[i] = (b0 + b1 + b2 + b3 + b4 + b5 + b6 + w * 0.5362) * 0.11;
				b6 = w * 0.115926;
			} else d[i] = w * 0.5;
		}
	}
	return buf;
}

/** A generated room: decorrelated stereo noise, exponential decay, darkening over time. */
function makeImpulse(ctx, seconds, decay) {
	const sr = ctx.sampleRate;
	const len = Math.floor(sr * seconds);
	const pre = Math.floor(sr * 0.018);
	const buf = ctx.createBuffer(2, len, sr);
	const r = rng(23);
	for (let ch = 0; ch < 2; ch++) {
		const d = buf.getChannelData(ch);
		let lp = 0;
		for (let i = pre; i < len; i++) {
			const t = (i - pre) / sr;
			const k = 0.92 - 0.82 * Math.min(1, t / seconds); // HF damping grows with time
			lp += k * (r() * 2 - 1 - lp);
			d[i] = lp * Math.exp(-t / decay) * 0.6;
		}
		// a few early reflections
		for (const [ms, g] of [[11, 0.5], [19, 0.35], [31, 0.28], [43, 0.2]]) {
			const i = pre + Math.floor((ms + ch * 3) * sr * 0.001);
			if (i < len) d[i] += g * (ch ? -1 : 1);
		}
	}
	return buf;
}

function makeProjector(ctx) {
	const sr = ctx.sampleRate;
	const len = sr; // one second loops cleanly at 24 frames
	const buf = ctx.createBuffer(2, len, sr);
	const r = rng(41);
	for (let ch = 0; ch < 2; ch++) {
		const d = buf.getChannelData(ch);
		for (let i = 0; i < len; i++) d[i] = (r() * 2 - 1) * 0.06;
		for (let f = 0; f < 24; f++) {
			const at = Math.floor((f / 24) * len + r() * 60);
			const amp = 0.5 + r() * 0.5;
			for (let j = 0; j < sr * 0.006; j++) {
				const i = at + j;
				if (i < len) d[i] += (r() * 2 - 1) * amp * Math.exp(-j / (sr * 0.0012));
			}
		}
	}
	return buf;
}

/**
 * Offline onset detection: high-frequency-weighted log energy, its positive flux against a
 * short history, an adaptive threshold and peak picking. Strengths are normalized to the
 * 90th percentile so templates can ask for "strong" hits portably.
 */
export function detectOnsets(buf) {
	const sr = buf.sampleRate;
	const a = buf.getChannelData(0);
	const bch = buf.numberOfChannels > 1 ? buf.getChannelData(1) : a;
	const hop = 512;
	const win = 1024;
	const n = Math.max(0, Math.floor((a.length - win) / hop));
	const e = new Float32Array(n);
	for (let i = 0; i < n; i++) {
		let s = 0;
		let prev = 0;
		const o = i * hop;
		for (let j = 0; j < win; j++) {
			const x = (a[o + j] + bch[o + j]) * 0.5;
			const d = x - prev;
			prev = x;
			s += d * d * 0.75 + x * x * 0.25;
		}
		e[i] = Math.log(1e-8 + s / win);
	}
	const flux = new Float32Array(n);
	for (let i = 3; i < n; i++) flux[i] = Math.max(0, e[i] - Math.max(e[i - 1], (e[i - 2] + e[i - 3]) / 2));
	const out = [];
	const W = 16;
	for (let i = 1; i < n - 1; i++) {
		let m = 0;
		let c = 0;
		for (let j = Math.max(0, i - W); j < Math.min(n, i + W); j++) {
			m += flux[j];
			c++;
		}
		m /= c;
		let isMax = true;
		for (let j = Math.max(0, i - 5); j <= Math.min(n - 1, i + 5); j++) if (flux[j] > flux[i]) isMax = false;
		if (isMax && flux[i] > m * 1.6 + 0.08) out.push({ t: (i * hop) / sr, s: flux[i] });
	}
	const sorted = out.map((o) => o.s).sort((x, y) => x - y);
	const p90 = sorted[Math.floor(sorted.length * 0.9)] || 1;
	return out.map((o) => ({ t: +o.t.toFixed(3), s: +Math.min(1.5, o.s / p90).toFixed(3) }));
}
