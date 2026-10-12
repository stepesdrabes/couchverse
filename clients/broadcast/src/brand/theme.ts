// The Couch TV theme, "hybrid epic" (D27): a D minor build (i, bVI, bVII) lifting to D major on
// the hit, the hook on brass, a D add9 final chord. Everything that sounds like the brand is
// here: `themeScore` scores an ident cut, `themeTag` is the hook condensed (promo tags and
// stings), `themeChime` the hook alone on the glockenspiel (the standby card).
//
// Grid: 120 bpm (a beat is 0.5 s). Picture (ident.ts) and score both read GRID, so the brackets
// snap shut on the hit and TV drops into its block on the hook's last note.
//
// Hook (the sonic logo): A4 D5 A5 G5 F#5, "da-da DAA da DAAA": up a fourth and a fifth (the
// brackets opening), down two steps to the major third (closing); two eighths, a quarter, an
// eighth, then the long note pushed onto the "and" of beat three.

import type { Bus } from '../audio/engine';
import type { Kit } from '../audio/kit';
import {
	braam,
	brass,
	choir,
	clap,
	crash,
	drum,
	glock,
	hz,
	roll,
	seedOf,
	shepard,
	snare,
	spiccato,
	strings,
	subNote,
	swell,
	taikos,
	themeBus,
	type SpiccatoStep
} from '../audio/orchestra';
import type { SoundFn } from '../core/timeline';

export interface Grid {
	cut: 'full' | 'short';
	/** The cut's length in the score (the clip rounds it to whole frames). */
	D: number;
	hit: number;
	final: number;
}

export const GRID: Record<Grid['cut'], Grid> = {
	full: { cut: 'full', D: 10.5, hit: 6.0, final: 8.0 },
	short: { cut: 'short', D: 4.5, hit: 2.0, final: 3.25 }
};

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
	Dm: { notes: ['D2', 'A2', 'D3', 'F3'], key: 'D minor' },
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
const F = (names: string[]) => names.map(hz);
const HOOKF = HOOK.notes.map(hz);

export interface ScoreEvent {
	at: number;
	/** Lets a long event start part-way through when playback starts inside it. */
	dur: number;
	fn: SoundFn;
}

/** An event list whose callbacks get the event's own (possibly past) start time. */
function events() {
	const list: ScoreEvent[] = [];
	const at = (t: number, fn: (when: number, b: Bus) => void, dur = 0) =>
		list.push({ at: t, dur, fn: (w, b, skip = 0) => fn(w - skip, b) });
	return { list, at };
}

