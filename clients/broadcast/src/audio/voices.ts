// The house instruments: Couch TV's generated palette, every voice synthesized (no samples):
// a felt piano, a warm analog pad, struck glass, a bowed swell, a sub, a soft impact, airy
// swells and risers, a felted pulse and a shimmer, all through one mix with a long generated
// hall. Chosen from the approved idents' scores (morph's piano, pad, bowed, sub, impact and
// air; assemble's glass, pulse and inhale), so every screen scored with them sounds like the
// idents' family.
//
// A voice is `voice(h, when, ...)`: `h` is the piece's mix (`house`), `when` an absolute
// context time, and anything that varies (phases, drift, noise) comes from its `seed`, so a
// render is a pure function of the score. Pitches are note names (`A4`, `F#3`) or Hz.

import { rng } from '../core/rng';
import type { Bus } from './engine';
import type { Kit } from './kit';
import { hz } from './pitch';

export type Pitch = string | number;

const freq = (p: Pitch) => (typeof p === 'number' ? p : hz(p));

/** One piece's mix: where voices play. Made once per piece bus. */
export interface House {
	k: Kit;
	b: Bus;
	/** The dry mix, into the piece's music bus. A piece may ride or fade it as it likes. */
	out: GainNode;
	/**
	 * The mix's last gain, before the music bus: only `ringOut` automates it, so a clip's ending
	 * never overlaps a ride or a fade on `out` (the browser refuses a curve over other automation).
	 */
	end: GainNode;
	/** The hall's input (a voice's send). */
	hall: GainNode;
	/** The pads' bus: chords sum here, through `padCut` (a sweepable low-pass) and `padLevel`. */
	pad: GainNode;
	padCut: BiquadFilterNode[];
	padLevel: GainNode;
	/** The low end (sub, impacts, pulses): dry and centred. */
	low: GainNode;
}

const houses = new WeakMap<Bus, House>();

/** The piece's mix, made on first use. */
export function house(k: Kit, b: Bus): House {
	let h = houses.get(b);
	if (!h) houses.set(b, (h = makeHouse(k, b)));
	return h;
}

/**
 * Pins a node to two channels. A node's channel count otherwise follows its inputs, so when a
 * stereo source ends among mono ones the chain drops to mono mid-note: the filters' state and
 * the panners' law change at once, and that step is a click.
 */
function stereo<T extends AudioNode>(node: T): T {
	node.channelCount = 2;
	node.channelCountMode = 'explicit';
	node.channelInterpretation = 'speakers';
	return node;
}

const tanhCurves = new Map<number, Float32Array<ArrayBuffer>>();

/** tanh saturation, normalized so full scale stays full scale. */
function warm(k: Kit, drive: number): WaveShaperNode {
	let c = tanhCurves.get(drive);
	if (!c) {
		const n = 4096;
		c = new Float32Array(n);
		for (let i = 0; i < n; i++) {
			const x = (i / (n - 1)) * 2 - 1;
			c[i] = Math.tanh(x * drive) / Math.tanh(drive);
		}
		tanhCurves.set(drive, c);
	}
	const s = k.ctx.createWaveShaper();
	s.curve = c;
	s.oversample = '2x';
	return s;
}

const halls = new Map<number, Float32Array<ArrayBuffer>[]>();

/**
 * The house hall: long and smooth, no discrete reflections (pads and piano want a clean
 * bloom), a 22 ms pre-delay and a diffuse 60 ms onset, then a tail that decays per band (lows
 * longest, highs first), decorrelated between the channels; its end fades to zero, so the
 * convolution never stops on a step. It depends on the rate alone: made once per rate. It is
 * 3.6 s, its lows gone 60 dB by the end: Chromium's convolver gets far slower past about that
 * (a 5.2 s hall doubled a promo's sound render), and every clip with house voices has one.
 */
function hallBuffer(ctx: BaseAudioContext, seconds = 3.6): AudioBuffer {
	const sr = ctx.sampleRate;
	const len = Math.floor(sr * seconds);
	let data = halls.get(sr);
	if (!data) {
		data = [new Float32Array(len), new Float32Array(len)];
		const pre = Math.floor(sr * 0.022);
		const T60 = { low: 3.6, mid: 2.9, high: 1.2 };
		const k = (t60: number) => 6.91 / t60;
		const aLow = 1 - Math.exp((-2 * Math.PI * 380) / sr);
		const aMid = 1 - Math.exp((-2 * Math.PI * 3600) / sr);
		data.forEach((d, ch) => {
			const r = rng(907 + ch * 331);
			let lo = 0;
			let mi = 0;
			for (let i = pre; i < len; i++) {
				const t = (i - pre) / sr;
				const w = r() * 2 - 1;
				lo += aLow * (w - lo);
				mi += aMid * (w - mi);
				const onset = 1 - Math.exp(-t / 0.06);
				const end = Math.min(1, (len - i) / (sr * 0.3));
				d[i] =
					(lo * 2.2 * Math.exp(-k(T60.low) * t) +
						(mi - lo) * Math.exp(-k(T60.mid) * t) +
						(w - mi) * 0.35 * Math.exp(-k(T60.high) * t)) *
					onset *
					end;
			}
		});
		halls.set(sr, data);
	}
	const buf = ctx.createBuffer(2, len, sr);
	data.forEach((d, c) => buf.copyToChannel(d, c));
	return buf;
}

