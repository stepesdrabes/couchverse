// The journey's instruments, synthesized: a lush detuned pad with slow filter motion, a soft
// shimmer above it, glass keys (additive, rendered sample by sample), a harp-like pluck
// (Karplus-Strong), a sub, a reverse swell, air (filtered noise moves) and the landing's impact. They share one mix chain
// per bus: a generated long reverb, a ping-pong echo and a gentle EQ, so the score sits in one
// space. Every sample is a function of the score and its seeds.

import type { Bus } from '../../../audio/engine';
import type { Kit } from '../../../audio/kit';
import { hz } from '../../../audio/orchestra';
import { rng } from '../../../core/rng';

export const note = (n: string | number) => (typeof n === 'number' ? n : hz(n));

// --- the mix chain ---------------------------------------------------------------------------

export interface Chain {
	/** Dry input (after the voice's own gain and pan). */
	dry: AudioNode;
	/** Reverb send. */
	verb: AudioNode;
	/** Echo send (dotted eighths, ping-pong; its repeats also feed the reverb). */
	echo: AudioNode;
	/** The score's master level (after the EQ, reverb and echo returns). */
	out: GainNode;
}

const chains = new WeakMap<Bus, Chain>();

/** The whole score's level into the engine. */
const MIX = 0.42;

const curves = new Map<number, Float32Array<ArrayBuffer>>();
function tanh(drive: number) {
	let c = curves.get(drive);
	if (!c) {
		const n = 4096;
		c = new Float32Array(n);
		for (let i = 0; i < n; i++) {
			const x = (i / (n - 1)) * 2 - 1;
			c[i] = Math.tanh(x * drive) / Math.tanh(drive);
		}
		curves.set(drive, c);
	}
	return c;
}

export function saturate(k: Kit, drive: number): WaveShaperNode {
	const s = k.ctx.createWaveShaper();
	s.curve = tanh(drive);
	s.oversample = '4x';
	return s;
}

function peaking(k: Kit, f: number, q: number, db: number) {
	const p = k.ctx.createBiquadFilter();
	p.type = 'peaking';
	p.frequency.value = f;
	p.Q.value = q;
	p.gain.value = db;
	return p;
}

/**
 * A generated hall: a diffuse noise tail that builds over 40 ms and decays over `seconds`, its
 * highs dying first (a one-pole low-pass that closes along the tail), decorrelated left and
 * right. Smooth by construction: no discrete echoes to flutter.
 */
function hall(ctx: BaseAudioContext, seconds: number, t60: number): AudioBuffer {
	const sr = ctx.sampleRate;
	const n = Math.floor(sr * seconds);
	const buf = ctx.createBuffer(2, n, sr);
	for (let ch = 0; ch < 2; ch++) {
		const r = rng(901 + ch * 77);
		const d = buf.getChannelData(ch);
		let lp = 0;
		let lp2 = 0;
		for (let i = 0; i < n; i++) {
			const t = i / sr;
			const w = r() * 2 - 1;
			// brightness falls from ~9 kHz to ~1.5 kHz along the tail
			const fc = 1500 + 7500 * Math.exp(-t / 0.9);
			const a = 1 - Math.exp((-2 * Math.PI * fc) / sr);
			lp += a * (w - lp);
			lp2 += a * (lp - lp2);
			const env = (1 - Math.exp(-t / 0.04)) * Math.exp((-6.91 * t) / t60);
			// the very end fades to zero, so the tail never clicks off
			const end = Math.min(1, (n - i) / (sr * 0.3));
			d[i] = lp2 * env * end;
		}
	}
	return buf;
}

