// The Couch TV theme and sonic logo (placeholder: the sample reel's score, pending the remade
// theme). Everything that sounds like the brand lives here, so a new theme replaces this
// module only: `themeScore` scores an ident cut, `themeTag` is the condensed mnemonic every
// promo tag and sting ends on.

import type { Bus } from '../audio/engine';
import type { Kit } from '../audio/kit';

const N = {
	D1: 36.71,
	Bb1: 58.27,
	D2: 73.42,
	F2: 87.31,
	A2: 110.0,
	Bb2: 116.54,
	D3: 146.83,
	E3: 164.81,
	F3: 174.61,
	Fs3: 185.0,
	A3: 220.0,
	D4: 293.66,
	E4: 329.63,
	F4: 349.23,
	Fs4: 369.99,
	A4: 440.0,
	E5: 659.25
};

/** The mnemonic: D up a fifth to A, down to F#, settling on E over a D add9 chord. */
export const LOGO = {
	notes: [N.D4, N.A4, N.Fs4, N.E4],
	rhythm: [0, 0.22, 0.44, 0.9],
	tagRhythm: [0, 0.19, 0.38, 0.76]
} as const;

/** An ident cut's cue table: picture and score both read it, so they cannot drift apart. */
export interface IdentCut {
	D: number;
	swell: number;
	b1: number | null;
	b2: number | null;
	ost: [number, number] | null;
	drums: [number, number][];
	riser: number;
	hit: number;
	mn: number;
	final: number;
}

export interface ScoreEvent {
	at: number;
	dur: number;
	fn: (when: number, b: Bus, skip: number) => void;
}

/** Low brass "braam": detuned saw stacks scooping into pitch, through an opening low-pass. */
function braam(
	k: Kit,
	b: Bus,
	when: number,
	notes: number[],
	o: { gain: number; dur: number; open: [number, number, number]; drive: number }
) {
	const { gain, dur, open, drive } = o;
	const lp = k.filter('lowpass', open[0], 1.1);
	lp.frequency.setValueAtTime(open[0], when);
	lp.frequency.exponentialRampToValueAtTime(open[1], when + 0.14);
	lp.frequency.exponentialRampToValueAtTime(open[2], when + dur);
	const env = k.gain(0);
	env.gain.setValueAtTime(0, when);
	env.gain.linearRampToValueAtTime(gain, when + 0.03);
	env.gain.exponentialRampToValueAtTime(gain * 0.55, when + 0.63);
	env.gain.exponentialRampToValueAtTime(gain * 0.0008, when + dur);
	const pre = k.gain(1.9 / Math.sqrt(notes.length * 4));
	notes.forEach((f, i) => {
		const p = k.pan(((i % 2) * 2 - 1) * 0.25 * (i / notes.length));
		[-14, -5, 6, 15].forEach((det) => {
			const o2 = k.osc(b, 'sawtooth', f, when, dur + 0.1);
			o2.detune.setValueAtTime(det - 45, when);
			o2.detune.linearRampToValueAtTime(det, when + 0.09);
			o2.connect(p);
		});
		p.connect(pre);
	});
	pre.connect(lp).connect(k.shaper(drive)).connect(env).connect(b.sfx);
	k.send(b, env, 0.35);
}

/** Taiko-style drum: a pitch-dropping sine body, a skin thump and a noise transient. */
function taiko(
	k: Kit,
	b: Bus,
	when: number,
	o: { gain: number; pitch?: number; pan?: number; size?: number; verb?: number }
) {
	const { gain, pitch = 1, pan = 0, size = 1, verb = 0.35 } = o;
	const out = k.pan(pan);
	const body = k.osc(b, 'sine', 150 * pitch, when, 1.2 * size + 0.1);
	body.frequency.setValueAtTime(150 * pitch, when);
	body.frequency.exponentialRampToValueAtTime(50 * pitch, when + 0.13);
	body.connect(k.ad(when, gain, 0.002, 0.95 * size)).connect(out);
	const skin = k.osc(b, 'triangle', 240 * pitch, when, 0.25);
	skin.frequency.setValueAtTime(240 * pitch, when);
	skin.frequency.exponentialRampToValueAtTime(120 * pitch, when + 0.05);
	skin.connect(k.ad(when, gain * 0.28, 0.001, 0.14)).connect(out);
	const n = k.noise(b, 'white', when, 0.06);
	n.connect(k.filter('bandpass', 1100, 0.8))
		.connect(k.ad(when, gain * 0.24, 0.001, 0.03))
		.connect(out);
	out.connect(b.sfx);
	k.send(b, out, verb);
}

