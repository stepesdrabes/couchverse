// The trailer's sound: every scene plays its own sound at one dialogue level, the title's
// soundtrack rises under them for the montage, the title and the end card and gives way to
// every line, and the house instruments (D major, 75 bpm) carry the stretches the soundtrack
// does not reach and punctuate the cuts, the cards and the hit. Without a soundtrack the house
// plays all of it.
//
// Under speech the music is down 13 dB (the bed through its own ride, the house through its
// mix's gain, both sample-accurate automation) and every scene's sound has short fades at its
// edges and never a fade that reaches a word, so a cut never clicks and no word is dimmed.

import { applyEnv, type LoadedBed } from '../audio/engine';
import type { Kit } from '../audio/kit';
import {
	BED_LUFS,
	DIALOGUE_LUFS,
	EFFECTS_LUFS,
	gainOf,
	levelling,
	loudnessOf,
	OVER,
	playRidden,
	playScene,
	ride,
	type Duck,
	type Line
} from '../audio/mix';
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
	pulse,
	riser,
	shimmer,
	sub,
	type House,
	type Pitch
} from '../audio/voices';
import { BEAT } from '../brand/sonic';
import type { TL } from './shared';
import { KNOBS, scenesOf, type Mood, type Plan, type Section } from './trailer-plan';

export interface ScoreScene {
	/** The scene's own sound, null when it did not load. */
	sound: LoadedBed | null;
	/** Its lines, seconds into the scene. */
	lines: [number, number][];
}

export interface ScoreInput {
	tl: TL;
	kit: Kit;
	mood: Mood;
	plan: Plan;
	/** The trailer's length with its end card. */
	D: number;
	/** When the channel tag's first note comes, on the trailer's clock. */
	tagAt: number;
	bed: LoadedBed | null;
	scenes: ScoreScene[];
	premiere: boolean;
}

/** The house sits further down under speech than the soundtrack: it is quiet to begin with. */
const HOUSE_DUCK: Duck = { depth: -9, attack: 0.3, release: 0.7, before: 0.05, after: 0.15 };
/** A scene's sound never fades in longer than this, nor out. */
const FADE = { in: 0.16, out: 0.3, min: 0.012 };

type CueFn = (at: number, fn: (h: House, w: number) => void, dur?: number) => void;

const find = <K extends Section['kind']>(plan: Plan, kind: K) =>
	plan.sections.filter((s): s is Extract<Section, { kind: K }> => s.kind === kind);

/** D major, wide and low: the title's chord. */
const D_CHORD: Pitch[] = ['D2', 'A2', 'D3', 'F#3', 'A3', 'E4'];
/** The underscore's chords: D, B minor, G and A, each a voicing that sits under a voice. */
const UNDER: Pitch[][] = [
	D_CHORD,
	['B1', 'F#2', 'B2', 'D3', 'F#3', 'A3'],
	['G1', 'D2', 'G2', 'B2', 'D3', 'F#3'],
	['A1', 'E2', 'A2', 'C#3', 'E3', 'B3']
];

/**
 * Where the soundtrack sits under the story, dB under its reference: with the speech's duck
 * (13 dB) that is 12 dB under the dialogue, and between the lines it comes up to meet it.
 */
export const STEADY = -6;

/**
 * The soundtrack's level on the trailer's clock, as `[t, dB]` keys under its reference: it comes
 * in (with the channel's moment when it is long enough, else with the riser), holds steady under
 * the story, builds through the riser and the montage, dips for a breath and lands on the hit,
 * sits under the button, bows to the channel tag and fades out at `stop` (the speech's duck comes
 * on top of this).
 */
export function bedKeys(plan: Plan, mood: Mood, tagAt: number, stop: number): [number, number][] {
	const montage = find(plan, 'montage')[0];
	const title = find(plan, 'title')[0];
	const at = plan.bedAt ?? 0;
	const hit = plan.hit;
	const B = plan.end;
	const keys: [number, number][] = [[at, -40]];
	// a soundtrack that came in with the story holds steady under it until the riser
	if (at + 2.4 < plan.riseAt - 0.3) keys.push([at + 2.4, STEADY], [plan.riseAt, STEADY]);
	keys.push(
		// (without a montage the riser leads straight to the hit)
		[montage?.at ?? Math.max(at + 0.1, hit - 0.4), -5.5],
		[hit - 0.3, -3.5],
		[hit - 0.09, -16],
		[hit, -3.5],
		[hit + 1.2, -1.5]
	);
	if (title.end < B - 0.01)
		keys.push([title.end - 0.6, -1], [title.end + 0.2, -11], [B - 0.2, -11]);
	else keys.push([title.end - 0.6, -3]);
	if (mood === 'grit') keys.push([stop - 0.08, -2.5]);
	else keys.push([tagAt - 0.25, -3], [tagAt, -8], [Math.max(tagAt + 0.8, stop - 1.4), -9]);
	const end: [number, number] = [stop, -80];
	return [...keys.filter(([t]) => t <= stop - 0.08), end].sort((x, y) => x[0] - y[0]);
}

