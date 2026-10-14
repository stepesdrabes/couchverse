// The assemble ident's instruments, all synthesized: a long generated room, lush detuned pads
// through a chorus, a glass mallet made of harmonic partials (no FM, so no harsh sidebands), a
// warm sub, a felted pulse, the locks' soft thuds, the landing's impact and the air that carries
// the build. Every voice takes an absolute context time and a seed, so a render is a pure
// function of the score.
//
// Everything plays into the piece's room: a dry chain (a rumble filter, a 300 Hz dip, a soft
// high shelf, gentle tape saturation) and a send into a 6 s generated reverb, both into the
// piece's music bus. Noise sources run as long as the voice they belong to: one that stops
// early while its chain is still live leaves a step in the output a few render quanta later.

import type { Bus } from '../../../audio/engine';
import type { Kit } from '../../../audio/kit';
import { hz } from '../../../audio/pitch';
import { rng } from '../../../core/rng';

export { hz };

interface Room {
	/** The dry mix input. */
	dry: AudioNode;
	/** The reverb input. */
	wet: AudioNode;
}

const rooms = new WeakMap<Bus, Room>();
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

function sat(k: Kit, drive: number): WaveShaperNode {
	const s = k.ctx.createWaveShaper();
	s.curve = tanh(drive);
	s.oversample = '4x';
	return s;
}

/**
 * A smooth, long room: decorrelated noise per channel whose lows ring longest and highs die
 * first, a 22 ms pre-delay and a soft 70 ms onset (no discrete reflections to flutter).
 */
function impulse(ctx: BaseAudioContext, seconds = 6): AudioBuffer {
	const sr = ctx.sampleRate;
	const len = Math.floor(sr * seconds);
	const pre = Math.floor(sr * 0.022);
	const buf = ctx.createBuffer(2, len, sr);
	const T60 = { low: 4.6, mid: 3.6, high: 1.5 };
	const k = (t60: number) => 6.91 / t60;
	const aLow = 1 - Math.exp((-2 * Math.PI * 380) / sr);
	const aMid = 1 - Math.exp((-2 * Math.PI * 3600) / sr);
	for (let ch = 0; ch < 2; ch++) {
		const r = rng(907 + ch * 31);
		const d = buf.getChannelData(ch);
		let lo = 0;
		let mi = 0;
		for (let i = pre; i < len; i++) {
			const t = (i - pre) / sr;
			const w = r() * 2 - 1;
			lo += aLow * (w - lo);
			mi += aMid * (w - mi);
			const onset = 1 - Math.exp(-t / 0.07);
			// the tail's end fades to zero, so the convolution never stops on a step
			const end = Math.min(1, (len - i) / (sr * 0.4));
			d[i] =
				(lo * 2.2 * Math.exp(-k(T60.low) * t) +
					(mi - lo) * Math.exp(-k(T60.mid) * t) +
					(w - mi) * 0.35 * Math.exp(-k(T60.high) * t)) *
				onset *
				end;
		}
	}
	return buf;
}

/** The piece's room (made once per bus). */
function room(k: Kit, b: Bus): Room {
	let r = rooms.get(b);
	if (r) return r;
	const ctx = k.ctx;
	// dry chain: rumble out, a dip where pads go muddy, a soft top, a little tape
	const dry = k.gain(1);
	const hp = k.filter('highpass', 28, 0.6);
	const dip = ctx.createBiquadFilter();
	dip.type = 'peaking';
	dip.frequency.value = 300;
	dip.Q.value = 0.8;
	dip.gain.value = -3.5;
	const presence = ctx.createBiquadFilter();
	presence.type = 'peaking';
	presence.frequency.value = 3200;
	presence.Q.value = 0.8;
	presence.gain.value = -1;
	const shelf = ctx.createBiquadFilter();
	shelf.type = 'highshelf';
	shelf.frequency.value = 9000;
	shelf.gain.value = -0.5;
	const tape = sat(k, 1.15);
	const out = k.gain(0.9);
	dry
		.connect(hp)
		.connect(dip)
		.connect(presence)
		.connect(shelf)
		.connect(tape)
		.connect(out)
		.connect(b.music);
	// the reverb: high-passed going in (no mud in the tail), soft on the way out
	const wet = k.gain(1);
	const whp = k.filter('highpass', 220, 0.6);
	const conv = ctx.createConvolver();
	conv.buffer = impulse(ctx);
	const wlp = k.filter('lowpass', 10500, 0.5);
	const wret = k.gain(0.9);
	wet.connect(whp).connect(conv).connect(wlp).connect(wret).connect(dry);
	r = { dry, wet };
	rooms.set(b, r);
	return r;
}

