// The score: one idea, the sonic logo, carried through the journey. 75 bpm in D major, a chord
// per place: G (the dunes), Em (the sea), Cmaj7#11 (the city, the bVII's Lydian shimmer), A7sus4
// (the rise), D on the landing: IV, ii, bVII, V, I.
//
// The motif is D5 A5 F#5: the channel's hook (A4 D5 A5 G5 F#5) condensed to its three pillars.
// It is heard slowly on glass keys over the sea (quarters), quicker over the city (eighths),
// and finally as the sonic logo on sixteenths, typing couch, t, v. A harp pulse on A, quickening
// into a climbing run, drives the build; the camera's rise opens into a held breath; the landing
// is a tuned sub drop with the D chord blooming over it, the loudest moment; then everything
// rings out into silence.

import type { Bus } from '../../../audio/engine';
import type { Kit } from '../../../audio/kit';
import { BEAT, DROP, LAND, NOTES } from './plan';
import { air, fadeOut, glass, impact, pad, pluck, shimmer, sub, swell } from './voices';

export type Ev = { at: number; dur: number; fn: (w: number, b: Bus) => void };

const B = BEAT;

const CHORDS = {
	G: { bass: 'G1', pad: ['G2', 'D3', 'A3', 'B3', 'F#4'] },
	Em: { bass: 'E1', pad: ['E2', 'B2', 'F#3', 'G3', 'D4'] },
	C: { bass: 'C2', pad: ['C3', 'G3', 'B3', 'E4', 'F#4'] },
	A: { bass: 'A1', pad: ['A2', 'E3', 'G3', 'D4', 'E4'] },
	D: { bass: 'D2', pad: ['D2', 'A2', 'D3', 'F#3', 'A3', 'E4', 'F#4'] }
};

/** The motif's three notes. */
const MOTIF = ['D5', 'A5', 'F#5'];