export function chain(k: Kit, b: Bus): Chain {
	let c = chains.get(b);
	if (c) return c;
	const ctx = k.ctx;
	// the master EQ: no rumble, a little room taken out of the low mids, the 2-5 kHz harshness
	// region eased, a touch of air
	const hp = k.filter('highpass', 26, 0.7);
	const mud = peaking(k, 320, 0.9, -2.5);
	const harsh = peaking(k, 3400, 1.2, -1);
	const air = k.ctx.createBiquadFilter();
	air.type = 'highshelf';
	air.frequency.value = 7500;
	air.gain.value = 3;
	// the mix sits under the engine's glue compressor, so the dynamics stay as written (the
	// render is loudness-normalized afterwards)
	const dry = k.gain(1);
	const out = k.gain(MIX);
	dry.connect(hp).connect(mud).connect(harsh).connect(air).connect(out).connect(b.music);

	const conv = ctx.createConvolver();
	conv.buffer = hall(ctx, 5.2, 3.9);
	const verbIn = k.gain(1);
	const verbHp = k.filter('highpass', 190, 0.7);
	const verbLp = k.filter('lowpass', 9500, 0.6);
	const verbOut = k.gain(0.9);
	verbIn.connect(verbHp).connect(verbLp).connect(conv).connect(verbOut).connect(hp);

	// ping-pong echo at a dotted eighth (0.6 s): left, then right, each repeat darker. The graph
	// enters the feedback loop only through the left delay (the right repeats come from a delay
	// outside it): Chromium pulls a loop from wherever the graph first reaches it and puts the
	// loop's one-quantum lag on the edge into that node, so a loop reached from two sides lagged
	// its repeats differently from render to render. Entered here, the lag stays where the
	// approved render had it.
	const echoIn = k.gain(1);
	const left = ctx.createDelay(2);
	const right = ctx.createDelay(2);
	const loop = ctx.createDelay(2);
	for (const d of [left, right, loop]) d.delayTime.value = 0.6;
	const fb = k.gain(0.42);
	const tone = k.filter('lowpass', 3800, 0.6);
	const thin = k.filter('highpass', 380, 0.6);
	const pl = k.pan(-0.7);
	const pr = k.pan(0.7);
	echoIn.connect(thin).connect(left);
	left.connect(pl);
	left.connect(right).connect(pr);
	left.connect(loop).connect(tone).connect(fb).connect(left);
	const echoOut = k.gain(0.55);
	pl.connect(echoOut);
	pr.connect(echoOut);
	echoOut.connect(hp);
	echoOut.connect(k.gain(0.5)).connect(verbIn);

	c = { dry, verb: verbIn, echo: echoIn, out };
	chains.set(b, c);
	return c;
}

export interface Out {
	gain?: number;
	pan?: number;
	verb?: number;
	echo?: number;
}

/** Send a voice into the chain: gain, pan, then dry, reverb and echo. */
export function route(k: Kit, b: Bus, node: AudioNode, o: Out) {
	const c = chain(k, b);
	const g = k.gain(o.gain ?? 1);
	const p = k.pan(o.pan ?? 0);
	node.connect(g).connect(p);
	p.connect(c.dry);
	if ((o.verb ?? 0) > 0) p.connect(k.gain(o.verb ?? 0)).connect(c.verb);
	if ((o.echo ?? 0) > 0) p.connect(k.gain(o.echo ?? 0)).connect(c.echo);
	return p;
}

function play(k: Kit, b: Bus, buf: AudioBuffer, when: number) {
	const s = k.ctx.createBufferSource();
	s.buffer = buf;
	s.start(Math.max(0, when));
	k.track(b, s);
	return s;
}

// --- glass keys --------------------------------------------------------------------------------

/**
 * Glass keys: a fundamental with soft harmonics and one faint inharmonic shimmer, each partial
 * decaying at its own rate, a felt-soft attack, two copies a few cents apart (left and right)
 * so the note breathes. Rendered into a buffer.
 */
export function glass(
	k: Kit,
	b: Bus,
	when: number,
	n: string | number,
	o: Out & { decay?: number; bright?: number; seed?: number; len?: number } = {}
) {
	const f = note(n);
	const { decay = 3.2, bright = 1, seed = 1 } = o;
	const len = o.len ?? decay * 4.7;
	const sr = k.ctx.sampleRate;
	const N = Math.floor(sr * len);
	const buf = k.ctx.createBuffer(2, N, sr);
	const partials: [number, number, number][] = [
		[1, 1, decay],
		[2, 0.4 * bright, decay * 0.5],
		[3, 0.15 * bright, decay * 0.3],
		[4, 0.07 * bright, decay * 0.2],
		[5, 0.03 * bright, decay * 0.14],
		[6.12, 0.02 * bright, decay * 0.07]
	];
	const r = rng(seed);
	for (let ch = 0; ch < 2; ch++) {
		const d = buf.getChannelData(ch);
		const cents = ch ? 2.2 : -2.2;
		for (const [ratio, amp, dec] of partials) {
			const fr = f * ratio * 2 ** (cents / 1200) * (1 + 0.00012 * ratio * ratio);
			if (fr > 15000) continue;
			const w = (2 * Math.PI * fr) / sr;
			const ph = r() * Math.PI * 2;
			const kdec = 1 / (dec * sr);
			for (let i = 0; i < N; i++) d[i] += amp * Math.sin(w * i + ph) * Math.exp(-i * kdec);
		}
		// a felt-soft attack (5 ms) and a fade at the end of the buffer
		const atk = sr * 0.005;
		for (let i = 0; i < N; i++) {
			const a = i < atk ? 0.5 - 0.5 * Math.cos((Math.PI * i) / atk) : 1;
			const z = Math.min(1, (N - i) / (sr * 0.2));
			d[i] *= a * z * 0.22;
		}
	}
	route(k, b, play(k, b, buf, when), o);
}