function makeHouse(k: Kit, b: Bus): House {
	const ctx = k.ctx;
	// the dry chain: rumble out, a dip where pads go muddy, a breath of air on top
	const out = stereo(k.gain(1));
	const rumble = k.filter('highpass', 26, 0.7);
	const dip = k.filter('peaking', 300, 0.8);
	dip.gain.value = -2.5;
	const air = k.filter('highshelf', 7000, 0.7);
	air.gain.value = 2;
	const end = stereo(k.gain(1));
	out.connect(rumble).connect(dip).connect(air).connect(end).connect(b.music);

	// the hall, kept out of the mud and the fizz
	const hall = stereo(k.gain(1));
	const conv = ctx.createConvolver();
	conv.buffer = hallBuffer(ctx);
	const hHp = k.filter('highpass', 180, 0.6);
	const hDip = k.filter('peaking', 420, 0.9);
	hDip.gain.value = -3;
	const hLp = k.filter('lowpass', 9000, 0.6);
	hall.connect(hHp).connect(hDip).connect(hLp).connect(conv).connect(k.gain(0.8)).connect(out);

	// the pads' bus: a sweepable two-pole pair, warmth, a stereo chorus, a send to the hall
	const pad = stereo(k.gain(1));
	const padCut = [k.filter('lowpass', 6000, 0.55), k.filter('lowpass', 6000, 0.55)];
	const padHp = k.filter('highpass', 140, 0.6);
	const padDip = k.filter('peaking', 310, 0.8);
	padDip.gain.value = -3.5;
	const padLevel = k.gain(1);
	pad
		.connect(padCut[0])
		.connect(padCut[1])
		.connect(padHp)
		.connect(padDip)
		.connect(warm(k, 1.25))
		.connect(padLevel);
	padLevel.connect(k.gain(0.7)).connect(out);
	// the chorus: a mono copy through two slowly swaying delays, one per side
	const merge = ctx.createChannelMerger(2);
	const t0 = ctx.currentTime;
	(
		[
			[0.0123, 0.23, 0],
			[0.0171, 0.31, 1]
		] as const
	).forEach(([base, rate, side]) => {
		const dl = ctx.createDelay(0.05);
		dl.delayTime.value = base;
		const lfo = ctx.createOscillator();
		lfo.frequency.value = rate;
		// part of a cycle early, so the two sides never sway together
		lfo.start(Math.max(0, t0 - side / (2 * rate)));
		lfo.stop(t0 + 40);
		k.track(b, lfo);
		lfo.connect(k.gain(0.0024)).connect(dl.delayTime);
		const mono = k.gain(1);
		mono.channelCount = 1;
		mono.channelCountMode = 'explicit';
		mono.channelInterpretation = 'speakers';
		padLevel.connect(mono).connect(dl).connect(k.gain(0.55)).connect(merge, 0, side);
	});
	merge.connect(out);
	padLevel.connect(k.gain(0.42)).connect(hall);

	const low = stereo(k.gain(1));
	low.connect(out);
	return { k, b, out, end, hall, pad, padCut, padLevel, low };
}

// --- shared bits -------------------------------------------------------------------------------

const waves = new WeakMap<BaseAudioContext, Record<string, PeriodicWave>>();

/** A periodic wave from harmonic amplitudes, made once per context. */
function wave(k: Kit, name: string, partials: () => Float32Array): PeriodicWave {
	let all = waves.get(k.ctx);
	if (!all) waves.set(k.ctx, (all = {}));
	if (!all[name]) {
		const imag = partials();
		all[name] = k.ctx.createPeriodicWave(new Float32Array(imag.length), imag);
	}
	return all[name];
}

/** A saw with its top rolled off a little: warmer than the bare saw before any filter. */
const softSaw = (k: Kit) =>
	wave(k, 'softSaw', () => {
		const imag = new Float32Array(64);
		for (let h = 1; h < 64; h++) imag[h] = (1 / h ** 1.08) * Math.exp(-h / 48);
		return imag;
	});

