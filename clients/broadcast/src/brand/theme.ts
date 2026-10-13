// The old C()UCH TV theme's tag (D27): its hook, A4 D5 A5 G5 F#5, condensed on brass over the
// theme's cadence. The stings still play it until they move to the new brand's sound (sonic.ts).

import type { Bus } from '../audio/engine';
import type { Kit } from '../audio/kit';
import {
	brass,
	choir,
	crash,
	glock,
	hz,
	strings,
	subNote,
	swell,
	taikos
} from '../audio/orchestra';

export const HOOK = {
	notes: ['A4', 'D5', 'A5', 'G5', 'F#5'],
	at: [0, 0.25, 0.5, 1.0, 1.25],
	/** The tag states it a touch quicker (and every offset is a whole number of frames). */
	tagAt: [0, 0.2, 0.4, 0.8, 1.0]
} as const;

export const SCALES = {
	'D minor': ['D', 'E', 'F', 'G', 'A', 'Bb', 'C'],
	'D major': ['D', 'E', 'F#', 'G', 'A', 'B', 'C#'],
	'D mixolydian': ['D', 'E', 'F#', 'G', 'A', 'B', 'C']
} as const;

/** Every chord of the theme with the key it must belong to (a unit test checks them). */
export const CHART = {
	Bb: { notes: ['Bb1', 'F2', 'Bb2', 'D3'], key: 'D minor' },
	C: { notes: ['C2', 'G2', 'C3', 'E3'], key: 'D minor' },
	Cstr: { notes: ['C3', 'G3', 'C4', 'E4'], key: 'D minor' },
	Chorn: { notes: ['G3', 'C4', 'E4'], key: 'D minor' },
	Dhit: { notes: ['D2', 'A2', 'D3', 'F#3'], key: 'D major' },
	Dstr: { notes: ['D3', 'A3', 'D4', 'F#4'], key: 'D major' },
	CDstr: { notes: ['D3', 'G3', 'C4', 'E4'], key: 'D mixolydian' },
	Dchoir: { notes: ['D4', 'F#4', 'A4'], key: 'D major' },
	Dfinal: { notes: ['D2', 'A2', 'D3', 'A3', 'E4', 'F#4', 'A4'], key: 'D major' },
	Dbrass: { notes: ['D3', 'A3', 'D4', 'F#4'], key: 'D major' },
	Dchoir2: { notes: ['A3', 'D4', 'F#4', 'A4'], key: 'D major' }
} as const satisfies Record<string, { notes: readonly string[]; key: keyof typeof SCALES }>;

const C = (name: keyof typeof CHART) => CHART[name].notes.map(hz);
const HOOKF = HOOK.notes.map(hz);

/**
 * The tag: the hook condensed over the theme's own cadence, first note at `when`. Its lead-in
 * (a swell) starts 0.35 s earlier, so a cue for it belongs at least that much before.
 */
export function themeTag(k: Kit, b: Bus, when: number, { gain = 0.5 } = {}) {
	const g = gain / 0.5;
	const R = HOOK.tagAt;
	const n5 = when + R[4];
	const tail = 1.6;
	swell(k, b, when, 0.35, { gain: 0.05 * g });
	R.forEach((r, i) => {
		const len = i < 4 ? R[i + 1] - r - 0.02 : 1.0;
		brass(k, b, when + r, [HOOKF[i]], len, {
			gain: 0.13 * g,
			bright: 0.95,
			a: 0.03,
			r: i === 4 ? 1.1 : 0.1,
			verb: 0.4,
			seed: 330 + i
		});
		brass(k, b, when + r, [HOOKF[i] / 2], len, {
			gain: 0.1 * g,
			bright: 0.45,
			a: 0.04,
			r: i === 4 ? 1.1 : 0.12,
			verb: 0.45,
			seed: 340 + i
		});
		glock(k, b, when + r, HOOKF[i] * 2, { gain: 0.035 * g, decay: i === 4 ? 2 : 0.8 });
	});
	for (const [chord, a, z] of [
		['Dstr', 0, R[2]],
		['CDstr', R[2], R[4]]
	] as const)
		strings(k, b, when + a, C(chord), z - a - 0.02, {
			gain: 0.07 * g,
			a: 0.04,
			r: 0.12,
			cutoff: 4000,
			seed: 350 + a * 10
		});
	strings(k, b, n5, C('Dfinal'), 0.5, {
		gain: 0.085 * g,
		a: 0.04,
		r: tail,
		cutoff: 4200,
		seed: 360
	});
	choir(k, b, n5, C('Dchoir2'), 0.5, { gain: 0.05 * g, a: 0.1, r: tail });
	subNote(k, b, n5, hz('D1'), 0.5, { gain: 0.2 * g, r: 1.1 });
	taikos(k, b, when, { gain: 0.3 * g, f: 50, decay: 1.1 });
	taikos(k, b, n5, { gain: 0.42 * g, f: 46, decay: 1.6 });
	crash(k, b, n5, { gain: 0.08 * g, decay: 2.2 });
}
