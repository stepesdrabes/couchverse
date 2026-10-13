// The sonic logo: A4, E5, D5 (the fifth, the ninth and the root of D major) on "couch", "t" and
// "v": "couch" on the beat after the popcorn lands, "t" half a beat later, "v" half a beat
// after that at 75 bpm, the last note held while D major (add 9) settles under it. As the
// approved morph ident plays it: a felt piano, glass an octave up blooming like a halo and a
// bowed swell under every note.
//
// Templates call it in four sizes: `sonicLogo` (the full statement, the short ident), `sonicSting`
// (a gap filler's, a little fuller), `sonicTag` (compact, over a promo's ducked bed as the
// lockup lands on its end card) and `sonicChime` (the motif alone on glass, the standby card);
// `popcornBuild`, `popcornFall` and `popcornLanding` are the moment before it, the popcorn
// dropping onto the couch. `when` is always the first note's time; a lead-in starts before it,
// so a cue that plays one belongs that much earlier (LEAD).

import type { Bus } from '../audio/engine';
import type { Kit } from '../audio/kit';
import { hz } from '../audio/pitch';
import {
	air,
	bowed,
	glass,
	house,
	impact,
	inhale,
	pad,
	padSweep,
	piano,
	riser,
	shimmer,
	sub,
	type House
} from '../audio/voices';

/** 75 bpm: a beat is 20 frames. */
export const BEAT = 0.8;

export const SONIC = {
	notes: ['A4', 'E5', 'D5'],
	/** Seconds from the first note: whole frames. */
	at: [0, BEAT / 2, BEAT],
	/** What the last note settles: D major, add 9. */
	chord: ['A2', 'D3', 'E4', 'F#4']
} as const;

/** How long before its first note a sonic logo's lead-in starts. */
export const LEAD = 0.4;

/** The full statement: piano, the glass halo and a bowed swell on every note; the chord settles. */
export function sonicLogo(h: House, when: number, { gain = 1 } = {}) {
	SONIC.notes.forEach((name, i) => {
		const t = when + SONIC.at[i];
		const last = i === SONIC.notes.length - 1;
		const f = hz(name);
		piano(h, t, f, {
			vel: 0.6 + 0.05 * i,
			dur: last ? 1.8 : 1.2,
			pan: -0.08 + 0.08 * i,
			verb: 0.6,
			gain: 1.45 * gain,
			seed: 400 + i
		});
		glass(h, t + 0.008, f * 2, {
			attack: 0.035,
			tone: 0.75,
			decay: last ? 2.4 : 1.8,
			gain: 0.08 * gain,
			pan: 0.18,
			verb: 0.75,
			seed: 410 + i
		});
		bowed(h, t, f, last ? 1.0 : 0.32, {
			gain: (last ? 1.3 : 1.0) * gain,
			attack: 0.22,
			release: last ? 1.0 : 0.5,
			verb: 0.6,
			seed: 420 + i
		});
	});
	// the last note settles the chord: the ninth joins, low and wide
	pad(h, when + SONIC.at[2] - 0.1, [...SONIC.chord], 0.5, {
		attack: 0.6,
		release: 1.2,
		level: 0.9 * gain,
		seed: 97
	});
}

/**
 * The build under the couch alone, from `from` to the landing: the dominant (A sus4) swelling
 * through the pads' filter opening, a sub under it, and the pads stepping back for the hit; the
 * filter closes again by `end`.
 */
export function popcornBuild(h: House, from: number, land: number, end: number, { gain = 1 } = {}) {
	padSweep(h, from, [
		[0, 900],
		[land - from - 0.05, 3800],
		[land - from + 0.05, 7000],
		[land - from + 1.4, 3600],
		[Math.max(land - from + 1.5, end - from), 1000]
	]);
	const pl = h.padLevel.gain;
	pl.setValueAtTime(1, from);
	pl.setValueAtTime(1, land - 0.12);
	pl.linearRampToValueAtTime(0.25, land - 0.02);
	pl.linearRampToValueAtTime(1, land + 0.02);
	const attack = Math.min(0.8, land - from - 0.1);
	pad(h, from, ['E3', 'A3', 'D4', 'E4', 'A4'], land - from - attack - 0.05, {
		attack,
		level: 1.3 * gain,
		cut: true,
		seed: 11
	});
	sub(h, from + 0.05, 'A1', Math.max(0.1, land - from - 0.65), {
		gain: 0.7 * gain,
		attack: Math.min(0.5, land - from - 0.2),
		release: 0.2
	});
}

/**
 * The popcorn's fall into `land`: the arrival chord's tones rising reversed and the air
 * inhaling, cut a breath before the hit so it lands on silence.
 */
export function popcornFall(h: House, land: number, dur: number, { gain = 1 } = {}) {
	riser(h, land, ['A2', 'E3', 'A3', 'E4', 'A4'], dur, { gain: 1.6 * gain, breath: 0.035 });
	inhale(h, land, Math.min(dur, 0.72), { gain: 0.5 * gain, breath: 0.045, seed: 71 });
}