const choruses = new WeakMap<Bus, AudioNode>();

/**
 * The pads' chorus (made once per bus): the dry signal in the middle and two copies through
 * slowly swaying delays (12 and 17 ms, a few ms of sway, out of phase) panned hard left and
 * right, the classic way an analog pad gets its width.
 */
function chorus(k: Kit, b: Bus): AudioNode {
	const c = choruses.get(b);
	if (c) return c;
	const ctx = k.ctx;
	const input = k.gain(1);
	const sum = k.gain(1);
	input.connect(k.gain(0.7)).connect(sum);
	const t0 = ctx.currentTime;
	(
		[
			[0.012, 0.27, -1, 0],
			[0.017, 0.31, 1, 0.5]
		] as const
	).forEach(([base, rate, side, phase]) => {
		const d = ctx.createDelay(0.05);
		d.delayTime.value = base;
		const lfo = ctx.createOscillator();
		lfo.frequency.value = rate;
		// start each sway part of a cycle early, so the two never move together
		lfo.start(Math.max(0, t0 - phase / rate));
		lfo.stop(t0 + 30);
		const depth = k.gain(0.0026);
		lfo.connect(depth).connect(d.delayTime);
		input.connect(d).connect(k.gain(0.62)).connect(k.pan(side)).connect(sum);
	});
	const r = room(k, b);
	sum.connect(r.dry);
	choruses.set(b, input);
	return input;
}

interface Out {
	gain?: number;
	pan?: number;
	/** Reverb send (0..1). */
	verb?: number;
	seed?: number;
}

/** Route a voice into the room: dry through a panner (or the pads' chorus), plus a reverb send. */
function out(k: Kit, b: Bus, node: AudioNode, o: Out, verb: number, wide = false) {
	const r = room(k, b);
	const g = k.gain(o.gain ?? 1);
	const p = k.pan(o.pan ?? 0);
	node
		.connect(g)
		.connect(p)
		.connect(wide ? chorus(k, b) : r.dry);
	const v = o.verb ?? verb;
	if (v > 0) {
		const s = k.gain(v);
		p.connect(s).connect(r.wet);
	}
	return p;
}

/** Attack, hold, release (linear attack, exponential tail) on a fresh gain. */
function ahr(k: Kit, when: number, a: number, hold: number, rel: number, peak = 1) {
	const g = k.gain(0);
	const p = g.gain;
	p.setValueAtTime(0, when);
	p.linearRampToValueAtTime(peak, when + a);
	p.setValueAtTime(peak, when + a + hold);
	p.setTargetAtTime(0, when + a + hold, rel / 4);
	return g;
}

const freq = (n: string | number) => (typeof n === 'number' ? n : hz(n));

// --- pads -----------------------------------------------------------------------------------

interface PadOptions extends Out {
	attack?: number;
	release?: number;
	/** Low-pass cutoff at the start and the end of the note (Hz). */
	cutoff?: [number, number];
	/** Detune spread of the saws (cents). */
	spread?: number;
	/** Saw versus triangle (0..1): the triangle is the warm body. */
	bright?: number;
	/** How wide the voices sit (0..1). */
	width?: number;
	/** Gain multiplier reached by the end of the hold (a crescendo). */
	swell?: number;
	/** High-pass (Hz): pads above the bass keep out of its way. */
	floor?: number;
	/** End on a 25 ms fade at the end of the hold instead of the release (into a breath). */
	cut?: boolean;
}

