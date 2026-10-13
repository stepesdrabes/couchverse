// The score: one idea. The couch is built from the notes that name it. In D major at 93.75 bpm
// the harmony walks down in thirds under the build (D, Bm9, Gmaj9, Em9, Asus4), changing chord
// on every lock, and each lock rings one note of the sonic logo on the glass: A when the arms
// lock, E when the dome locks, D when the slab does. A felted pulse carries the build from bar 1
// and stops when the popcorn starts to fall; the last chord swells, a reverse swell inhales into
// a breath of silence, and the landing is the loudest moment: the impact and D major (add 9),
// wide and low. Then the sonic logo for real, A, E, D on "couch", "t", "v", and the room rings
// out to near silence.

import type { Bus } from '../../../audio/engine';
import type { Kit } from '../../../audio/kit';
import { at, B, T } from './beats';
import {
	air,
	bass,
	bloom,
	glass,
	hz,
	impact,
	inhale,
	pad,
	pulse,
	shimmer,
	sub,
	swish,
	thud,
	tick
} from './voices';

export type Ev = { at: number; dur: number; fn: (w: number, b: Bus) => void };

/** The chords: [from (s), root, pad voicing]; the first is the opening's, the rest the build's. */
const CHORDS: [number, string, string[]][] = [
	[0, 'D2', ['A3', 'D4', 'E4']],
	[T.eclipse, 'B1', ['F#3', 'A3', 'D4', 'C#5']],
	[T.armsLock, 'G1', ['B3', 'D4', 'F#4', 'A4']],
	[T.domeLock, 'E1', ['G3', 'B3', 'D4', 'F#4']],
	[T.baseLock, 'A1', ['A3', 'D4', 'E4', 'A4']]
];

/** The sonic logo: couch, t, v. */
export const SONIC = ['A4', 'E5', 'D5'];

/** A note an octave up (B1 -> B2). */
const up = (n: string) => n.replace(/\d$/, (d) => String(Number(d) + 1));

