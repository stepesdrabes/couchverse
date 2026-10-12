// The synthesized SFX kit: whooshes, risers, impacts, ticks, shimmer, bells, pads, typewriter,
// projector and ambient beds. Every voice is a function of its start time and options; noise
// comes from seeded buffers, so an offline render is repeatable.

import { rng } from '../core/rng';
import { AudioEngine, bellCurve, type Bus } from './engine';

// D major pentatonic, high: UI ticks and sparkles stay in the channel's key
export const SPARKLE = [
	1174.66, 1318.51, 1479.98, 1760.0, 1975.53, 2349.32, 2637.02, 2959.96
] as const;

export class Kit extends AudioEngine {
	/** Filtered-noise sweep, panned with the motion it accompanies. */
	whoosh(
		b: Bus,
		when: number,
		o: {
			dur?: number;
			pan?: [number, number];
			gain?: number;
			lo?: number;
			hi?: number;
			q?: number;
			verb?: number;
			peak?: number;
			body?: number;
		} = {}
	) {
		const {
			dur = 0.8,
			pan = [-0.5, 0.5],
			gain = 0.2,
			lo = 380,
			hi = 2800,
			q = 1.1,
			verb = 0.2,
			peak = 0.6,
			body = 0.5
		} = o;
		const out = this.pan(pan[0]);
		out.pan.setValueAtTime(pan[0], when);
		out.pan.linearRampToValueAtTime(pan[1], when + dur);
		const layer = (l: number, h: number, g: number, rate: number) => {
			const src = this.noise(b, 'pink', when, dur + 0.05, rate);
			const hp = this.filter('highpass', 90);
			const bp = this.filter('bandpass', l, q);
			bp.frequency.setValueAtTime(l, when);
			bp.frequency.exponentialRampToValueAtTime(h, when + dur * peak);
			bp.frequency.exponentialRampToValueAtTime(l * 1.3, when + dur);
			const env = this.gain(0);
			env.gain.setValueCurveAtTime(bellCurve(peak, g), when, dur);
			src.connect(hp).connect(bp).connect(env).connect(out);
		};
		layer(lo, hi, gain, 1);
		if (body > 0) layer(lo * 0.35, hi * 0.3, gain * body, 0.8);
		out.connect(b.sfx);
		this.send(b, out, verb);
	}

	/** Rising noise plus a gliding, trembling tone; peaks exactly at when + dur. */
	riser(
		b: Bus,
		when: number,
		o: {
			dur?: number;
			gain?: number;
			lo?: number;
			hi?: number;
			tone?: number;
			verb?: number;
			root?: number;
		} = {}
	) {
		const { dur = 1.4, gain = 0.12, lo = 400, hi = 7500, tone = 0.5, verb = 0.35, root = 110 } = o;
		const end = when + dur;
		const src = this.noise(b, 'white', when, dur + 0.05);
		const hp = this.filter('highpass', lo, 0.7);
		hp.frequency.setValueAtTime(lo, when);
		hp.frequency.exponentialRampToValueAtTime(hi, end);
		const bp = this.filter('bandpass', lo * 2, 0.9);
		bp.frequency.setValueAtTime(lo * 2, when);
		bp.frequency.exponentialRampToValueAtTime(Math.min(hi * 1.4, 16000), end);
		const env = this.gain(0);
		env.gain.setValueAtTime(0.0001, when);
		env.gain.exponentialRampToValueAtTime(gain, end - 0.02);
		env.gain.linearRampToValueAtTime(0, end + 0.04);
		const pan = this.pan(0);
		src.connect(hp).connect(bp).connect(env).connect(pan);
		if (tone > 0) {
			const lp = this.filter('lowpass', 500, 1.5);
			lp.frequency.setValueAtTime(500, when);
			lp.frequency.exponentialRampToValueAtTime(5000, end);
			const tg = this.gain(0);
			tg.gain.setValueAtTime(0.0001, when);
			tg.gain.exponentialRampToValueAtTime(gain * tone * 0.5, end - 0.02);
			tg.gain.linearRampToValueAtTime(0, end + 0.03);
			const trem = this.gain(1);
			const lfo = this.osc(b, 'sine', 5, when, dur + 0.1);
			lfo.frequency.setValueAtTime(5, when);
			lfo.frequency.exponentialRampToValueAtTime(19, end);
			const depth = this.gain(0.45);
			lfo.connect(depth).connect(trem.gain);
			for (const det of [-9, 9]) {
				const o2 = this.osc(b, 'sawtooth', root, when, dur + 0.1);
				o2.detune.value = det;
				o2.frequency.setValueAtTime(root, when);
				o2.frequency.exponentialRampToValueAtTime(root * 4, end);
				o2.connect(lp);
			}
			lp.connect(tg).connect(trem).connect(pan);
		}
		pan.connect(b.sfx);
		this.send(b, pan, verb);
	}

