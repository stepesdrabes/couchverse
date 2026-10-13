// The score: one idea, in D major at 100 bpm on the picture's 0.6 s grid.
//
// The sonic logo is three notes, 5-1-3 (A, D, F#: "couch", "t", "v"). At dusk it is heard
// first as a slow, wistful echo in B minor (F#, B, D) on a felt piano over a dark pad. The
// chords climb with the camera: Bm(add9) at dusk, Gmaj9 as the town turns into view, Em11 as we
// push into the window, Asus4 swelling while the room goes dark, a felt pulse on D ticking
// underneath. The popcorn lands on D major with a deep impact; then the logo, fast and bright,
// and a long ring-out into silence.

import type { Bus } from '../../../audio/engine';
import type { Kit } from '../../../audio/kit';
import { rng } from '../../../core/rng';
import { air, felt, hz, impact, mixer, pad, shimmer, sub, swell, wind } from './sound';
import { at, T } from './time';

const notes = (ns: string[]) => ns.map(hz);

export function score(k: Kit, b: Bus, when: number, D: number) {
	const m = mixer(k, b);
	const w = (s: number) => when + s;

	// --- the chords ---
	pad(k, b, m, w(0), notes(['B2', 'F#3', 'C#4', 'D4', 'F#4']), at(4) + 0.2, {
		attack: 1.8,
		release: 1.3,
		cutoff: [480, 1150],
		gain: 0.25,
		seed: 11,
		hall: 0.55
	});
	pad(k, b, m, w(at(4) - 0.25), notes(['G2', 'D3', 'F#3', 'A3', 'B3', 'F#4']), at(2) + 0.35, {
		attack: 0.9,
		release: 1.1,
		cutoff: [650, 1500],
		gain: 0.5,
		seed: 12
	});
	pad(k, b, m, w(at(6) - 0.25), notes(['E2', 'B2', 'D3', 'G3', 'A3', 'E4']), at(2) + 0.35, {
		attack: 0.8,
		release: 1,
		cutoff: [800, 2000],
		gain: 1.02,
		seed: 13
	});
	// the suspension holds through the dark and swells into the landing, then stops dead
	pad(
		k,
		b,
		m,
		w(at(8) - 0.25),
		notes(['A2', 'E3', 'G3', 'D4', 'E4', 'A4']),
		T.land - at(8) + 0.25,
		{
			attack: 0.9,
			release: 0.18,
			cutoff: [900, 4000],
			gain: 1.18,
			seed: 14,
			swell: [
				[0, 1],
				[1.2, 1],
				[T.land - at(8) + 0.2, 2]
			]
		}
	);
	// the landing: D major, wide, bright at first and closing as it rings
	pad(k, b, m, w(T.land), notes(['D2', 'A2', 'F#3', 'A3', 'E4', 'F#4', 'A4']), D - T.land - 1.7, {
		attack: 0.05,
		release: 2.3,
		cutoff: [4400, 1000],
		gain: 0.76,
		seed: 15,
		hall: 0.6,
		// it blooms on the hit and settles under the sonic logo
		swell: [
			[0, 1],
			[0.9, 0.55],
			[2.2, 0.4]
		]
	});
	// --- the bass ---
	sub(k, b, m, w(0.2), hz('B1'), at(4) - 0.2, { gain: 0.08, attack: 1.4, release: 0.6 });
	sub(k, b, m, w(at(4)), hz('G1'), at(2), { gain: 0.1, attack: 0.3, release: 0.5 });
	sub(k, b, m, w(at(6)), hz('E2'), at(2), { gain: 0.1, attack: 0.3, release: 0.5 });
	sub(k, b, m, w(at(8)), hz('A1'), T.land - at(8) - 0.05, {
		gain: 0.15,
		attack: 0.3,
		release: 0.08
	});
	sub(k, b, m, w(T.land), hz('D2'), D - T.land - 2.2, {
		gain: 0.085,
		attack: 0.02,
		release: 1.8
	});
	// --- the motif at dusk: F#, B, D, slow, far away ---
	(
		[
			['F#4', at(1)],
			['B4', at(2)],
			['D5', at(3)]
		] as const
	).forEach(([n, t], i) =>
		felt(k, b, m, w(t), hz(n), {
			gain: 0.27 + i * 0.04,
			decay: 3.8,
			bright: 0.78,
			hall: 0.75,
			echo: 0.18,
			tone: 4200,
			pan: [-0.15, 0.1, 0][i]
		})
	);

	// --- the pulse: D ticking in eighths from the town to the fall, growing ---
	const from = at(4);
	const to = T.land - at(1);
	// a player's hand: each note a little louder or softer than the last, from a seed
	const touch = rng(77);
	for (let t = from; t < to - 1e-6; t += at(0.5)) {
		const p = (t - from) / (to - from);
		const beat = Math.abs(t / at(1) - Math.round(t / at(1))) < 1e-3;
		felt(k, b, m, w(t), hz('D5'), {
			gain: (0.12 + 0.48 * p) * (beat ? 1 : 0.72) * (0.9 + 0.2 * touch()),
			decay: 1.3,
			bright: 0.74,
			tone: 2200 + 3200 * p,
			hall: 0.35,
			echo: 0.32,
			pan: beat ? -0.12 : 0.12
		});
	}

	// the last two beats before the fall: a high D on the off-beats, tightening the pulse
	for (let t = at(8.5); t < to - 1e-6; t += at(1)) {
		const p = (t - at(8)) / (to - at(8));
		felt(k, b, m, w(t), hz('D6'), {
			gain: 0.08 + 0.12 * p,
			decay: 0.9,
			bright: 0.7,
			tone: 4800,
			hall: 0.4,
			echo: 0.3,
			pan: 0.3
		});
	}

	// --- air: wind on the hill at dusk, gone as the camera turns to town ---
	wind(k, b, m, w(0), T.panTo - 0.15, { gain: 0.32, fadeIn: 0.7, fadeOut: 1.1, seed: 41 });

	// --- the camera's moves ---
	// the whip: air rushing past, left to right with the turn, at its fastest mid-turn
	air(k, b, m, w(T.panFrom), T.panTo - T.panFrom, {
		gain: 0.3,
		lo: 260,
		hi: 1500,
		pan: [-0.55, 0.55],
		peak: 0.5,
		seed: 21
	});
	// through the window
	air(k, b, m, w(T.dollyPeak - 0.95), 1.15, {
		gain: 0.36,
		lo: 180,
		hi: 900,
		pan: [0, 0],
		peak: 0.55,
		seed: 22
	});

	// --- the fall and the landing ---
	swell(k, b, m, w(T.land), 1.5, { gain: 0.75, from: 260, to: 3600, seed: 31 });
	impact(k, b, m, w(T.land), hz('D1'), { gain: 0.62, seed: 32 });
	felt(k, b, m, w(T.land), hz('D3'), { gain: 0.4, decay: 4, bright: 0.7, hall: 0.6, tone: 2400 });
	felt(k, b, m, w(T.land), hz('A3'), { gain: 0.3, decay: 4, bright: 0.7, hall: 0.6, tone: 2600 });

	// --- the sonic logo: A, D, F# ("couch", "t", "v") ---
	(['A4', 'D5', 'F#5'] as const).forEach((n, i) => {
		const t = T.notes[i];
		const last = i === 2;
		felt(k, b, m, w(t), hz(n), {
			gain: last ? 0.8 : 0.72,
			decay: last ? 5 : 3.6,
			bright: 0.86,
			hall: 0.62,
			echo: 0.16,
			tone: 6200,
			pan: [-0.08, 0.06, 0][i]
		});
		felt(k, b, m, w(t), hz(n) / 2, { gain: 0.26, decay: 3.6, bright: 0.7, hall: 0.5, tone: 2600 });
		shimmer(k, b, m, w(t), hz(n) * 2, {
			gain: last ? 1 : 0.75,
			decay: last ? 2.4 : 1.4,
			pan: [-0.2, 0.15, 0][i]
		});
	});
	// the last note blooms into a soft chord
	pad(k, b, m, w(T.notes[2]), notes(['D4', 'F#4', 'A4', 'D5']), D - T.notes[2] - 1.8, {
		attack: 0.35,
		release: 1.9,
		cutoff: [2600, 1100],
		gain: 0.4,
		seed: 16,
		hall: 0.7
	});
}