export function score(k: Kit, D: number): Ev[] {
	const ev: Ev[] = [];
	const add = (t: number, fn: (w: number, b: Bus) => void, dur = 0) => ev.push({ at: t, dur, fn });

	// --- the void: air, a low D, an open chord, all out of nothing ------------------------------
	const [, root0, chord0] = CHORDS[0];
	add(
		0,
		(w, b) => {
			air(k, b, w, 1.1, { band: [700, 2400], attack: 0.7, release: 1.4, gain: 2.2, seed: 3 });
			sub(k, b, w, root0, 1.0, { attack: 0.6, release: 0.7, gain: 0.14 });
			pad(k, b, w, chord0, 0.6, {
				attack: 0.7,
				release: 1.2,
				cutoff: [900, 2600],
				gain: 0.42,
				seed: 4
			});
			shimmer(k, b, w, chord0, 0.6, { attack: 0.8, release: 1.2, gain: 0.5, seed: 8 });
		},
		3
	);
	// the near circle pulls away from the lens, then the white one swings past it
	add(
		0.05,
		(w, b) => swish(k, b, w, 1.0, { pans: [0.1, -0.35], band: [250, 700], gain: 4, seed: 5 }),
		1.2
	);
	add(
		0.75,
		(w, b) => swish(k, b, w, 1.0, { pans: [0.5, -0.1], band: [300, 900], gain: 5, seed: 6 }),
		1.2
	);

	// --- the build: a chord per lock -------------------------------------------------------------
	CHORDS.slice(1).forEach(([from, root, voicing], i) => {
		const last = i + 2 >= CHORDS.length;
		// the last chord holds through the fall, swelling, and stops on the breath before the landing
		const next = last ? T.land - 0.045 : CHORDS[i + 2][0];
		const len = next - from;
		const low = (last ? T.fall - from : len) - 0.1;
		const lift = i / 3;
		add(
			from,
			(w, b) => {
				pad(k, b, w, voicing, len - (last ? 0.25 : 0.2), {
					attack: i === 0 ? 0.5 : 0.25,
					release: 1.1,
					cutoff: [1500 + 900 * lift, last ? 7000 : 2800 + 1500 * lift],
					gain: 0.85 + 0.6 * lift,
					swell: last ? 2.8 : undefined,
					cut: last,
					seed: 10 + i
				});
				shimmer(k, b, w, voicing, len - 0.2, {
					attack: 0.4,
					release: last ? 0.05 : 1,
					gain: 0.5 + 0.5 * lift,
					seed: 30 + i
				});
				sub(k, b, w, root, low - 0.05, { attack: 0.08, release: 0.5, gain: 0.1 + 0.1 * lift });
				bass(k, b, w, up(root), low, {
					attack: 0.03,
					release: 0.4,
					cutoff: 600 + 500 * lift,
					gain: 0.5 + 0.35 * lift,
					seed: 20 + i
				});
			},
			len + 2
		);
	});
	// the pulse: felted eighths on the root from bar 1, opening up as the build climbs
	for (let t = T.eclipse; t < T.fall - 1e-6; t += B / 2) {
		const chord = [...CHORDS].reverse().find(([from]) => t >= from - 1e-6) ?? CHORDS[1];
		const p = (t - T.eclipse) / (T.fall - T.eclipse);
		const onBeat = Math.abs(t / B - Math.round(t / B)) < 1e-6;
		add(t, (w, b) =>
			pulse(k, b, w, up(chord[1]), {
				open: 0.35 + 0.6 * p,
				gain: (onBeat ? 0.95 : 0.7) * (0.45 + 0.85 * p),
				pan: onBeat ? -0.12 : 0.12,
				seed: Math.round(t * 100)
			})
		);
	}

	// --- the locks: a soft thud and a note of the sonic logo -------------------------------------
	const locks: [number, string, string, number][] = [
		[T.armsLock, 'G1', SONIC[0], 0.7],
		[T.domeLock, 'E1', SONIC[1], 0.85],
		[T.baseLock, 'A1', SONIC[2], 1]
	];
	for (const [t, low, note, g] of locks)
		add(
			t,
			(w, b) => {
				thud(k, b, w, { pitch: hz(low), gain: 0.38 * g, seed: Math.round(t * 10) });
				glass(k, b, w, note, {
					decay: 2.4,
					gain: 0.55 * g,
					pan: 0.1,
					seed: Math.round(t * 10) + 1
				});
			},
			3
		);
	// the dome's rise: air lifting from under its horizon
	add(
		at(2.75),
		(w, b) =>
			air(k, b, w, T.domeLock - at(2.75) - 0.3, {
				band: [500, 2200],
				attack: 0.9,
				release: 0.5,
				gain: 3.5,
				seed: 9
			}),
		3
	);
	// the slab glides in from the left
	add(
		T.baseLock - 1.05,
		(w, b) => swish(k, b, w, 1.0, { pans: [-0.8, 0], band: [220, 900], gain: 6, seed: 12 }),
		1.2
	);
	// the feet: two small clicks
	add(T.footL, (w, b) => tick(k, b, w, { pitch: 1500, gain: 1.6, pan: -0.25, seed: 51 }));
	add(T.footR, (w, b) => tick(k, b, w, { pitch: 1650, gain: 1.6, pan: 0.25, seed: 52 }));
	// the camera pulls back from the finished couch: air falling away
	add(
		T.fuse - 0.1,
		(w, b) => swish(k, b, w, 0.8, { pans: [0, 0], band: [1400, 380], gain: 4, seed: 14 }),
		1
	);
	// one glyph: the chord opens upward and the air rises toward the fall
	add(
		T.fuse,
		(w, b) => {
			pad(k, b, w, ['E4', 'A4', 'D5'], T.land - T.fuse - 0.3, {
				attack: 0.4,
				release: 0.3,
				cutoff: [1600, 5200],
				gain: 0.5,
				swell: 1.6,
				cut: true,
				seed: 61
			});
			air(k, b, w, T.land - T.fuse - 0.6, {
				band: [700, 3800],
				attack: 0.6,
				release: 0.2,
				gain: 3,
				seed: 62
			});
		},
		2
	);

	// --- the fall: everything holds its breath ------------------------------------------------
	add(
		T.fall,
		(w, b) =>
			inhale(k, b, w + (T.land - T.fall), T.land - T.fall, { breath: 0.045, gain: 14, seed: 71 }),
		1
	);

	// --- the landing ----------------------------------------------------------------------------
	add(
		T.land,
		(w, b) => {
			impact(k, b, w, { root: hz('D1'), gain: 0.95, seed: 81 });
			sub(k, b, w, 'D1', 0.6, { attack: 0.005, release: 0.7, gain: 0.35 });
			bloom(k, b, w, ['D3', 'A3', 'D4', 'F#4'], 0.45, {
				release: 1.0,
				top: 3400,
				gain: 1.6,
				seed: 86
			});
			bass(k, b, w, 'D2', 0.8, { attack: 0.005, release: 0.8, cutoff: 1100, gain: 0.9, seed: 82 });
			const wide = {
				attack: 0.06,
				cutoff: [7000, 2400] as [number, number],
				spread: 22,
				bright: 0.75,
				width: 1
			};
			pad(k, b, w, ['D3', 'A3', 'E4', 'F#4'], 0.7, {
				...wide,
				floor: 110,
				release: 1.3,
				gain: 2.8,
				seed: 83
			});
			// the top of the chord clears quickly, out of the sonic logo's way
			pad(k, b, w, ['A4', 'D5'], 0.25, { ...wide, release: 0.45, gain: 2.1, seed: 87 });
			pad(k, b, w, ['D2', 'A2'], 0.6, {
				attack: 0.05,
				release: 0.8,
				cutoff: [1600, 600],
				bright: 0.5,
				gain: 0.6,
				floor: 50,
				seed: 84
			});
			shimmer(k, b, w, ['D4', 'F#4', 'A4', 'E5'], 0.8, {
				attack: 0.12,
				release: 1.6,
				gain: 1.6,
				seed: 85
			});
		},
		6
	);

	// --- the sonic logo: couch, t, v --------------------------------------------------------------
	SONIC.forEach((n, i) => {
		const last = i === SONIC.length - 1;
		add(
			T.notes[i],
			(w, b) => {
				glass(k, b, w, n, {
					decay: last ? 2.9 : 2.4,
					gain: last ? 1.9 : 1.6,
					tone: 1.3,
					pan: (i - 1) * 0.12,
					seed: 90 + i
				});
				glass(k, b, w, hz(n) * 2, {
					decay: last ? 2 : 1.4,
					gain: 0.35,
					pan: -(i - 1) * 0.2,
					seed: 95 + i
				});
				if (last) {
					pad(k, b, w, ['A4', 'D5', 'F#5', 'A5'], 0.9, {
						attack: 0.18,
						release: 1.9,
						cutoff: [3000, 1300],
						gain: 0.75,
						seed: 99
					});
					sub(k, b, w, 'D2', 0.6, { attack: 0.08, release: 0.9, gain: 0.2 });
				}
			},
			5
		);
	});
	// the room under the logo: D major (add 9), low and soft, gone by the end
	add(
		T.notes[0] - 0.1,
		(w, b) =>
			pad(k, b, w, ['D3', 'A3', 'E4', 'F#4'], D - T.notes[0] - 2.5, {
				attack: 0.5,
				release: 1.3,
				cutoff: [2600, 1100],
				gain: 0.55,
				seed: 101
			}),
		D - T.notes[0]
	);
	// the last breath of the room fades away, so the cut to whatever follows lands on silence
	add(D - 0.62, (w, b) => b.fade(0, w, 0.6), 0.62);
	return ev;
}