/**
 * A lush pad chord: per note three detuned saws and a triangle, each drifting slowly in pitch,
 * spread across the stereo field, through one low-pass that moves over the note.
 */
export function pad(
	k: Kit,
	b: Bus,
	when: number,
	chord: (string | number)[],
	dur: number,
	o: PadOptions = {}
) {
	const {
		attack = 0.8,
		release = 1.6,
		cutoff = [900, 1800],
		spread = 16,
		bright = 0.6,
		width = 0.8,
		seed = 3
	} = o;
	const r = rng(seed);
	const fs = chord.map(freq);
	const end = when + attack + dur + release * 1.6;
	const lp = k.filter('lowpass', cutoff[0], 0.45);
	lp.frequency.setValueAtTime(cutoff[0], when);
	lp.frequency.linearRampToValueAtTime(cutoff[1], when + attack + dur);
	lp.frequency.linearRampToValueAtTime(cutoff[0] * 0.8, end);
	const hp = k.filter('highpass', o.floor ?? 170, 0.5);
	const peak = 0.055 / Math.sqrt(fs.length);
	let amp: GainNode;
	if (o.swell || o.cut) {
		amp = k.gain(0);
		const g = amp.gain;
		g.setValueAtTime(0, when);
		g.linearRampToValueAtTime(peak, when + attack);
		const top = peak * (o.swell ?? 1);
		if (o.cut) {
			g.linearRampToValueAtTime(top, when + attack + dur - 0.025);
			g.linearRampToValueAtTime(0, when + attack + dur);
		} else {
			g.linearRampToValueAtTime(top, when + attack + dur);
			g.setTargetAtTime(0, when + attack + dur, release / 4);
		}
	} else amp = ahr(k, when, attack, dur, release, peak);
	lp.connect(hp).connect(amp);
	fs.forEach((f, i) => {
		const side = (i % 2 ? 1 : -1) * width;
		const voices: [OscillatorType, number, number, number][] = [
			['sawtooth', -spread / 2, bright * 0.9, -side],
			['sawtooth', spread * 0.08, bright, side * 0.15],
			['sawtooth', spread / 2, bright * 0.9, side],
			['triangle', 0, 1.4 - bright, 0]
		];
		for (const [type, cents, level, pan] of voices) {
			const osc = k.oscAt(b, type, f, when, end - when + 0.05, r);
			osc.detune.value = cents + (r() - 0.5) * 3;
			// slow drift: a sub-hertz wobble of a few cents, its phase from the seed
			const lfo = k.oscAt(b, 'sine', 0.12 + r() * 0.25, when, end - when + 0.05, r);
			const depth = k.gain(2.5 + r() * 2);
			lfo.connect(depth).connect(osc.detune);
			const g = k.gain(level);
			osc
				.connect(g)
				.connect(k.pan(Math.max(-1, Math.min(1, pan))))
				.connect(lp);
		}
	});
	out(k, b, amp, o, 0.55, true);
}

/**
 * A shimmer: the chord two octaves up on soft detuned saws, high-passed so only its sheen
 * remains (air above 5 kHz, little in the 2-5 kHz band where it would turn harsh).
 */