// --- harp pluck --------------------------------------------------------------------------------

/** A plucked string (Karplus-Strong): a soft, filtered excitation, a warm damping loop. */
export function pluck(
	k: Kit,
	b: Bus,
	when: number,
	n: string | number,
	o: Out & { decay?: number; seed?: number; soft?: number } = {}
) {
	const f = note(n);
	const { decay = 2.2, seed = 3, soft = 0.5 } = o;
	const sr = k.ctx.sampleRate;
	const N = Math.floor(sr * decay * 1.3);
	const buf = k.ctx.createBuffer(1, N, sr);
	const y = buf.getChannelData(0);
	// the loop's two-point average adds half a sample: the rest of the period is the delay
	const D = sr / f - 0.5;
	const at = (m: number) => {
		if (m < 0) return 0;
		const i = Math.floor(m);
		return y[i] + (y[i + 1] - y[i]) * (m - i);
	};
	// the loss per period that gives a T60 of `decay` seconds
	const g = Math.pow(10, -3 / (decay * f));
	const r = rng(seed);
	// excitation: one period of noise, low-passed (a felt finger, not a pick)
	let lp = 0;
	const ka = 0.12 + 0.6 * (1 - soft);
	const P = Math.ceil(D);
	for (let i = 0; i < N; i++) {
		let x = 0;
		if (i < P) {
			lp += ka * (r() * 2 - 1 - lp);
			x = lp;
		}
		y[i] = x + g * 0.5 * (at(i - D) + at(i - D - 1));
	}
	let mean = 0;
	const M = Math.min(N, 9600);
	for (let i = 0; i < M; i++) mean += y[i];
	mean /= M;
	const onset = sr * 0.0015;
	for (let i = 0; i < N; i++)
		y[i] = (y[i] - mean) * Math.min(1, (N - i) / (sr * 0.15), i / onset) * 0.7;
	const src = play(k, b, buf, when);
	const tone = k.filter('lowpass', Math.min(9000, f * 8), 0.6);
	src.connect(tone);
	route(k, b, tone, o);
}

// --- pad ---------------------------------------------------------------------------------------

export interface PadOptions extends Out {
	attack?: number;
	release?: number;
	/** Filter cutoff at the start and at the end of the note (Hz). */
	from?: number;
	to?: number;
	detune?: number;
	seed?: number;
	/** Amplitude through the note: [seconds from when, level 0..1] (default: flat). */
	shape?: [number, number][];
}

/**
 * A lush pad: three detuned saws a note (spread left, centre, right), a 24 dB low-pass moving
 * slowly, a gentle tanh, a slow chorus for width. Never a bare saw: the filter keeps it warm.
 */