/** A brass note: saws with an octave-down layer, a bright attack and late vibrato. */
function brass(
	k: Kit,
	b: Bus,
	when: number,
	f: number,
	dur: number,
	o: { gain: number; pan?: number; bright?: number; verb?: number; release?: number }
) {
	const { gain, pan = 0, bright = 1, verb = 0.45, release = 0.3 } = o;
	const lp = k.filter('lowpass', 300, 1.0);
	lp.frequency.setValueAtTime(300, when);
	lp.frequency.exponentialRampToValueAtTime(900 + 2000 * bright, when + 0.07);
	lp.frequency.exponentialRampToValueAtTime(700 + 900 * bright, when + 0.45);
	const lfo = k.osc(b, 'sine', 5.4, when, dur + release + 0.1);
	const depth = k.gain(0);
	depth.gain.setValueAtTime(0, when + Math.min(0.2, dur * 0.5));
	depth.gain.linearRampToValueAtTime(7, when + Math.min(0.6, dur));
	lfo.connect(depth);
	const pre = k.gain(0.45);
	for (const [ratio, det, lvl] of [
		[1, -7, 1],
		[1, 0, 1],
		[1, 7, 1],
		[0.5, 0, 0.6]
	]) {
		const o2 = k.osc(b, 'sawtooth', f * ratio, when, dur + release + 0.1);
		o2.detune.setValueAtTime(det - 30, when);
		o2.detune.linearRampToValueAtTime(det, when + 0.05);
		depth.connect(o2.detune);
		o2.connect(k.gain(lvl)).connect(pre);
	}
	const env = k.gain(0);
	env.gain.setValueAtTime(0, when);
	env.gain.linearRampToValueAtTime(gain, when + 0.035);
	env.gain.exponentialRampToValueAtTime(gain * 0.72, when + 0.3);
	env.gain.setValueAtTime(gain * 0.72, when + dur);
	env.gain.exponentialRampToValueAtTime(gain * 0.001, when + dur + release);
	const p = k.pan(pan);
	pre.connect(lp).connect(k.shaper(1.3)).connect(env).connect(p).connect(b.sfx);
	k.send(b, p, verb);
}

function subDrop(
	k: Kit,
	b: Bus,
	when: number,
	o: { gain: number; from: number; to: number; dur: number }
) {
	const s = k.osc(b, 'sine', o.from, when, o.dur + 0.2);
	s.frequency.setValueAtTime(o.from, when);
	s.frequency.exponentialRampToValueAtTime(o.to, when + o.dur * 0.8);
	const env = k.gain(0);
	env.gain.setValueAtTime(0, when);
	env.gain.linearRampToValueAtTime(o.gain, when + 0.012);
	env.gain.setValueAtTime(o.gain, when + 0.25);
	env.gain.exponentialRampToValueAtTime(o.gain * 0.001, when + o.dur);
	s.connect(k.shaper(1.8)).connect(env).connect(b.sfx);
}

/** A clean sine sub swelling into the hit; it ends a breath before, so the hit lands on silence. */
function subSwell(k: Kit, b: Bus, when: number, dur: number, skip: number, gain: number) {
	const left = dur - skip;
	if (left <= 0.05) return;
	const shape = (x: number) => 0.04 + 0.96 * Math.pow(x, 2.2);
	const env = k.gain(0);
	env.gain.setValueAtTime(gain * shape(skip / dur), when);
	for (let i = 1; i <= 24; i++) {
		const x = skip / dur + (i / 24) * (left / dur);
		env.gain.linearRampToValueAtTime(gain * shape(Math.min(1, x)), when + (i / 24) * left);
	}
	env.gain.linearRampToValueAtTime(0, when + left + 0.03);
	for (const [f, g] of [
		[N.D1, 1],
		[N.D2, 0.55]
	]) {
		k.osc(b, 'sine', f, when, left + 0.1)
			.connect(k.gain(g))
			.connect(env);
	}
	env.connect(b.sfx);
}