export function shimmer(
	k: Kit,
	b: Bus,
	when: number,
	chord: (string | number)[],
	dur: number,
	o: Out & { attack?: number; release?: number } = {}
) {
	const { attack = 0.8, release = 1.4, seed = 7 } = o;
	const r = rng(seed);
	const end = when + attack + dur + release * 1.6;
	const hp = k.filter('highpass', 4200, 0.6);
	const lp = k.filter('lowpass', 12500, 0.5);
	const amp = ahr(k, when, attack, dur, release, 0.02 / Math.sqrt(chord.length));
	hp.connect(lp).connect(amp);
	chord.map(freq).forEach((f, i) => {
		for (const c of [-9, 8]) {
			const osc = k.oscAt(b, 'sawtooth', f * 4, when, end - when + 0.05, r);
			osc.detune.value = c + (r() - 0.5) * 4;
			osc.connect(k.pan((c > 0 ? 0.7 : -0.7) * (i % 2 ? -1 : 1))).connect(hp);
		}
	});
	out(k, b, amp, o, 0.7);
}

/**
 * A warm swell for the landing's body: stacked detuned saws whose filter blooms open with the
 * hit and closes slowly, like a low brass section played by a synth.
 */
export function bloom(
	k: Kit,
	b: Bus,
	when: number,
	chord: (string | number)[],
	dur: number,
	o: Out & { release?: number; top?: number } = {}
) {
	const { release = 1.6, top = 3200, seed = 13 } = o;
	const r = rng(seed);
	const end = when + dur + release * 1.6;
	const lp = k.filter('lowpass', 400, 0.7);
	lp.frequency.setValueAtTime(500, when);
	lp.frequency.exponentialRampToValueAtTime(top, when + 0.18);
	lp.frequency.exponentialRampToValueAtTime(top * 0.35, when + dur + release);
	const amp = ahr(k, when, 0.07, dur, release, 0.07 / Math.sqrt(chord.length));
	lp.connect(sat(k, 1.4)).connect(amp);
	chord.map(freq).forEach((f, i) => {
		for (const c of [-10, -3, 4, 11]) {
			const osc = k.oscAt(b, 'sawtooth', f, when, end - when + 0.05, r);
			osc.detune.value = c + (r() - 0.5) * 3;
			osc.connect(k.pan((c / 11) * 0.6 * (i % 2 ? -1 : 1))).connect(lp);
		}
	});
	out(k, b, amp, o, 0.5);
}

// --- glass mallet ----------------------------------------------------------------------------

/**
 * A glass mallet: harmonic partials that each decay at their own rate (the highs first), a
 * second voice a few cents up on the other side, and a soft felt tap. Bell-like without FM's
 * inharmonic edge.
 */
export function glass(
	k: Kit,
	b: Bus,
	when: number,
	note: string | number,
	o: Out & { decay?: number; tone?: number } = {}
) {
	const { decay = 2.6, tone = 1, seed = 17 } = o;
	const f = freq(note);
	const r = rng(seed);
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
		for (const [h, a, dk] of partials) {
			if (f * h > 14000) continue;
			const osc = k.oscAt(b, 'sine', f * h, when, decay * 1.9 + 0.3, r);
			osc.detune.value = cents;
			const g = k.gain(0);
			g.gain.setValueAtTime(0, when);
			g.gain.linearRampToValueAtTime(a * 0.5, when + 0.004);
			g.gain.setTargetAtTime(0, when + 0.004, (decay * dk) / 3.2);
			osc.connect(g).connect(p);
		}
	}
	// warmth: a quiet sine an octave below, slow to fade
	if (f > 300) {
		const low = k.oscAt(b, 'sine', f / 2, when, decay * 1.9 + 0.3, r);
		const lg = k.gain(0);
		lg.gain.setValueAtTime(0, when);
		lg.gain.linearRampToValueAtTime(0.16, when + 0.012);
		lg.gain.setTargetAtTime(0, when + 0.012, decay / 2.6);
		low.connect(lg).connect(sum);
	}
	// the felt: a soft, dark tap
	const n = k.noiseAt(b, 'pink', when, decay * 1.9 + 0.3, seed);
	const lp = k.filter('lowpass', Math.min(6000, f * 3), 0.7);
	n.connect(lp)
		.connect(k.ad(when, 0.05, 0.001, 0.02))
		.connect(sum);
	const soft = k.filter('lowpass', 7000, 0.5);
	sum.connect(soft);
	out(k, b, soft, { ...o, gain: (o.gain ?? 1) * 0.16 }, 0.5);
}