/** An oscillator started part of a cycle early (from `r`), so stacked voices never start in phase. */
function osc(
	h: House,
	type: OscillatorType | PeriodicWave,
	f: number,
	when: number,
	end: number,
	r: () => number
) {
	const o = h.k.ctx.createOscillator();
	if (type instanceof PeriodicWave) o.setPeriodicWave(type);
	else o.type = type;
	o.frequency.value = f;
	o.start(Math.max(0, when - r() / f));
	o.stop(end);
	return h.k.track(h.b, o);
}

/** Seeded noise running from `when` until `end` (a source that stops while its chain still sounds leaves a step). */
const noise = (
	h: House,
	kind: 'white' | 'pink',
	when: number,
	end: number,
	seed: number,
	rate = 1
) => h.k.noiseAt(h.b, kind, when, end - when, seed, rate);

/** A raised-cosine swell from 0 to `peak`, for setValueCurveAtTime. */
function swell(peak: number, n = 64): Float32Array<ArrayBuffer> {
	const c = new Float32Array(n);
	for (let i = 0; i < n; i++) c[i] = peak * (0.5 - 0.5 * Math.cos((Math.PI * i) / (n - 1)));
	return c;
}

/** Hold: a raised-cosine attack to `peak`, held until `off`, then an exponential release. */
function hold(
	h: House,
	when: number,
	attack: number,
	off: number,
	release: number,
	peak: number
): GainNode {
	const g = h.k.gain(0);
	const a = Math.max(0.005, attack);
	// an event inside the curve's span throws: the hold starts just after it
	const at = Math.max(off, when + a + 1e-4);
	g.gain.setValueCurveAtTime(swell(peak), when, a);
	g.gain.setValueAtTime(peak, at);
	g.gain.setTargetAtTime(0, at, release / 3.5);
	return g;
}

/** A gain through points `[t, v]` (seconds from `when`), linear between them. */
function points(h: House, when: number, pts: [number, number][]): GainNode {
	const g = h.k.gain(0);
	g.gain.setValueAtTime(pts[0][1], when + pts[0][0]);
	for (const [t, v] of pts.slice(1)) g.gain.linearRampToValueAtTime(v, when + t);
	return g;
}

export interface Place {
	/** Linear gain on the voice's own level. */
	gain?: number;
	pan?: number;
	/** How much goes to the hall (0..1). */
	verb?: number;
	seed?: number;
}

/** Into the mix through a panner, with a send to the hall. */
function place(h: House, node: AudioNode, pan: number, verb: number, dest: AudioNode = h.out) {
	const p = h.k.pan(pan);
	node.connect(p).connect(dest);
	if (verb > 0) p.connect(h.k.gain(verb)).connect(h.hall);
	return p;
}

// --- the felt piano ----------------------------------------------------------------------------

export interface PianoOptions extends Place {
	/** 0..1: louder is brighter and harder. */
	vel?: number;
	/** Seconds until the damper; the note rings until then. */
	dur?: number;
}

/**
 * A felt piano note: up to 14 inharmonic partials (stiff-string stretch), each a pair of
 * slightly detuned strings beating against each other, decaying in two stages (the prompt
 * sound, then the long aftersound), higher partials first; a soft felt hammer under it; the
 * damper fades it out.
 */