export function scoreTrailer(c: ScoreInput) {
	const { tl, kit, mood, plan, D, tagAt, bed } = c;
	const K = KNOBS[mood];
	const grit = mood === 'grit';
	const bright = mood === 'bright';
	const B = plan.end;
	const scenes = scenesOf(plan);
	// how much quieter than the dialogue's level each line stayed, by its scene's start and its index
	const quieter = new Map<string, number>();
	const montage = find(plan, 'montage')[0];
	const title = find(plan, 'title')[0];
	// the hit is the montage's end; a trailer short of picture has no montage, the riser leads to it
	const m0 = montage?.at ?? plan.hit;
	const m1 = plan.hit;
	const rise = plan.riseAt;
	const cue: CueFn = (at, fn, dur = 0) =>
		tl.sound(at, (w, b, skip) => fn(house(kit, b), w - skip), dur);

	// --- the scenes' own sound ---------------------------------------------------------------

	for (const s of scenes) {
		const own = c.scenes[s.scene];
		if (!own.sound) continue;
		const inside = own.lines.filter((l) => l[0] >= s.soundFrom - 1e-6 && l[1] <= s.soundTo + 1e-6);
		// the dialogue is levelled by its lines (a little around each); a scene without any by
		// the whole stretch, a little under
		const spoken = inside.length
			? inside.map(([a, b]): [number, number] => [
					own.sound!.in + a - 0.05,
					own.sound!.in + b + 0.05
				])
			: [[own.sound.in + s.soundFrom, own.sound.in + s.soundTo] as [number, number]];
		const target = inside.length ? DIALOGUE_LUFS : EFFECTS_LUFS;
		const whole = loudnessOf(own.sound.buffer, [
			[own.sound.in + s.soundFrom, own.sound.in + s.soundTo]
		]);
		const heard = inside.length ? loudnessOf(own.sound.buffer, spoken) : whole;
		const over = inside.length ? OVER.dialogue : OVER.effects;
		const lift = levelling(heard.lufs, target, whole.peak, target + over);
		const gain = gainOf(lift) * own.sound.gain;
		// a line the levelling could not lift to the dialogue's level (it lifts 14 dB at most, and
		// never past its scene's loudest moment's room) takes the music further down under it
		inside.forEach(([a, b], i) => {
			const own1 = loudnessOf(own.sound!.buffer, [
				[own.sound!.in + a - 0.05, own.sound!.in + b + 0.05]
			]);
			if (Number.isFinite(own1.lufs))
				quieter.set(`${s.at}:${i}`, Math.min(12, Math.max(0, DIALOGUE_LUFS - (own1.lufs + lift))));
		});
		const first = inside[0];
		const last = inside.at(-1);
		// a fade stops short of the first word and starts after the last
		const fadeIn =
			s.role === 'open'
				? Math.min(1.0, Math.max(0.3, first ? first[0] - s.soundFrom - 0.06 : 1))
				: first
					? Math.min(FADE.in, Math.max(FADE.min, first[0] - s.soundFrom - 0.05))
					: 0.02;
		const fadeOut = last
			? Math.min(FADE.out, Math.max(FADE.min, s.soundTo - last[1] - 0.04))
			: 0.12;
		const start = s.at - (s.from - s.soundFrom);
		// a sound shorter than its scene ends where it ends, faded there
		const to = Math.min(s.soundTo, own.sound.length);
		const length = to - s.soundFrom;
		const sound = { loaded: own.sound, from: s.soundFrom, to, gain, fadeIn, fadeOut };
		tl.sound(start, (w, b, skip) => playScene(kit, b, w, sound, skip), length + 0.05);
	}

	const lines: Line[] = scenes.flatMap((s) =>
		s.lines.map(([a, b], i): Line => [a, b, quieter.get(`${s.at}:${i}`) ?? 0])
	);

	// --- the soundtrack, ridden ----------------------------------------------------------------

	const bedLevel = (length: number) =>
		bed
			? gainOf(levelling(loudnessOf(bed.buffer, [[bed.in, bed.in + length]]).lufs, BED_LUFS)) *
				bed.gain
			: 0;
	if (bed && plan.bedAt !== null) {
		const at = plan.bedAt;
		// grit's soundtrack stops a beat before the tag; the others bow to it and fade with the clip
		const length = Math.min(bed.length, (grit ? tagAt - BEAT : D) - at);
		const base = bedKeys(plan, mood, tagAt, at + length);
		const level = bedLevel(length);
		const points = ride(base, lines, at, at + length).map(([t, g]): [number, number] => [
			t - at,
			g * level
		]);
		tl.sound(
			at,
			(w, b, skip) => playRidden(kit, b, w, bed, { from: 0, length, points }, skip),
			length
		);
	}
	const bedOn = (t: number) => !!bed && plan.bedAt !== null && t >= plan.bedAt;

	// --- the house: its mix gives way to every line ------------------------------------------------

	// without a soundtrack the house is the music, and carries the montage and the title as one
	// does: its mix rises with them, over what its voices were set for under a promo
	const trim: [number, number][] = bed
		? [[0, 0]]
		: [
				[0, 0],
				[rise, 0],
				...(montage ? [[m0, 4] as [number, number], [m1 - 0.2, 8] as [number, number]] : []),
				[m1, 12],
				[m1 + 2, 10],
				[title.end, 5],
				[B - 0.3, 3],
				[B, 6]
			];
	const down = ride(trim, lines, 0, D, { duck: HOUSE_DUCK });
	tl.sound(
		0,
		(w, b, skip) => {
			const h = house(kit, b);
			applyEnv(h.out.gain, down, w, skip);
		},
		D
	);

	const presents = find(plan, 'presents')[0];
	const cards = find(plan, 'card');
	const premiere = find(plan, 'premiere')[0];
	const acts = scenes.filter((s) => s.role === 'act');

	// the underscore: the chords change at the channel's moment, at every card and with every
	// other act, until a soundtrack takes over (it fades under the soundtrack's entry, at the
	// riser or earlier when the excerpt is long enough). Where the soundtrack plays the house only
	// punctuates, unpitched: its key is not the house's.
	const bedFrom = bed && plan.bedAt !== null ? plan.bedAt : null;
	const underEnd = bedFrom === null ? rise : bedFrom + 2;
	const pitched = (t: number) => !bedOn(t);
	const turns = [
		0,
		presents.at,
		...cards.map((k) => k.at),
		...acts.filter((_, i) => i % 2 === 1).map((s) => s.at),
		...(premiere ? [premiere.at] : [])
	]
		.filter((t) => t === 0 || t < (bedFrom === null ? rise - 1.5 : bedFrom - 0.5))
		.sort((a, b) => a - b)
		.filter((t, i, all) => i === 0 || t - all[i - 1] > 2.5);
	const under = grit ? ['D2', 'A2', 'D3', 'F3'] : null;
	turns.forEach((t, i) => {
		const next = turns[i + 1] ?? underEnd;
		const chord = under ?? UNDER[i % UNDER.length];
		cue(
			t,
			(h, w) =>
				pad(h, w, chord, Math.max(0.6, next - t - 0.3), {
					attack: i ? 1.2 : 2.4,
					release: 1.6,
					level: (grit ? 2.0 : 1.7) * (1 + 0.08 * i),
					cut: !!bed && next >= underEnd - 0.01,
					seed: 800 + i
				}),
			next - t + 2
		);
	});
	cue(
		0,
		(h, w) =>
			padSweep(h, w, [
				[0, 600],
				[presents.at, 1000],
				[rise, 1800],
				[m0, 3400],
				[m1 + 1.6, 2400],
				[D, 900]
			]),
		D
	);
	// the cold open breathes in: air over the quiet line
	cue(0.2, (h, w) => air(h, w, 3.2, { peakAt: 2, band: [220, 1000], gain: 0.3, seed: 810 }));

	// the channel's moment: a sub, light over it, nothing the sonic logo would be mistaken for
	cue(presents.at - 0.3, (h, w) =>
		air(h, w, 1.0, { peakAt: 0.7, band: [300, 1600], gain: 0.25, seed: 811 })
	);
	if (pitched(presents.at)) {
		cue(presents.at, (h, w) =>
			sub(h, w, 'D1', presents.end - presents.at - 0.4, { gain: 0.4, attack: 0.5, release: 1.0 })
		);
		cue(presents.at + 0.2, (h, w) =>
			glass(h, w, 'A5', { decay: 2.4, gain: 0.35, pan: 0.15, verb: 0.75, seed: 812 })
		);
	} else cue(presents.at, (h, w) => impact(h, w, { gain: 0.2, root: 'D1', seed: 4399 }));

	// the cards land: a soft impact and a sub under each, the words struck low in grit
	// the red field crossing the frame (bright's cards and its title): a breath of air with it
	const crossing = (at: number, seed: number) =>
		cue(at - 0.4, (h, w) =>
			air(h, w, 0.8, { peakAt: 0.4, band: [500, 3200], gain: 0.2, pans: [0.6, -0.4], seed })
		);
	if (bright) crossing(m1, 892);
	cards.forEach((k, i) => {
		if (bright) crossing(k.at, 893 + i);
		cue(k.at, (h, w) => impact(h, w, { gain: grit ? 0.3 : 0.22, root: 'D1', seed: 4400 + i }));
		if (!pitched(k.at)) return;
		cue(k.at, (h, w) =>
			sub(h, w, i % 2 ? 'B1' : 'D2', 1.0, { gain: 0.3, attack: 0.04, release: 0.8 })
		);
		if (grit)
			(['D2', 'A2', 'D3'] as const).forEach((n, j) =>
				cue(k.at + 0.004 * j, (h, w) =>
					piano(h, w, n, {
						vel: 0.85,
						dur: 1.2,
						pan: -0.1 + 0.1 * j,
						verb: 0.5,
						gain: 0.5,
						seed: 820 + j + i
					})
				)
			);
		else
			cue(k.at + 0.35, (h, w) =>
				glass(h, w, i % 2 ? 'E5' : 'A5', {
					decay: 2.2,
					gain: 0.4,
					pan: 0.2 - 0.4 * (i % 2),
					verb: 0.75,
					seed: 830 + i
				})
			);
	});
	if (premiere) {
		cue(premiere.at - 0.8, (h, w) => inhale(h, w + 0.8, 0.8, { gain: 0.3, seed: 840 }));
		cue(premiere.at, (h, w) => impact(h, w, { gain: 0.28, root: 'A1', seed: 4410 }));
		if (pitched(premiere.at))
			cue(premiere.at + 0.2, (h, w) =>
				glass(h, w, 'E5', { decay: 2.2, gain: 0.5, pan: 0.15, verb: 0.7, seed: 841 })
			);
	}

	// each act's first moment: a low bowed note under the new scene (never inside a line)
	acts.forEach((s, i) => {
		if (grit || i % 2 || !pitched(s.at)) return;
		const note = (['F#3', 'A3', 'E3'] as const)[Math.floor(i / 2) % 3];
		cue(s.at + 0.1, (h, w) =>
			bowed(h, w, note, 2.2, { gain: 0.4, attack: 0.9, release: 1.2, verb: 0.6, seed: 850 + i })
		);
	});

	// --- the riser into the montage -------------------------------------------------------------

	if (bedOn(rise)) {
		cue(m0 - Math.min(K.rise, 1.6), (h, w) =>
			inhale(h, w + Math.min(K.rise, 1.6), Math.min(K.rise, 1.6), { gain: 0.35, seed: 860 })
		);
		cue(rise, (h, w) =>
			air(h, w, K.rise + 0.2, {
				peakAt: K.rise,
				band: [300, 2400],
				gain: 0.28,
				pans: [-0.5, 0.5],
				seed: 861
			})
		);
	} else {
		cue(rise, (h, w) =>
			riser(h, w + K.rise, ['A2', 'E3', 'A3', 'E4', 'A4'], K.rise, {
				gain: 0.8,
				breath: 0.04,
				seed: 862
			})
		);
		cue(m0 - Math.min(K.rise, 1.4), (h, w) =>
			inhale(h, w + Math.min(K.rise, 1.4), Math.min(K.rise, 1.4), { gain: 0.4, seed: 863 })
		);
		cue(
			rise,
			(h, w) =>
				pad(h, w, ['A1', 'E2', 'A2', 'D3', 'E3', 'A3'], K.rise - 0.1, {
					attack: Math.min(1.2, K.rise / 2),
					level: 0.9,
					cut: true,
					seed: 864
				}),
			K.rise
		);
	}

	// --- the montage ----------------------------------------------------------------------------

	const cuts = montage?.cuts ?? [];
	if (!bed && montage) {
		// the house is the music: chords lifting G, A, B minor, A under pulses and glass on the cuts
		const chords: Pitch[][] = [
			['G1', 'D2', 'G2', 'B2', 'D3', 'F#3', 'A3'],
			['A1', 'E2', 'A2', 'C#3', 'E3', 'A3'],
			['B1', 'F#2', 'B2', 'D3', 'F#3', 'A3'],
			['A1', 'E2', 'A2', 'D3', 'E3', 'A3']
		];
		chords.forEach((chord, i) => {
			const a = m0 + ((m1 - m0) * i) / chords.length;
			const b = m0 + ((m1 - m0) * (i + 1)) / chords.length;
			cue(
				a,
				(h, w) =>
					pad(h, w, chord, Math.max(0.4, b - a - 0.1), {
						attack: 0.25,
						release: 0.5,
						level: 0.85,
						cut: i === chords.length - 1,
						seed: 870 + i
					}),
				b - a + 1
			);
		});
		const scale = ['D5', 'E5', 'F#5', 'A5', 'B5', 'D6', 'B5', 'A5'];
		cuts.forEach((t, i) => {
			const progress = (t - m0) / Math.max(1e-6, m1 - m0);
			cue(t, (h, w) =>
				pulse(h, w, i % 2 ? 'A2' : 'D2', {
					gain: 0.45 + 0.7 * progress,
					open: 0.2 + 0.3 * progress,
					decay: i % 2 ? 0.2 : 0.3,
					seed: 880 + i
				})
			);
			if (!grit)
				cue(t + 0.003, (h, w) =>
					glass(h, w, scale[i % scale.length], {
						decay: 0.9,
						gain: (i % 2 ? 0.4 : 0.55) * (0.7 + 0.5 * progress),
						pan: i % 2 ? 0.3 : -0.3,
						verb: 0.5,
						seed: 900 + i
					})
				);
		});
	} else if (bed) {
		// over the soundtrack only unpitched punctuation: air into the cuts that open the montage
		if (montage)
			cue(m0 + (m1 - m0) * 0.5, (h, w) =>
				air(h, w, 1.4, { peakAt: 1.0, band: [400, 2600], gain: 0.2, pans: [0.5, -0.5], seed: 890 })
			);
		cue(m1 - 1.2, (h, w) => inhale(h, w + 1.2, 1.2, { gain: 0.3, seed: 891 }));
	}

	// --- the hit and the title ------------------------------------------------------------------

	cue(m1, (h, w) => impact(h, w, { gain: bed ? 0.2 : 0.4, root: 'D1', seed: 4420 }));
	cue(m1, (h, w) =>
		sub(h, w, 'D1', title.end - m1 - 0.6, { gain: bed ? 0.18 : 0.55, attack: 0.06, release: 1.2 })
	);
	if (!bed) {
		const hold = title.end - m1;
		cue(
			m1,
			(h, w) =>
				pad(h, w, D_CHORD, hold + 1.0, {
					attack: 0.15,
					release: 2.0,
					level: 1.1,
					spread: 26,
					seed: 910
				}),
			hold + 2
		);
		(['D3', 'A3', 'F#4'] as const).forEach((n, i) =>
			cue(m1 + 0.006 * i, (h, w) =>
				piano(h, w, n, {
					vel: 0.62,
					dur: 3.2,
					pan: -0.12 + 0.12 * i,
					verb: 0.7,
					gain: 0.9,
					seed: 915 + i
				})
			)
		);
		cue(m1 + 0.3, (h, w) =>
			shimmer(h, w, ['A5', 'D6', 'F#6'], 2.2, { gain: 0.9, attack: 0.6, release: 2.0, seed: 920 })
		);
		cue(m1 + 1.6, (h, w) =>
			glass(h, w, 'A5', { decay: 2.6, gain: 0.5, pan: 0.2, verb: 0.75, seed: 921 })
		);
	}

	// --- the button and the end card -------------------------------------------------------------

	const button = scenes.find((s) => s.role === 'button');
	if (button && !bed)
		cue(
			title.end - 0.2,
			(h, w) =>
				pad(h, w, ['D2', 'A2', 'D3', 'F#3'], button.end - title.end, {
					attack: 0.6,
					release: 1.2,
					level: 0.4,
					seed: 925
				}),
			button.end - title.end + 2
		);
	// the end card: the subdominant under the panels, which the tag's D resolves
	cue(
		B - 0.4,
		(h, w) =>
			pad(h, w, ['G2', 'D3', 'G3', 'B3', 'D4'], Math.max(0.4, tagAt - B - 0.2), {
				attack: 1.0,
				release: 1.6,
				level: bright ? 0.55 : 0.6,
				seed: 930
			}),
		3
	);
	cue(B - 0.4, (h, w) =>
		air(h, w, 1.6, { peakAt: 0.7, band: [300, 1800], gain: 0.3, pans: [-0.5, 0.5], seed: 931 })
	);
}