// --- bass ------------------------------------------------------------------------------------

/** A warm sub: a sine and a quiet octave, a little drive so small speakers find it. */
export function sub(
	k: Kit,
	b: Bus,
	when: number,
	note: string | number,
	dur: number,
	o: Out & { attack?: number; release?: number } = {}
) {
	const { attack = 0.06, release = 0.8 } = o;
	const f = freq(note);
	const end = attack + dur + release * 1.6 + 0.1;
	const osc = k.osc(b, 'sine', f, when, end);
	const oct = k.osc(b, 'sine', f * 2, when, end);
	const og = k.gain(0.12);
	oct.connect(og);
	const sum = k.gain(1);
	osc.connect(sum);
	og.connect(sum);
	const amp = ahr(k, when, attack, dur, release, 0.32);
	sum.connect(sat(k, 1.5)).connect(amp);
	out(k, b, amp, { ...o, verb: o.verb ?? 0 }, 0);
}

/** A soft analog bass an octave over the sub: a filtered saw and square, for the line's shape. */
export function bass(
	k: Kit,
	b: Bus,
	when: number,
	note: string | number,
	dur: number,
	o: Out & { attack?: number; release?: number; cutoff?: number } = {}
) {
	const { attack = 0.05, release = 0.6, cutoff = 420, seed = 5 } = o;
	const f = freq(note);
	const r = rng(seed);
	const end = attack + dur + release * 1.6 + 0.1;
	const lp = k.filter('lowpass', cutoff, 0.8);
	for (const [type, c, lvl] of [
		['sawtooth', -5, 0.6],
		['square', 5, 0.35]
	] as [OscillatorType, number, number][]) {
		const osc = k.oscAt(b, type, f, when, end, r);
		osc.detune.value = c;
		osc.connect(k.gain(lvl)).connect(lp);
	}
	const amp = ahr(k, when, attack, dur, release, 0.09);
	lp.connect(amp);
	out(k, b, amp, { ...o, verb: o.verb ?? 0.05 }, 0.05);
}

// --- pulse -----------------------------------------------------------------------------------

/**
 * A round, felted pulse: a sine with a soft octave and a whisper of fifth, a dark felt touch and
 * a quick swell rather than a click, so it reads as a heartbeat under the pads, never a pluck.
 * `open` lets a little more of the upper partials through (0..1).
 */
export function pulse(
	k: Kit,
	b: Bus,
	when: number,
	note: string | number,
	o: Out & { open?: number; decay?: number } = {}
) {
	const { open = 0.3, decay = 0.3, seed = 23 } = o;
	const f = freq(note);
	const r = rng(seed);
	const sum = k.gain(1);
	for (const [h, a] of [
		[1, 1],
		[2, 0.22 + 0.25 * open],
		[3, 0.05 + 0.1 * open]
	]) {
		const osc = k.oscAt(b, 'sine', f * h, when, decay * 2 + 0.2, r);
		const g = k.gain(0);
		g.gain.setValueAtTime(0, when);
		g.gain.linearRampToValueAtTime(a, when + 0.012);
		g.gain.setTargetAtTime(0, when + 0.012, (decay / 3) * (h === 1 ? 1 : 0.6));
		osc.connect(g).connect(sum);
	}
	const n = k.noiseAt(b, 'pink', when, decay * 2 + 0.2, seed);
	const lp = k.filter('lowpass', 900 + 900 * open, 0.7);
	n.connect(lp)
		.connect(k.ad(when, 0.08, 0.003, 0.03))
		.connect(sum);
	const amp = k.gain(0.11);
	sum.connect(sat(k, 1.3)).connect(amp);
	out(k, b, amp, o, 0.22);
}