export function piano(h: House, when: number, note: Pitch, o: PianoOptions = {}) {
	const { vel = 0.6, dur = 4, pan = 0, verb = 0.5, gain = 1, seed = 1 } = o;
	const k = h.k;
	const f0 = freq(note);
	const r = rng(seed);
	const B = 0.00018 * Math.max(0.6, Math.min(2.2, f0 / 260));
	const end = when + dur + 1.2;
	// the hammer's noise is stereo and the strings are not: one stereo stream throughout
	const sum = stereo(k.gain(1));
	const tone = k.filter('lowpass', Math.min(16000, f0 * (5 + 9 * vel) + 1200), 0.5);
	sum.connect(tone);
	const level = 0.075 * gain * (0.35 + 0.65 * vel);
	// felt-muted, low notes ring longest
	const sustain = Math.max(0.8, 2.6 - Math.log2(f0 / 110) * 0.55);
	const top = Math.min(14, Math.floor(7000 / f0));
	for (let n = 1; n <= top; n++) {
		const fn = n * f0 * Math.sqrt(1 + B * n * n);
		// a soft hammer rolls the spectrum off; harder hits keep more of the top
		const a = (1 / n ** 1.2) * Math.exp(-(n - 1) / (1.6 + 4.2 * vel));
		if (a < 0.004) continue;
		const slow = sustain / (1 + 0.32 * (n - 1));
		const fast = Math.min(slow, 0.26 / (1 + 0.1 * n));
		const strings = n <= 7 ? 2 : 1;
		for (let s = 0; s < strings; s++) {
			const o2 = osc(h, 'sine', fn, when, end, r);
			if (strings > 1) o2.detune.value = (s ? 1 : -1) * (0.35 + r() * 0.7);
			const g = k.gain(0);
			const peak = (level * a) / strings;
			const hit = when + 0.004 + 0.002 * n;
			g.gain.setValueAtTime(0, when);
			g.gain.linearRampToValueAtTime(peak, hit);
			g.gain.setTargetAtTime(peak * 0.32, hit, fast);
			g.gain.setTargetAtTime(0, hit + fast * 2.2, slow);
			g.gain.setTargetAtTime(0, Math.max(hit + fast * 2.2 + 0.01, when + dur), 0.12);
			o2.connect(g).connect(sum);
		}
	}
	// the hammer: a breath of low-passed noise
	const hn = noise(h, 'pink', when, end, seed + 3);
	const hg = k.gain(0);
	hg.gain.setValueAtTime(0, when);
	hg.gain.linearRampToValueAtTime(level * 0.16 * (0.4 + vel), when + 0.002);
	hg.gain.setTargetAtTime(0, when + 0.003, 0.012);
	hn.connect(k.filter('lowpass', 900 + 2600 * vel, 0.7))
		.connect(k.filter('highpass', 120, 0.7))
		.connect(hg)
		.connect(sum);
	place(h, tone, pan, verb);
}

// --- the warm analog pad -----------------------------------------------------------------------

export interface PadOptions {
	attack?: number;
	release?: number;
	/** Linear level (1 is the house's pad level). */
	level?: number;
	/** Cents between the outer voices of a note. */
	spread?: number;
	/** A low-pass of the chord's own, from the first value to the second over the note (Hz). */
	cutoff?: [number, number];
	/** End on a 25 ms fade instead of the release, into a breath before a hit. */
	cut?: boolean;
	seed?: number;
}

/**
 * A pad chord into the pads' bus: six detuned soft saws per note, spread across the stereo
 * field, each drifting slowly on its own; it swells in, holds `dur` and fades by its release.
 */
export function pad(h: House, when: number, chord: Pitch[], dur: number, o: PadOptions = {}) {
	const { attack = 1, release = 1.4, level = 1, spread = 22, seed = 7 } = o;
	const k = h.k;
	const r = rng(seed);
	const fs = chord.map(freq);
	const off = when + attack + dur;
	const end = off + (o.cut ? 0.05 : release * 2.4) + 0.2;
	const peak = (0.11 * level) / Math.sqrt(fs.length);
	let env: GainNode;
	if (o.cut) {
		env = k.gain(0);
		env.gain.setValueCurveAtTime(swell(peak), when, attack);
		env.gain.setValueAtTime(peak, Math.max(when + attack + 1e-4, off - 0.025));
		env.gain.linearRampToValueAtTime(0, Math.max(when + attack + 2e-4, off));
	} else env = hold(h, when, attack, off, release, peak);
	let into: AudioNode = env;
	if (o.cutoff) {
		const lp = k.filter('lowpass', o.cutoff[0], 0.5);
		lp.frequency.setValueAtTime(o.cutoff[0], when);
		lp.frequency.exponentialRampToValueAtTime(o.cutoff[1], off);
		lp.connect(env);
		into = lp;
	}
	env.connect(h.pad);
	const w = softSaw(k);
	fs.forEach((f, i) => {
		for (let v = 0; v < 6; v++) {
			const o2 = osc(h, w, f, when, end, r);
			o2.detune.value = ((v / 5) * 2 - 1) * (spread / 2) + (r() - 0.5) * 3;
			// a slow, private drift: the voices never lock
			const lfo = osc(h, 'sine', 0.07 + r() * 0.2, when, end, r);
			lfo.connect(k.gain(2.5 + r() * 2.5)).connect(o2.detune);
			const side = ((v % 2 ? 1 : -1) * (0.25 + 0.6 * (v / 6)) + (i % 2 ? 0.08 : -0.08)) * 0.95;
			o2.connect(k.gain(1 / 6))
				.connect(k.pan(Math.max(-1, Math.min(1, side))))
				.connect(into);
		}
	});
}

/** Sweep the pads' bus low-pass through `[t, Hz]` points (seconds from `when`). */
export function padSweep(h: House, when: number, pts: [number, number][]) {
	for (const f of h.padCut) {
		f.frequency.setValueAtTime(pts[0][1], when + pts[0][0]);
		for (const [t, v] of pts.slice(1)) f.frequency.exponentialRampToValueAtTime(v, when + t);
	}
}