export function pad(
	k: Kit,
	b: Bus,
	when: number,
	chord: (string | number)[],
	dur: number,
	o: PadOptions = {}
) {
	const { attack = 1.2, release = 1.8, from = 700, to = 1400, detune = 9, seed = 5 } = o;
	const r = rng(seed);
	// the voices run until the release has died away completely (ten of its time constants)
	const total = attack + dur + release * 2.5;
	const lp1 = k.filter('lowpass', from, 0.5);
	const lp2 = k.filter('lowpass', from, 0.6);
	for (const lp of [lp1, lp2]) {
		lp.frequency.setValueAtTime(from, when);
		lp.frequency.exponentialRampToValueAtTime(to, when + attack + dur);
		lp.frequency.exponentialRampToValueAtTime(Math.max(200, to * 0.5), when + total);
	}
	const sum = k.gain(1);
	const fs = chord.map(note);
	const level = 0.11 / Math.sqrt(fs.length);
	fs.forEach((f, i) => {
		// low notes speak softer, so the chord's weight sits in the mids, not the bass
		const tilt = k.gain(Math.min(1, Math.sqrt(f / 260)));
		tilt.connect(sum);
		for (let v = 0; v < 3; v++) {
			const osc = k.oscAt(b, 'sawtooth', f, when, total + 0.05, r);
			osc.detune.value = (v - 1) * detune + (r() - 0.5) * 3;
			// a slow drift per voice keeps the chorus alive
			const lfo = k.oscAt(b, 'sine', 0.13 + r() * 0.2, when, total + 0.05, r);
			lfo.connect(k.gain(2.5)).connect(osc.detune);
			const side = (v - 1) * (0.55 + 0.3 * (i / fs.length)) * (i % 2 ? 1 : -1);
			osc.connect(k.pan(side)).connect(tilt);
		}
	});
	const env = k.gain(0);
	const p = env.gain;
	p.setValueAtTime(0, when);
	if (o.shape) {
		p.linearRampToValueAtTime(level * o.shape[0][1], when + Math.max(0.01, o.shape[0][0]));
		for (const [ts, v] of o.shape.slice(1)) p.linearRampToValueAtTime(level * v, when + ts);
	} else {
		p.linearRampToValueAtTime(level, when + attack);
		p.setValueAtTime(level, when + attack + dur);
	}
	p.setTargetAtTime(0, when + attack + dur, release / 4);
	const hp = k.filter('highpass', 90, 0.6);
	sum.connect(lp1).connect(lp2).connect(saturate(k, 1.3)).connect(hp).connect(env);
	// chorus: two short modulated delays, one each side
	const out = k.gain(1);
	env.connect(out);
	[-1, 1].forEach((side, j) => {
		const dl = k.ctx.createDelay(0.1);
		dl.delayTime.value = 0.018 + j * 0.006;
		const mod = k.oscAt(b, 'sine', 0.31 + j * 0.13, when, total + 0.1, r);
		mod.connect(k.gain(0.0015)).connect(dl.delayTime);
		env
			.connect(dl)
			.connect(k.pan(side * 0.85))
			.connect(k.gain(0.45))
			.connect(out);
	});
	route(k, b, out, o);
}

// --- sub -----------------------------------------------------------------------------------------

/** A sub note: a sine with a breath of second harmonic so small speakers still hear it. */
export function sub(
	k: Kit,
	b: Bus,
	when: number,
	n: string | number,
	dur: number,
	o: Out & { attack?: number; release?: number } = {}
) {
	const { attack = 0.3, release = 1.2 } = o;
	const f = note(n);
	const osc = k.osc(b, 'sine', f, when, attack + dur + release * 2);
	const env = k.gain(0);
	env.gain.setValueAtTime(0, when);
	env.gain.linearRampToValueAtTime(0.5, when + attack);
	env.gain.setValueAtTime(0.5, when + attack + dur);
	env.gain.setTargetAtTime(0, when + attack + dur, release / 4);
	osc
		.connect(saturate(k, 1.8))
		.connect(k.filter('lowpass', 180, 0.7))
		.connect(env);
	route(k, b, env, { ...o, verb: 0, echo: 0 });
}

// --- swells and air ------------------------------------------------------------------------------

/**
 * A reverse swell into `end`: the chord as a smear of detuned partials and soft noise growing
 * exponentially (a reverb tail played backwards), cut with a 6 ms fade right on the hit.
 */
export function swell(
	k: Kit,
	b: Bus,
	end: number,
	dur: number,
	chord: (string | number)[],
	o: Out & { seed?: number; tau?: number } = {}
) {
	const { seed = 9, tau = 0.32 } = o;
	const sr = k.ctx.sampleRate;
	const N = Math.floor(sr * dur);
	const buf = k.ctx.createBuffer(2, N, sr);
	const r = rng(seed);
	const fs = chord.map(note);
	for (let ch = 0; ch < 2; ch++) {
		const d = buf.getChannelData(ch);
		for (const f of fs)
			for (let v = 0; v < 3; v++) {
				const fr = f * 2 ** (((v - 1) * 7 + (r() - 0.5) * 4) / 1200);
				const w = (2 * Math.PI * fr) / sr;
				const ph = r() * 6.283;
				const a = 1 / (fs.length * 3);
				for (let i = 0; i < N; i++) d[i] += a * Math.sin(w * i + ph);
			}
		let lp = 0;
		let lp2 = 0;
		for (let i = 0; i < N; i++) {
			const t = i / sr;
			const w = r() * 2 - 1;
			lp += 0.18 * (w - lp);
			lp2 += 0.18 * (lp - lp2);
			const env = Math.exp((t - dur) / tau);
			const cut = Math.min(1, (N - i) / (sr * 0.006));
			d[i] = (d[i] * 0.8 + lp2 * 0.9) * env * cut * 0.5;
		}
	}
	route(k, b, play(k, b, buf, end - dur), o);
}