export function score(k: Kit, D: number): Ev[] {
	const ev: Ev[] = [];
	const add = (at: number, fn: (w: number, b: Bus) => void, dur = 4) => ev.push({ at, dur, fn });

	// --- the bed: one pad chord per place, opening and swelling as the journey goes on --------
	add(0, (w, b) =>
		pad(k, b, w, CHORDS.G.pad, 2 * B - 0.3, {
			attack: 0.12,
			release: 1.4,
			from: 600,
			to: 1200,
			gain: 0.43,
			verb: 0.55,
			seed: 11
		})
	);
	add(2 * B, (w, b) =>
		pad(k, b, w, CHORDS.Em.pad, 2 * B - 0.3, {
			attack: 0.7,
			release: 1.3,
			from: 900,
			to: 1700,
			gain: 0.4,
			verb: 0.55,
			seed: 12
		})
	);
	add(4 * B, (w, b) =>
		pad(k, b, w, CHORDS.C.pad, 2 * B - 0.3, {
			attack: 0.6,
			release: 1.2,
			from: 1200,
			to: 2600,
			gain: 0.5,
			verb: 0.55,
			seed: 13,
			shape: [
				[0.6, 0.8],
				[1.9, 1.15]
			]
		})
	);
	// the rise: the dominant opens all the way, then holds its breath under the falling popcorn
	add(6 * B - 0.45, (w, b) =>
		pad(k, b, w, CHORDS.A.pad, LAND - 6 * B - 0.35, {
			attack: 0.95,
			release: 0.25,
			from: 1700,
			to: 5200,
			gain: 1.5,
			verb: 0.55,
			seed: 14,
			shape: [
				[0.95, 0.75],
				[1.45, 1],
				[1.7, 0.6],
				[2.35, 0.55]
			]
		})
	);
	// the landing: D, wide and bright, closing slowly as it rings out
	add(
		LAND,
		(w, b) =>
			pad(k, b, w, CHORDS.D.pad, 1.6, {
				attack: 0.03,
				release: 2.4,
				from: 7000,
				to: 1600,
				gain: 1.22,
				verb: 0.6,
				seed: 15,
				shape: [
					[0.03, 1],
					[0.6, 0.62],
					[1.63, 0.5]
				]
			}),
		6
	);

	// light over the sunset, and over the logo
	add(0.4, (w, b) =>
		shimmer(k, b, w, ['A3', 'B3', 'F#4'], 2.6, {
			attack: 1.4,
			release: 1.2,
			gain: 0.9,
			verb: 0.6,
			seed: 16
		})
	);
	add(
		LAND,
		(w, b) =>
			shimmer(k, b, w, ['A3', 'D4', 'F#4', 'A4'], 1.8, {
				attack: 0.15,
				release: 1.4,
				gain: 1.2,
				verb: 0.6,
				seed: 17
			}),
		5
	);

	// --- the sub ----------------------------------------------------------------------------------
	add(0, (w, b) =>
		sub(k, b, w, CHORDS.G.bass, 2 * B - 0.5, { attack: 0.8, release: 0.8, gain: 0.04 })
	);
	add(2 * B, (w, b) =>
		sub(k, b, w, CHORDS.Em.bass, 2 * B - 0.4, { attack: 0.5, release: 0.7, gain: 0.05 })
	);
	add(4 * B, (w, b) =>
		sub(k, b, w, CHORDS.C.bass, 2 * B - 0.4, { attack: 0.4, release: 0.6, gain: 0.07 })
	);
	add(6 * B, (w, b) =>
		sub(k, b, w, CHORDS.A.bass, B - 0.2, { attack: 0.3, release: 0.5, gain: 0.09 })
	);
	add(
		LAND + 0.6,
		(w, b) => sub(k, b, w, CHORDS.D.bass, 1.0, { attack: 0.4, release: 1.4, gain: 0.12 }),
		6
	);

	// --- the motif ----------------------------------------------------------------------------------
	// over the sea: slow quarters, the F# arriving as the couch crosses the sun
	MOTIF.forEach((n, i) =>
		add((1 + i) * B, (w, b) =>
			glass(k, b, w, n, {
				decay: 1.7,
				gain: 0.2,
				pan: [-0.25, 0.15, 0][i],
				verb: 0.7,
				echo: 0.4,
				seed: 30 + i
			})
		)
	);
	// over the city: eighths, the colours turned Lydian by the C beneath
	MOTIF.forEach((n, i) =>
		add(4 * B + i * 0.5 * B, (w, b) =>
			glass(k, b, w, n, {
				decay: 1.5,
				gain: 0.27,
				pan: [0.2, -0.15, 0.05][i],
				verb: 0.65,
				echo: 0.4,
				seed: 40 + i
			})
		)
	);

	// --- the pulse: a harp on A, eighths through the city, then sixteenths climbing the rise --
	for (let i = 0; i < 3; i++) {
		const at = 4 * B + i * 0.5 * B;
		add(at, (w, b) =>
			pluck(k, b, w, i % 2 ? 'A4' : 'A3', {
				decay: 1.4,
				gain: 0.22 + i * 0.03,
				pan: i % 2 ? 0.3 : -0.3,
				verb: 0.35,
				echo: 0.3,
				seed: 60 + i
			})
		);
	}
	['A3', 'A4', 'E4', 'A4', 'D5', 'E5'].forEach((n, i) => {
		const at = 5.5 * B + i * 0.25 * B;
		add(at, (w, b) =>
			pluck(k, b, w, n, {
				decay: 1.1,
				gain: 0.3 + i * 0.035,
				pan: i % 2 ? 0.35 : -0.35,
				verb: 0.35,
				echo: 0.3,
				seed: 70 + i
			})
		);
	});

	// --- air: the places, and the motion through them --------------------------------------------
	add(0, (w, b) =>
		air(k, b, w, 2.6, {
			freq: [520, 900],
			q: 0.7,
			peak: 0.06,
			panFrom: -0.4,
			panTo: 0.3,
			gain: 0.11,
			verb: 0.3,
			seed: 81,
			lfo: 0.45
		})
	);
	add(1.4, (w, b) =>
		air(k, b, w, 2.6, {
			freq: [380, 260],
			q: 0.5,
			peak: 0.45,
			panFrom: 0.3,
			panTo: -0.3,
			gain: 0.08,
			verb: 0.4,
			seed: 82,
			lfo: 0.3
		})
	);
	// the camera climbs
	add(4.1, (w, b) =>
		air(k, b, w, 1.55, {
			freq: [300, 3200],
			q: 1.1,
			peak: 0.85,
			panFrom: -0.2,
			panTo: 0.2,
			gain: 0.16,
			verb: 0.45,
			seed: 83
		})
	);
	// the popcorn falls into the frame
	add(DROP + 0.2, (w, b) =>
		air(k, b, w, LAND - DROP - 0.2, {
			freq: [2600, 700],
			q: 1.2,
			peak: 0.97,
			gain: 0.12,
			verb: 0.2,
			seed: 84
		})
	);
	// and the D chord draws in its breath before the hit
	add(LAND - 0.9, (w, b) =>
		swell(k, b, w + 0.9, 0.9, ['A3', 'D4', 'F#4', 'A4', 'D5'], {
			gain: 1.1,
			verb: 0.3,
			seed: 85,
			tau: 0.26
		})
	);

	// --- the landing ------------------------------------------------------------------------------
	add(LAND, (w, b) => impact(k, b, w, { gain: 0.33 }), 4);
	// a fast strum across the chord gives the bloom its edge
	['D3', 'A3', 'D4', 'F#4', 'A4', 'D5'].forEach((n, i) =>
		add(LAND + i * 0.014, (w, b) =>
			pluck(k, b, w, n, {
				decay: 2.6,
				gain: 0.31,
				pan: (i / 5) * 1.2 - 0.6,
				verb: 0.55,
				echo: 0.15,
				seed: 90 + i
			})
		)
	);

	// --- the sonic logo: couch, t, v --------------------------------------------------------------
	MOTIF.forEach((n, i) => {
		const last = i === MOTIF.length - 1;
		add(
			NOTES[i],
			(w, b) => {
				glass(k, b, w, n, {
					decay: last ? 1.15 : 0.8,
					gain: 0.42,
					bright: 1.15,
					pan: [-0.12, 0.12, 0][i],
					verb: 0.5,
					echo: last ? 0.3 : 0.18,
					seed: 100 + i
				});
				// an octave below for warmth, a harp for the attack
				glass(
					k,
					b,
					w,
					n.replace(/\d$/, (d) => String(Number(d) - 1)),
					{ decay: last ? 1.1 : 0.7, gain: 0.2, bright: 0.6, verb: 0.4, seed: 110 + i }
				);
				pluck(k, b, w, n, { decay: 1.0, gain: 0.18, verb: 0.3, seed: 120 + i });
			},
			5
		);
	});
	// the ring-out ends in silence
	add(D - 0.95, (w, b) => fadeOut(k, b, w, 0.93), 1);
	return ev;
}