// --- struck glass and the bowed swell ----------------------------------------------------------

export interface GlassOptions extends Place {
	/** Seconds the fundamental takes to die away. */
	decay?: number;
	/** The upper partials' strength (1 is the house glass). */
	tone?: number;
	/** The strike: 4 ms is struck, longer blooms in like a halo. */
	attack?: number;
}

/**
 * Struck glass: harmonic partials that each decay at their own rate (the highs first), a
 * second voice a few cents up on the other side, a quiet sine an octave below for warmth and a
 * soft felt tap. Bell-like without FM's inharmonic edge.
 */
export function glass(h: House, when: number, note: Pitch, o: GlassOptions = {}) {
	const { decay = 2.6, tone = 1, attack = 0.004, pan = 0, verb = 0.55, gain = 1, seed = 17 } = o;
	const k = h.k;
	const f = freq(note);
	const r = rng(seed);
	const end = when + decay * 1.9 + 0.3;
	const sum = k.gain(1);
	const partials: [number, number, number][] = [
		[1, 1, 1],
		[2, 0.3 * tone, 0.5],
		[3, 0.1 * tone, 0.28],
		[4, 0.045 * tone, 0.18],
		[6, 0.016 * tone, 0.1]
	];
	for (const [side, cents] of [
		[-0.35, -3],
		[0.35, 4]
	]) {
		const p = k.pan(side);
		p.connect(sum);
		for (const [n, a, dk] of partials) {
			if (f * n > 14000) continue;
			const o2 = osc(h, 'sine', f * n, when, end, r);
			o2.detune.value = cents;
			const g = k.gain(0);
			g.gain.setValueAtTime(0, when);
			g.gain.linearRampToValueAtTime(a * 0.5, when + attack);
			g.gain.setTargetAtTime(0, when + attack, (decay * dk) / 3.2);
			o2.connect(g).connect(p);
		}
	}
	if (f > 300) {
		const low = osc(h, 'sine', f / 2, when, end, r);
		const lg = k.gain(0);
		lg.gain.setValueAtTime(0, when);
		lg.gain.linearRampToValueAtTime(0.16, when + Math.max(0.012, attack));
		lg.gain.setTargetAtTime(0, when + Math.max(0.012, attack), decay / 2.6);
		low.connect(lg).connect(sum);
	}
	// the felt: a soft, dark tap (none when it blooms)
	if (attack < 0.02) {
		const n = noise(h, 'pink', when, end, seed);
		n.connect(k.filter('lowpass', Math.min(6000, f * 3), 0.7))
			.connect(k.ad(when, 0.05, 0.001, 0.02))
			.connect(sum);
	}
	const soft = k.filter('lowpass', 7000, 0.5);
	const level = k.gain(0.16 * gain);
	sum.connect(soft).connect(level);
	place(h, level, pan, verb);
}

export interface BowedOptions extends Place {
	attack?: number;
	release?: number;
}

/** A bowed swell: three detuned soft saws, filtered dark, a late vibrato; it sustains a melody note. */
export function bowed(h: House, when: number, note: Pitch, dur: number, o: BowedOptions = {}) {
	const { gain = 1, attack = 0.28, release = 0.9, pan = 0, verb = 0.55, seed = 31 } = o;
	const k = h.k;
	const f0 = freq(note);
	const r = rng(seed);
	const end = when + attack + dur + release * 2.4 + 0.2;
	const lp = k.filter('lowpass', f0 * 3.2, 0.6);
	lp.frequency.setValueAtTime(f0 * 1.6, when);
	lp.frequency.linearRampToValueAtTime(f0 * 3.6, when + attack + 0.2);
	// a second pole pair: a string's top is smooth, not buzzy
	const lp2 = k.filter('lowpass', Math.min(12000, f0 * 5), 0.6);
	const env = hold(h, when, attack, when + attack + dur, release, 0.03 * gain);
	const w = softSaw(k);
	for (let v = 0; v < 3; v++) {
		const o2 = osc(h, w, f0, when, end, r);
		o2.detune.value = (v - 1) * 7 + (r() - 0.5) * 2;
		const vib = osc(h, 'sine', 4.8 + r() * 0.6, when, end, r);
		const depth = k.gain(0);
		depth.gain.setValueAtTime(0, when);
		depth.gain.linearRampToValueAtTime(0, when + attack);
		depth.gain.linearRampToValueAtTime(7, when + attack + 0.5);
		vib.connect(depth).connect(o2.detune);
		o2.connect(k.pan((v - 1) * 0.5)).connect(lp);
	}
	lp.connect(lp2).connect(env);
	place(h, env, pan, verb);
}