/** The ident's score for one cut of the grid. */
export function themeScore(k: Kit, g: Grid): ScoreEvent[] {
	const { list, at } = events();
	const H = g.hit;
	const end = g.final;

	// --- the build -----------------------------------------------------------------------------
	if (g.cut === 'full') {
		at(
			0,
			(w, b) => {
				taikos(k, b, w, { gain: 0.3, f: 50, decay: 1.4 });
				braam(k, b, w, C('Dm'), { gain: 0.24, dur: 2.2, drive: 1.3, open: [160, 1500, 520] });
				subNote(k, b, w, hz('D1'), 1.8, { gain: 0.1, a: 0.05, r: 0.4 });
			},
			2
		);
		// low strings hold the tonic, then lean into the build
		at(
			0.05,
			(w, b) =>
				strings(k, b, w, F(['D2', 'A2', 'D3']), 5.7, {
					gain: 0.112,
					a: 1.2,
					r: 0.2,
					cutoff: 1400,
					hp: 50,
					swell: [
						[0, 0.6],
						[4, 1],
						[5.8, 1.4]
					]
				}),
			5.9
		);
		for (const [t, f, gn] of [
			[1.0, 62, 0.16],
			[1.5, 70, 0.22],
			[1.75, 70, 0.16]
		])
			at(t, (w, b) => drum(k, b, w, { gain: gn, f, decay: 0.7, pan: -0.2, seed: seedOf(t) }));
		at(
			2.0,
			(w, b) => {
				taikos(k, b, w, { gain: 0.46, f: 47, decay: 1.4 });
				braam(k, b, w, C('Bb'), { gain: 0.26, dur: 2.1, drive: 1.3, open: [170, 1800, 560] });
				subNote(k, b, w, hz('Bb1'), 1.8, { gain: 0.09, a: 0.02, r: 0.3 });
			},
			2
		);
		for (const [t, f, gn, p] of [
			[2.75, 88, 0.24, 0.3],
			[3.0, 66, 0.3, -0.2],
			[3.5, 88, 0.26, 0.35],
			[3.75, 100, 0.22, -0.3]
		])
			at(t, (w, b) => drum(k, b, w, { gain: gn, f, decay: 0.6, pan: p, seed: seedOf(t, 2) }));
		at(
			4.0,
			(w, b) => {
				taikos(k, b, w, { gain: 0.5, f: 52, decay: 1.2 });
				braam(k, b, w, C('C'), { gain: 0.27, dur: 1.9, drive: 1.35, open: [180, 2000, 600] });
				subNote(k, b, w, hz('C2'), 1.8, { gain: 0.07, a: 0.02, r: 0.1 });
				brass(k, b, w, C('Chorn'), 0.32, { gain: 0.242, bright: 0.7, r: 0.25, seed: 4 });
			},
			2
		);
		at(5.0, (w, b) =>
			brass(k, b, w, C('Chorn'), 0.32, { gain: 0.286, bright: 0.9, r: 0.25, seed: 5 })
		);
		at(
			4.0,
			(w, b) =>
				strings(k, b, w, C('Cstr'), 1.86, {
					gain: 0.112,
					a: 0.08,
					r: 0.04,
					trem: 0.7,
					cutoff: 3600,
					swell: [
						[0, 0.5],
						[1.86, 1.25]
					]
				}),
			1.9
		);
		at(4.0, (w, b) =>
			roll(k, b, w, w + 1.9, {
				voice: 'drum',
				f: 78,
				gain: 0.3,
				from: 0.35,
				to: 1,
				pitch: [1, 1.3],
				verb: 0.25
			})
		);
		at(5.0, (w, b) =>
			roll(k, b, w, w + 0.9, {
				voice: 'snare',
				gain: 0.17,
				from: 0.25,
				to: 1,
				grid: [0.125, 0.0625, 0.03125]
			})
		);
		at(4.4, (w, b) => swell(k, b, w + 1.5, 1.5, { gain: 0.1 }));
		at(4.0, (w, b) => shepard(k, b, w, 1.9, { gain: 0.045 }));
		// the pulse: eighths on D, then sixteenths through B-flat and C, out a beat before the hit
		at(0.5, (w, b) => {
			const steps: SpiccatoStep[] = [];
			for (let t = 0.5; t < 5.5 - 1e-6; t += t < 2 ? 0.25 : 0.125) {
				const i = Math.round(t / 0.125);
				const pat =
					t < 2
						? ['D2', 'D3']
						: t < 4
							? ['Bb1', 'Bb2', 'F2', 'Bb2']
							: t < 5
								? ['C2', 'C3', 'G2', 'C3']
								: ['C3', 'G3', 'C4', 'G3'];
				steps.push({
					when: w + t - 0.5,
					f: hz(pat[i % pat.length]),
					gain: (0.05 + 0.07 * (t / 5.5)) * (i % 4 === 0 ? 1.3 : 1),
					pan: i % 2 ? 0.25 : -0.25
				});
			}
			spiccato(k, b, steps, { cutoff: [500, 3200], verb: 0.2 });
		});
	} else {
		at(
			0,
			(w, b) => {
				taikos(k, b, w, { gain: 0.42, f: 47, decay: 1.2 });
				braam(k, b, w, C('Bb'), { gain: 0.25, dur: 1.1, drive: 1.3, open: [170, 1800, 560] });
				subNote(k, b, w, hz('Bb1'), 0.9, { gain: 0.08, a: 0.02, r: 0.1 });
			},
			1
		);
		at(
			1.0,
			(w, b) => {
				taikos(k, b, w, { gain: 0.46, f: 52, decay: 1.0 });
				braam(k, b, w, C('C'), { gain: 0.26, dur: 0.95, drive: 1.35, open: [180, 2000, 600] });
				subNote(k, b, w, hz('C2'), 0.85, { gain: 0.065, a: 0.02, r: 0.05 });
				brass(k, b, w, C('Chorn'), 0.3, { gain: 0.264, bright: 0.8, r: 0.2, seed: 6 });
				strings(k, b, w, C('Cstr'), 0.88, {
					gain: 0.112,
					a: 0.06,
					r: 0.03,
					trem: 0.7,
					cutoff: 3600,
					swell: [
						[0, 0.6],
						[0.88, 1.3]
					]
				});
			},
			1
		);
		at(0.25, (w, b) =>
			roll(k, b, w, w + 1.65, {
				voice: 'drum',
				f: 78,
				gain: 0.28,
				from: 0.3,
				to: 1,
				pitch: [1, 1.3],
				grid: [0.25, 0.125, 0.0625]
			})
		);
		at(1.25, (w, b) =>
			roll(k, b, w, w + 0.65, {
				voice: 'snare',
				gain: 0.16,
				from: 0.3,
				to: 1,
				grid: [0.0625, 0.03125]
			})
		);
		at(0.6, (w, b) => swell(k, b, w + 1.3, 1.3, { gain: 0.1 }));
		at(0, (w, b) => shepard(k, b, w, 1.9, { gain: 0.045 }));
		at(0, (w, b) => {
			const steps: SpiccatoStep[] = [];
			for (let t = 0; t < 1.5 - 1e-6; t += 0.125) {
				const i = Math.round(t / 0.125);
				const pat = t < 1 ? ['Bb1', 'Bb2', 'F2', 'Bb2'] : ['C3', 'G3', 'C4', 'G3'];
				steps.push({
					when: w + t,
					f: hz(pat[i % 4]),
					gain: (0.06 + 0.07 * (t / 1.5)) * (i % 4 === 0 ? 1.3 : 1),
					pan: i % 2 ? 0.25 : -0.25
				});
			}
			spiccato(k, b, steps, { cutoff: [700, 3200], verb: 0.2 });
		});
	}

	// --- the hit: D major ---------------------------------------------------------------------
	at(
		H,
		(w, b) => {
			taikos(k, b, w, { gain: 0.42, f: 48, decay: 1.6 });
			k.impact(b, w, {
				gain: 0.15,
				size: 1.2,
				tone: 44,
				click: 0.4,
				verb: 0.2,
				dest: themeBus(k, b).in
			});
			crash(k, b, w, { gain: 0.15, decay: 2.4 });
			braam(k, b, w, C('Dhit'), { gain: 0.3, dur: 2.0, drive: 1.4, open: [220, 2600, 700] });
			subNote(k, b, w, hz('D1'), end - H - 0.05, { gain: 0.11, a: 0.01, r: 0.2 });
			choir(k, b, w, C('Dchoir'), end - H - 0.1, { gain: 0.091, a: 0.3, r: 0.5 });
		},
		end - H
	);
	// strings move with the hook: D, C over D, D
	for (const [dt, chord, len] of [
		[0, 'Dstr', 0.5],
		[0.5, 'CDstr', 0.75],
		[1.25, 'Dstr', end - H - 1.25]
	] as const)
		at(
			H + dt,
			(w, b) =>
				strings(k, b, w, C(chord), len, {
					gain: 0.136,
					a: 0.04,
					r: 0.25,
					cutoff: 4200,
					seed: seedOf(dt, 7)
				}),
			len
		);
	// the hook: trumpets at pitch, horns an octave below, a glockenspiel an octave above
	const last = end - H - 1.25 + (g.cut === 'full' ? 0.9 : 0.6);
	HOOK.at.forEach((r, i) => {
		const len = i < 4 ? HOOK.at[i + 1] - r - 0.02 : last;
		const f = HOOKF[i];
		at(
			H + r,
			(w, b) => {
				brass(k, b, w, [f], len, {
					gain: 0.33,
					bright: 1.0,
					a: 0.03,
					r: i === 4 ? 1.2 : 0.12,
					pan: 0.1,
					verb: 0.4,
					seed: 70 + i
				});
				brass(k, b, w, [f / 2], len, {
					gain: 0.264,
					bright: 0.45,
					a: 0.04,
					r: i === 4 ? 1.2 : 0.14,
					verb: 0.45,
					seed: 80 + i
				});
				glock(k, b, w, f * 2, { gain: 0.056, decay: i === 4 ? 2.2 : 0.9, pan: 0.2 });
			},
			len
		);
	});
	// drums under the hook: big drums on 1 and 3, a fat snare and clap on 2 and 4, a fill
	const groove: [number, 'T' | 't' | 'S'][] = [
		[0.25, 't'],
		[0.5, 'S'],
		[0.75, 't'],
		[1.0, 'T'],
		[1.25, 't'],
		[1.5, 'S'],
		[1.625, 't'],
		[1.75, 't'],
		[1.875, 't']
	];
	for (const [dt, kind] of groove) {
		if (H + dt >= end - 1e-6) continue;
		at(H + dt, (w, b) => {
			if (kind === 'T') taikos(k, b, w, { gain: 0.4, f: 50, decay: 1.0 });
			else if (kind === 'S') {
				snare(k, b, w, { gain: 0.528, tone: 175, verb: 0.35, seed: seedOf(dt, 3) });
				clap(k, b, w, { gain: 0.396, verb: 0.3, seed: seedOf(dt, 4) });
			} else
				drum(k, b, w, {
					gain: 0.2,
					f: 90 + dt * 20,
					decay: 0.45,
					pan: dt % 0.5 ? 0.3 : -0.3,
					seed: seedOf(dt, 5)
				});
		});
	}

	// --- the final chord: D add9 ----------------------------------------------------------------
	const tail = g.D - end;
	at(
		end,
		(w, b) => {
			taikos(k, b, w, { gain: g.cut === 'full' ? 0.45 : 0.4, f: 48, decay: 1.8 });
			crash(k, b, w, { gain: 0.13, decay: 3 });
			braam(k, b, w, C('Dhit'), {
				gain: 0.22,
				dur: tail + 0.6,
				drive: 1.25,
				open: [200, 2000, 500]
			});
			strings(k, b, w, C('Dfinal'), tail * 0.55, {
				gain: 0.16,
				a: 0.05,
				r: tail * 0.7,
				cutoff: 4400,
				seed: 91
			});
			brass(k, b, w, C('Dbrass'), tail * 0.35, {
				gain: 0.264,
				bright: 0.6,
				a: 0.05,
				r: tail * 0.6,
				seed: 92
			});
			choir(k, b, w, C('Dchoir2'), tail * 0.5, { gain: 0.0845, a: 0.15, r: tail * 0.6 });
			glock(k, b, w, hz('F#6'), { gain: 0.064, decay: 2.6, pan: 0.25 });
			glock(k, b, w + 0.12, hz('A6'), { gain: 0.048, decay: 2.4, pan: -0.25 });
			subNote(k, b, w, hz('D1'), tail * 0.5, { gain: 0.12, a: 0.01, r: tail * 0.6 });
		},
		tail
	);
	return list;
}

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

/** The hook alone on the glockenspiel, an octave up, in the tag's rhythm (the standby card). */
export function themeChime(k: Kit, b: Bus, when: number, { gain = 0.5 } = {}) {
	const g = gain / 0.5;
	HOOK.tagAt.forEach((r, i) =>
		glock(k, b, when + r, HOOKF[i] * 2, {
			gain: (i === 4 ? 0.09 : 0.065) * g,
			decay: i === 4 ? 2.4 : 1.1,
			pan: [-0.2, 0.1, 0.2, -0.1, 0][i]
		})
	);
	subNote(k, b, when + HOOK.tagAt[4], hz('D2'), 0.4, { gain: 0.1 * g, r: 1.2 });
}