// --- percussion and sound design ---------------------------------------------------------------

/** A lock: a soft, deep, tuned thud and a felt tock, the sound of a part finding its place. */
export function thud(k: Kit, b: Bus, when: number, o: Out & { pitch?: number } = {}) {
	const { pitch = 55, seed = 29 } = o;
	const osc = k.osc(b, 'sine', pitch, when, 1.2);
	osc.frequency.setValueAtTime(pitch * 2.6, when);
	osc.frequency.exponentialRampToValueAtTime(pitch, when + 0.09);
	const amp = k.gain(0);
	amp.gain.setValueAtTime(0, when);
	amp.gain.linearRampToValueAtTime(0.5, when + 0.003);
	amp.gain.setTargetAtTime(0, when + 0.02, 0.16);
	osc.connect(sat(k, 1.8)).connect(amp);
	const n = k.noiseAt(b, 'pink', when, 1.2, seed);
	const bp = k.filter('bandpass', 700, 1.2);
	n.connect(bp)
		.connect(k.ad(when, 0.22, 0.001, 0.035))
		.connect(amp);
	out(k, b, amp, o, 0.32);
}

/** A tiny, precise click (the feet): a dark tick with a short pitched ring. */
export function tick(k: Kit, b: Bus, when: number, o: Out & { pitch?: number } = {}) {
	const { pitch = 1600, seed = 31 } = o;
	const n = k.noiseAt(b, 'white', when, 0.12, seed);
	const bp = k.filter('bandpass', pitch * 1.6, 1.4);
	const amp = k.gain(1);
	n.connect(bp)
		.connect(k.ad(when, 0.12, 0.0008, 0.012))
		.connect(amp);
	const osc = k.osc(b, 'sine', pitch, when, 0.12);
	osc.connect(k.ad(when, 0.05, 0.001, 0.05)).connect(amp);
	out(k, b, amp, o, 0.25);
}

/** Air: band-passed pink noise that swells and breathes, the space between things. */
export function air(
	k: Kit,
	b: Bus,
	when: number,
	dur: number,
	o: Out & { band?: [number, number]; q?: number; attack?: number; release?: number } = {}
) {
	const { band = [600, 1400], q = 0.8, attack = 1, release = 1, seed = 37 } = o;
	const n = k.noiseAt(b, 'pink', when, attack + dur + release * 1.6 + 0.1, seed);
	const bp = k.filter('bandpass', band[0], q);
	bp.frequency.setValueAtTime(band[0], when);
	bp.frequency.exponentialRampToValueAtTime(band[1], when + attack + dur);
	const amp = ahr(k, when, attack, dur, release, 0.05);
	n.connect(bp).connect(amp);
	out(k, b, amp, o, 0.5);
}

/** A swish of air past the lens, panned along with what passes. */
export function swish(
	k: Kit,
	b: Bus,
	when: number,
	dur: number,
	o: Out & { pans?: [number, number]; band?: [number, number] } = {}
) {
	const { pans = [-0.4, 0.4], band = [300, 1200], seed = 41 } = o;
	const n = k.noiseAt(b, 'pink', when, dur + 0.1, seed);
	const bp = k.filter('bandpass', band[0], 0.9);
	bp.frequency.setValueAtTime(band[0], when);
	bp.frequency.exponentialRampToValueAtTime(band[1], when + dur * 0.55);
	// a sweep that falls keeps falling; one that rises settles back a little
	bp.frequency.exponentialRampToValueAtTime(
		band[1] < band[0] ? band[1] * 0.8 : band[0] * 1.2,
		when + dur
	);
	const amp = k.gain(0);
	const peak = when + dur * 0.55;
	amp.gain.setValueAtTime(0, when);
	amp.gain.linearRampToValueAtTime(0.0001, when + 0.01);
	amp.gain.exponentialRampToValueAtTime(0.07, peak);
	amp.gain.exponentialRampToValueAtTime(0.0005, when + dur);
	const p = k.pan(pans[0]);
	p.pan.setValueAtTime(pans[0], when);
	p.pan.linearRampToValueAtTime(pans[1], when + dur);
	n.connect(bp).connect(amp).connect(p);
	out(k, b, p, o, 0.35);
}