	/** Sub boom: a pitch-dropping sine through a soft clipper, a click and a low rumble tail. */
	impact(
		b: Bus,
		when: number,
		o: {
			gain?: number;
			size?: number;
			tone?: number;
			verb?: number;
			click?: number;
			pan?: number;
			/** Where it plays (the theme bus for the ident's hit); the piece's sfx bus by default. */
			dest?: AudioNode;
		} = {}
	) {
		const { gain = 0.6, size = 1, tone = 48, verb = 0.25, click = 1, pan = 0 } = o;
		const out = this.pan(pan);
		const sub = this.osc(b, 'sine', tone * 2.8, when, 1.6 * size + 0.1);
		sub.frequency.setValueAtTime(tone * 2.8, when);
		sub.frequency.exponentialRampToValueAtTime(tone, when + 0.11);
		sub.frequency.exponentialRampToValueAtTime(tone * 0.78, when + 1.4 * size);
		const lp = this.filter('lowpass', 900);
		sub
			.connect(this.shaper(2.2))
			.connect(lp)
			.connect(this.ad(when, gain, 0.004, 1.3 * size))
			.connect(out);
		if (click > 0) {
			const n = this.noise(b, 'white', when, 0.05);
			const bp = this.filter('bandpass', 2600, 0.8);
			n.connect(bp)
				.connect(this.ad(when, gain * 0.45 * click, 0.001, 0.035))
				.connect(out);
		}
		const rum = this.noise(b, 'pink', when, 1.8 * size);
		const rlp = this.filter('lowpass', 170, 0.9);
		rum
			.connect(rlp)
			.connect(this.ad(when, gain * 0.9, 0.01, 1.5 * size))
			.connect(out);
		out.connect(o.dest ?? b.sfx);
		this.send(b, out, verb);
	}

	/** Grit hit: a short punch with a snare-like crack (for hard cuts). */
	punch(b: Bus, when: number, o: { gain?: number; pan?: number; verb?: number } = {}) {
		const { gain = 0.5, pan = 0, verb = 0.12 } = o;
		// punches sit under the music: the soundtrack carries the energy, the punch the edge
		this.impact(b, when, { gain: gain * 0.58, size: 0.45, tone: 56, click: 1.2, verb: 0.08, pan });
		const out = this.pan(pan);
		const n = this.noise(b, 'white', when, 0.3);
		const bp = this.filter('bandpass', 1700, 0.7);
		const hp = this.filter('highpass', 600);
		n.connect(hp)
			.connect(bp)
			.connect(this.ad(when, gain * 0.36, 0.002, 0.16))
			.connect(out);
		out.connect(b.sfx);
		this.send(b, out, verb);
	}

	/** Soft UI tick: a tiny pitched "tock" plus a filtered click. */
	tick(
		b: Bus,
		when: number,
		o: {
			freq?: number;
			gain?: number;
			pan?: number;
			decay?: number;
			type?: OscillatorType;
			verb?: number;
		} = {}
	) {
		const { freq = 2200, gain = 0.06, pan = 0, decay = 0.06, type = 'sine', verb = 0.1 } = o;
		const out = this.pan(pan);
		const osc = this.osc(b, type, freq * 1.5, when, decay + 0.05);
		osc.frequency.setValueAtTime(freq * 1.5, when);
		osc.frequency.exponentialRampToValueAtTime(freq, when + 0.012);
		osc.connect(this.ad(when, gain, 0.0015, decay)).connect(out);
		const n = this.noise(b, 'white', when, 0.02);
		const hp = this.filter('highpass', 4500);
		n.connect(hp)
			.connect(this.ad(when, gain * 0.35, 0.0008, 0.008))
			.connect(out);
		out.connect(b.sfx);
		this.send(b, out, verb);
	}

