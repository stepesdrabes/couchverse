// The morph ident's instruments, all synthesized: a felt piano (inharmonic partials, two-stage
// decay, detuned unison strings, a soft hammer), a warm analog pad (detuned soft saws with
// slow drift through one shared filter), a glass layer, a bowed swell, a sub, a heartbeat, a
// layered impact, airy swells and a generated hall. Every voice takes an absolute context
// time and its own seed, so a render is a pure function of the score.

import type { Bus } from '../../../audio/engine';
import type { Kit } from '../../../audio/kit';
import { rng } from '../../../core/rng';

/** A note name (A4, F#3, Bb1) in Hz. */
export function hz(name: string): number {
	const m = /^([A-G])([b#]?)(-?\d)$/.exec(name);
	if (!m) throw new Error(`bad note ${name}`);
	const pc =
		{ C: 0, D: 2, E: 4, F: 5, G: 7, A: 9, B: 11 }[m[1]]! +
		(m[2] === '#' ? 1 : m[2] === 'b' ? -1 : 0);
	return 440 * 2 ** ((pc + 12 * (Number(m[3]) + 1) - 69) / 12);
}

// --- the mix: buses, the hall ------------------------------------------------------------------

export interface Mix {
	k: Kit;
	b: Bus;
	/** Everything ends here (into the piece's music bus). */
	out: GainNode;
	/** The hall's input. */
	verb: GainNode;
	/** The pad bus: its filter is the score's slow sweep, its gain the score's swells. */
	pad: GainNode;
	padCut: BiquadFilterNode[];
	padLevel: GainNode;
	/** The low end: sub, heartbeat and impact (dry, mono-centred). */
	low: GainNode;
	noise: AudioBuffer;
}

const halls = new WeakMap<BaseAudioContext, AudioBuffer>();
const noises = new WeakMap<BaseAudioContext, AudioBuffer>();

/**
 * A long, smooth hall: no discrete reflections (pads and piano want a clean bloom), a diffuse
 * onset over 60 ms after a 22 ms pre-delay, then a tail that decays per band (lows longest,
 * highs first), decorrelated between the channels, with a slow fade of brightness.
 */
function hall(ctx: BaseAudioContext, seconds = 5.6): AudioBuffer {
	const cached = halls.get(ctx);
	if (cached) return cached;
	const sr = ctx.sampleRate;
	const len = Math.floor(sr * seconds);
	const pre = Math.floor(sr * 0.022);
	const buf = ctx.createBuffer(2, len, sr);
	const T60 = { low: 4.6, mid: 3.6, high: 1.5 };
	const k = (t60: number) => 6.91 / t60;
	const aLow = 1 - Math.exp((-2 * Math.PI * 380) / sr);
	const aMid = 1 - Math.exp((-2 * Math.PI * 3600) / sr);
	for (let ch = 0; ch < 2; ch++) {
		const r = rng(907 + ch * 331);
		const d = buf.getChannelData(ch);
		let lo = 0;
		let mi = 0;
		for (let i = pre; i < len; i++) {
			const t = (i - pre) / sr;
			const w = r() * 2 - 1;
			lo += aLow * (w - lo);
			mi += aMid * (w - mi);
			const build = 1 - Math.exp(-t / 0.06);
			// the very end fades to zero so the convolution never stops on a step
			const end = Math.min(1, (len - i) / (sr * 0.25));
			d[i] =
				(lo * 2.2 * Math.exp(-k(T60.low) * t) +
					(mi - lo) * Math.exp(-k(T60.mid) * t) +
					(w - mi) * 0.35 * Math.exp(-k(T60.high) * t)) *
				build *
				end;
		}
	}
	halls.set(ctx, buf);
	return buf;
}

/** Twelve seconds of seeded pink noise per channel: long enough never to loop audibly. */
function pink(ctx: BaseAudioContext): AudioBuffer {
	const cached = noises.get(ctx);
	if (cached) return cached;
	const sr = ctx.sampleRate;
	const len = sr * 12;
	const buf = ctx.createBuffer(2, len, sr);
	for (let ch = 0; ch < 2; ch++) {
		const r = rng(5101 + ch * 17);
		const d = buf.getChannelData(ch);
		let b0 = 0,
			b1 = 0,
			b2 = 0,
			b3 = 0,
			b4 = 0,
			b5 = 0,
			b6 = 0;
		for (let i = 0; i < len; i++) {
			const w = r() * 2 - 1;
			b0 = 0.99886 * b0 + w * 0.0555179;
			b1 = 0.99332 * b1 + w * 0.0750759;
			b2 = 0.969 * b2 + w * 0.153852;
			b3 = 0.8665 * b3 + w * 0.3104856;
			b4 = 0.55 * b4 + w * 0.5329522;
			b5 = -0.7616 * b5 - w * 0.016898;
			d[i] = (b0 + b1 + b2 + b3 + b4 + b5 + b6 + w * 0.5362) * 0.11;
			b6 = w * 0.115926;
		}
	}
	noises.set(ctx, buf);
	return buf;
}

/**
 * Pins a node to two channels. A node's channel count otherwise follows its inputs, so when a
 * stereo source (noise) ends among mono ones (sines), the chain drops to mono mid-note: the
 * filters' state and the panners' law change at once, and that step is a click.
 */
export function stereo<T extends AudioNode>(node: T): T {
	node.channelCount = 2;
	node.channelCountMode = 'explicit';
	node.channelInterpretation = 'speakers';
	return node;
}

const curves = new Map<number, Float32Array<ArrayBuffer>>();

/** tanh saturation, normalized so full scale stays full scale. */
export function warm(k: Kit, drive: number): WaveShaperNode {
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
	const s = k.ctx.createWaveShaper();
	s.curve = c;
	s.oversample = '4x';
	return s;
}

export function makeMix(k: Kit, b: Bus): Mix {
	const ctx = k.ctx;
	const out = stereo(k.gain(1));
	const rumble = k.filter('highpass', 24, 0.7);
	// a breath of air on top: the mix is warm, not dull
	const air = k.filter('highshelf', 6500, 0.7);
	air.gain.value = 2.5;
	out.connect(rumble).connect(air).connect(b.music);

	// the hall, kept out of the mud and the fizz
	const verb = stereo(k.gain(1));
	const conv = ctx.createConvolver();
	conv.buffer = hall(ctx);
	const vHp = k.filter('highpass', 170, 0.6);
	const vDip = k.filter('peaking', 420, 0.9);
	vDip.gain.value = -3;
	const vLp = k.filter('lowpass', 8500, 0.6);
	const vOut = k.gain(0.8);
	verb.connect(vHp).connect(vDip).connect(vLp).connect(conv).connect(vOut).connect(out);

	// the pad bus: two gentle lowpasses (the sweep), warmth, a dip at 300 Hz, a stereo chorus
	const pad = stereo(k.gain(1));
	const padCut = [k.filter('lowpass', 600, 0.55), k.filter('lowpass', 600, 0.55)];
	const padHp = k.filter('highpass', 150, 0.6);
	const padDip = k.filter('peaking', 310, 0.8);
	padDip.gain.value = -3.5;
	const sat = warm(k, 1.25);
	const padLevel = k.gain(1);
	pad
		.connect(padCut[0])
		.connect(padCut[1])
		.connect(padHp)
		.connect(padDip)
		.connect(sat)
		.connect(padLevel);
	const dry = k.gain(0.7);
	padLevel.connect(dry).connect(out);
	// chorus: two slowly modulated delays, one per side
	const merge = ctx.createChannelMerger(2);
	[0, 1].forEach((side) => {
		const dl = ctx.createDelay(0.05);
		dl.delayTime.value = side ? 0.0171 : 0.0123;
		const lfo = ctx.createOscillator();
		lfo.frequency.value = side ? 0.31 : 0.23;
		const depth = k.gain(0.0024);
		lfo.connect(depth).connect(dl.delayTime);
		lfo.start(0);
		lfo.stop(12);
		k.track(b, lfo);
		const g = k.gain(0.55);
		const mono = k.gain(1);
		mono.channelCount = 1;
		mono.channelCountMode = 'explicit';
		mono.channelInterpretation = 'speakers';
		padLevel.connect(mono).connect(dl).connect(g).connect(merge, 0, side);
	});
	merge.connect(out);
	const padSend = k.gain(0.42);
	padLevel.connect(padSend).connect(verb);

	const low = stereo(k.gain(1));
	low.connect(out);
	return { k, b, out, verb, pad, padCut, padLevel, low, noise: pink(ctx) };
}

// --- shared bits -------------------------------------------------------------------------------

const waves = new WeakMap<BaseAudioContext, PeriodicWave>();

/** A saw with its top rolled off a little: warmer than the bare saw before any filter. */
function softSaw(k: Kit): PeriodicWave {
	let w = waves.get(k.ctx);
	if (!w) {
		const n = 64;
		const real = new Float32Array(n);
		const imag = new Float32Array(n);
		for (let h = 1; h < n; h++) imag[h] = (1 / h ** 1.08) * Math.exp(-h / 48);
		w = k.ctx.createPeriodicWave(real, imag);
		waves.set(k.ctx, w);
	}
	return w;
}

function osc(
	m: Mix,
	type: OscillatorType | PeriodicWave,
	f: number,
	when: number,
	end: number,
	phase: number
) {
	const o = m.k.ctx.createOscillator();
	if (typeof type === 'string') o.type = type as OscillatorType;
	else o.setPeriodicWave(type);
	o.frequency.value = f;
	// started part of a cycle early, so stacked voices never start in phase
	o.start(Math.max(0, when - phase / f));
	o.stop(end);
	m.k.track(m.b, o);
	return o;
}

function noise(m: Mix, when: number, dur: number, seed: number, rate = 1) {
	const s = m.k.ctx.createBufferSource();
	s.buffer = m.noise;
	s.loop = true;
	s.playbackRate.value = rate;
	s.start(when, ((seed * 0.6180339) % 1) * 10);
	s.stop(when + dur);
	m.k.track(m.b, s);
	return s;
}

/** A gain following points [t, v] (seconds from `when`), linear between them. */
function curve(m: Mix, when: number, pts: [number, number][]): GainNode {
	const g = m.k.gain(0);
	g.gain.setValueAtTime(pts[0][1], when + pts[0][0]);
	for (const [t, v] of pts.slice(1)) g.gain.linearRampToValueAtTime(v, when + t);
	return g;
}

function panTo(m: Mix, node: AudioNode, pan: number, dest: AudioNode, verb = 0) {
	const p = m.k.pan(pan);
	node.connect(p).connect(dest);
	if (verb > 0) p.connect(m.k.gain(verb)).connect(m.verb);
	return p;
}

// --- the felt piano ----------------------------------------------------------------------------

export interface PianoOptions {
	/** 0..1: louder is brighter and harder. */
	vel?: number;
	/** Seconds until the damper; the note rings until then. */
	dur?: number;
	pan?: number;
	verb?: number;
	gain?: number;
	seed?: number;
	dest?: AudioNode;
}

/**
 * A felt piano note: up to 14 inharmonic partials (stiff-string stretch), each a pair of
 * slightly detuned strings beating against each other, decaying in two stages (the prompt
 * sound, then the long aftersound), higher partials dying first; a soft felt hammer under it;
 * the damper fades it out.
 */
export function piano(m: Mix, when: number, f0: number, o: PianoOptions = {}) {
	const { vel = 0.6, dur = 4, pan = 0, verb = 0.5, gain = 1, seed = 1 } = o;
	const k = m.k;
	const r = rng(seed);
	const B = 0.00018 * Math.max(0.6, Math.min(2.2, f0 / 260));
	// sources stop only when their envelopes are long gone (no residual step)
	const end = when + dur + 1.2;
	// the hammer's noise is stereo and the strings are not: one stereo stream throughout
	const sum = stereo(k.gain(1));
	const tone = k.filter('lowpass', Math.min(16000, f0 * (5 + 9 * vel) + 1200), 0.5);
	sum.connect(tone);
	const level = 0.075 * gain * (0.35 + 0.65 * vel);
	// the register's sustain (the aftersound's time constant): felt-muted, low notes longest
	const sustain = Math.max(0.8, 2.6 - Math.log2(f0 / 110) * 0.55);
	const top = Math.min(14, Math.floor(7000 / f0));
	for (let n = 1; n <= top; n++) {
		const fn = n * f0 * Math.sqrt(1 + B * n * n);
		// felt: a soft hammer rolls the spectrum off, harder hits keep more of the top
		const a = (1 / n ** 1.2) * Math.exp(-(n - 1) / (1.6 + 4.2 * vel));
		if (a < 0.004) continue;
		const slow = sustain / (1 + 0.32 * (n - 1));
		const fast = Math.min(slow, 0.26 / (1 + 0.1 * n));
		const strings = n <= 7 ? 2 : 1;
		for (let s = 0; s < strings; s++) {
			const o2 = osc(m, 'sine', fn, when, end, r());
			if (strings > 1) o2.detune.value = (s ? 1 : -1) * (0.35 + r() * 0.7);
			const g = k.gain(0);
			const peak = (level * a) / strings;
			const hit = when + 0.004 + 0.002 * n;
			g.gain.setValueAtTime(0, when);
			g.gain.linearRampToValueAtTime(peak, hit);
			// prompt sound, then aftersound, then the damper
			g.gain.setTargetAtTime(peak * 0.32, hit, fast);
			g.gain.setTargetAtTime(0, hit + fast * 2.2, slow);
			g.gain.setTargetAtTime(0, Math.max(hit + fast * 2.2 + 0.01, when + dur), 0.12);
			o2.connect(g).connect(sum);
		}
	}
	// the hammer: a breath of low-passed noise and a soft knock
	const hn = noise(m, when, 0.3, seed + 3);
	const hlp = k.filter('lowpass', 900 + 2600 * vel, 0.7);
	const hbp = k.filter('highpass', 120, 0.7);
	const hg = k.gain(0);
	hg.gain.setValueAtTime(0, when);
	hg.gain.linearRampToValueAtTime(level * 0.16 * (0.4 + vel), when + 0.002);
	hg.gain.setTargetAtTime(0, when + 0.003, 0.012);
	hn.connect(hlp).connect(hbp).connect(hg).connect(sum);
	panTo(m, tone, pan, o.dest ?? m.out, verb);
}

// --- the pad -----------------------------------------------------------------------------------

export interface PadOptions {
	attack?: number;
	release?: number;
	level?: number;
	/** Cents between the outer voices of a note. */
	spread?: number;
	seed?: number;
}

/**
 * A pad chord into the pad bus: six detuned soft saws per note, spread across the stereo
 * field, each drifting slowly on its own; held for `dur`, crossfading by its envelope.
 */
export function pad(m: Mix, when: number, notes: number[], dur: number, o: PadOptions = {}) {
	const { attack = 1.0, release = 1.4, level = 1, spread = 22, seed = 7 } = o;
	const k = m.k;
	const r = rng(seed);
	const end = when + attack + dur + release * 2.4 + 0.2;
	const env = k.gain(0);
	const peak = (0.11 * level) / Math.sqrt(notes.length);
	// an S-shaped swell in (the gain starts at 0), a long fade out
	env.gain.setValueCurveAtTime(swell(peak), when, attack);
	env.gain.setValueAtTime(peak, when + attack + dur);
	env.gain.setTargetAtTime(0, when + attack + dur, release / 3.5);
	env.connect(m.pad);
	const wave = softSaw(k);
	notes.forEach((f, i) => {
		const voices = 6;
		for (let v = 0; v < voices; v++) {
			const o2 = osc(m, wave, f, when, end, r());
			const c = ((v / (voices - 1)) * 2 - 1) * (spread / 2);
			o2.detune.value = c + (r() - 0.5) * 3;
			// a slow, private drift: the voices never lock
			const lfo = osc(m, 'sine', 0.07 + r() * 0.2, when, end, r() * 0.07);
			lfo.connect(k.gain(2.5 + r() * 2.5)).connect(o2.detune);
			const side = ((v % 2 ? 1 : -1) * (0.25 + 0.6 * (v / voices)) + (i % 2 ? 0.08 : -0.08)) * 0.95;
			const p = k.pan(Math.max(-1, Math.min(1, side)));
			o2.connect(k.gain(1 / voices))
				.connect(p)
				.connect(env);
		}
	});
}

/** A raised-cosine swell from 0 to peak (setValueCurveAtTime). */
function swell(peak: number, n = 64): Float32Array<ArrayBuffer> {
	const c = new Float32Array(n);
	for (let i = 0; i < n; i++) c[i] = peak * (0.5 - 0.5 * Math.cos((Math.PI * i) / (n - 1)));
	return c;
}

// --- glass and bowed ---------------------------------------------------------------------------

/** A glass tone: a sine with soft harmonic partials, long and pure (no FM, no clang). */
export function glass(
	m: Mix,
	when: number,
	f0: number,
	o: { gain?: number; decay?: number; pan?: number; verb?: number; seed?: number } = {}
) {
	const { gain = 1, decay = 3.5, pan = 0, verb = 0.6, seed = 21 } = o;
	const k = m.k;
	const r = rng(seed);
	const sum = k.gain(1);
	const parts: [number, number, number][] = [
		[1, 1, decay],
		[2, 0.22, decay * 0.55],
		[3, 0.06, decay * 0.35],
		[4.02, 0.025, decay * 0.25]
	];
	for (const [ratio, a, d] of parts) {
		if (f0 * ratio > 9000) continue;
		const o2 = osc(m, 'sine', f0 * ratio, when, when + d * 3 + 0.2, r());
		const g = k.gain(0);
		g.gain.setValueAtTime(0, when);
		// a bloom, not a strike: the glass is a halo on the piano, never a beep of its own
		g.gain.linearRampToValueAtTime(0.026 * gain * a, when + 0.035);
		g.gain.setTargetAtTime(0, when + 0.035, d / 3);
		o2.connect(g).connect(sum);
	}
	panTo(m, sum, pan, m.out, verb);
}

/** A bowed swell: three detuned saws, filtered dark, a late vibrato; sustains a melody note. */
export function bowed(
	m: Mix,
	when: number,
	f0: number,
	dur: number,
	o: {
		gain?: number;
		attack?: number;
		release?: number;
		pan?: number;
		verb?: number;
		seed?: number;
	} = {}
) {
	const { gain = 1, attack = 0.28, release = 0.9, pan = 0, verb = 0.55, seed = 31 } = o;
	const k = m.k;
	const r = rng(seed);
	const end = when + attack + dur + release * 2.4 + 0.2;
	const lp = k.filter('lowpass', f0 * 3.2, 0.6);
	lp.frequency.setValueAtTime(f0 * 1.6, when);
	lp.frequency.linearRampToValueAtTime(f0 * 3.6, when + attack + 0.2);
	// a second pole pair: a string's top is smooth, not buzzy
	const lp2 = k.filter('lowpass', Math.min(12000, f0 * 5), 0.6);
	const g = k.gain(0);
	const peak = 0.03 * gain;
	g.gain.setValueCurveAtTime(swell(peak), when, attack);
	g.gain.setValueAtTime(peak, when + attack + dur);
	g.gain.setTargetAtTime(0, when + attack + dur, release / 3.5);
	const wave = softSaw(k);
	for (let v = 0; v < 3; v++) {
		const o2 = osc(m, wave, f0, when, end, r());
		o2.detune.value = (v - 1) * 7 + (r() - 0.5) * 2;
		const vib = osc(m, 'sine', 4.8 + r() * 0.6, when, end, r());
		const depth = k.gain(0);
		depth.gain.setValueAtTime(0, when);
		depth.gain.linearRampToValueAtTime(0, when + attack);
		depth.gain.linearRampToValueAtTime(7, when + attack + 0.5);
		vib.connect(depth).connect(o2.detune);
		o2.connect(k.pan((v - 1) * 0.5)).connect(lp);
	}
	lp.connect(lp2).connect(g);
	panTo(m, g, pan, m.out, verb);
}

// --- the low end -------------------------------------------------------------------------------

/** A sub note: a sine with a touch of its octave, glided in, softly saturated. */
export function sub(
	m: Mix,
	when: number,
	f: number,
	dur: number,
	o: { gain?: number; attack?: number; release?: number; glideFrom?: number } = {}
) {
	const { gain = 1, attack = 0.5, release = 0.8 } = o;
	const k = m.k;
	const end = when + attack + dur + release * 2.4 + 0.3;
	const o1 = osc(m, 'sine', f, when, end, 0);
	if (o.glideFrom) {
		o1.frequency.setValueAtTime(o.glideFrom, when);
		o1.frequency.exponentialRampToValueAtTime(f, when + 0.35);
	}
	const o2 = osc(m, 'sine', f * 2, when, end, 0.25);
	if (o.glideFrom) {
		o2.frequency.setValueAtTime(o.glideFrom * 2, when);
		o2.frequency.exponentialRampToValueAtTime(f * 2, when + 0.35);
	}
	const mix = k.gain(1);
	o1.connect(mix);
	o2.connect(k.gain(0.1)).connect(mix);
	const g = k.gain(0);
	const peak = 0.058 * gain;
	g.gain.setValueCurveAtTime(swell(peak), when, attack);
	g.gain.setValueAtTime(peak, when + attack + dur);
	g.gain.setTargetAtTime(0, when + attack + dur, release / 3.5);
	mix.connect(warm(k, 1.4)).connect(g).connect(m.low);
}

/** A heartbeat: a low, round thump with no click. */
export function thump(m: Mix, when: number, o: { gain?: number; f?: number } = {}) {
	const { gain = 1, f = 52 } = o;
	const k = m.k;
	const o1 = osc(m, 'sine', f, when, when + 1.0, 0);
	o1.frequency.setValueAtTime(f * 2.1, when);
	o1.frequency.exponentialRampToValueAtTime(f, when + 0.07);
	const g = k.gain(0);
	g.gain.setValueAtTime(0, when);
	g.gain.linearRampToValueAtTime(0.22 * gain, when + 0.006);
	g.gain.setTargetAtTime(0, when + 0.02, 0.085);
	const lp = k.filter('lowpass', 180, 0.6);
	o1.connect(warm(k, 1.8)).connect(lp).connect(g).connect(m.low);
	// a felt-soft upper body so it reads on small speakers
	const n = noise(m, when, 0.3, Math.round(when * 1000));
	const bp = k.filter('bandpass', 160, 1.1);
	const ng = k.gain(0);
	ng.gain.setValueAtTime(0, when);
	ng.gain.linearRampToValueAtTime(0.05 * gain, when + 0.004);
	ng.gain.setTargetAtTime(0, when + 0.008, 0.025);
	n.connect(bp).connect(ng).connect(m.low);
}

/**
 * The landing: a sub drop to the tonic, a round body, a short soft crack of noise, a low
 * rumble and the hall: deep and wide, not a bang.
 */
export function impact(m: Mix, when: number, o: { gain?: number; to?: number } = {}) {
	const { gain = 1, to = 36.71 } = o;
	const k = m.k;
	// the drop
	const s = osc(m, 'sine', to, when, when + 4.4, 0);
	s.frequency.setValueAtTime(to * 3.2, when);
	s.frequency.exponentialRampToValueAtTime(to * 1.25, when + 0.09);
	s.frequency.exponentialRampToValueAtTime(to, when + 0.6);
	const sg = k.gain(0);
	sg.gain.setValueAtTime(0, when);
	sg.gain.linearRampToValueAtTime(0.45 * gain, when + 0.004);
	sg.gain.setTargetAtTime(0.14 * gain, when + 0.02, 0.22);
	sg.gain.setTargetAtTime(0, when + 0.45, 0.55);
	const slp = k.filter('lowpass', 220, 0.6);
	s.connect(warm(k, 2.0)).connect(slp).connect(sg).connect(m.low);
	// the body
	const bo = osc(m, 'sine', 70, when, when + 0.8, 0);
	bo.frequency.setValueAtTime(150, when);
	bo.frequency.exponentialRampToValueAtTime(64, when + 0.1);
	const bg = k.gain(0);
	bg.gain.setValueAtTime(0, when);
	bg.gain.linearRampToValueAtTime(0.22 * gain, when + 0.003);
	bg.gain.setTargetAtTime(0, when + 0.01, 0.09);
	bo.connect(warm(k, 2.4)).connect(bg);
	panTo(m, bg, 0, m.low, 0.25);
	// the crack: brief, dark noise, mostly into the hall
	const n = noise(m, when, 0.5, 4242);
	const nhp = k.filter('highpass', 500, 0.7);
	const nlp = k.filter('lowpass', 5200, 0.6);
	nlp.frequency.setValueAtTime(5200, when);
	nlp.frequency.exponentialRampToValueAtTime(900, when + 0.25);
	const ng = k.gain(0);
	ng.gain.setValueAtTime(0, when);
	ng.gain.linearRampToValueAtTime(0.16 * gain, when + 0.0015);
	ng.gain.setTargetAtTime(0, when + 0.003, 0.03);
	n.connect(nhp).connect(nlp).connect(ng);
	panTo(m, ng, 0, m.out, 1.2);
	// the rumble that the hall carries away
	const ru = noise(m, when, 4.4, 777, 0.5);
	const rlp = k.filter('lowpass', 140, 0.7);
	const rg = k.gain(0);
	rg.gain.setValueAtTime(0, when);
	rg.gain.linearRampToValueAtTime(0.1 * gain, when + 0.02);
	rg.gain.setTargetAtTime(0, when + 0.05, 0.7);
	ru.connect(rlp).connect(rg).connect(m.low);
}

// --- air ---------------------------------------------------------------------------------------

/**
 * An airy swell for a morph: pink noise through a soft band that opens and closes, panned
 * along with the move, peaking at `peakAt` (seconds after when).
 */
export function air(
	m: Mix,
	when: number,
	dur: number,
	o: {
		peakAt?: number;
		gain?: number;
		lo?: number;
		hi?: number;
		pan?: [number, number];
		verb?: number;
		seed?: number;
	} = {}
) {
	const {
		peakAt = dur * 0.6,
		gain = 1,
		lo = 300,
		hi = 1800,
		pan = [0, 0],
		verb = 0.5,
		seed = 61
	} = o;
	const k = m.k;
	const n = noise(m, when, dur + 0.1, seed);
	const bp = k.filter('bandpass', lo, 0.8);
	bp.frequency.setValueAtTime(lo, when);
	bp.frequency.exponentialRampToValueAtTime(hi, when + peakAt);
	bp.frequency.exponentialRampToValueAtTime(lo * 1.2, when + dur);
	const lp = k.filter('lowpass', 5000, 0.5);
	const g = curve(m, when, [
		[0, 0],
		[peakAt * 0.5, 0.06 * gain],
		[peakAt, 0.22 * gain],
		[peakAt + (dur - peakAt) * 0.35, 0.07 * gain],
		[dur, 0]
	]);
	const p = k.pan(pan[0]);
	p.pan.setValueAtTime(pan[0], when);
	p.pan.linearRampToValueAtTime(pan[1], when + dur);
	n.connect(bp).connect(lp).connect(g).connect(p).connect(m.out);
	if (verb > 0) p.connect(k.gain(verb)).connect(m.verb);
}

/**
 * A reversed swell into `at`: chord partials rising from nothing, the higher ones later, the
 * way a reversed piano or a reversed hall sounds; cut short just before `at`.
 */
export function reverse(
	m: Mix,
	at: number,
	notes: number[],
	dur: number,
	o: { gain?: number; seed?: number; cut?: number } = {}
) {
	const { gain = 1, seed = 71, cut = 0.03 } = o;
	const k = m.k;
	const r = rng(seed);
	const when = at - dur;
	const stop = at - cut;
	const sum = k.gain(1);
	const lp = k.filter('lowpass', 500, 0.6);
	lp.frequency.setValueAtTime(400, when);
	lp.frequency.exponentialRampToValueAtTime(5200, stop);
	notes.forEach((f, i) => {
		for (const [n, a] of [
			[1, 1],
			[2, 0.4],
			[3, 0.16],
			[4, 0.07]
		] as const) {
			const o2 = osc(m, 'sine', f * n, when, stop + 0.02, r());
			o2.detune.value = (r() - 0.5) * 8;
			const g = k.gain(0);
			const peak = (0.045 * gain * a) / Math.sqrt(notes.length);
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
	panTo(m, lp, 0, m.out, 0.35);
	// the breath of it: noise rising with the chord
	const nz = noise(m, when, dur, seed + 5);
	const nbp = k.filter('bandpass', 600, 0.7);
	nbp.frequency.setValueAtTime(600, when);
	nbp.frequency.exponentialRampToValueAtTime(4200, stop);
	const ng = k.gain(0);
	ng.gain.setValueAtTime(0.0001, when);
	ng.gain.exponentialRampToValueAtTime(0.09 * gain, stop);
	ng.gain.linearRampToValueAtTime(0, stop + 0.01);
	nz.connect(nbp).connect(ng);
	panTo(m, ng, 0, m.out, 0.3);
}

const shimmers = new WeakMap<BaseAudioContext, PeriodicWave>();

/**
 * A shimmer: high, soft voices (a sine with a few gentle harmonics), slowly beating against
 * each other, high-passed and mostly in the hall. The light over the arrival.
 */
export function shimmer(
	m: Mix,
	when: number,
	notes: number[],
	dur: number,
	o: { gain?: number; attack?: number; release?: number; seed?: number } = {}
) {
	const { gain = 1, attack = 0.4, release = 1.6, seed = 81 } = o;
	const k = m.k;
	const r = rng(seed);
	let wave = shimmers.get(k.ctx);
	if (!wave) {
		const real = new Float32Array(7);
		const imag = Float32Array.from([0, 1, 0.42, 0.18, 0.08, 0.035, 0.015]);
		wave = k.ctx.createPeriodicWave(real, imag);
		shimmers.set(k.ctx, wave);
	}
	const end = when + attack + dur + release * 2.4 + 0.3;
	const hp = k.filter('highpass', 900, 0.6);
	const g = k.gain(0);
	const peak = (0.03 * gain) / Math.sqrt(notes.length);
	g.gain.setValueCurveAtTime(swell(peak), when, attack);
	g.gain.setValueAtTime(peak, when + attack + dur);
	g.gain.setTargetAtTime(0, when + attack + dur, release / 3.5);
	notes.forEach((f, i) => {
		for (let v = 0; v < 4; v++) {
			const o2 = osc(m, wave, f, when, end, r());
			o2.detune.value = (v - 1.5) * 5 + (r() - 0.5) * 2;
			const p = k.pan(((v % 2 ? 1 : -1) * (0.3 + 0.5 * (v / 4)) + (i % 2 ? 0.1 : -0.1)) * 0.9);
			o2.connect(k.gain(0.25)).connect(p).connect(hp);
		}
	});
	hp.connect(g);
	g.connect(m.out);
	g.connect(k.gain(0.9)).connect(m.verb);
}