// --- the low end -------------------------------------------------------------------------------

export interface SubOptions {
	gain?: number;
	attack?: number;
	release?: number;
	/** Glide in from this pitch over 0.35 s. */
	glideFrom?: Pitch;
}

/** A sub note: a sine with a touch of its octave, softly saturated so small speakers find it. */
export function sub(h: House, when: number, note: Pitch, dur: number, o: SubOptions = {}) {
	const { gain = 1, attack = 0.5, release = 0.8 } = o;
	const k = h.k;
	const f = freq(note);
	const end = when + attack + dur + release * 2.4 + 0.3;
	const flat = () => 0;
	const o1 = osc(h, 'sine', f, when, end, flat);
	const o2 = osc(h, 'sine', f * 2, when, end, () => 0.25);
	if (o.glideFrom) {
		const g0 = freq(o.glideFrom);
		o1.frequency.setValueAtTime(g0, when);
		o1.frequency.exponentialRampToValueAtTime(f, when + 0.35);
		o2.frequency.setValueAtTime(g0 * 2, when);
		o2.frequency.exponentialRampToValueAtTime(f * 2, when + 0.35);
	}
	const mix = k.gain(1);
	o1.connect(mix);
	o2.connect(k.gain(0.1)).connect(mix);
	const env = hold(h, when, attack, when + attack + dur, release, 0.058 * gain);
	mix.connect(warm(k, 1.4)).connect(env).connect(h.low);
}

/**
 * The landing: a sub drop to the root, a round body, a short soft crack of noise into the hall
 * and a low rumble: deep and wide, not a bang.
 */
export function impact(
	h: House,
	when: number,
	o: { gain?: number; root?: Pitch; seed?: number } = {}
) {
	const { gain = 1, seed = 4242 } = o;
	const k = h.k;
	const to = freq(o.root ?? 'D1');
	const flat = () => 0;
	const s = osc(h, 'sine', to, when, when + 4.4, flat);
	s.frequency.setValueAtTime(to * 3.2, when);
	s.frequency.exponentialRampToValueAtTime(to * 1.25, when + 0.09);
	s.frequency.exponentialRampToValueAtTime(to, when + 0.6);
	const sg = k.gain(0);
	sg.gain.setValueAtTime(0, when);
	sg.gain.linearRampToValueAtTime(0.45 * gain, when + 0.004);
	sg.gain.setTargetAtTime(0.14 * gain, when + 0.02, 0.22);
	sg.gain.setTargetAtTime(0, when + 0.45, 0.55);
	s.connect(warm(k, 2))
		.connect(k.filter('lowpass', 220, 0.6))
		.connect(sg)
		.connect(h.low);
	// the body
	const bo = osc(h, 'sine', 70, when, when + 0.8, flat);
	bo.frequency.setValueAtTime(150, when);
	bo.frequency.exponentialRampToValueAtTime(64, when + 0.1);
	const bg = k.gain(0);
	bg.gain.setValueAtTime(0, when);
	bg.gain.linearRampToValueAtTime(0.22 * gain, when + 0.003);
	bg.gain.setTargetAtTime(0, when + 0.01, 0.09);
	bo.connect(warm(k, 2.4)).connect(bg);
	place(h, bg, 0, 0.25, h.low);
	// the crack: brief, dark noise, mostly into the hall
	const n = noise(h, 'pink', when, when + 0.6, seed);
	const nlp = k.filter('lowpass', 5200, 0.6);
	nlp.frequency.setValueAtTime(5200, when);
	nlp.frequency.exponentialRampToValueAtTime(900, when + 0.25);
	const ng = k.gain(0);
	ng.gain.setValueAtTime(0, when);
	ng.gain.linearRampToValueAtTime(0.16 * gain, when + 0.0015);
	ng.gain.setTargetAtTime(0, when + 0.003, 0.03);
	n.connect(k.filter('highpass', 500, 0.7))
		.connect(nlp)
		.connect(ng);
	place(h, ng, 0, 1.2);
	// the rumble the hall carries away
	const ru = noise(h, 'pink', when, when + 4.4, seed + 1, 0.5);
	const rg = k.gain(0);
	rg.gain.setValueAtTime(0, when);
	rg.gain.linearRampToValueAtTime(0.1 * gain, when + 0.02);
	rg.gain.setTargetAtTime(0, when + 0.05, 0.7);
	ru.connect(k.filter('lowpass', 140, 0.7))
		.connect(rg)
		.connect(h.low);
}