	/** Glow sweep: a cascade of high pentatonic pings over a bright air swell. */
	shimmer(
		b: Bus,
		when: number,
		o: { dur?: number; gain?: number; count?: number; verb?: number; seed?: number } = {}
	) {
		const { dur = 1.1, gain = 0.05, count = 14, verb = 0.7, seed = 7 } = o;
		const r = rng(seed);
		const out = this.gain(1);
		for (let i = 0; i < count; i++) {
			const t = when + dur * Math.pow(i / count, 0.85) + r() * 0.03;
			const f = SPARKLE[Math.floor(r() * SPARKLE.length)] * (r() < 0.3 ? 2 : 1);
			const p = this.pan(r() * 1.4 - 0.7);
			const o2 = this.osc(b, 'sine', f, t, 1);
			o2.connect(this.ad(t, gain * (0.5 + r() * 0.5), 0.003, 0.5 + r() * 0.5))
				.connect(p)
				.connect(out);
		}
		const air = this.noise(b, 'white', when, dur + 0.1);
		const bp = this.filter('bandpass', 6000, 1.2);
		bp.frequency.setValueAtTime(5000, when);
		bp.frequency.exponentialRampToValueAtTime(11000, when + dur);
		const env = this.gain(0);
		env.gain.setValueCurveAtTime(bellCurve(0.45, gain * 0.9), when, dur);
		air.connect(bp).connect(env).connect(out);
		out.connect(b.sfx);
		this.send(b, out, verb);
	}

	/** One FM tine/bell voice (carrier:modulator 1:1 plus an inharmonic 3.5 glint). */
	bell(
		b: Bus,
		when: number,
		freq: number,
		o: { gain?: number; decay?: number; index?: number; pan?: number; verb?: number } = {}
	) {
		const { gain = 0.2, decay = 2.4, index = 2, pan = 0, verb = 0.45 } = o;
		const car = this.osc(b, 'sine', freq, when, decay + 0.2);
		const mod = this.osc(b, 'sine', freq, when, decay + 0.2);
		const mg = this.gain(0);
		mg.gain.setValueAtTime(freq * index, when);
		mg.gain.exponentialRampToValueAtTime(freq * 0.12, when + Math.min(1.2, decay * 0.5));
		mod.connect(mg).connect(car.frequency);
		const mod2 = this.osc(b, 'sine', freq * 3.51, when, 0.4);
		const mg2 = this.gain(0);
		mg2.gain.setValueAtTime(freq * 0.9, when);
		mg2.gain.exponentialRampToValueAtTime(freq * 0.005, when + 0.2);
		mod2.connect(mg2).connect(car.frequency);
		const p = this.pan(pan);
		car.connect(this.ad(when, gain, 0.003, decay)).connect(p);
		// the hammer: a short octave-and-a-fifth partial
		const ham = this.osc(b, 'sine', freq * 3, when, 0.2);
		ham.connect(this.ad(when, gain * 0.12, 0.001, 0.09)).connect(p);
		p.connect(b.sfx);
		this.send(b, p, verb);
	}

	/** Warm pad chord: detuned saws through a closing low-pass. */
	pad(
		b: Bus,
		when: number,
		freqs: number[],
		o: {
			gain?: number;
			attack?: number;
			hold?: number;
			release?: number;
			cutoff?: [number, number];
			verb?: number;
			pan?: number;
		} = {}
	) {
		const {
			gain = 0.05,
			attack = 0.08,
			hold = 0.4,
			release = 2.4,
			cutoff = [2400, 650],
			verb = 0.5,
			pan = 0.35
		} = o;
		const total = attack + hold + release;
		const lp = this.filter('lowpass', cutoff[0], 0.6);
		lp.frequency.setValueAtTime(cutoff[0], when);
		lp.frequency.exponentialRampToValueAtTime(cutoff[1], when + total);
		const env = this.gain(0);
		env.gain.setValueAtTime(0, when);
		env.gain.linearRampToValueAtTime(gain, when + attack);
		env.gain.setValueAtTime(gain, when + attack + hold);
		env.gain.exponentialRampToValueAtTime(gain * 0.001, when + total);
		freqs.forEach((f, i) => {
			const p = this.pan(((i % 2) * 2 - 1) * pan * (i / freqs.length));
			for (const det of [-7, 6]) {
				const o2 = this.osc(b, i === 0 ? 'triangle' : 'sawtooth', f, when, total + 0.1);
				o2.detune.value = det;
				o2.connect(p);
			}
			p.connect(lp);
		});
		lp.connect(env).connect(b.sfx);
		this.send(b, env, verb);
	}

