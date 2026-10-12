// The theme's instruments (the parts of the sample reel's synth.js the theme plays): taikos,
// toms, snare, clap, crash, swells and risers, a sub, brass, strings, spiccato, choir,
// glockenspiel and the braam. Every voice takes an absolute context time and its own seed,
// never Math.random or the scheduling order, so a render is a pure function of the score.
//
// Voices feed the piece's theme bus (a rumble filter, a 300 Hz dip and an air shelf into the
// music bus) unless `dest` says otherwise. Gains are linear; saturation is tanh with headroom,
// so a hot input rounds off instead of hitting the WaveShaper's hard clamp.

import { rng } from '../core/rng';
import type { Bus } from './engine';
import type { Kit } from './kit';

const PC: Record<string, number> = {
	C: 0,
	'C#': 1,
	Db: 1,
	D: 2,
	'D#': 3,
	Eb: 3,
	E: 4,
	F: 5,
	'F#': 6,
	Gb: 6,
	G: 7,
	'G#': 8,
	Ab: 8,
	A: 9,
	'A#': 10,
	Bb: 10,
	B: 11
};

/** A note name (A4, F#3, Bb1) in Hz, equal temperament. */
export function hz(name: string): number {
	const m = /^([A-G][b#]?)(-?\d)$/.exec(name);
	if (!m) throw new Error(`bad note ${name}`);
	return 440 * 2 ** ((PC[m[1]] + 12 * (Number(m[2]) + 1) - 69) / 12);
}

export const pitchClass = (name: string): number => PC[/^([A-G][b#]?)/.exec(name)?.[1] ?? ''];

/** A stable seed from numbers (a voice's time and pitch), so phases never depend on order. */
export function seedOf(...xs: number[]): number {
	let h = 2166136261;
	for (const x of xs) {
		h ^= Math.round(x * 1000) | 0;
		h = Math.imul(h, 16777619);
	}
	return h >>> 0;
}

export interface Voice {
	/** The node to feed; the piece's theme bus by default. */
	dest?: AudioNode;
}

const curves = new Map<string, Float32Array<ArrayBuffer>>();

/** tanh over inputs [-range, range] (fed through 1/range), normalized at full scale. */
function clipCurve(drive: number, range = 1): Float32Array<ArrayBuffer> {
	const key = `${drive}/${range}`;
	let c = curves.get(key);
	if (!c) {
		const n = 4096;
		c = new Float32Array(n);
		for (let i = 0; i < n; i++) {
			const x = ((i / (n - 1)) * 2 - 1) * range;
			c[i] = Math.tanh(x * drive) / Math.tanh(drive);
		}
		curves.set(key, c);
	}
	return c;
}

function shaper(k: Kit, drive: number): WaveShaperNode {
	const s = k.ctx.createWaveShaper();
	s.curve = clipCurve(drive);
	s.oversample = '2x';
	return s;
}

/** Saturate `from` with headroom: the shaper's curve up to full scale, soft up to `range`. */
function sat(k: Kit, from: AudioNode, drive: number, range = 4): WaveShaperNode {
	const s = k.ctx.createWaveShaper();
	s.curve = clipCurve(drive, range);
	s.oversample = '2x';
	from.connect(k.gain(1 / range)).connect(s);
	return s;
}

/** ADSR on a fresh gain: attack to `peak`, decay to `s`, hold until `off`, release. */
function adsr(
	k: Kit,
	when: number,
	{
		peak = 1,
		a = 0.01,
		d = 0.2,
		s = 0.7,
		off,
		r = 0.3
	}: { peak?: number; a?: number; d?: number; s?: number; off: number; r?: number }
): GainNode {
	const g = k.gain(0);
	const p = g.gain;
	p.setValueAtTime(0, when);
	p.linearRampToValueAtTime(peak, when + a);
	const sus = Math.max(1e-4, peak * s);
	if (off > when + a + 0.001) {
		p.setTargetAtTime(sus, when + a, d / 3);
		p.setValueAtTime(sus + (peak - sus) * Math.exp(-(off - when - a) / (d / 3)), off);
	}
	p.exponentialRampToValueAtTime(Math.max(1e-5, sus * 0.0005), Math.max(off, when + a) + r);
	return g;
}

interface ThemeBus {
	in: GainNode;
}

const themeBuses = new WeakMap<Bus, ThemeBus>();

/** The piece's theme bus, made on first use: rumble filter, a 300 Hz dip, an air shelf, trim. */
export function themeBus(k: Kit, b: Bus): ThemeBus {
	let tb = themeBuses.get(b);
	if (tb) return tb;
	const input = k.gain(1);
	const mud = k.filter('peaking', 300, 0.9);
	mud.gain.value = -2.5;
	const air = k.filter('highshelf', 6500);
	air.gain.value = 2.5;
	input
		.connect(k.filter('highpass', 24, 0.7))
		.connect(mud)
		.connect(air)
		.connect(k.gain(0.7))
		.connect(b.music);
	tb = { in: input };
	themeBuses.set(b, tb);
	return tb;
}

const dest = (k: Kit, b: Bus, o: Voice) => o.dest ?? themeBus(k, b).in;

/** A stereo chorus: two modulated delays (left, right) under the dry signal. */
function chorus(
	k: Kit,
	b: Bus,
	input: AudioNode,
	when: number,
	dur: number,
	{ mix = 0.45, depth = 0.0026, rate = [0.53, 0.71] } = {}
): GainNode {
	const out = k.gain(1);
	input.connect(out);
	[-1, 1].forEach((side, i) => {
		const d = k.ctx.createDelay(0.05);
		d.delayTime.value = 0.014 + i * 0.004;
		k.oscAt(b, 'sine', rate[i], when, dur + 0.2, null)
			.connect(k.gain(depth))
			.connect(d.delayTime);
		input
			.connect(d)
			.connect(k.gain(mix))
			.connect(k.pan(side * 0.8))
			.connect(out);
	});
	return out;
}

// --- drums -------------------------------------------------------------------------------

/** Clap: four noise bursts ~10 ms apart and a short tail, in two bands for width. */
export function clap(
	k: Kit,
	b: Bus,
	when: number,
	o: Voice & { gain?: number; verb?: number; seed?: number; pan?: number } = {}
) {
	const { gain = 0.3, verb = 0.22, seed = 3, pan = 0 } = o;
	const out = k.pan(pan);
	(
		[
			[1150, 0.85, -0.18, 1],
			[2000, 1.4, 0.18, 0.4]
		] as const
	).forEach(([f, q, p, g], li) => {
		const env = k.gain(0);
		const e = env.gain;
		for (let i = 0; i < 4; i++) {
			const t = when + i * 0.0105;
			e.setValueAtTime(0.0001, t);
			e.linearRampToValueAtTime(g * (i === 3 ? 1 : 0.75), t + 0.0008);
			if (i < 3) e.exponentialRampToValueAtTime(g * 0.08, t + 0.0095);
		}
		e.exponentialRampToValueAtTime(0.0005, when + 0.0315 + 0.18);
		k.noiseAt(b, 'white', when, 0.32, seed + li * 7)
			.connect(k.filter('highpass', 650, 0.7))
			.connect(k.filter('bandpass', f, q))
			.connect(env)
			.connect(k.pan(p))
			.connect(out);
	});
	const g = k.gain(gain * 1.6);
	out.connect(g).connect(dest(k, b, o));
	k.send(b, g, verb);
}

/** Snare: a tuned body and a noise wire layer. */
export function snare(
	k: Kit,
	b: Bus,
	when: number,
	o: Voice & {
		gain?: number;
		tone?: number;
		verb?: number;
		seed?: number;
		pan?: number;
		decay?: number;
	} = {}
) {
	const { gain = 0.3, tone = 185, verb = 0.25, seed = 5, pan = 0, decay = 0.18 } = o;
	const out = k.pan(pan);
	const body = k.oscAt(b, 'triangle', tone * 1.5, when, 0.2, null);
	body.frequency.setValueAtTime(tone * 1.5, when);
	body.frequency.exponentialRampToValueAtTime(tone, when + 0.03);
	body.connect(k.ad(when, 0.55, 0.001, 0.11)).connect(out);
	k.noiseAt(b, 'white', when, decay + 0.05, seed)
		.connect(k.filter('highpass', 1500, 0.7))
		.connect(k.filter('lowpass', 7500, 0.6))
		.connect(k.ad(when, 0.6, 0.001, decay))
		.connect(out);
	const g = k.gain(gain);
	out.connect(g).connect(dest(k, b, o));
	k.send(b, g, verb);
}

/** Crash: two decorrelated noise washes and a metallic partial cluster. */
export function crash(
	k: Kit,
	b: Bus,
	when: number,
	o: Voice & { gain?: number; decay?: number; verb?: number; seed?: number } = {}
) {
	const { gain = 0.16, decay = 2.2, verb = 0.3, seed = 13 } = o;
	const out = k.gain(gain);
	[-0.5, 0.5].forEach((p, i) => {
		const pk = k.filter('peaking', 8200, 0.8);
		pk.gain.value = 4;
		k.noiseAt(b, 'white', when, decay + 0.1, seed + i * 31)
			.connect(k.filter('highpass', 3800, 0.6))
			.connect(pk)
			.connect(k.ad(when, 0.7, 0.002, decay))
			.connect(k.pan(p))
			.connect(out);
	});
	const metal = k.gain(0);
	metal.gain.setValueAtTime(0.18, when);
	metal.gain.exponentialRampToValueAtTime(0.001, when + decay * 0.6);
	for (const f of [411, 547, 698, 879, 1213, 1597])
		k.oscAt(b, 'square', f, when, decay, null).connect(metal);
	metal.connect(k.filter('highpass', 6000, 0.7)).connect(out);
	out.connect(dest(k, b, o));
	k.send(b, out, verb);
}

/** A reversed-cymbal swell that ends exactly at `end` (cut dead for the hit). */
export function swell(
	k: Kit,
	b: Bus,
	end: number,
	dur: number,
	o: Voice & { gain?: number; seed?: number } = {}
) {
	const { gain = 0.12, seed = 17 } = o;
	const when = end - dur;
	const out = k.gain(0);
	const n = 64;
	const c = new Float32Array(n);
	for (let i = 0; i < n; i++) c[i] = gain * Math.pow(i / (n - 1), 2.6);
	out.gain.setValueCurveAtTime(c, when, dur);
	out.gain.setValueAtTime(gain, end);
	out.gain.linearRampToValueAtTime(0, end + 0.01);
	[-0.45, 0.45].forEach((p, i) => {
		const lp = k.filter('lowpass', 2500, 0.7);
		lp.frequency.setValueAtTime(2500, when);
		lp.frequency.exponentialRampToValueAtTime(15000, end);
		k.noiseAt(b, 'white', when, dur + 0.05, seed + i * 11)
			.connect(k.filter('highpass', 2400, 0.6))
			.connect(lp)
			.connect(k.pan(p))
			.connect(out);
	});
	out.connect(dest(k, b, o));
	k.send(b, out, 0.35);
}

export interface DrumOptions extends Voice {
	gain?: number;
	f?: number;
	decay?: number;
	pan?: number;
	verb?: number;
	slap?: number;
	seed?: number;
}

/** Tuned drum (tom, taiko): a pitch-dropping body, an inharmonic mode and a skin slap. */
export function drum(k: Kit, b: Bus, when: number, o: DrumOptions = {}) {
	const { gain = 0.4, f = 90, decay = 0.6, pan = 0, verb = 0.3, slap = 0.4, seed = 21 } = o;
	const out = k.pan(pan);
	const body = k.oscAt(b, 'sine', f * 2.3, when, decay + 0.1, null);
	body.frequency.setValueAtTime(f * 2.3, when);
	body.frequency.exponentialRampToValueAtTime(f * 1.06, when + 0.035);
	body.frequency.exponentialRampToValueAtTime(f, when + 0.14);
	body.frequency.exponentialRampToValueAtTime(f * 0.93, when + decay);
	body
		.connect(shaper(k, 1.5))
		.connect(k.ad(when, 1, 0.002, decay))
		.connect(out);
	k.oscAt(b, 'sine', f * 1.59, when, decay * 0.4 + 0.05, null)
		.connect(k.ad(when, 0.22, 0.002, decay * 0.3))
		.connect(out);
	// the shell: a short bright partial that gives the hit its attack in the mids
	const shell = k.oscAt(b, 'triangle', f * 3.4, when, 0.16, null);
	shell.frequency.setValueAtTime(f * 3.4, when);
	shell.frequency.exponentialRampToValueAtTime(f * 2.6, when + 0.04);
	shell.connect(k.ad(when, 0.3, 0.001, 0.08)).connect(out);
	if (slap > 0) {
		const n = k.noiseAt(b, 'white', when, 0.08, seed);
		n.connect(k.filter('bandpass', 950, 0.7))
			.connect(k.ad(when, slap * 1.4, 0.001, 0.05))
			.connect(out);
		n.connect(k.filter('highpass', 2500, 0.7))
			.connect(k.ad(when, slap * 0.25, 0.0005, 0.012))
			.connect(out);
	}
	const g = k.gain(gain);
	out.connect(g).connect(dest(k, b, o));
	k.send(b, g, verb);
}

/** Three taikos a few milliseconds apart across the stereo field: one ensemble hit. */
export function taikos(k: Kit, b: Bus, when: number, o: DrumOptions = {}) {
	const { gain = 0.5, f = 58, decay = 1.1, verb = 0.35 } = o;
	(
		[
			[0, 1, -0.35],
			[0.006, 1.1, 0.05],
			[0.013, 1.22, 0.4]
		] as const
	).forEach(([dt, r, pan], i) =>
		drum(k, b, when + dt, {
			...o,
			gain: gain * 0.48,
			f: f * r,
			decay,
			pan,
			verb,
			slap: 0.45,
			seed: 40 + i
		})
	);
}

/** A roll from `t0` to `t1`: strokes accelerate through `grid` and crescendo. */
export function roll(
	k: Kit,
	b: Bus,
	t0: number,
	t1: number,
	o: Voice & {
		voice?: 'snare' | 'drum';
		gain?: number;
		from?: number;
		to?: number;
		pitch?: [number, number];
		grid?: number[];
		f?: number;
		verb?: number;
	} = {}
) {
	const {
		voice = 'snare',
		from = 0.3,
		to = 1,
		pitch = [1, 1.25],
		grid = [0.5, 0.25, 0.125, 0.0625],
		f = 90
	} = o;
	const span = t1 - t0;
	let t = t0;
	let i = 0;
	while (t < t1 - 1e-4) {
		const x = (t - t0) / span;
		const g = from + (to - from) * Math.pow(x, 1.4);
		const p = pitch[0] + (pitch[1] - pitch[0]) * x;
		const pan = i % 2 ? 0.18 : -0.18;
		const seed = 60 + i;
		if (voice === 'snare')
			snare(k, b, t, { ...o, gain: (o.gain ?? 0.24) * g, tone: 185 * p, pan, seed, decay: 0.14 });
		else drum(k, b, t, { ...o, gain: (o.gain ?? 0.3) * g, f: f * p, decay: 0.5, pan, seed });
		t += grid[Math.min(grid.length - 1, Math.floor(x * grid.length))];
		i++;
	}
}

// --- bass and synths --------------------------------------------------------------------

/** A clean sub: a sine with a quiet second harmonic so it reads on small speakers. */
export function subNote(
	k: Kit,
	b: Bus,
	when: number,
	f: number,
	dur: number,
	o: Voice & { gain?: number; a?: number; r?: number } = {}
) {
	const { gain = 0.3, a = 0.01, r = 0.3 } = o;
	const env = adsr(k, when, { peak: gain, a, d: 0.1, s: 1, off: when + dur, r });
	k.oscAt(b, 'sine', f, when, dur + r + 0.05, null).connect(env);
	k.oscAt(b, 'sine', f * 2, when, dur + r + 0.05, null)
		.connect(k.gain(0.12))
		.connect(env);
	env.connect(dest(k, b, o));
}

/**
 * Low brass "braam": detuned saw stacks scooping into pitch, through an opening low-pass and a
 * soft clipper (which also bounds the level however many notes are stacked).
 */
export function braam(
	k: Kit,
	b: Bus,
	when: number,
	notes: number[],
	o: Voice & {
		gain?: number;
		dur?: number;
		open?: [number, number, number];
		attack?: number;
		verb?: number;
		drive?: number;
		pan?: number;
		seed?: number;
	} = {}
) {
	const {
		gain = 0.4,
		dur = 2.6,
		open = [180, 2400, 650],
		attack = 0.03,
		verb = 0.35,
		drive = 1.6,
		pan = 0.25,
		seed = 3
	} = o;
	const r = rng(seed);
	const lp = k.filter('lowpass', open[0], 1.1);
	lp.frequency.setValueAtTime(open[0], when);
	lp.frequency.exponentialRampToValueAtTime(open[1], when + attack + 0.11);
	lp.frequency.exponentialRampToValueAtTime(open[2], when + dur);
	const env = k.gain(0);
	env.gain.setValueAtTime(0, when);
	env.gain.linearRampToValueAtTime(gain, when + attack);
	env.gain.exponentialRampToValueAtTime(gain * 0.55, when + attack + 0.6);
	env.gain.exponentialRampToValueAtTime(gain * 0.0008, when + dur);
	const pre = k.gain(1.9 / Math.sqrt(notes.length * 4));
	notes.forEach((f, i) => {
		const p = k.pan(((i % 2) * 2 - 1) * pan * (i / notes.length));
		for (const det of [-14, -5, 6, 15]) {
			const osc = k.oscAt(b, 'sawtooth', f, when, dur + 0.1, r);
			osc.detune.setValueAtTime(det - 45, when);
			osc.detune.linearRampToValueAtTime(det, when + 0.09);
			osc.connect(p);
		}
		p.connect(pre);
	});
	sat(k, pre.connect(lp), drive)
		.connect(env)
		.connect(dest(k, b, o));
	k.send(b, env, verb);
}

/** Shepard-style riser: octave-spaced voices gliding up under a bell-shaped spectral window. */
export function shepard(
	k: Kit,
	b: Bus,
	when: number,
	dur: number,
	o: Voice & { gain?: number; base?: number; voices?: number; octaves?: number } = {}
) {
	const { gain = 0.07, base = 73.42, voices = 5, octaves = 1.2 } = o;
	const out = k.gain(1);
	const lp = k.filter('lowpass', 2600, 0.5);
	for (let i = 0; i < voices; i++) {
		const osc = k.osc(b, 'sawtooth', base * 2 ** i, when, dur + 0.05);
		osc.frequency.setValueAtTime(base * 2 ** i, when);
		osc.frequency.exponentialRampToValueAtTime(base * 2 ** (i + octaves), when + dur);
		const curve = new Float32Array(64);
		for (let c = 0; c < 64; c++) {
			const tau = c / 63;
			const pos = (i + octaves * tau) / voices;
			const window = Math.exp(-((pos - 0.55) ** 2) / (2 * 0.2 ** 2));
			curve[c] = gain * window * (0.12 + 0.88 * tau ** 1.6) * (c === 63 ? 0 : 1);
		}
		const g = k.gain(0);
		g.gain.setValueCurveAtTime(curve, when, dur);
		osc
			.connect(g)
			.connect(k.pan(i % 2 ? 0.3 : -0.3))
			.connect(lp);
	}
	lp.connect(out).connect(dest(k, b, o));
	k.send(b, out, 0.4);
}

// --- the orchestra -------------------------------------------------------------------------

/**
 * Brass section: per note three detuned saws and a sub-octave square, a pitch scoop, a filter
 * that blooms with the attack and settles, breath, and late vibrato.
 */
export function brass(
	k: Kit,
	b: Bus,
	when: number,
	notes: number[],
	dur: number,
	o: Voice & {
		gain?: number;
		bright?: number;
		a?: number;
		r?: number;
		pan?: number;
		verb?: number;
		drive?: number;
		seed?: number;
		scoop?: number;
	} = {}
) {
	const {
		gain = 0.16,
		bright = 0.8,
		a = 0.045,
		r = 0.35,
		pan = 0.3,
		verb = 0.4,
		drive = 1.5,
		seed = 31,
		scoop = 35
	} = o;
	const rr = rng(seed);
	const total = dur + r + 0.08;
	const pre = k.gain(1.1 / Math.sqrt(notes.length));
	notes.forEach((f, i) => {
		const p = k.pan(notes.length > 1 ? (i / (notes.length - 1) - 0.5) * 2 * pan : 0);
		const lp = k.filter('lowpass', 280, 0.7);
		lp.frequency.setValueAtTime(280, when);
		lp.frequency.exponentialRampToValueAtTime(700 + 2600 * bright, when + a + 0.05);
		lp.frequency.exponentialRampToValueAtTime(600 + 1500 * bright, when + a + 0.45);
		const vd = k.gain(0);
		vd.gain.setValueAtTime(0, when + 0.3);
		vd.gain.linearRampToValueAtTime(6, when + 0.7);
		k.oscAt(b, 'sine', 5.1 + i * 0.11, when, total, null).connect(vd);
		for (const [type, ratio, det, lvl] of [
			['sawtooth', 1, -6, 0.33],
			['sawtooth', 1, 1, 0.33],
			['sawtooth', 1, 7, 0.33],
			['square', 0.5, 0, 0.16]
		] as const) {
			const osc = k.oscAt(b, type, f * ratio, when, total, rr);
			osc.detune.setValueAtTime(det - scoop, when);
			osc.detune.linearRampToValueAtTime(det, when + 0.07);
			vd.connect(osc.detune);
			osc.connect(k.gain(lvl)).connect(lp);
		}
		lp.connect(p).connect(pre);
	});
	k.noiseAt(b, 'pink', when, 0.15, seed)
		.connect(k.filter('bandpass', 1400, 0.9))
		.connect(k.ad(when, 0.05, 0.01, 0.08))
		.connect(pre);
	const env = adsr(k, when, { peak: gain, a, d: 0.3, s: 0.78, off: when + dur, r });
	sat(k, pre, drive)
		.connect(env)
		.connect(dest(k, b, o));
	k.send(b, env, verb);
}

/**
 * Strings: four detuned saws per note with their own slow vibrato, a soft low-pass, a dip
 * where violins get edgy, an optional tremolo and swell, and a chorus for the section's width.
 */
export function strings(
	k: Kit,
	b: Bus,
	when: number,
	notes: number[],
	dur: number,
	o: Voice & {
		gain?: number;
		a?: number;
		r?: number;
		cutoff?: number;
		hp?: number;
		trem?: number;
		verb?: number;
		/** A crescendo across the note: `[t, level]` pairs relative to `when`. */
		swell?: [number, number][] | null;
		seed?: number;
	} = {}
) {
	const {
		gain = 0.1,
		a = 0.25,
		r = 0.8,
		cutoff = 3200,
		hp = 140,
		trem = 0,
		verb = 0.45,
		swell: sw = null,
		seed = 41
	} = o;
	const rr = rng(seed);
	const total = dur + r + 0.1;
	const L = k.pan(-0.7);
	const R = k.pan(0.7);
	const sum = k.gain(1.2 / Math.sqrt(notes.length * 4));
	L.connect(sum);
	R.connect(sum);
	notes.forEach((f, i) => {
		const vd = k.gain(0);
		vd.gain.setValueAtTime(0, when + 0.2);
		vd.gain.linearRampToValueAtTime(7, when + 0.8);
		k.oscAt(b, 'sine', 4.9 + i * 0.23, when, total, rr).connect(vd);
		[-12, -4, 5, 13].forEach((det, vi) => {
			const osc = k.oscAt(b, 'sawtooth', f, when, total, rr);
			osc.detune.value = det;
			vd.connect(osc.detune);
			osc.connect((vi + i) % 2 ? L : R);
		});
	});
	const deharsh = k.filter('peaking', 2800, 0.9);
	deharsh.gain.value = -4;
	const env = adsr(k, when, { peak: gain, a, d: 0.4, s: 0.9, off: when + dur, r });
	let chain: AudioNode = sum
		.connect(k.filter('highpass', hp, 0.6))
		.connect(k.filter('lowpass', cutoff, 0.5))
		.connect(deharsh)
		.connect(env);
	if (sw) {
		const sg = k.gain(sw[0][1]);
		sg.gain.setValueAtTime(sw[0][1], when);
		for (const [t, v] of sw.slice(1)) sg.gain.linearRampToValueAtTime(v, when + t);
		chain = chain.connect(sg);
	}
	if (trem > 0) {
		const tg = k.gain(1 - trem / 2);
		k.oscAt(b, 'sine', 12, when, total, null)
			.connect(k.gain(trem / 2))
			.connect(tg.gain);
		chain = chain.connect(tg);
	}
	const out = chorus(k, b, chain, when, total, { mix: 0.4 });
	out.connect(dest(k, b, o));
	k.send(b, out, verb);
}

export interface SpiccatoStep {
	when: number;
	f: number;
	gain: number;
	pan?: number;
}

/** Spiccato strings: short detuned saws through one low-pass that opens across the phrase. */
export function spiccato(
	k: Kit,
	b: Bus,
	steps: SpiccatoStep[],
	o: Voice & { cutoff?: [number, number]; verb?: number; gain?: number } = {}
) {
	const { cutoff = [700, 3200], verb = 0.22, gain = 1 } = o;
	if (!steps.length) return;
	const t0 = steps[0].when;
	const t1 = steps[steps.length - 1].when + 0.2;
	const lp = k.filter('lowpass', cutoff[0], 1.1);
	lp.frequency.setValueAtTime(cutoff[0], t0);
	lp.frequency.exponentialRampToValueAtTime(cutoff[1], t1);
	const out = k.gain(gain);
	for (const s of steps) {
		const env = k.ad(s.when, s.gain, 0.004, 0.12);
		for (const det of [-7, 6]) {
			const osc = k.oscAt(b, 'sawtooth', s.f, s.when, 0.2, null);
			osc.detune.value = det;
			osc.connect(env);
		}
		env.connect(k.pan(s.pan ?? 0)).connect(lp);
	}
	lp.connect(out).connect(dest(k, b, o));
	k.send(b, out, verb);
}

/** Choir "aah": saws through a vowel's three formants, slow attack, vibrato and chorus. */
export function choir(
	k: Kit,
	b: Bus,
	when: number,
	notes: number[],
	dur: number,
	o: Voice & { gain?: number; a?: number; r?: number; verb?: number; seed?: number } = {}
) {
	const { gain = 0.08, a = 0.35, r = 1.0, verb = 0.6, seed = 51 } = o;
	const rr = rng(seed);
	const total = dur + r + 0.1;
	const src = k.gain(1 / Math.sqrt(notes.length * 3));
	notes.forEach((f, i) => {
		const vd = k.gain(0);
		vd.gain.setValueAtTime(0, when + 0.25);
		vd.gain.linearRampToValueAtTime(9, when + 0.9);
		k.oscAt(b, 'sine', 4.7 + i * 0.19, when, total, rr).connect(vd);
		for (const det of [-9, 0, 10]) {
			const osc = k.oscAt(b, 'sawtooth', f, when, total, rr);
			osc.detune.value = det;
			vd.connect(osc.detune);
			osc.connect(src);
		}
	});
	const formants = k.gain(1);
	for (const [f, q, g] of [
		[760, 5, 1],
		[1150, 7, 0.55],
		[2650, 9, 0.12]
	])
		src
			.connect(k.filter('bandpass', f, q))
			.connect(k.gain(g * 3.2))
			.connect(formants);
	const env = adsr(k, when, { peak: gain, a, d: 0.5, s: 0.9, off: when + dur, r });
	const out = chorus(
		k,
		b,
		formants.connect(k.filter('lowpass', 4200, 0.5)).connect(env),
		when,
		total,
		{ mix: 0.5 }
	);
	out.connect(dest(k, b, o));
	k.send(b, out, verb);
}

/** Glockenspiel: three bar partials, the upper ones dying first. */
export function glock(
	k: Kit,
	b: Bus,
	when: number,
	f: number,
	o: Voice & { gain?: number; decay?: number; pan?: number; verb?: number } = {}
) {
	const { gain = 0.05, decay = 1.8, pan = 0, verb = 0.5 } = o;
	const p = k.pan(pan);
	for (const [r, a, dk] of [
		[1, 1, 1],
		[2.756, 0.28, 0.35],
		[5.404, 0.1, 0.18]
	])
		k.oscAt(b, 'sine', f * r, when, decay * dk + 0.05, null)
			.connect(k.ad(when, gain * a, 0.0012, decay * dk))
			.connect(p);
	p.connect(dest(k, b, o));
	k.send(b, p, verb);
}