/**
 * Air: pink noise through a moving band-pass, a smooth bell of level, panned along with the
 * picture's motion (wind over the dunes, the sea, the rise, the fall).
 */
export function air(
	k: Kit,
	b: Bus,
	when: number,
	dur: number,
	o: Out & {
		freq: [number, number];
		q?: number;
		peak?: number;
		panFrom?: number;
		panTo?: number;
		seed?: number;
		lfo?: number;
	}
) {
	const { q = 0.9, peak = 0.5, panFrom = 0, panTo = 0, seed = 11 } = o;
	const src = k.noiseAt(b, 'pink', when, dur + 0.1, seed);
	const bp = k.filter('bandpass', o.freq[0], q);
	bp.frequency.setValueAtTime(o.freq[0], when);
	bp.frequency.exponentialRampToValueAtTime(o.freq[1], when + dur);
	const env = k.gain(0);
	const n = 64;
	const curve = new Float32Array(n);
	for (let i = 0; i < n; i++) {
		const x = i / (n - 1);
		const y =
			x < peak
				? Math.sin((x / peak) * Math.PI * 0.5) ** 2
				: Math.cos(((x - peak) / (1 - peak)) * Math.PI * 0.5) ** 2;
		curve[i] = y;
	}
	env.gain.setValueCurveAtTime(curve, when, dur);
	const pan = k.pan(panFrom);
	pan.pan.setValueAtTime(panFrom, when);
	pan.pan.linearRampToValueAtTime(panTo, when + dur);
	let node: AudioNode = src
		.connect(k.filter('highpass', 120, 0.6))
		.connect(bp)
		.connect(env);
	if (o.lfo) {
		// slow gusts
		const g = k.gain(0.75);
		const l = k.osc(b, 'sine', o.lfo, when, dur + 0.1);
		l.connect(k.gain(0.25)).connect(g.gain);
		node = node.connect(g);
	}
	node.connect(pan);
	route(k, b, pan, o);
}

// --- the landing ---------------------------------------------------------------------------------

/**
 * The impact: a sub drop tuned to land on the key's root, a body thump, a soft short transient
 * (filtered, never a click) and a long low tail into the reverb.
 */