	sub(b: Bus, when: number, freq: number, o: { gain?: number; decay?: number } = {}) {
		const { gain = 0.3, decay = 1.6 } = o;
		const s = this.osc(b, 'sine', freq, when, decay + 0.2);
		s.connect(this.ad(when, gain, 0.01, decay)).connect(b.sfx);
	}

	/** A modern keyboard key: a short bright tick over a soft low thock (typed text). */
	keyClick(
		b: Bus,
		when: number,
		o: { gain?: number; pan?: number; seed?: number; pitch?: number } = {}
	) {
		const { gain = 0.05, pan = 0, seed = 1, pitch = 1 } = o;
		const r = rng(seed);
		const out = this.pan(pan);
		this.noiseAt(b, 'white', when, 0.03, seed)
			.connect(this.filter('highpass', 1800, 0.7))
			.connect(this.filter('bandpass', (3600 + r() * 900) * pitch, 1.4))
			.connect(this.ad(when, gain, 0.0005, 0.012))
			.connect(out);
		const th = this.osc(b, 'sine', (210 + r() * 30) * pitch, when, 0.06);
		th.frequency.setValueAtTime((210 + r() * 30) * pitch, when);
		th.frequency.exponentialRampToValueAtTime(140 * pitch, when + 0.03);
		th.connect(this.ad(when, gain * 0.9, 0.001, 0.03)).connect(out);
		out.connect(b.sfx);
		this.send(b, out, 0.05);
	}

	/** The brackets: a tight, pitched woody knock (open is higher than shut). */
	clack(
		b: Bus,
		when: number,
		o: { gain?: number; pan?: number; freq?: number; seed?: number } = {}
	) {
		const { gain = 0.06, pan = 0, freq = 1175, seed = 2 } = o;
		const out = this.pan(pan);
		const s = this.osc(b, 'sine', freq * 1.6, when, 0.08);
		s.frequency.setValueAtTime(freq * 1.6, when);
		s.frequency.exponentialRampToValueAtTime(freq, when + 0.012);
		s.connect(this.ad(when, gain, 0.0008, 0.05)).connect(out);
		this.noiseAt(b, 'white', when, 0.02, seed)
			.connect(this.filter('bandpass', 2400, 1.2))
			.connect(this.ad(when, gain * 0.5, 0.0005, 0.008))
			.connect(out);
		out.connect(b.sfx);
		this.send(b, out, 0.12);
	}

	/** Typewriter key: a bright clack and a low thunk. */
	key(b: Bus, when: number, o: { gain?: number; pan?: number; seed?: number } = {}) {
		const { gain = 0.08, pan = 0, seed = 1 } = o;
		const r = rng(seed);
		const out = this.pan(pan);
		const n = this.noise(b, 'white', when, 0.03);
		const bp = this.filter('bandpass', 2400 + r() * 1800, 2.2);
		n.connect(bp)
			.connect(this.ad(when, gain, 0.0008, 0.018))
			.connect(out);
		const th = this.osc(b, 'sine', 150 + r() * 40, when, 0.06);
		th.connect(this.ad(when, gain * 0.6, 0.001, 0.035)).connect(out);
		out.connect(b.sfx);
		this.send(b, out, 0.06);
	}