/**
 * A soft pulse: a sine with a soft octave and a whisper of fifth, a dark felt touch and a quick
 * swell rather than a click, so it reads as a heartbeat under the pads, never a pluck. `open`
 * lets a little more of the upper partials through (0..1).
 */
export function pulse(
	h: House,
	when: number,
	note: Pitch,
	o: Place & { open?: number; decay?: number } = {}
) {
	const { open = 0.3, decay = 0.3, pan = 0, verb = 0.22, gain = 1, seed = 23 } = o;
	const k = h.k;
	const f = freq(note);
	const r = rng(seed);
	const end = when + decay * 2 + 0.2;
	const sum = k.gain(1);
	for (const [n, a] of [
		[1, 1],
		[2, 0.22 + 0.25 * open],
		[3, 0.05 + 0.1 * open]
	]) {
		const o2 = osc(h, 'sine', f * n, when, end, r);
		const g = k.gain(0);
		g.gain.setValueAtTime(0, when);
		g.gain.linearRampToValueAtTime(a, when + 0.012);
		g.gain.setTargetAtTime(0, when + 0.012, (decay / 3) * (n === 1 ? 1 : 0.6));
		o2.connect(g).connect(sum);
	}
	noise(h, 'pink', when, end, seed)
		.connect(k.filter('lowpass', 900 + 900 * open, 0.7))
		.connect(k.ad(when, 0.08, 0.003, 0.03))
		.connect(sum);
	const level = k.gain(0.11 * gain);
	sum.connect(warm(k, 1.3)).connect(level);
	place(h, level, pan, verb, h.low);
}

// --- air ---------------------------------------------------------------------------------------

export interface AirOptions extends Place {
	/** Seconds after `when` where it is loudest. */
	peakAt?: number;
	/** The band it opens through, Hz. */
	band?: [number, number];
	/** Pans from the first value to the second over its length. */
	pans?: [number, number];
}

/** An airy swell: pink noise through a soft band that opens and closes, peaking at `peakAt`. */
export function air(h: House, when: number, dur: number, o: AirOptions = {}) {
	const {
		peakAt = dur * 0.6,
		band = [300, 1800],
		pans = [0, 0],
		verb = 0.5,
		gain = 1,
		seed = 61
	} = o;
	const k = h.k;
	const n = noise(h, 'pink', when, when + dur + 0.1, seed);
	const bp = k.filter('bandpass', band[0], 0.8);
	bp.frequency.setValueAtTime(band[0], when);
	bp.frequency.exponentialRampToValueAtTime(band[1], when + peakAt);
	bp.frequency.exponentialRampToValueAtTime(band[0] * 1.2, when + dur);
	const g = points(h, when, [
		[0, 0],
		[peakAt * 0.5, 0.06 * gain],
		[peakAt, 0.22 * gain],
		[peakAt + (dur - peakAt) * 0.35, 0.07 * gain],
		[dur, 0]
	]);
	const p = k.pan(pans[0]);
	p.pan.setValueAtTime(pans[0], when);
	p.pan.linearRampToValueAtTime(pans[1], when + dur);
	n.connect(bp)
		.connect(k.filter('lowpass', 5000, 0.5))
		.connect(g)
		.connect(p)
		.connect(h.out);
	if (verb > 0) p.connect(k.gain(verb)).connect(h.hall);
}

/**
 * A rise into a hit at `at`: noise climbing in pitch and level, accelerating, cut a `breath`
 * before `at` so the hit lands on silence.
 */
export function inhale(h: House, at: number, dur: number, o: Place & { breath?: number } = {}) {
	const { breath = 0.045, gain = 1, pan = 0, verb = 0.2, seed = 43 } = o;
	const k = h.k;
	const when = at - dur;
	const stop = at - breath;
	const n = noise(h, 'pink', when, at + 0.05, seed);
	const bp = k.filter('bandpass', 500, 0.7);
	bp.frequency.setValueAtTime(500, when);
	bp.frequency.exponentialRampToValueAtTime(5200, stop);
	const g = k.gain(0);
	g.gain.setValueAtTime(0.0001, when);
	g.gain.exponentialRampToValueAtTime(0.16 * gain, stop - 0.01);
	g.gain.linearRampToValueAtTime(0, stop);
	n.connect(bp).connect(g);
	place(h, g, pan, verb);
}

/**
 * A reversed swell into `at`: a chord's partials rising from nothing, the upper ones later, the
 * way a reversed piano or hall sounds; cut a breath before `at`.
 */
