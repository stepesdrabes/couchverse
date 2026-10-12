// Instruments for the Couch TV themes, mixed into AudioEngine's prototype. Every voice takes
// an absolute AudioContext time and a seed, never Math.random or the scheduling order, so a
// render is a pure function of the cue table (offline or live, from any start point).
//
// Conventions: `b` is the piece bus (for stopping sources), `o.dest` the node to feed (the
// theme bus by default, its pump input for sidechained parts). Gains are linear; a voice at
// gain 1 peaks near full scale on its own.

import { rng } from './core.js';

const PC = { C: 0, 'C#': 1, Db: 1, D: 2, 'D#': 3, Eb: 3, E: 4, F: 5, 'F#': 6, Gb: 6, G: 7, 'G#': 8, Ab: 8, A: 9, 'A#': 10, Bb: 10, B: 11 };

/** Note name (A4, F#3, Bb1) to Hz, equal temperament. */
export function hz(name) {
	const m = /^([A-G][b#]?)(-?\d)$/.exec(name);
	if (!m) throw new Error('bad note ' + name);
	return 440 * 2 ** ((PC[m[1]] + 12 * (+m[2] + 1) - 69) / 12);
}
export const pitchClass = (name) => PC[/^([A-G][b#]?)/.exec(name)[1]];

/** A stable seed from numbers (a voice's time and pitch), so phases never depend on order. */
export function seedOf(...xs) {
	let h = 2166136261;
	for (const x of xs) {
		h ^= Math.round(x * 1000) | 0;
		h = Math.imul(h, 16777619);
	}
	return h >>> 0;
}

const shaperCurves = new Map();
/** tanh saturation over inputs [-range, range] (fed through a 1/range gain, see _sat), so a
 *  hot input rounds off instead of hitting the WaveShaper's hard clamp at +-1. */
function clipCurve(drive, range = 1) {
	const key = drive + '/' + range;
	if (!shaperCurves.has(key)) {
		const n = 4096;
		const c = new Float32Array(n);
		for (let i = 0; i < n; i++) {
			const x = ((i / (n - 1)) * 2 - 1) * range;
			c[i] = Math.tanh(x * drive) / Math.tanh(drive);
		}
		shaperCurves.set(key, c);
	}
	return shaperCurves.get(key);
}

// JP-8000 style supersaw spread: the outer voices fan out faster than the inner ones
const SPREAD7 = [-1, -0.62, -0.28, 0, 0.28, 0.62, 1];

export const SYNTH = {
	// --- plumbing ----------------------------------------------------------------------

	_shaper(drive, oversample = '2x') {
		const s = this.ctx.createWaveShaper();
		s.curve = clipCurve(drive);
		s.oversample = oversample;
		return s;
	},

	/** Saturate `from` with headroom: the same curve as _shaper up to +-1, soft up to +-4. */
	_sat(from, drive, range = 4) {
		const s = this.ctx.createWaveShaper();
		s.curve = clipCurve(drive, range);
		s.oversample = '2x';
		from.connect(this._gain(1 / range)).connect(s);
		return s;
	},

	/** A looping noise source starting at a seeded offset (order-independent). */
	_noiseAt(b, kind, when, dur, seed = 1, rate = 1) {
		const s = this.ctx.createBufferSource();
		s.buffer = kind === 'pink' ? this.pink : this.white;
		s.loop = true;
		s.playbackRate.value = rate;
		s.start(when, (seed % 997) / 997 * 1.6);
		s.stop(when + dur);
		return this._track(b, s);
	},

	/** An oscillator started part of a cycle early (seeded), so stacked voices never start in phase. */
	_oscP(b, type, f, when, dur, r) {
		const o = this.ctx.createOscillator();
		o.type = type;
		o.frequency.value = f;
		const lead = r ? r() / f : 0;
		o.start(Math.max(0, when - lead));
		o.stop(when + dur);
		return this._track(b, o);
	},

	/** ADSR on a fresh gain: attack to `peak`, decay to `sus`, hold until `off`, release. */
	_adsr(when, { peak = 1, a = 0.01, d = 0.2, s = 0.7, off, r = 0.3 }) {
		const g = this._gain(0);
		const p = g.gain;
		p.setValueAtTime(0, when);
		p.linearRampToValueAtTime(peak, when + a);
		const sus = Math.max(1e-4, peak * s);
		if (off > when + a + 0.001) {
			p.setTargetAtTime(sus, when + a, d / 3);
			p.setValueAtTime(this._approach(peak, sus, off - when - a, d / 3), off);
		}
		p.exponentialRampToValueAtTime(Math.max(1e-5, sus * 0.0005), Math.max(off, when + a) + r);
		return g;
	},
	_approach(from, to, t, tau) {
		return to + (from - to) * Math.exp(-t / tau);
	},

	/** The per-piece theme bus: rumble filter, a gentle low-mid dip, a pump (sidechain) input
	 *  and a ping-pong delay send, created on first use. */
	themeBus(b) {
		if (b.themeBus) return b.themeBus;
		const ctx = this.ctx;
		const input = this._gain(1);
		const hp = this._filter('highpass', 24, 0.7);
		const mud = ctx.createBiquadFilter();
		mud.type = 'peaking';
		mud.frequency.value = 300;
		mud.Q.value = 0.9;
		mud.gain.value = -1.5;
		// a touch of air on top, set per arrangement (theme.js)
		const air = ctx.createBiquadFilter();
		air.type = 'highshelf';
		air.frequency.value = 6500;
		air.gain.value = 0;
		const out = this._gain(1);
		input.connect(hp).connect(mud).connect(air).connect(out).connect(b.music);
		const pump = this._gain(1);
		pump.connect(input);
		// ping-pong delay, a dotted eighth at 120 bpm, darkened each repeat
		const dIn = this._gain(1);
		const dl = ctx.createDelay(1);
		const dr = ctx.createDelay(1);
		dl.delayTime.value = 0.375;
		dr.delayTime.value = 0.375;
		const fb = this._gain(0.32);
		const tone = this._filter('lowpass', 3400, 0.5);
		const tl = this._pan(-0.75);
		const tr = this._pan(0.75);
		dIn.connect(dl);
		dl.connect(tl).connect(input);
		dl.connect(dr);
		dr.connect(tr).connect(input);
		dr.connect(tone).connect(fb).connect(dl);
		b.themeBus = { in: input, pump, delay: dIn, out, air: air.gain, mud: mud.gain };
		return b.themeBus;
	},

	/** Duck the pump input on every kick: a fast dip and an exponential recovery. */
	pumpAt(b, times, { depth = 0.62, recover = 0.22 } = {}) {
		const p = this.themeBus(b).pump.gain;
		const n = 48;
		for (let i = 0; i < times.length; i++) {
			const t = times[i];
			// a play started mid-piece skips the dips already gone (curves may not overlap)
			if (t < this.ctx.currentTime) continue;
			const next = times[i + 1] ?? t + 1;
			const len = Math.min(recover * 2.2, next - t - 0.004);
			const c = new Float32Array(n);
			for (let k = 0; k < n; k++) {
				const x = (k / (n - 1)) * len;
				c[k] = x < 0.006 ? 1 - depth * (x / 0.006) : 1 - depth * Math.exp(-(x - 0.006) / (recover / 3));
			}
			c[n - 1] = 1;
			p.setValueCurveAtTime(c, t, len);
		}
	},

	/** A stereo chorus: two modulated delays (left, right) under the dry signal. */
	_chorus(b, input, when, dur, { mix = 0.45, depth = 0.0026, rate = [0.53, 0.71] } = {}) {
		const out = this._gain(1);
		input.connect(out);
		[-1, 1].forEach((side, i) => {
			const d = this.ctx.createDelay(0.05);
			d.delayTime.value = 0.014 + i * 0.004;
			const lfo = this._oscP(b, 'sine', rate[i], when, dur + 0.2, null);
			const amt = this._gain(depth);
			lfo.connect(amt).connect(d.delayTime);
			const p = this._pan(side * 0.8);
			input.connect(d).connect(this._gain(mix)).connect(p).connect(out);
		});
		return out;
	},

	_dest(b, o) {
		return o.dest || this.themeBus(b).in;
	},

	// --- drums ---------------------------------------------------------------------------

	/** Kick: a sine with a fast pitch envelope through a soft clipper, plus a beater click. */
	kick(b, when, o = {}) {
		const { gain = 0.7, tone = 50, decay = 0.36, click = 0.25, seed = 1 } = o;
		const out = this._gain(gain);
		const s = this._oscP(b, 'sine', tone * 3.4, when, decay + 0.1, null);
		s.frequency.setValueAtTime(tone * 3.4, when);
		s.frequency.exponentialRampToValueAtTime(tone * 1.3, when + 0.025);
		s.frequency.exponentialRampToValueAtTime(tone, when + 0.1);
		s.frequency.exponentialRampToValueAtTime(tone * 0.86, when + decay);
		const env = this._gain(0);
		env.gain.setValueAtTime(0, when);
		env.gain.linearRampToValueAtTime(1, when + 0.0015);
		env.gain.setValueAtTime(1, when + 0.035);
		env.gain.exponentialRampToValueAtTime(0.001, when + decay);
		s.connect(this._shaper(1.7)).connect(env).connect(out);
		if (click > 0) {
			const n = this._noiseAt(b, 'white', when, 0.02, seed);
			n.connect(this._filter('bandpass', 2600, 0.8)).connect(this._ad(when, click, 0.0005, 0.007)).connect(out);
		}
		out.connect(this._dest(b, o));
	},

	/** Clap: four noise bursts ~10 ms apart and a short tail, in two bands for width. */
	clap(b, when, o = {}) {
		const { gain = 0.3, verb = 0.22, seed = 3, pan = 0 } = o;
		const out = this._pan(pan);
		[
			[1150, 0.85, -0.18, 1],
			[2000, 1.4, 0.18, 0.4]
		].forEach(([f, q, p, g], li) => {
			const n = this._noiseAt(b, 'white', when, 0.32, seed + li * 7);
			const env = this._gain(0);
			const e = env.gain;
			for (let i = 0; i < 4; i++) {
				const t = when + i * 0.0105;
				e.setValueAtTime(0.0001, t);
				e.linearRampToValueAtTime(g * (i === 3 ? 1 : 0.75), t + 0.0008);
				if (i < 3) e.exponentialRampToValueAtTime(g * 0.08, t + 0.0095);
			}
			e.exponentialRampToValueAtTime(0.0005, when + 0.0315 + 0.18);
			const pp = this._pan(p);
			n.connect(this._filter('highpass', 650, 0.7)).connect(this._filter('bandpass', f, q)).connect(env).connect(pp).connect(out);
		});
		const g = this._gain(gain * 1.6);
		out.connect(g).connect(this._dest(b, o));
		this._send(b, g, verb);
	},

	/** Snare: a tuned body and a noise wire layer. */
	snare(b, when, o = {}) {
		const { gain = 0.3, tone = 185, verb = 0.25, seed = 5, pan = 0, decay = 0.18 } = o;
		const out = this._pan(pan);
		const body = this._oscP(b, 'triangle', tone * 1.5, when, 0.2, null);
		body.frequency.setValueAtTime(tone * 1.5, when);
		body.frequency.exponentialRampToValueAtTime(tone, when + 0.03);
		body.connect(this._ad(when, 0.55, 0.001, 0.11)).connect(out);
		const n = this._noiseAt(b, 'white', when, decay + 0.05, seed);
		n.connect(this._filter('highpass', 1500, 0.7))
			.connect(this._filter('lowpass', 7500, 0.6))
			.connect(this._ad(when, 0.6, 0.001, decay))
			.connect(out);
		const g = this._gain(gain);
		out.connect(g).connect(this._dest(b, o));
		this._send(b, g, verb);
	},

	/** Hi-hat: bright noise, closed or open. */
	hat(b, when, o = {}) {
		const { gain = 0.06, open = false, seed = 9, pan = 0.2 } = o;
		const n = this._noiseAt(b, 'white', when, open ? 0.3 : 0.06, seed);
		const p = this._pan(pan);
		n.connect(this._filter('highpass', 7600, 0.7))
			.connect(this._ad(when, gain, 0.0008, open ? 0.2 : 0.032))
			.connect(p)
			.connect(this._dest(b, o));
	},

	/** Crash: two decorrelated noise washes and a metallic partial cluster. */
	crash(b, when, o = {}) {
		const { gain = 0.16, decay = 2.2, verb = 0.3, seed = 13 } = o;
		const out = this._gain(gain);
		[-0.5, 0.5].forEach((p, i) => {
			const n = this._noiseAt(b, 'white', when, decay + 0.1, seed + i * 31);
			const pk = this._filter('peaking', 8200, 0.8);
			pk.gain.value = 4;
			n.connect(this._filter('highpass', 3800, 0.6)).connect(pk).connect(this._ad(when, 0.7, 0.002, decay)).connect(this._pan(p)).connect(out);
		});
		const metal = this._gain(0);
		metal.gain.setValueAtTime(0.18, when);
		metal.gain.exponentialRampToValueAtTime(0.001, when + decay * 0.6);
		for (const f of [411, 547, 698, 879, 1213, 1597]) this._oscP(b, 'square', f, when, decay, null).connect(metal);
		metal.connect(this._filter('highpass', 6000, 0.7)).connect(out);
		out.connect(this._dest(b, o));
		this._send(b, out, verb);
	},

	/** A reversed-cymbal swell that ends exactly at `end` (cut dead for the drop). */
	swell(b, end, dur, o = {}) {
		const { gain = 0.12, seed = 17 } = o;
		const when = end - dur;
		const out = this._gain(0);
		const n = 64;
		const c = new Float32Array(n);
		for (let k = 0; k < n; k++) c[k] = gain * Math.pow(k / (n - 1), 2.6);
		out.gain.setValueCurveAtTime(c, when, dur);
		out.gain.setValueAtTime(gain, end);
		out.gain.linearRampToValueAtTime(0, end + 0.01);
		[-0.45, 0.45].forEach((p, i) => {
			const src = this._noiseAt(b, 'white', when, dur + 0.05, seed + i * 11);
			const lp = this._filter('lowpass', 2500, 0.7);
			lp.frequency.setValueAtTime(2500, when);
			lp.frequency.exponentialRampToValueAtTime(15000, end);
			src.connect(this._filter('highpass', 2400, 0.6)).connect(lp).connect(this._pan(p)).connect(out);
		});
		out.connect(this._dest(b, o));
		this._send(b, out, 0.35);
	},

	/** Tuned drum (tom, taiko): a pitch-dropping body, an inharmonic mode and a skin slap. */
	drum(b, when, o = {}) {
		const { gain = 0.4, f = 90, decay = 0.6, pan = 0, verb = 0.3, slap = 0.4, seed = 21 } = o;
		const out = this._pan(pan);
		const body = this._oscP(b, 'sine', f * 2.3, when, decay + 0.1, null);
		body.frequency.setValueAtTime(f * 2.3, when);
		body.frequency.exponentialRampToValueAtTime(f * 1.06, when + 0.035);
		body.frequency.exponentialRampToValueAtTime(f, when + 0.14);
		body.frequency.exponentialRampToValueAtTime(f * 0.93, when + decay);
		const sh = this._shaper(1.5);
		body.connect(sh).connect(this._ad(when, 1, 0.002, decay)).connect(out);
		const mode = this._oscP(b, 'sine', f * 1.59, when, decay * 0.4 + 0.05, null);
		mode.connect(this._ad(when, 0.22, 0.002, decay * 0.3)).connect(out);
		// the shell: a short bright partial that gives the hit its attack in the mids
		const shell = this._oscP(b, 'triangle', f * 3.4, when, 0.16, null);
		shell.frequency.setValueAtTime(f * 3.4, when);
		shell.frequency.exponentialRampToValueAtTime(f * 2.6, when + 0.04);
		shell.connect(this._ad(when, 0.3, 0.001, 0.08)).connect(out);
		if (slap > 0) {
			const n = this._noiseAt(b, 'white', when, 0.08, seed);
			n.connect(this._filter('bandpass', 950, 0.7)).connect(this._ad(when, slap * 1.4, 0.001, 0.05)).connect(out);
			n.connect(this._filter('highpass', 2500, 0.7)).connect(this._ad(when, slap * 0.25, 0.0005, 0.012)).connect(out);
		}
		const g = this._gain(gain);
		out.connect(g).connect(this._dest(b, o));
		this._send(b, g, verb);
	},

	/** Three taikos a few milliseconds apart across the stereo field: one big ensemble hit. */
	taikos(b, when, o = {}) {
		const { gain = 0.5, f = 58, decay = 1.1, verb = 0.35 } = o;
		[
			[0, 1, -0.35],
			[0.006, 1.1, 0.05],
			[0.013, 1.22, 0.4]
		].forEach(([dt, k, pan], i) => this.drum(b, when + dt, { ...o, gain: gain * 0.48, f: f * k, decay, pan, verb, slap: 0.45, seed: 40 + i }));
	},

	/** Timpani: a tuned membrane (inharmonic partials) with a slight pitch settle. */
	timpani(b, when, f, o = {}) {
		const { gain = 0.3, decay = 2.0, pan = 0, verb = 0.4, seed = 25 } = o;
		const out = this._pan(pan);
		[
			[1, 1, 1],
			[1.5, 0.5, 0.7],
			[1.98, 0.32, 0.5],
			[2.44, 0.16, 0.36],
			[2.9, 0.09, 0.26]
		].forEach(([r, a, dk]) => {
			const s = this._oscP(b, 'sine', f * r * 1.015, when, decay * dk + 0.1, null);
			s.frequency.setValueAtTime(f * r * 1.015, when);
			s.frequency.exponentialRampToValueAtTime(f * r, when + 0.12);
			s.connect(this._ad(when, a, 0.003, decay * dk)).connect(out);
		});
		const n = this._noiseAt(b, 'white', when, 0.05, seed);
		n.connect(this._filter('lowpass', 1100, 0.7)).connect(this._ad(when, 0.25, 0.001, 0.03)).connect(out);
		const g = this._gain(gain);
		out.connect(g).connect(this._dest(b, o));
		this._send(b, g, verb);
	},

	/** A drum roll from `t0` to `t1`: strokes accelerate (quarters to 32nds) and crescendo. */
	roll(b, t0, t1, o = {}) {
		const { voice = 'snare', from = 0.3, to = 1, pitch = [1, 1.25], grid = [0.5, 0.25, 0.125, 0.0625], f = 90 } = o;
		const span = t1 - t0;
		let t = t0;
		let i = 0;
		while (t < t1 - 1e-4) {
			const x = (t - t0) / span;
			const g = from + (to - from) * Math.pow(x, 1.4);
			const k = pitch[0] + (pitch[1] - pitch[0]) * x;
			const seed = 60 + i;
			const pan = i % 2 ? 0.18 : -0.18;
			if (voice === 'snare') this.snare(b, t, { ...o, gain: (o.gain ?? 0.24) * g, tone: 185 * k, pan, seed, decay: 0.14 });
			else if (voice === 'timpani') this.timpani(b, t, f * k, { ...o, gain: (o.gain ?? 0.2) * g, decay: 1.1, pan, seed });
			else this.drum(b, t, { ...o, gain: (o.gain ?? 0.3) * g, f: f * k, decay: 0.5, pan, seed });
			const step = grid[Math.min(grid.length - 1, Math.floor(x * grid.length))];
			t += step;
			i++;
		}
	},

	// --- bass ------------------------------------------------------------------------------

	/** A clean sub: a sine with a quiet second harmonic so it reads on small speakers. */
	subNote(b, when, f, dur, o = {}) {
		const { gain = 0.3, a = 0.01, r = 0.3 } = o;
		const env = this._adsr(when, { peak: gain, a, d: 0.1, s: 1, off: when + dur, r });
		this._oscP(b, 'sine', f, when, dur + r + 0.05, null).connect(env);
		this._oscP(b, 'sine', f * 2, when, dur + r + 0.05, null).connect(this._gain(0.12)).connect(env);
		env.connect(this._dest(b, o));
	},

	/** Synth bass: a sub sine under a filtered saw an octave up. */
	bass(b, when, f, dur, o = {}) {
		const { gain = 0.3, cutoff = 420, r = 0.06, seed = 1 } = o;
		const rr = rng(seed);
		const env = this._adsr(when, { peak: 1, a: 0.004, d: 0.12, s: 0.85, off: when + dur, r });
		const sub = this._oscP(b, 'sine', f, when, dur + r + 0.05, rr);
		sub.connect(this._gain(0.7)).connect(env);
		const lp = this._filter('lowpass', cutoff * 2.2, 1.1);
		lp.frequency.setValueAtTime(cutoff * 2.2, when);
		lp.frequency.exponentialRampToValueAtTime(cutoff, when + 0.12);
		for (const det of [-5, 5]) {
			const s = this._oscP(b, 'sawtooth', f * 2, when, dur + r + 0.05, rr);
			s.detune.value = det;
			s.connect(this._gain(0.22)).connect(lp);
		}
		lp.connect(env);
		this._sat(env, 1.3).connect(this._gain(gain)).connect(this._dest(b, o));
	},

	// --- synths ---------------------------------------------------------------------------

	/**
	 * Supersaw chord: seven detuned saws per note (phases seeded), split left and right,
	 * high-passed out of the bass, through a 24 dB low-pass with its own envelope.
	 */
	supersaw(b, when, notes, dur, o = {}) {
		const { gain = 0.16, detune = 22, a = 0.008, d = 0.3, s = 0.85, r = 0.35, cut = [[0, 6000]], q = 0.5, hp = 170, width = 0.85, verb = 0.25, seed = 7 } = o;
		const rr = rng(seed);
		const L = this._pan(-width);
		const R = this._pan(width);
		const C = this._pan(0);
		const sum = this._gain(1.4 / Math.sqrt(notes.length * 7));
		[L, R, C].forEach((n) => n.connect(sum));
		const total = dur + r + 0.05;
		notes.forEach((f, ni) => {
			SPREAD7.forEach((sp, vi) => {
				const osc = this._oscP(b, 'sawtooth', f, when, total, rr);
				osc.detune.value = sp * detune + (ni - notes.length / 2) * 0.7;
				const lvl = sp === 0 ? 1 : 0.72;
				osc.connect(this._gain(lvl)).connect(sp === 0 ? C : vi % 2 ? L : R);
			});
		});
		const hpf = this._filter('highpass', hp, 0.6);
		const lp1 = this._filter('lowpass', cut[0][1], q);
		const lp2 = this._filter('lowpass', cut[0][1], q);
		for (const lp of [lp1, lp2]) {
			lp.frequency.setValueAtTime(cut[0][1], when);
			for (const [t, f] of cut.slice(1)) lp.frequency.exponentialRampToValueAtTime(f, when + t);
		}
		const env = this._adsr(when, { peak: gain, a, d, s, off: when + dur, r });
		sum.connect(hpf).connect(lp1).connect(lp2).connect(env).connect(this._dest(b, o));
		this._send(b, env, verb);
		return env;
	},

	/** Pluck: saw and square through a snapping low-pass. */
	pluck(b, when, f, o = {}) {
		const { gain = 0.06, decay = 0.28, bright = 1, pan = 0, verb = 0.2, delay = 0, seed = 3 } = o;
		const rr = rng(seed);
		const lp = this._filter('lowpass', 400, 1.2);
		lp.frequency.setValueAtTime(Math.min(12000, f * 9 * bright), when);
		lp.frequency.exponentialRampToValueAtTime(Math.max(300, f * 1.4), when + decay);
		this._oscP(b, 'sawtooth', f, when, decay + 0.1, rr).connect(lp);
		this._oscP(b, 'square', f, when, decay + 0.1, rr).connect(this._gain(0.4)).connect(lp);
		const p = this._pan(pan);
		lp.connect(this._ad(when, gain, 0.002, decay)).connect(p).connect(this._dest(b, o));
		this._send(b, p, verb);
		if (delay > 0) p.connect(this._gain(delay)).connect(this.themeBus(b).delay);
	},

	/** Lead for the hook (theme 2): detuned saws and a sub-octave square, glide and late vibrato. */
	lead(b, when, f, dur, o = {}) {
		const { gain = 0.14, glide = null, bright = 1, pan = 0, verb = 0.3, delay = 0.18, r = 0.25, seed = 11 } = o;
		const rr = rng(seed);
		const total = dur + r + 0.05;
		const vib = this._oscP(b, 'sine', 5.6, when, total, null);
		const vd = this._gain(0);
		vd.gain.setValueAtTime(0, when + 0.22);
		vd.gain.linearRampToValueAtTime(11, when + 0.5);
		vib.connect(vd);
		const pre = this._gain(0.5);
		for (const [type, ratio, det, lvl] of [
			['sawtooth', 1, -9, 1],
			['sawtooth', 1, 9, 1],
			['square', 0.5, 0, 0.32],
			['sawtooth', 2, 3, 0.12]
		]) {
			const osc = this._oscP(b, type, f * ratio, when, total, rr);
			if (glide) {
				osc.frequency.setValueAtTime(glide * ratio, when);
				osc.frequency.exponentialRampToValueAtTime(f * ratio, when + 0.045);
			}
			osc.detune.value = det;
			vd.connect(osc.detune);
			osc.connect(this._gain(lvl)).connect(pre);
		}
		const lp1 = this._filter('lowpass', 900, 0.8);
		const lp2 = this._filter('lowpass', 900, 0.5);
		for (const lp of [lp1, lp2]) {
			lp.frequency.setValueAtTime(900, when);
			lp.frequency.exponentialRampToValueAtTime(5200 * bright, when + 0.012);
			lp.frequency.exponentialRampToValueAtTime(3000 * bright, when + 0.25);
		}
		const env = this._adsr(when, { peak: gain, a: 0.006, d: 0.15, s: 0.8, off: when + dur, r });
		const p = this._pan(pan);
		this._sat(pre.connect(lp1).connect(lp2), 1.25).connect(env).connect(p).connect(this._dest(b, o));
		this._send(b, p, verb);
		if (delay > 0) p.connect(this._gain(delay)).connect(this.themeBus(b).delay);
	},

	/** Warm keys (theme 3): a felt-piano tone, triangle and sines with a hammer thump. */
	keys(b, when, f, o = {}) {
		const { gain = 0.07, decay = 1.6, bright = 0.6, pan = 0, verb = 0.35, seed = 5 } = o;
		const rr = rng(seed);
		const out = this._gain(1);
		const lp = this._filter('lowpass', 1200 + 2400 * bright, 0.5);
		lp.frequency.setValueAtTime(1500 + 3000 * bright, when);
		lp.frequency.exponentialRampToValueAtTime(700 + 1000 * bright, when + decay * 0.6);
		[
			['triangle', 1, -3, 1],
			['sine', 1, 3, 0.8],
			['sine', 2, 0, 0.22],
			['sine', 3, 2, 0.07]
		].forEach(([type, ratio, det, lvl]) => {
			const osc = this._oscP(b, type, f * ratio, when, decay + 0.1, rr);
			osc.detune.value = det;
			osc.connect(this._gain(lvl)).connect(lp);
		});
		lp.connect(out);
		const n = this._noiseAt(b, 'white', when, 0.03, seed);
		n.connect(this._filter('lowpass', 1400, 0.7)).connect(this._gain(0.15)).connect(out);
		const p = this._pan(pan);
		out.connect(this._ad(when, gain, 0.004, decay)).connect(p).connect(this._dest(b, o));
		this._send(b, p, verb);
	},

	/** Glockenspiel/celesta: three bar partials, the upper ones dying first. */
	glock(b, when, f, o = {}) {
		const { gain = 0.05, decay = 1.8, pan = 0, verb = 0.5 } = o;
		const p = this._pan(pan);
		[
			[1, 1, 1],
			[2.756, 0.28, 0.35],
			[5.404, 0.1, 0.18]
		].forEach(([r, a, dk]) => {
			this._oscP(b, 'sine', f * r, when, decay * dk + 0.05, null).connect(this._ad(when, gain * a, 0.0012, decay * dk)).connect(p);
		});
		p.connect(this._dest(b, o));
		this._send(b, p, verb);
	},

	/** Harp: a plucked string (triangle plus a soft octave) that rings. */
	harp(b, when, f, o = {}) {
		const { gain = 0.05, decay = 1.4, pan = 0, verb = 0.5 } = o;
		const p = this._pan(pan);
		const lp = this._filter('lowpass', Math.min(9000, f * 6), 0.5);
		this._oscP(b, 'triangle', f, when, decay + 0.05, null).connect(lp);
		this._oscP(b, 'sine', f * 2, when, decay * 0.5, null).connect(this._gain(0.3)).connect(lp);
		lp.connect(this._ad(when, gain, 0.002, decay)).connect(p).connect(this._dest(b, o));
		this._send(b, p, verb);
	},

	/** A harp glissando up `notes` from t0 to t1 (accelerating), panned left to right. */
	gliss(b, t0, t1, notes, o = {}) {
		notes.forEach((f, i) => {
			const x = i / (notes.length - 1);
			this.harp(b, t0 + (t1 - t0) * Math.pow(x, 0.8), f, { ...o, pan: -0.6 + 1.2 * x });
		});
	},

	// --- orchestra ---------------------------------------------------------------------------

	/**
	 * Brass section: per note three detuned saws and a sub-octave square, a pitch scoop, a
	 * filter that blooms with the attack (the "blat") and settles, breath, and late vibrato.
	 */
	brass(b, when, notes, dur, o = {}) {
		const { gain = 0.16, bright = 0.8, a = 0.045, r = 0.35, pan = 0.3, verb = 0.4, drive = 1.5, seed = 31, scoop = 35 } = o;
		const rr = rng(seed);
		const total = dur + r + 0.08;
		const pre = this._gain(1.1 / Math.sqrt(notes.length));
		notes.forEach((f, i) => {
			const p = this._pan(notes.length > 1 ? (i / (notes.length - 1) - 0.5) * 2 * pan : 0);
			const lp = this._filter('lowpass', 280, 0.7);
			lp.frequency.setValueAtTime(280, when);
			lp.frequency.exponentialRampToValueAtTime(700 + 2600 * bright, when + a + 0.05);
			lp.frequency.exponentialRampToValueAtTime(600 + 1500 * bright, when + a + 0.45);
			const vib = this._oscP(b, 'sine', 5.1 + i * 0.11, when, total, null);
			const vd = this._gain(0);
			vd.gain.setValueAtTime(0, when + 0.3);
			vd.gain.linearRampToValueAtTime(6, when + 0.7);
			vib.connect(vd);
			for (const [type, ratio, det, lvl] of [
				['sawtooth', 1, -6, 0.33],
				['sawtooth', 1, 1, 0.33],
				['sawtooth', 1, 7, 0.33],
				['square', 0.5, 0, 0.16]
			]) {
				const osc = this._oscP(b, type, f * ratio, when, total, rr);
				osc.detune.setValueAtTime(det - scoop, when);
				osc.detune.linearRampToValueAtTime(det, when + 0.07);
				vd.connect(osc.detune);
				osc.connect(this._gain(lvl)).connect(lp);
			}
			lp.connect(p).connect(pre);
		});
		const breath = this._noiseAt(b, 'pink', when, 0.15, seed);
		breath.connect(this._filter('bandpass', 1400, 0.9)).connect(this._ad(when, 0.05, 0.01, 0.08)).connect(pre);
		const env = this._adsr(when, { peak: gain, a, d: 0.3, s: 0.78, off: when + dur, r });
		this._sat(pre, drive).connect(env).connect(this._dest(b, o));
		this._send(b, env, verb);
		return env;
	},

	/**
	 * Strings: four detuned saws per note with their own slow vibrato, a soft low-pass, a dip
	 * where violins get edgy, an optional tremolo and a chorus for the section's width.
	 */
	strings(b, when, notes, dur, o = {}) {
		const { gain = 0.1, a = 0.25, r = 0.8, cutoff = 3200, hp = 140, trem = 0, verb = 0.45, swell = null, seed = 41 } = o;
		const rr = rng(seed);
		const total = dur + r + 0.1;
		const L = this._pan(-0.7);
		const R = this._pan(0.7);
		const sum = this._gain(1.2 / Math.sqrt(notes.length * 4));
		L.connect(sum);
		R.connect(sum);
		notes.forEach((f, i) => {
			const vib = this._oscP(b, 'sine', 4.9 + i * 0.23, when, total, rr);
			const vd = this._gain(0);
			vd.gain.setValueAtTime(0, when + 0.2);
			vd.gain.linearRampToValueAtTime(7, when + 0.8);
			vib.connect(vd);
			[-12, -4, 5, 13].forEach((det, vi) => {
				const osc = this._oscP(b, 'sawtooth', f, when, total, rr);
				osc.detune.value = det;
				vd.connect(osc.detune);
				osc.connect((vi + i) % 2 ? L : R);
			});
		});
		const deharsh = this.ctx.createBiquadFilter();
		deharsh.type = 'peaking';
		deharsh.frequency.value = 2800;
		deharsh.Q.value = 0.9;
		deharsh.gain.value = -4;
		const env = this._adsr(when, { peak: gain, a, d: 0.4, s: 0.9, off: when + dur, r });
		let chain = sum.connect(this._filter('highpass', hp, 0.6)).connect(this._filter('lowpass', cutoff, 0.5)).connect(deharsh).connect(env);
		if (swell) {
			// a crescendo across the note: [[t, level], ...] relative to `when`
			const sg = this._gain(swell[0][1]);
			sg.gain.setValueAtTime(swell[0][1], when);
			for (const [t, v] of swell.slice(1)) sg.gain.linearRampToValueAtTime(v, when + t);
			chain = chain.connect(sg);
		}
		if (trem > 0) {
			const tg = this._gain(1 - trem / 2);
			const lfo = this._oscP(b, 'sine', 12, when, total, null);
			lfo.connect(this._gain(trem / 2)).connect(tg.gain);
			chain = chain.connect(tg);
		}
		const out = this._chorus(b, chain, when, total, { mix: 0.4 });
		out.connect(this._dest(b, o));
		this._send(b, out, verb);
	},

	/** Spiccato strings: short detuned saws through one low-pass that opens across the phrase. */
	spiccato(b, steps, o = {}) {
		const { cutoff = [700, 3200], verb = 0.22, gain = 1 } = o;
		if (!steps.length) return;
		const t0 = steps[0].when;
		const t1 = steps[steps.length - 1].when + 0.2;
		const lp = this._filter('lowpass', cutoff[0], 1.1);
		lp.frequency.setValueAtTime(cutoff[0], t0);
		lp.frequency.exponentialRampToValueAtTime(cutoff[1], t1);
		const out = this._gain(gain);
		for (const s of steps) {
			const env = this._ad(s.when, s.gain, 0.004, 0.12);
			for (const det of [-7, 6]) {
				const osc = this._oscP(b, 'sawtooth', s.f, s.when, 0.2, null);
				osc.detune.value = det;
				osc.connect(env);
			}
			env.connect(this._pan(s.pan || 0)).connect(lp);
		}
		lp.connect(out).connect(this._dest(b, o));
		this._send(b, out, verb);
	},

	/** Choir "aah": saws through a vowel's three formants, slow attack, vibrato and chorus. */
	choir(b, when, notes, dur, o = {}) {
		const { gain = 0.08, a = 0.35, r = 1.0, verb = 0.6, seed = 51 } = o;
		const rr = rng(seed);
		const total = dur + r + 0.1;
		const src = this._gain(1 / Math.sqrt(notes.length * 3));
		notes.forEach((f, i) => {
			const vib = this._oscP(b, 'sine', 4.7 + i * 0.19, when, total, rr);
			const vd = this._gain(0);
			vd.gain.setValueAtTime(0, when + 0.25);
			vd.gain.linearRampToValueAtTime(9, when + 0.9);
			vib.connect(vd);
			[-9, 0, 10].forEach((det) => {
				const osc = this._oscP(b, 'sawtooth', f, when, total, rr);
				osc.detune.value = det;
				vd.connect(osc.detune);
				osc.connect(src);
			});
		});
		const formants = this._gain(1);
		for (const [f, q, g] of [
			[760, 5, 1],
			[1150, 7, 0.55],
			[2650, 9, 0.12]
		]) {
			src.connect(this._filter('bandpass', f, q)).connect(this._gain(g * 3.2)).connect(formants);
		}
		const env = this._adsr(when, { peak: gain, a, d: 0.5, s: 0.9, off: when + dur, r });
		const out = this._chorus(b, formants.connect(this._filter('lowpass', 4200, 0.5)).connect(env), when, total, { mix: 0.5 });
		out.connect(this._dest(b, o));
		this._send(b, out, verb);
	},

	/** Warm pad: detuned saws (triangle in the bass) through a slow low-pass, with chorus. */
	warmPad(b, when, notes, dur, o = {}) {
		const { gain = 0.05, a = 0.5, r = 1.2, cutoff = [900, 1800], verb = 0.5, seed = 61, hp = 120 } = o;
		const rr = rng(seed);
		const total = dur + r + 0.1;
		const sum = this._gain(1 / Math.sqrt(notes.length * 2));
		notes.forEach((f, i) => {
			for (const det of [-8, 7]) {
				const osc = this._oscP(b, i === 0 ? 'triangle' : 'sawtooth', f, when, total, rr);
				osc.detune.value = det;
				osc.connect(this._pan(det < 0 ? -0.5 : 0.5)).connect(sum);
			}
		});
		const lp = this._filter('lowpass', cutoff[0], 0.6);
		lp.frequency.setValueAtTime(cutoff[0], when);
		lp.frequency.linearRampToValueAtTime(cutoff[1], when + Math.max(0.2, dur));
		const env = this._adsr(when, { peak: gain, a, d: 0.5, s: 1, off: when + dur, r });
		const out = this._chorus(b, sum.connect(this._filter('highpass', hp, 0.6)).connect(lp).connect(env), when, total);
		out.connect(this._dest(b, o));
		this._send(b, out, verb);
	}
};