	/** 24 fps projector rattle (a looped procedural buffer). */
	projector(
		b: Bus,
		when: number,
		dur: number,
		skip = 0,
		o: { gain?: number; fadeOut?: number } = {}
	) {
		const { gain = 0.05, fadeOut = 0.3 } = o;
		const left = dur - skip;
		if (left <= 0) return;
		const s = this.ctx.createBufferSource();
		s.buffer = this.projectorBuf;
		s.loop = true;
		const bp = this.filter('bandpass', 1500, 0.5);
		const env = this.gain(0);
		env.gain.setValueAtTime(skip > 0 ? gain : 0, when);
		env.gain.linearRampToValueAtTime(gain, when + 0.08);
		env.gain.setValueAtTime(gain, when + Math.max(0.1, left - fadeOut));
		env.gain.linearRampToValueAtTime(0, when + left);
		s.connect(bp).connect(env).connect(b.sfx);
		s.start(when);
		s.stop(when + left + 0.05);
		this.track(b, s);
	}

	/** Synthesized ambient bed (the lineup): two slow chords, sub and drifting air. */
	ambient(b: Bus, when: number, dur: number, skip = 0, o: { gain?: number } = {}) {
		const { gain = 0.05 } = o;
		const chords = [
			[146.83, 220.0, 277.18, 329.63, 369.99],
			[123.47, 185.0, 220.0, 277.18, 293.66]
		];
		const left = dur - skip;
		if (left <= 0.2) return;
		const fadeIn = skip > 0 ? 0.3 : 1.6;
		const master = this.gain(0);
		master.gain.setValueAtTime(0, when);
		master.gain.linearRampToValueAtTime(gain, when + fadeIn);
		master.gain.setValueAtTime(gain, when + Math.max(fadeIn, left - 1.8));
		master.gain.linearRampToValueAtTime(0, when + left);
		const lp = this.filter('lowpass', 900, 0.5);
		const lfo = this.osc(b, 'sine', 0.13, when, left + 0.1);
		lfo.connect(this.gain(380)).connect(lp.frequency);
		lp.connect(master);
		const half = dur / 2;
		chords.forEach((chord, ci) => {
			const c0 = ci * half;
			const c1 = c0 + half;
			if (c1 <= skip) return;
			const start = when + Math.max(0, c0 - skip);
			const end = when + (c1 - skip) + 1.2;
			const cg = this.gain(0);
			cg.gain.setValueAtTime(0, start);
			cg.gain.linearRampToValueAtTime(1, start + (ci === 0 ? 0.05 : 1.6));
			cg.gain.setValueAtTime(1, Math.max(start + 1.7, end - 1.6));
			cg.gain.linearRampToValueAtTime(0, end);
			chord.forEach((f, i) => {
				const p = this.pan((i / (chord.length - 1)) * 1.2 - 0.6);
				for (const det of [-8, 7]) {
					const osc = this.osc(b, i === 0 ? 'triangle' : 'sawtooth', f, start, end - start + 0.1);
					osc.detune.value = det + i;
					osc.connect(p);
				}
				p.connect(cg);
			});
			cg.connect(lp);
		});
		const s = this.osc(b, 'sine', 73.42, when, left + 0.1);
		s.connect(this.gain(0.55)).connect(master);
		const air = this.noise(b, 'pink', when, left + 0.1);
		air
			.connect(this.filter('bandpass', 3800, 0.4))
			.connect(this.gain(0.08))
			.connect(master);
		master.connect(b.sfx);
		this.send(b, master, 0.5);
	}

	/**
	 * A drone that loops seamlessly every `period` seconds: partials snapped to whole cycles
	 * per period and a filter swell with the same period (the standby card).
	 */
	loopDrone(b: Bus, when: number, period: number, dur: number, o: { gain?: number } = {}) {
		const { gain = 0.05 } = o;
		const snap = (f: number) => Math.round(f * period) / period;
		const out = this.gain(gain);
		const lp = this.filter('lowpass', 700, 0.6);
		const lfo = this.osc(b, 'sine', 1 / period, when, dur);
		lfo.connect(this.gain(320)).connect(lp.frequency);
		const chord = [73.42, 146.83, 220.0, 277.18, 329.63];
		chord.forEach((f, i) => {
			const p = this.pan((i / (chord.length - 1)) * 1.0 - 0.5);
			for (const det of [-0.21, 0.19]) {
				const o2 = this.osc(b, i < 2 ? 'sine' : 'triangle', snap(f + det), when, dur);
				o2.connect(this.gain(i < 2 ? 0.7 : 0.35)).connect(p);
			}
			p.connect(lp);
		});
		lp.connect(out).connect(b.sfx);
		this.send(b, out, 0.4);
	}
}
