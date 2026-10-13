// The morph ident's music: one idea. A three-note figure (A, E, D: the fifth, the ninth, the
// root) is the sonic logo. It is heard first slowly on a felt piano in the night, recoloured by
// each chord under it; then an octave up in space; and last, quick and whole, on "couch", "t",
// "v". Under it a warm pad opens like a dawn in one long filter sweep through D minor,
// Bbmaj7(#11), Gm9 and Asus4 to A, a heartbeat enters in space and doubles on the stage, the
// popcorn's fall is a reversed swell, and the landing is the D major arrival: a sub drop, a
// round body, a soft crack, the hall. 75 bpm (a beat is 0.8 s, 20 frames), so the picture's
// moves sit on the beat grid.

import type { Bus } from '../../../audio/engine';
import type { Kit } from '../../../audio/kit';
import { T } from './scene';
import {
	air,
	bowed,
	glass,
	hz,
	impact,
	makeMix,
	pad,
	piano,
	reverse,
	shimmer,
	sub,
	thump
} from './voices';

const BEAT = 0.8;

const n = (names: string) => names.split(' ').map(hz);

/**
 * The pad's chords, each swelling in ahead of its morph: when, how long it holds after its
 * swell, its release, the voicing. The dominant lets go just before the landing.
 */
const CHORDS: { at: number; dur: number; release: number; pad: string; level: number }[] = [
	{ at: 0, dur: 0.45, release: 0.9, pad: 'D3 A3 E4 F4', level: 0.42 },
	{ at: 1.6 - 0.45, dur: 1.15, release: 1.0, pad: 'F3 A3 D4 E4', level: 0.66 },
	{ at: 3.2 - 0.45, dur: 1.15, release: 1.0, pad: 'F3 Bb3 D4 A4', level: 1.0 },
	{ at: 4.8 - 0.4, dur: 0.45, release: 0.9, pad: 'E3 A3 D4 E4 A4', level: 1.2 },
	{ at: 5.6 - 0.3, dur: 0.31, release: 0.3, pad: 'E3 A3 C#4 E4 A4', level: 1.85 }
];

/** The bass under them: one sustained note per root (a repeated note would cancel itself). */
const BASS: { at: number; until: number; note: string; gain: number }[] = [
	{ at: 0.15, until: 1.45, note: 'D2', gain: 0.5 },
	{ at: 1.4, until: 3.0, note: 'Bb1', gain: 0.62 },
	{ at: 3.0, until: 4.55, note: 'G1', gain: 0.74 },
	{ at: 4.65, until: 6.25, note: 'A1', gain: 0.86 }
];

/** The figure: A, E, D. */
const FIGURE = ['A', 'E', 'D'];