/** The landing: the impact, D major (add 9) blooming wide and low, the light above it. */
export function popcornLanding(h: House, land: number, { gain = 1, ring = 1.7 } = {}) {
	impact(h, land, { gain: 0.6 * gain, root: 'D1' });
	pad(h, land, ['D3', 'A3', 'F#4', 'A4', 'E5'], ring, {
		attack: 0.09,
		release: 1.6,
		level: 0.8 * gain,
		spread: 26,
		seed: 91
	});
	sub(h, land, 'D2', ring - 0.1, { gain: 0.4 * gain, attack: 0.05, release: 1.2 });
	shimmer(h, land + 0.02, ['A5', 'D6', 'F#6', 'A6'], ring + 0.2, {
		gain: gain,
		attack: 0.5,
		release: 1.8,
		seed: 83
	});
	(['D2', 'A2', 'D3'] as const).forEach((name, i) =>
		piano(h, land + 0.005 * i, name, {
			vel: 0.72,
			dur: 2.8,
			pan: -0.15 + 0.15 * i,
			verb: 0.6,
			gain: 0.5 * gain,
			seed: 300 + i
		})
	);
}

/**
 * A gap filler's statement: `lead` seconds of air swelling into the first note with a low pad
 * under it (none after a landing), the motif, and the chord settling fuller and lower than the
 * tag's (the root on the piano and a sub under it).
 */
export function sonicSting(k: Kit, b: Bus, when: number, { gain = 1, lead = LEAD } = {}) {
	const h = house(k, b);
	if (lead > 0) {
		air(h, when - lead, lead + 0.5, {
			peakAt: lead,
			band: [400, 2400],
			gain: 0.5 * gain,
			seed: 67
		});
		pad(h, when - lead, ['A2', 'D3', 'A3', 'E4'], Math.max(0.05, lead - 0.5), {
			attack: Math.min(0.5, lead),
			release: 0.8,
			level: 0.55 * gain,
			seed: 66
		});
	}
	sonicLogo(h, when, { gain });
	const last = when + SONIC.at[2];
	piano(h, last, 'D2', { vel: 0.55, dur: 2.2, pan: -0.1, verb: 0.6, gain: 0.55 * gain, seed: 430 });
	sub(h, last - 0.05, 'D2', 0.9, { gain: 0.3 * gain, attack: 0.15, release: 1.2 });
}

/**
 * The compact tag, as a promo's end card lands the lockup: the popcorn's fall breathing in and
 * its soft round landing half a beat before the first note, then the motif on piano and glass,
 * the bow only under the last note, the D held into the hall over a short chord; `gain` 0.5 is
 * its designed level over a ducked bed.
 */
export function sonicTag(k: Kit, b: Bus, when: number, { gain = 0.5 } = {}) {
	const h = house(k, b);
	const g = gain / 0.5;
	const land = when - SONIC.at[1];
	air(h, land - 0.42, 0.6, { peakAt: 0.42, band: [500, 2600], gain: 0.4 * g, seed: 69 });
	impact(h, land, { gain: 0.22 * g, root: 'D1', seed: 4243 });
	SONIC.notes.forEach((name, i) => {
		const t = when + SONIC.at[i];
		const last = i === SONIC.notes.length - 1;
		const f = hz(name);
		piano(h, t, f, {
			vel: 0.62 + 0.05 * i,
			dur: last ? 1.4 : 0.9,
			pan: -0.06 + 0.06 * i,
			verb: 0.5,
			gain: 1.2 * g,
			seed: 440 + i
		});
		glass(h, t + 0.008, f * 2, {
			attack: 0.02,
			tone: 0.8,
			decay: last ? 1.8 : 1.2,
			gain: 0.07 * g,
			pan: 0.16,
			verb: 0.7,
			seed: 450 + i
		});
		if (last)
			bowed(h, t, f, 0.6, { gain: 1.0 * g, attack: 0.18, release: 0.8, verb: 0.55, seed: 460 });
	});
	pad(h, when + SONIC.at[2] - 0.08, [...SONIC.chord], 0.3, {
		attack: 0.4,
		release: 0.9,
		level: 0.7 * g,
		seed: 98
	});
	sub(h, when + SONIC.at[2], 'D2', 0.4, { gain: 0.25 * g, attack: 0.08, release: 0.8 });
}

/** The motif alone on struck glass, an octave up, over a soft sub (the standby card). */
export function sonicChime(k: Kit, b: Bus, when: number, { gain = 0.5 } = {}) {
	const h = house(k, b);
	const g = gain / 0.5;
	SONIC.notes.forEach((name, i) => {
		const last = i === SONIC.notes.length - 1;
		glass(h, when + SONIC.at[i], hz(name) * 2, {
			decay: last ? 2.8 : 1.8,
			gain: (last ? 1.5 : 1.25) * g,
			tone: 1.1,
			pan: [-0.2, 0.15, 0][i],
			verb: 0.7,
			seed: 470 + i
		});
	});
	sub(h, when + SONIC.at[2], 'D2', 0.3, { gain: 0.4 * g, attack: 0.1, release: 1.2 });
}