/** Shepard-style riser: octave-spaced voices gliding up under a bell-shaped spectral window. */
function shepard(k: Kit, b: Bus, when: number, dur: number, gain: number) {
	const out = k.gain(1);
	const lp = k.filter('lowpass', 2600, 0.5);
	const voices = 5;
	const octaves = 1.2;
	for (let i = 0; i < voices; i++) {
		const osc = k.osc(b, 'sawtooth', N.D2 * 2 ** i, when, dur + 0.05);
		osc.frequency.setValueAtTime(N.D2 * 2 ** i, when);
		osc.frequency.exponentialRampToValueAtTime(N.D2 * 2 ** (i + octaves), when + dur);
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
	lp.connect(out).connect(b.sfx);
	k.send(b, out, 0.4);
}

/** A cut's score as timeline events. */
export function themeScore(k: Kit, c: IdentCut): ScoreEvent[] {
	const ev: ScoreEvent[] = [];
	const at = (t: number | null, fn: ScoreEvent['fn'], dur = 0) => {
		if (t != null) ev.push({ at: t, fn, dur });
	};
	const H = c.hit;
	const F = c.final;
	const tail = Math.max(1.6, c.D - F - 0.4);

	at(c.swell, (w, b, skip) => subSwell(k, b, w, H - 0.06 - c.swell, skip, 0.24), H - c.swell);
	at(c.b1, (w, b) => {
		braam(k, b, w, [N.D2, N.A2, N.D3], { gain: 0.3, dur: 2.4, drive: 1.2, open: [180, 2000, 600] });
		k.sub(b, w, N.D1, { gain: 0.22, decay: 1.8 });
		taiko(k, b, w, { gain: 0.3, size: 1.3 });
	});
	at(c.b2, (w, b) => {
		braam(k, b, w, [N.Bb1, N.F2, N.Bb2], {
			gain: 0.32,
			dur: 2.2,
			drive: 1.2,
			open: [200, 2400, 650]
		});
		k.sub(b, w, N.Bb1, { gain: 0.18, decay: 1.6 });
		taiko(k, b, w, { gain: 0.3, size: 1.2 });
	});
	const ost = c.ost;
	if (ost) {
		at(ost[0], (w, b) => {
			const n = Math.round((ost[1] - ost[0]) / 0.125);
			const lp = k.filter('lowpass', 600, 1.4);
			lp.frequency.setValueAtTime(600, w);
			lp.frequency.exponentialRampToValueAtTime(3600, w + n * 0.125);
			const pat = [N.D3, N.D3, N.A3, N.D3, N.F3, N.D3, N.A3, N.D3];
			const up = [N.D3, N.A3, N.D4, N.A3, N.D3, N.A3, N.D4, N.F4];
			for (let i = 0; i < n; i++) {
				const t = ost[0] + i * 0.125;
				// the last bar before the hit drops out: the riser carries it
				if (t >= H - 0.5) break;
				const f = (t >= H - 1.0 ? up : pat)[i % 8];
				const env = k.ad(
					w + i * 0.125,
					(0.045 + (0.08 * i) / n) * (i % 4 === 0 ? 1.25 : 1),
					0.004,
					0.13
				);
				for (const det of [-6, 6]) {
					const o2 = k.osc(b, 'sawtooth', f, w + i * 0.125, 0.2);
					o2.detune.value = det;
					o2.connect(env);
				}
				env.connect(k.pan(i % 2 ? 0.2 : -0.2)).connect(lp);
			}
			lp.connect(b.sfx);
		});
	}
	for (const [t, g] of c.drums)
		at(t, (w, b) =>
			taiko(k, b, w, {
				gain: g * 0.62,
				pitch: 1 + (t % 0.5) * 0.3,
				pan: ((t * 7) % 1) - 0.5,
				size: 0.8,
				verb: 0.25
			})
		);
	at(c.riser, (w, b) => shepard(k, b, w, H - 0.06 - c.riser, 0.055));

	at(H, (w, b) => {
		braam(k, b, w, [N.D2, N.A2, N.D3, N.Fs3], {
			gain: 0.28,
			dur: 2.4,
			open: [220, 2800, 700],
			drive: 1.4
		});
		taiko(k, b, w, { gain: 0.42, size: 1.6, pitch: 0.9, verb: 0.3 });
		subDrop(k, b, w, { gain: 0.26, from: 90, to: 31, dur: 1.8 });
	});
	at(H + 0.05, (w, b) =>
		k.pad(b, w, [N.D3, N.A3, N.D4, N.Fs4], {
			gain: 0.03,
			attack: 0.4,
			hold: Math.max(0.3, F - H - 0.5),
			release: 0.6,
			cutoff: [1800, 900]
		})
	);
	const R = LOGO.rhythm;
	[0, 1, 2].forEach((i) => {
		at(c.mn + R[i], (w, b) => {
			brass(k, b, w, LOGO.notes[i], R[i + 1] - R[i] - 0.02, {
				gain: 0.17,
				pan: [-0.12, 0.12, -0.04][i],
				release: 0.16,
				verb: 0.35
			});
			k.bell(b, w, LOGO.notes[i] * 2, {
				gain: 0.1,
				decay: 1.3,
				index: 1.5,
				pan: [-0.15, 0.15, 0][i]
			});
		});
	});
	at(F, (w, b) => {
		brass(k, b, w, N.E4, tail * 0.5, { gain: 0.18, release: tail * 0.45, bright: 1.1, verb: 0.5 });
		brass(k, b, w, N.E3, tail * 0.45, {
			gain: 0.08,
			release: tail * 0.4,
			bright: 0.6,
			pan: -0.2,
			verb: 0.5
		});
		k.bell(b, w, N.E5, { gain: 0.12, decay: Math.min(4, tail), index: 1.3, verb: 0.65 });
		k.pad(b, w, [N.D3, N.A3, N.E4, N.Fs4], {
			gain: 0.034,
			attack: 0.25,
			hold: tail * 0.25,
			release: tail * 0.7,
			cutoff: [2000, 700],
			verb: 0.6
		});
		k.sub(b, w, N.D1, { gain: 0.22, decay: tail * 0.7 });
		taiko(k, b, w, { gain: 0.24, size: 1.4, pitch: 0.95 });
	});
	return ev;
}

/** The tag: the theme condensed (the mnemonic on brass and bell, a small final chord). */
export function themeTag(k: Kit, b: Bus, when: number, { gain = 0.5 } = {}) {
	const g = gain / 0.5;
	const R = LOGO.tagRhythm;
	[0, 1, 2].forEach((i) => {
		brass(k, b, when + R[i], LOGO.notes[i], R[i + 1] - R[i] - 0.02, {
			gain: 0.13 * g,
			pan: [-0.12, 0.12, -0.04][i],
			release: 0.14,
			bright: 0.8,
			verb: 0.35
		});
		k.bell(b, when + R[i], LOGO.notes[i] * 2, { gain: 0.08 * g, decay: 1.1, index: 1.5 });
	});
	const t = when + R[3];
	brass(k, b, t, N.E4, 0.9, { gain: 0.14 * g, release: 1.3, bright: 1, verb: 0.5 });
	k.bell(b, t, N.E5, { gain: 0.1 * g, decay: 2.2, index: 1.3, verb: 0.6 });
	k.pad(b, t, [N.D3, N.A3, N.E4, N.Fs4], {
		gain: 0.026 * g,
		attack: 0.15,
		hold: 0.4,
		release: 1.8,
		cutoff: [1800, 700],
		verb: 0.55
	});
	k.sub(b, t, N.D1, { gain: 0.22 * g, decay: 1.5 });
	taiko(k, b, t, { gain: 0.24 * g, size: 1.2 });
}

/** The mnemonic alone on bells (stings too short for the brass). */
export function themeBells(k: Kit, b: Bus, when: number, { gain = 0.5 } = {}) {
	const g = gain / 0.5;
	const R = LOGO.tagRhythm;
	LOGO.notes.forEach((f, i) =>
		k.bell(b, when + R[i], f * 2, {
			gain: (i === 3 ? 0.12 : 0.09) * g,
			decay: i === 3 ? 2.4 : 1.1,
			index: 1.4
		})
	);
	k.sub(b, when + R[3], N.D2, { gain: 0.14 * g, decay: 1.2 });
}