export function impact(
	k: Kit,
	b: Bus,
	when: number,
	o: Out & { root?: number; seed?: number } = {}
) {
	const { root = hz('D2'), seed = 21 } = o;
	const g = o.gain ?? 1;
	// sub drop: from well above down to the root (D2: TV speakers still hear it), with the
	// octave below underneath for big systems
	const s = k.osc(b, 'sine', root * 4, when, 4);
	s.frequency.setValueAtTime(root * 3.2, when);
	s.frequency.exponentialRampToValueAtTime(root * 1.15, when + 0.16);
	s.frequency.exponentialRampToValueAtTime(root, when + 0.6);
	const se = k.gain(0);
	se.gain.setValueAtTime(0, when);
	se.gain.linearRampToValueAtTime(0.8, when + 0.006);
	se.gain.setTargetAtTime(0.3, when + 0.05, 0.15);
	se.gain.setTargetAtTime(0, when + 0.45, 0.45);
	s.connect(saturate(k, 2))
		.connect(k.filter('lowpass', 320, 0.7))
		.connect(se);
	route(k, b, se, { gain: g, verb: 0.06 });
	const deep = k.osc(b, 'sine', root, when, 3.5);
	deep.frequency.setValueAtTime(root, when);
	deep.frequency.exponentialRampToValueAtTime(root / 2, when + 0.5);
	const de = k.gain(0);
	de.gain.setValueAtTime(0, when);
	de.gain.linearRampToValueAtTime(0.2, when + 0.02);
	de.gain.setTargetAtTime(0, when + 0.4, 0.55);
	deep.connect(de);
	route(k, b, de, { gain: g });
	// body: a short tuned thump and a puff of low noise
	const body = k.osc(b, 'sine', 150, when, 0.5);
	body.frequency.setValueAtTime(170, when);
	body.frequency.exponentialRampToValueAtTime(62, when + 0.11);
	const be = k.gain(0);
	be.gain.setValueAtTime(0, when);
	be.gain.linearRampToValueAtTime(0.6, when + 0.003);
	be.gain.setTargetAtTime(0, when + 0.02, 0.07);
	body.connect(saturate(k, 1.6)).connect(be);
	route(k, b, be, { gain: g, verb: 0.35 });
	const thud = k.noiseAt(b, 'pink', when, 0.6, seed);
	const tl = k.filter('lowpass', 700, 0.7);
	tl.frequency.setValueAtTime(1400, when);
	tl.frequency.exponentialRampToValueAtTime(260, when + 0.25);
	const te = k.gain(0);
	te.gain.setValueAtTime(0, when);
	te.gain.linearRampToValueAtTime(0.55, when + 0.004);
	te.gain.setTargetAtTime(0, when + 0.02, 0.09);
	thud.connect(tl).connect(te);
	route(k, b, te, { gain: g, verb: 0.5 });
	// the transient: a soft, band-limited tick of air
	const tick = k.noiseAt(b, 'white', when, 0.05, seed + 1);
	const tb = k.filter('bandpass', 2600, 0.8);
	const tk = k.gain(0);
	tk.gain.setValueAtTime(0, when);
	tk.gain.linearRampToValueAtTime(0.16, when + 0.0015);
	tk.gain.setTargetAtTime(0, when + 0.002, 0.009);
	tick.connect(tb).connect(tk);
	route(k, b, tk, { gain: g, verb: 0.6 });
	// the tail: low rumble breathing out over two seconds, mostly into the reverb
	const rum = k.noiseAt(b, 'pink', when, 3.2, seed + 2);
	const rl = k.filter('lowpass', 150, 0.8);
	const re = k.gain(0);
	re.gain.setValueAtTime(0, when);
	re.gain.linearRampToValueAtTime(0.5, when + 0.03);
	re.gain.setTargetAtTime(0, when + 0.1, 0.7);
	rum.connect(rl).connect(re);
	route(k, b, re, { gain: g, verb: 0.8 });
}

/**
 * Shimmer: the chord's upper notes an octave up on soft triangles, band-limited to the air
 * above 2 kHz, breathing slowly. It sits behind the pad and lends it light.
 */
export function shimmer(
	k: Kit,
	b: Bus,
	when: number,
	chord: (string | number)[],
	dur: number,
	o: Out & { attack?: number; release?: number; seed?: number } = {}
) {
	const { attack = 1, release = 1.5, seed = 7 } = o;
	const r = rng(seed);
	const total = attack + dur + release * 2.5;
	const sum = k.gain(1);
	chord.map(note).forEach((f, i) => {
		const osc = k.oscAt(b, 'triangle', f * 2, when, total + 0.05, r);
		osc.detune.value = (r() - 0.5) * 8;
		const trem = k.gain(0.7);
		const l = k.oscAt(b, 'sine', 0.35 + r() * 0.4, when, total + 0.05, r);
		l.connect(k.gain(0.3)).connect(trem.gain);
		osc
			.connect(trem)
			.connect(k.pan((i % 2 ? 1 : -1) * (0.4 + 0.4 * r())))
			.connect(sum);
	});
	const env = k.gain(0);
	env.gain.setValueAtTime(0, when);
	env.gain.linearRampToValueAtTime(0.05 / Math.sqrt(chord.length), when + attack);
	env.gain.setValueAtTime(0.05 / Math.sqrt(chord.length), when + attack + dur);
	env.gain.setTargetAtTime(0, when + attack + dur, release / 4);
	sum
		.connect(k.filter('highpass', 1600, 0.6))
		.connect(k.filter('lowpass', 9000, 0.5))
		.connect(env);
	route(k, b, env, o);
}

/** The last of the ring-out, faded so the very end is silence (a sine-shaped fade). */
export function fadeOut(k: Kit, b: Bus, when: number, dur: number) {
	const g = chain(k, b).out.gain;
	const n = 64;
	const curve = new Float32Array(n);
	for (let i = 0; i < n; i++) curve[i] = MIX * Math.cos((i / (n - 1)) * Math.PI * 0.5) ** 2;
	g.setValueCurveAtTime(curve, when, dur);
}