/**
 * A reverse swell into a hit: noise rising in pitch and level, accelerating, cut a breath before
 * `end` so the hit lands on silence.
 */
export function inhale(
	k: Kit,
	b: Bus,
	end: number,
	dur: number,
	o: Out & { breath?: number } = {}
) {
	const { breath = 0.04, seed = 43 } = o;
	const when = end - dur;
	const stop = end - breath;
	const n = k.noiseAt(b, 'pink', when, dur + 0.05, seed);
	const bp = k.filter('bandpass', 500, 0.7);
	bp.frequency.setValueAtTime(500, when);
	bp.frequency.exponentialRampToValueAtTime(5200, stop);
	const amp = k.gain(0);
	amp.gain.setValueAtTime(0.0001, when);
	amp.gain.exponentialRampToValueAtTime(0.16, stop - 0.01);
	amp.gain.linearRampToValueAtTime(0, stop);
	n.connect(bp).connect(amp);
	out(k, b, amp, o, 0.2);
}

/**
 * The landing: a sub drop, a body thump, a short bright transient and a burst of air whose
 * tail the room carries for seconds.
 */
export function impact(k: Kit, b: Bus, when: number, o: Out & { root?: number } = {}) {
	const { root = hz('D1'), seed = 47 } = o;
	const sum = k.gain(1);
	// sub drop
	const s = k.osc(b, 'sine', root * 2.2, when, 3);
	s.frequency.setValueAtTime(root * 2.2, when);
	s.frequency.exponentialRampToValueAtTime(root, when + 0.12);
	s.frequency.exponentialRampToValueAtTime(root * 0.84, when + 2.4);
	const sg = k.gain(0);
	sg.gain.setValueAtTime(0, when);
	sg.gain.linearRampToValueAtTime(0.75, when + 0.004);
	sg.gain.setTargetAtTime(0, when + 0.1, 0.38);
	s.connect(sat(k, 2)).connect(sg).connect(sum);
	// body
	const bd = k.osc(b, 'sine', 160, when, 0.6);
	bd.frequency.setValueAtTime(190, when);
	bd.frequency.exponentialRampToValueAtTime(62, when + 0.16);
	const bg = k.gain(0);
	bg.gain.setValueAtTime(0, when);
	bg.gain.linearRampToValueAtTime(0.45, when + 0.002);
	bg.gain.setTargetAtTime(0, when + 0.01, 0.07);
	bd.connect(sat(k, 2.5)).connect(bg).connect(sum);
	// transient: short, and darker than a crack
	const n = k.noiseAt(b, 'white', when, 3, seed);
	const hp = k.filter('highpass', 900, 0.7);
	const lp = k.filter('lowpass', 6500, 0.7);
	n.connect(hp)
		.connect(lp)
		.connect(k.ad(when, 0.2, 0.001, 0.025))
		.connect(sum);
	// air burst: the hit's breath, closing down
	const a = k.noiseAt(b, 'pink', when, 2.6, seed + 1);
	const alp = k.filter('lowpass', 5000, 0.6);
	alp.frequency.setValueAtTime(5000, when);
	alp.frequency.exponentialRampToValueAtTime(500, when + 1.6);
	const ag = k.gain(0);
	ag.gain.setValueAtTime(0, when);
	ag.gain.linearRampToValueAtTime(0.11, when + 0.006);
	ag.gain.setTargetAtTime(0, when + 0.02, 0.45);
	a.connect(alp).connect(ag).connect(sum);
	out(k, b, sum, o, 0.42);
}