export function riser(
	h: House,
	at: number,
	chord: Pitch[],
	dur: number,
	o: Place & { breath?: number } = {}
) {
	const { gain = 1, breath = 0.035, seed = 71 } = o;
	const k = h.k;
	const r = rng(seed);
	const when = at - dur;
	const stop = at - breath;
	const fs = chord.map(freq);
	const sum = k.gain(1);
	const lp = k.filter('lowpass', 500, 0.6);
	lp.frequency.setValueAtTime(400, when);
	lp.frequency.exponentialRampToValueAtTime(5200, stop);
	fs.forEach((f, i) => {
		for (const [n, a] of [
			[1, 1],
			[2, 0.4],
			[3, 0.16],
			[4, 0.07]
		] as const) {
			const o2 = osc(h, 'sine', f * n, when, stop + 0.02, r);
			o2.detune.value = (r() - 0.5) * 8;
			const g = k.gain(0);
			const peak = (0.045 * gain * a) / Math.sqrt(fs.length);
			// an exponential rise, the upper partials arriving last
			g.gain.setValueAtTime(peak * 0.0008, when);
			g.gain.exponentialRampToValueAtTime(peak * 0.02, when + dur * (0.45 + 0.08 * n));
			g.gain.exponentialRampToValueAtTime(peak, stop);
			g.gain.linearRampToValueAtTime(0, stop + 0.012);
			o2.connect(g)
				.connect(k.pan((i % 2 ? 0.35 : -0.35) * (n > 1 ? -1 : 1)))
				.connect(sum);
		}
	});
	sum.connect(lp);
	place(h, lp, 0, 0.35);
	// its breath: noise rising with the chord
	const nz = noise(h, 'pink', when, stop + 0.05, seed + 5);
	const nbp = k.filter('bandpass', 600, 0.7);
	nbp.frequency.setValueAtTime(600, when);
	nbp.frequency.exponentialRampToValueAtTime(4200, stop);
	const ng = k.gain(0);
	ng.gain.setValueAtTime(0.0001, when);
	ng.gain.exponentialRampToValueAtTime(0.09 * gain, stop);
	ng.gain.linearRampToValueAtTime(0, stop + 0.01);
	nz.connect(nbp).connect(ng);
	place(h, ng, 0, 0.3);
}

/**
 * A shimmer: high, soft voices (a sine with a few gentle harmonics) slowly beating against each
 * other, high-passed and mostly in the hall: the light over an arrival.
 */
export function shimmer(
	h: House,
	when: number,
	chord: Pitch[],
	dur: number,
	o: { gain?: number; attack?: number; release?: number; seed?: number } = {}
) {
	const { gain = 1, attack = 0.4, release = 1.6, seed = 81 } = o;
	const k = h.k;
	const r = rng(seed);
	const w = wave(k, 'shimmer', () => Float32Array.from([0, 1, 0.42, 0.18, 0.08, 0.035, 0.015]));
	const fs = chord.map(freq);
	const end = when + attack + dur + release * 2.4 + 0.3;
	const hp = k.filter('highpass', 900, 0.6);
	const env = hold(
		h,
		when,
		attack,
		when + attack + dur,
		release,
		(0.03 * gain) / Math.sqrt(fs.length)
	);
	fs.forEach((f, i) => {
		for (let v = 0; v < 4; v++) {
			const o2 = osc(h, w, f, when, end, r);
			o2.detune.value = (v - 1.5) * 5 + (r() - 0.5) * 2;
			const side = ((v % 2 ? 1 : -1) * (0.3 + 0.5 * (v / 4)) + (i % 2 ? 0.1 : -0.1)) * 0.9;
			o2.connect(k.gain(0.25)).connect(k.pan(side)).connect(hp);
		}
	});
	hp.connect(env);
	env.connect(h.out);
	env.connect(k.gain(0.9)).connect(h.hall);
}

/**
 * Ring a piece out: from `at` the whole mix, the hall's tail with it, dies away by `depth` dB
 * at a steady rate (a decay, the way a struck chord falls) and closes to silence over the last
 * `close` seconds, `dur` in all, so a clip's sound is over before its last frame without a
 * step anywhere. Once per piece: it owns `end`, which multiplies whatever `out` is doing.
 */
export function ringOut(h: House, at: number, dur: number, { depth = 42, close = 0.2 } = {}) {
	const n = 256;
	const curve = new Float32Array(n);
	const shut = Math.min(1, close / dur);
	for (let i = 0; i < n; i++) {
		const x = i / (n - 1);
		const tail = x > 1 - shut ? 0.5 + 0.5 * Math.cos((Math.PI * (x - (1 - shut))) / shut) : 1;
		curve[i] = 10 ** ((-depth * x) / 20) * tail;
	}
	h.end.gain.setValueCurveAtTime(curve, at, dur);
}