export function score(k: Kit, b: Bus, w: number, D: number) {
	const m = makeMix(k, b);
	const at = (t: number) => w + t;
	const land = T.land;

	// the pad's one long sweep: closed in the night, open at the landing, closing as it rings out
	const sweep: [number, number][] = [
		[0, 380],
		[1.6, 760],
		[3.2, 1250],
		[4.8, 2300],
		[6.3, 4200],
		[land + 0.05, 7000],
		[8.2, 3600],
		[D, 900]
	];
	for (const f of m.padCut) {
		f.frequency.setValueAtTime(sweep[0][1], at(0));
		for (const [t, v] of sweep.slice(1)) f.frequency.exponentialRampToValueAtTime(v, at(t));
	}
	// the breath before the landing: the pad steps back for the hit
	const pl = m.padLevel.gain;
	pl.setValueAtTime(1, at(0));
	pl.setValueAtTime(1, at(land - 0.12));
	pl.linearRampToValueAtTime(0.25, at(land - 0.02));
	pl.linearRampToValueAtTime(1, at(land + 0.02));

	// the hall carries the end down to silence
	const fade = m.out.gain;
	fade.setValueAtTime(1, at(0));
	fade.setValueAtTime(1, at(D - 1.4));
	fade.exponentialRampToValueAtTime(0.06, at(D));

	// --- the build ---------------------------------------------------------------------------
	CHORDS.forEach((c, i) => {
		pad(m, at(c.at), n(c.pad), c.dur, {
			attack: i === 0 ? 1.3 : 0.75,
			release: c.release,
			level: c.level,
			seed: 11 + i * 7
		});
	});
	BASS.forEach((s, i) => {
		const attack = i === 0 ? 1.0 : 0.45;
		sub(m, at(s.at), hz(s.note), s.until - s.at - attack, {
			gain: s.gain,
			attack,
			release: i === BASS.length - 1 ? 0.25 : 0.5
		});
	});

	// the figure, slowly, in the night (A3 E4 D4, a beat apart)...
	FIGURE.forEach((name, i) => {
		piano(m, at(i * BEAT), hz(`${name}${i ? 4 : 3}`), {
			vel: 0.36 + 0.04 * i,
			dur: 2.6,
			pan: -0.12 + 0.1 * i,
			verb: 0.7,
			gain: 0.9,
			seed: 100 + i
		});
	});
	// ...and an octave up in space, with glass above it
	FIGURE.forEach((name, i) => {
		const t = 3.2 + i * BEAT;
		piano(m, at(t), hz(`${name}${i ? 5 : 4}`), {
			vel: 0.42 + 0.05 * i,
			dur: 2.2,
			pan: 0.1 - 0.1 * i,
			verb: 0.75,
			gain: 0.85,
			seed: 200 + i
		});
		glass(m, at(t + 0.01), hz(`${name}${i ? 6 : 5}`), {
			gain: 0.32,
			decay: 2.6,
			pan: 0.25,
			verb: 0.8,
			seed: 210 + i
		});
	});

	// air through the morphs, moving with the circle
	air(m, at(T.m1[0] - 0.1), 1.8, {
		peakAt: 0.8,
		gain: 0.8,
		lo: 260,
		hi: 2300,
		pan: [0.45, -0.05],
		seed: 61
	});
	air(m, at(T.m2[0] - 0.05), 1.5, {
		peakAt: 0.62,
		gain: 0.75,
		lo: 320,
		hi: 1700,
		pan: [0, -0.4],
		seed: 62
	});
	air(m, at(T.m3[0] - 0.05), 1.4, {
		peakAt: 0.62,
		gain: 0.7,
		lo: 240,
		hi: 1300,
		pan: [-0.4, 0.05],
		seed: 63
	});

	// the heartbeat: on the beat in space, twice as fast on the stage
	const beats = [3.2, 4.0, 4.8, 5.2, 5.6, 6.0];
	beats.forEach((t, i) => {
		thump(m, at(t), { gain: 0.45 + 0.1 * i, f: t < 4.8 ? hz('G1') : hz('A1') });
	});

	// the fall: a reversed swell of the arrival chord, cut just before the hit
	// the tones the dominant and the arrival share (A, E), so it rises clean over the A chord
	reverse(m, at(land), n('A2 E3 A3 E4 A4'), land - T.drop + 0.1, { gain: 1.6, cut: 0.035 });

	// --- the landing ---------------------------------------------------------------------------
	impact(m, at(land), { gain: 0.6 });
	pad(m, at(land), n('D3 A3 F#4 A4 E5'), 1.7, {
		attack: 0.09,
		release: 1.6,
		level: 0.8,
		spread: 26,
		seed: 91
	});
	sub(m, at(land), hz('D2'), 1.6, { gain: 0.4, attack: 0.05, release: 1.2 });
	shimmer(m, at(land + 0.02), n('A5 D6 F#6 A6'), 1.9, {
		gain: 1,
		attack: 0.5,
		release: 1.8,
		seed: 83
	});
	for (const [name, i] of [
		['D2', 0],
		['A2', 1],
		['D3', 2]
	] as const)
		piano(m, at(land + 0.005 * i), hz(name), {
			vel: 0.72,
			dur: 2.8,
			pan: -0.15 + 0.15 * i,
			verb: 0.6,
			gain: 0.5,
			seed: 300 + i
		});

	// --- couch, t, v ---------------------------------------------------------------------------
	const LOGO = ['A4', 'E5', 'D5'];
	LOGO.forEach((name, i) => {
		const t = T.notes[i];
		const last = i === LOGO.length - 1;
		const f = hz(name);
		piano(m, at(t), f, {
			vel: 0.6 + 0.05 * i,
			dur: last ? 1.8 : 1.2,
			pan: -0.08 + 0.08 * i,
			verb: 0.6,
			gain: 1.45,
			seed: 400 + i
		});
		glass(m, at(t + 0.008), f * 2, {
			gain: 0.5,
			decay: last ? 2.4 : 1.8,
			pan: 0.18,
			verb: 0.75,
			seed: 410 + i
		});
		bowed(m, at(t), f, last ? 1.0 : 0.32, {
			gain: last ? 1.3 : 1.0,
			attack: 0.22,
			release: last ? 1.0 : 0.5,
			verb: 0.6,
			seed: 420 + i
		});
	});
	// the last note settles the chord: the ninth joins, low and wide
	pad(m, at(T.notes[2] - 0.1), n('A2 D3 E4 F#4'), 0.5, {
		attack: 0.6,
		release: 1.2,
		level: 0.9,
		seed: 97
	});
}
