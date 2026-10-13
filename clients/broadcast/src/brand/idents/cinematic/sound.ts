// The score's instruments, all synthesized: a mix bus (gentle EQ and tape warmth), a long hall
// from a generated impulse response, a stereo chorus and a ping-pong delay; analog pads (detuned
// saws through a slowly moving filter), a felt piano rendered additively into buffers, a sub, an
// impact, swells and air. Every voice takes an absolute context time; noise and phases come from
// seeds, so a render is a pure function of the score.

import type { Bus } from '../../../audio/engine';
import type { Kit } from '../../../audio/kit';
import { rng } from '../../../core/rng';

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

/** A note name (A4, F#3) in Hz. */
export function hz(n: string): number {
	const m = /^([A-G][b#]?)(-?\d)$/.exec(n);
	if (!m) throw new Error(`bad note ${n}`);
	return 440 * 2 ** ((PC[m[1]] + 12 * (Number(m[2]) + 1) - 69) / 12);
}

export interface Mix {
	/** Dry music in (EQ, warmth, then the piece's music bus). */
	dry: GainNode;
	/** Into the hall. */
	hall: GainNode;
	/** Into the ping-pong delay (which feeds the hall too). */
	echo: GainNode;
	/** The pads' chorus, into the dry bus. */
	pads: GainNode;
}

const cache = new WeakMap<BaseAudioContext, Map<string, AudioBuffer>>();

function cached(ctx: BaseAudioContext, key: string, make: () => AudioBuffer): AudioBuffer {
	let m = cache.get(ctx);
	if (!m) cache.set(ctx, (m = new Map()));
	let b = m.get(key);
	if (!b) m.set(key, (b = make()));
	return b;
}

function tanhCurve(drive: number) {
	const n = 4096;
	const c = new Float32Array(n);
	for (let i = 0; i < n; i++) {
		const x = (i / (n - 1)) * 2 - 1;
		c[i] = Math.tanh(x * drive) / Math.tanh(drive);
	}
	return c;
}

/**
 * The hall: six seconds of decorrelated noise in four bands, each decaying at its own rate (the
 * lows ring longest, the air dies first), blooming in over 20 ms rather than starting on a click.
 */
function hallImpulse(ctx: BaseAudioContext): AudioBuffer {
	return cached(ctx, 'hall', () => {
		const sr = ctx.sampleRate;
		const secs = 6;
		const n = Math.floor(sr * secs);
		const buf = ctx.createBuffer(2, n, sr);
		const a = (f: number) => 1 - Math.exp((-2 * Math.PI * f) / sr);
		const a1 = a(320);
		const a2 = a(1500);
		const a3 = a(4800);
		const k = (t60: number) => 6.91 / t60;
		const [k1, k2, k3, k4] = [k(5.2), k(4.3), k(2.9), k(1.4)];
		for (let ch = 0; ch < 2; ch++) {
			const r = rng(7001 + ch * 977);
			const d = buf.getChannelData(ch);
			let l1 = 0;
			let l2 = 0;
			let l3 = 0;
			for (let i = 0; i < n; i++) {
				const t = i / sr;
				const w = r() * 2 - 1;
				l1 += a1 * (w - l1);
				l2 += a2 * (w - l2);
				l3 += a3 * (w - l3);
				const v =
					l1 * 3.2 * Math.exp(-k1 * t) +
					(l2 - l1) * 1.5 * Math.exp(-k2 * t) +
					(l3 - l2) * 0.75 * Math.exp(-k3 * t) +
					(w - l3) * 0.22 * Math.exp(-k4 * t);
				d[i] = v * (1 - Math.exp(-t / 0.02));
			}
		}
		return buf;
	});
}

export function mixer(k: Kit, b: Bus): Mix {
	const ctx = k.ctx;
	const peak = (f: number, g: number, q: number) => {
		const p = ctx.createBiquadFilter();
		p.type = 'peaking';
		p.frequency.value = f;
		p.gain.value = g;
		p.Q.value = q;
		return p;
	};
	// everything leaves through one trim, low enough that the channel's glue compressor only
	// touches the landing, so the arc of the score survives it
	const master = k.gain(0.3);
	master.connect(b.music);
	// dry: clear the rumble, take a little out of the mud and the harsh band, warm it slightly
	const dry = k.gain(1);
	const sat = ctx.createWaveShaper();
	sat.curve = tanhCurve(1.15);
	sat.oversample = '4x';
	const satIn = k.gain(0.7);
	const satOut = k.gain(1 / 0.7);
	dry
		.connect(k.filter('highpass', 28, 0.7))
		.connect(peak(300, -2.5, 0.8))
		.connect(peak(3400, -2, 0.7))
		.connect(satIn)
		.connect(sat)
		.connect(satOut)
		.connect(master);
	// the hall
	const hall = k.gain(1);
	const pre = ctx.createDelay(0.2);
	pre.delayTime.value = 0.032;
	const conv = ctx.createConvolver();
	conv.buffer = hallImpulse(ctx);
	hall
		.connect(pre)
		.connect(conv)
		.connect(k.filter('highpass', 160, 0.6))
		.connect(k.filter('lowpass', 7000, 0.5))
		.connect(peak(420, -2, 0.9))
		.connect(k.gain(0.9))
		.connect(master);
	// ping-pong delay, a dotted eighth, darker on every repeat
	const echo = k.gain(1);
	const merger = ctx.createChannelMerger(2);
	const left = ctx.createDelay(2);
	const right = ctx.createDelay(2);
	left.delayTime.value = 0.45;
	right.delayTime.value = 0.45;
	const fb = k.gain(0.38);
	const tone = k.filter('lowpass', 2600, 0.6);
	const tone2 = k.filter('highpass', 300, 0.6);
	echo.connect(tone2).connect(left);
	left.connect(right);
	right.connect(tone).connect(fb).connect(left);
	left.connect(merger, 0, 0);
	right.connect(merger, 0, 1);
	const echoOut = k.gain(0.55);
	merger.connect(echoOut);
	echoOut.connect(dry);
	echoOut.connect(k.gain(0.5)).connect(hall);
	// the pads' chorus: two slowly swept short delays, one each side
	const pads = k.gain(1);
	const padHp = k.filter('highpass', 105, 0.6);
	const padHp2 = k.filter('highpass', 105, 0.6);
	pads.connect(padHp).connect(padHp2);
	const split = ctx.createChannelSplitter(2);
	const join = ctx.createChannelMerger(2);
	padHp2.connect(dry);
	padHp2.connect(split);
	const lfoRate = [0.31, 0.43];
	[0, 1].forEach((side) => {
		const d = ctx.createDelay(0.1);
		d.delayTime.value = 0.014 + side * 0.004;
		const lfo = ctx.createOscillator();
		lfo.frequency.value = lfoRate[side];
		const depth = k.gain(0.0032);
		lfo.connect(depth).connect(d.delayTime);
		lfo.start(0);
		k.track(b, lfo);
		split.connect(d, side);
		d.connect(join, 0, 1 - side);
	});
	join.connect(k.gain(0.75)).connect(dry);
	return { dry, hall, echo, pads };
}

/** Send `node` to the hall and echo by the given amounts. */
function sends(k: Kit, m: Mix, node: AudioNode, hall: number, echo = 0) {
	if (hall > 0) node.connect(k.gain(hall)).connect(m.hall);
	if (echo > 0) node.connect(k.gain(echo)).connect(m.echo);
}

// --- pads ------------------------------------------------------------------------------------

export interface PadOptions {
	attack?: number;
	release?: number;
	/** Filter cutoff at the start and where it has opened to by the end of the note. */
	cutoff?: [number, number];
	q?: number;
	gain?: number;
	/** Cents between the three saws of each note. */
	detune?: number;
	/** Stereo spread of the voices (0..1). */
	spread?: number;
	seed?: number;
	hall?: number;
	/** Gain multipliers over the note: [seconds from start, multiplier]. */
	swell?: [number, number][];
}

/** An analog pad chord: three detuned saws a note through one slowly opening low-pass. */
export function pad(
	k: Kit,
	b: Bus,
	m: Mix,
	when: number,
	freqs: number[],
	dur: number,
	o: PadOptions = {}
) {
	const {
		attack = 1.2,
		release = 2,
		cutoff = [900, 1800],
		q = 0.8,
		gain = 1,
		detune = 9,
		spread = 0.95,
		seed = 1,
		hall = 0.45
	} = o;
	const ctx = k.ctx;
	const r = rng(seed);
	const sum = k.gain(1);
	const lp = k.filter('lowpass', cutoff[0], q);
	const end = when + dur;
	lp.frequency.setValueAtTime(cutoff[0], when);
	lp.frequency.exponentialRampToValueAtTime(cutoff[1], end);
	// a slow wander on the filter, so the pad breathes
	const wob = ctx.createOscillator();
	wob.frequency.value = 0.13 + r() * 0.08;
	const wobDepth = k.gain(cutoff[0] * 0.12);
	wob.connect(wobDepth).connect(lp.frequency);
	wob.start(when);
	wob.stop(end + release * 2.5);
	k.track(b, wob);
	const amp = k.gain(0);
	const level = (0.085 * gain) / Math.sqrt(freqs.length);
	amp.gain.setValueAtTime(0, when);
	amp.gain.setTargetAtTime(level, when, attack / 3);
	amp.gain.setTargetAtTime(0, end, release / 4.5);
	const box = k.gain(1);
	if (o.swell) {
		box.gain.setValueAtTime(o.swell[0]?.[1] ?? 1, when);
		for (const [t, v] of o.swell) box.gain.linearRampToValueAtTime(v, when + t);
	}
	sum.connect(lp).connect(amp).connect(box).connect(m.pads);
	sends(k, m, box, hall);
	freqs.forEach((f, i) => {
		for (let v = 0; v < 3; v++) {
			const osc = ctx.createOscillator();
			osc.type = 'sawtooth';
			osc.frequency.value = f;
			osc.detune.value = (v - 1) * detune + (r() - 0.5) * 3;
			// a slow drift per voice, like an old synth's oscillators
			const drift = ctx.createOscillator();
			drift.frequency.value = 0.07 + r() * 0.11;
			const dd = k.gain(2.2);
			drift.connect(dd).connect(osc.detune);
			const p = k.pan(Math.max(-1, Math.min(1, (v - 1) * spread * 0.8 + (i % 2 ? 0.12 : -0.12))));
			osc.connect(p).connect(sum);
			const t0 = Math.max(0, when - r() / f);
			osc.start(t0);
			drift.start(t0);
			const stop = end + release * 2.5;
			osc.stop(stop);
			drift.stop(stop);
			k.track(b, osc);
			k.track(b, drift);
		}
	});
}

// --- felt piano --------------------------------------------------------------------------------

/**
 * One note of a felt piano: two slightly detuned strings of twelve stretched partials, each
 * partial decaying faster the higher it is, the hammer's position notching the spectrum, a soft
 * felt thump, a 4 ms attack. Stereo: the strings sit a little apart.
 */
function feltBuffer(ctx: BaseAudioContext, f: number, decay: number, bright: number): AudioBuffer {
	return cached(ctx, `felt:${f.toFixed(3)}:${decay}:${bright}`, () => {
		const sr = ctx.sampleRate;
		const len = Math.floor(sr * Math.min(7, decay * 1.6 + 0.3));
		const buf = ctx.createBuffer(2, len, sr);
		const r = rng(Math.round(f * 100));
		const B = 0.00032;
		const parts: { w: number; a: number; k: number; ph: number }[][] = [[], []];
		for (let s = 0; s < 2; s++) {
			const fs = f * (1 + (s ? 0.00042 : -0.00042));
			for (let n = 1; n <= 12; n++) {
				const fn = fs * n * Math.sqrt(1 + B * n * n);
				if (fn > 9500) break;
				const hammer = Math.abs(Math.sin(Math.PI * n * 0.118));
				const a = (n === 1 ? 1 : 0.62 / n ** 1.25) * bright ** (n - 1) * (0.35 + 0.65 * hammer);
				const tn = decay / (1 + 0.42 * (n - 1) ** 1.15);
				parts[s].push({
					w: (2 * Math.PI * fn) / sr,
					a,
					k: 1 / ((tn * sr) / 6.91),
					ph: r() * Math.PI * 2
				});
			}
		}
		const L = buf.getChannelData(0);
		const R = buf.getChannelData(1);
		let lp = 0;
		const al = 1 - Math.exp((-2 * Math.PI * 1600) / sr);
		const nr = rng(Math.round(f * 7));
		for (let i = 0; i < len; i++) {
			const t = i / sr;
			const att = t < 0.004 ? 0.5 - 0.5 * Math.cos((Math.PI * t) / 0.004) : 1;
			let a = 0;
			let c = 0;
			for (const p of parts[0]) a += p.a * Math.exp(-p.k * i) * Math.sin(p.w * i + p.ph);
			for (const p of parts[1]) c += p.a * Math.exp(-p.k * i) * Math.sin(p.w * i + p.ph);
			lp += al * (nr() * 2 - 1 - lp);
			const thump = lp * Math.exp(-t / 0.007) * 0.35;
			L[i] = (a * 0.62 + c * 0.38 + thump) * att * 0.3;
			R[i] = (a * 0.38 + c * 0.62 + thump) * att * 0.3;
		}
		return buf;
	});
}

export interface KeyOptions {
	gain?: number;
	decay?: number;
	bright?: number;
	pan?: number;
	hall?: number;
	echo?: number;
	/** Low-pass on the note (the felt's softness). */
	tone?: number;
}

export function felt(k: Kit, b: Bus, m: Mix, when: number, note: number, o: KeyOptions = {}) {
	const { gain = 1, decay = 3.2, bright = 0.82, pan = 0, hall = 0.5, echo = 0, tone = 5200 } = o;
	const src = k.ctx.createBufferSource();
	src.buffer = feltBuffer(k.ctx, note, decay, bright);
	const lp = k.filter('lowpass', tone, 0.5);
	const g = k.gain(gain);
	const p = k.pan(pan);
	src.connect(lp).connect(g).connect(p).connect(m.dry);
	sends(k, m, p, hall, echo);
	src.start(when);
	k.track(b, src);
}

// --- bass and sound design -------------------------------------------------------------------

/** A sub note: a sine and a little of its octave, a slow attack and release. */
export function sub(
	k: Kit,
	b: Bus,
	m: Mix,
	when: number,
	f: number,
	dur: number,
	o: { gain?: number; attack?: number; release?: number } = {}
) {
	const { gain = 1, attack = 0.4, release = 1.2 } = o;
	const ctx = k.ctx;
	const amp = k.gain(0);
	amp.gain.setValueAtTime(0, when);
	amp.gain.setTargetAtTime(0.32 * gain, when, attack / 3);
	amp.gain.setTargetAtTime(0, when + dur, release / 4.5);
	const stop = when + dur + release * 2.5;
	const s = ctx.createOscillator();
	s.frequency.value = f;
	const o2 = ctx.createOscillator();
	o2.frequency.value = f * 2;
	const g2 = k.gain(0.12);
	s.connect(amp);
	o2.connect(g2).connect(amp);
	amp.connect(m.dry);
	for (const x of [s, o2]) {
		x.start(when);
		x.stop(stop);
		k.track(b, x);
	}
}

/**
 * The landing: a sub that drops to the key's root and rings, a body thump, a short soft noise
 * transient, and air for the hall's long tail.
 */
export function impact(
	k: Kit,
	b: Bus,
	m: Mix,
	when: number,
	root: number,
	o: { gain?: number; seed?: number } = {}
) {
	const { gain = 1, seed = 5 } = o;
	const ctx = k.ctx;
	const sr = ctx.sampleRate;
	// sub drop
	const s = ctx.createOscillator();
	s.frequency.setValueAtTime(root * 3.2, when);
	s.frequency.exponentialRampToValueAtTime(root * 1.25, when + 0.09);
	s.frequency.exponentialRampToValueAtTime(root, when + 0.7);
	const sg = k.gain(0);
	sg.gain.setValueAtTime(0, when);
	sg.gain.linearRampToValueAtTime(0.44 * gain, when + 0.006);
	sg.gain.setTargetAtTime(0.28 * gain, when + 0.02, 0.22);
	sg.gain.setTargetAtTime(0, when + 0.5, 0.7);
	const shaper = ctx.createWaveShaper();
	shaper.curve = tanhCurve(1.6);
	shaper.oversample = '4x';
	s.connect(shaper).connect(sg).connect(m.dry);
	s.start(when);
	s.stop(when + 5);
	k.track(b, s);
	// body: a low thump with a fast pitch fall
	const body = ctx.createOscillator();
	body.type = 'triangle';
	body.frequency.setValueAtTime(180, when);
	body.frequency.exponentialRampToValueAtTime(62, when + 0.07);
	const bg = k.gain(0);
	bg.gain.setValueAtTime(0, when);
	bg.gain.linearRampToValueAtTime(0.5 * gain, when + 0.003);
	bg.gain.setTargetAtTime(0, when + 0.01, 0.09);
	const blp = k.filter('lowpass', 900, 0.7);
	body.connect(blp).connect(bg).connect(m.dry);
	sends(k, m, bg, 0.35);
	body.start(when);
	body.stop(when + 1);
	k.track(b, body);
	// the transient: a short burst of noise, low-passed so it lands soft
	const nb = ctx.createBuffer(2, Math.floor(sr * 1.6), sr);
	const r = rng(seed);
	for (let ch = 0; ch < 2; ch++) {
		const d = nb.getChannelData(ch);
		let lp = 0;
		for (let i = 0; i < d.length; i++) {
			const t = i / sr;
			lp += 0.5 * (r() * 2 - 1 - lp);
			d[i] = lp * (Math.exp(-t / 0.016) * 0.9 + Math.exp(-t / 0.14) * 0.035);
		}
	}
	const n = ctx.createBufferSource();
	n.buffer = nb;
	const nlp = k.filter('lowpass', 3800, 0.6);
	const nhp = k.filter('highpass', 140, 0.6);
	const ng = k.gain(0.5 * gain);
	n.connect(nhp).connect(nlp).connect(ng).connect(m.dry);
	sends(k, m, ng, 0.45);
	n.start(when);
	k.track(b, n);
}

/**
 * A swell into `end`: noise rising through a band-pass and a resonant sweep, cut a hair before
 * the hit so the hit has the air to itself.
 */
export function swell(
	k: Kit,
	b: Bus,
	m: Mix,
	end: number,
	dur: number,
	o: { gain?: number; seed?: number; from?: number; to?: number } = {}
) {
	const { gain = 1, seed = 9, from = 300, to = 3600 } = o;
	const ctx = k.ctx;
	const sr = ctx.sampleRate;
	const when = end - dur;
	const nb = ctx.createBuffer(2, Math.floor(sr * (dur + 0.1)), sr);
	const r = rng(seed);
	for (let ch = 0; ch < 2; ch++) {
		const d = nb.getChannelData(ch);
		let p0 = 0;
		let p1 = 0;
		for (let i = 0; i < d.length; i++) {
			// pink-ish: two leaky integrators blended with white
			const w = r() * 2 - 1;
			p0 = 0.997 * p0 + w * 0.05;
			p1 = 0.96 * p1 + w * 0.2;
			d[i] = (p0 + p1 + w * 0.12) * 0.6;
		}
	}
	const src = ctx.createBufferSource();
	src.buffer = nb;
	const bp = k.filter('bandpass', from, 0.9);
	bp.frequency.setValueAtTime(from, when);
	bp.frequency.exponentialRampToValueAtTime(to, end);
	const g = k.gain(0);
	g.gain.setValueAtTime(0.0001, when);
	g.gain.exponentialRampToValueAtTime(0.5 * gain, end - 0.03);
	g.gain.linearRampToValueAtTime(0, end - 0.005);
	src
		.connect(bp)
		.connect(k.filter('lowpass', 4500, 0.6))
		.connect(g)
		.connect(m.dry);
	sends(k, m, g, 0.6);
	src.start(when);
	src.stop(end + 0.05);
	k.track(b, src);
}

/** Air moving past: band-passed noise swelling and fading, panned with the motion. */
export function air(
	k: Kit,
	b: Bus,
	m: Mix,
	when: number,
	dur: number,
	o: {
		gain?: number;
		seed?: number;
		lo?: number;
		hi?: number;
		pan?: [number, number];
		peak?: number;
	} = {}
) {
	const { gain = 1, seed = 13, lo = 300, hi = 1800, pan = [0, 0], peak = 0.55 } = o;
	const ctx = k.ctx;
	const sr = ctx.sampleRate;
	const nb = ctx.createBuffer(1, Math.floor(sr * (dur + 0.1)), sr);
	const r = rng(seed);
	const d = nb.getChannelData(0);
	let p = 0;
	for (let i = 0; i < d.length; i++) {
		p = 0.985 * p + (r() * 2 - 1) * 0.12;
		d[i] = p + (r() * 2 - 1) * 0.04;
	}
	const src = ctx.createBufferSource();
	src.buffer = nb;
	const bp = k.filter('bandpass', lo, 0.8);
	bp.frequency.setValueAtTime(lo, when);
	bp.frequency.exponentialRampToValueAtTime(hi, when + dur * peak);
	bp.frequency.exponentialRampToValueAtTime(lo * 0.8, when + dur);
	const g = k.gain(0);
	const steps = 48;
	const curve = new Float32Array(steps);
	for (let i = 0; i < steps; i++) {
		const x = i / (steps - 1);
		curve[i] =
			(x < peak
				? Math.sin((x / peak) * Math.PI * 0.5) ** 2
				: Math.cos(((x - peak) / (1 - peak)) * Math.PI * 0.5) ** 2) *
			gain *
			0.6;
	}
	g.gain.setValueCurveAtTime(curve, when, dur);
	const pn = k.pan(pan[0]);
	pn.pan.setValueAtTime(pan[0], when);
	pn.pan.linearRampToValueAtTime(pan[1], when + dur);
	src
		.connect(bp)
		.connect(k.filter('lowpass', 2400, 0.6))
		.connect(g)
		.connect(pn)
		.connect(m.dry);
	sends(k, m, pn, 0.4);
	src.start(when);
	src.stop(when + dur + 0.05);
	k.track(b, src);
}

/**
 * Wind over the hills: two decorrelated bands of noise whose pitch and level wander slowly, the
 * gusts different on each side.
 */
export function wind(
	k: Kit,
	b: Bus,
	m: Mix,
	when: number,
	dur: number,
	o: { gain?: number; seed?: number; fadeIn?: number; fadeOut?: number } = {}
) {
	const { gain = 1, seed = 41, fadeIn = 0.8, fadeOut = 1 } = o;
	const ctx = k.ctx;
	const sr = ctx.sampleRate;
	const len = Math.floor(sr * (dur + 0.1));
	const nb = ctx.createBuffer(2, len, sr);
	for (let ch = 0; ch < 2; ch++) {
		const r = rng(seed + ch * 31);
		const d = nb.getChannelData(ch);
		let p = 0;
		// gusts: a sum of slow sines with seeded phases
		const ph = [r() * 6.28, r() * 6.28, r() * 6.28];
		for (let i = 0; i < len; i++) {
			const t = i / sr;
			p = 0.993 * p + (r() * 2 - 1) * 0.1;
			const gust =
				0.62 +
				0.22 * Math.sin(t * 1.3 + ph[0]) +
				0.12 * Math.sin(t * 2.9 + ph[1]) +
				0.06 * Math.sin(t * 6.1 + ph[2]);
			d[i] = p * gust;
		}
	}
	const src = ctx.createBufferSource();
	src.buffer = nb;
	const bp = k.filter('bandpass', 520, 0.55);
	const lfo = ctx.createOscillator();
	lfo.frequency.value = 0.21;
	const depth = k.gain(220);
	lfo.connect(depth).connect(bp.frequency);
	const g = k.gain(0);
	g.gain.setValueAtTime(0, when);
	g.gain.linearRampToValueAtTime(0.5 * gain, when + fadeIn);
	g.gain.setValueAtTime(0.5 * gain, when + dur - fadeOut);
	g.gain.linearRampToValueAtTime(0, when + dur);
	src
		.connect(k.filter('highpass', 160, 0.6))
		.connect(bp)
		.connect(k.filter('lowpass', 2200, 0.6))
		.connect(g)
		.connect(m.dry);
	sends(k, m, g, 0.25);
	src.start(when);
	src.stop(when + dur + 0.05);
	lfo.start(when);
	lfo.stop(when + dur + 0.05);
	k.track(b, src);
	k.track(b, lfo);
}

/** A celesta-like shimmer: a pure tone and a faint fourth partial, decaying fast, into the hall. */
export function shimmer(
	k: Kit,
	b: Bus,
	m: Mix,
	when: number,
	f: number,
	o: { gain?: number; decay?: number; pan?: number } = {}
) {
	const { gain = 1, decay = 1.6, pan = 0 } = o;
	const ctx = k.ctx;
	const g = k.gain(0);
	g.gain.setValueAtTime(0, when);
	g.gain.linearRampToValueAtTime(0.09 * gain, when + 0.005);
	g.gain.setTargetAtTime(0, when + 0.01, decay / 5);
	const p = k.pan(pan);
	g.connect(p).connect(m.dry);
	sends(k, m, p, 0.8, 0.25);
	for (const [ratio, amp] of [
		[1, 1],
		[4.0, 0.06]
	]) {
		const o2 = ctx.createOscillator();
		o2.frequency.value = f * ratio;
		const a = k.gain(amp);
		o2.connect(a).connect(g);
		o2.start(when);
		o2.stop(when + decay * 1.6);
		k.track(b, o2);
	}
}
