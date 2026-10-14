// The trailer's plan: what plays when, from the facts of its scenes and its soundtrack. Pure, so
// the rules (a length of 60 to 90 s in whole frames, cuts that never touch a line, the scenes
// left out when there are too many) are table-tested without a browser; the template only draws
// it.
//
// A trailer is cut the way trailers are: a cold open on a quiet scene, the channel's presence,
// scenes with their lines and the tagline cards between them, a riser into a montage of quick
// cuts on the music's beats, the title on the music's peak, one last short line, the end card.

import type { Onset } from '../audio/engine';
import { FPS, frame } from '../core/timeline';

export type Mood = 'cinematic' | 'grit' | 'bright';

/** What the plan needs to know of a scene (its sound is never read here). */
export interface SceneFacts {
	/** Seconds, a whole number of frames. */
	len: number;
	/** `[from, to]` seconds into the scene, in order. */
	lines: [number, number][];
}

export interface PlanInput {
	mood: Mood;
	premiere: boolean;
	scenes: SceneFacts[];
	/** The taglines' (else the words') lengths in characters: how long each card is read. */
	cards: number[];
	/** The soundtrack's onsets (seconds into its excerpt); null without a soundtrack. */
	bed: { onsets: Onset[] } | null;
}

export interface PlannedScene {
	/** Its index among the spec's scenes. */
	scene: number;
	role: 'open' | 'act' | 'button';
	/** The picture's first frame and the frame after its last, on the trailer's clock. */
	at: number;
	end: number;
	/** The picture, seconds into the scene. */
	from: number;
	to: number;
	/**
	 * The sound, seconds into the scene. It plays in step with the picture, but a J-cut starts it
	 * a few frames before the picture does and an L-cut lets it ring on after the picture's end
	 * (the picture's frames are trimmed instead, so nothing falls out of sync).
	 */
	soundFrom: number;
	soundTo: number;
	/** Its lines on the trailer's clock. */
	lines: [number, number][];
}

interface Span {
	at: number;
	end: number;
}

export type Section =
	| ({ kind: 'scene' } & PlannedScene)
	| ({ kind: 'presents' } & Span)
	| ({ kind: 'card'; card: number } & Span)
	| ({ kind: 'premiere' } & Span)
	| ({ kind: 'montage'; cuts: number[] } & Span)
	| ({ kind: 'title' } & Span);

export type SceneSection = Extract<Section, { kind: 'scene' }>;

export interface Plan {
	sections: Section[];
	/** The cut to the end card, a whole frame: the body's length. */
	end: number;
	/** Where the soundtrack starts on the trailer's clock; null without one. */
	bedAt: number | null;
	/** The montage's riser starts here. */
	riseAt: number;
}

export interface Knobs {
	presents: number;
	/** How long a card is held: its reading time within these. */
	card: [number, number];
	premiere: number;
	montage: number;
	montageMax: number;
	title: number;
	/** Seconds between the montage's cuts, from the first to the last. */
	gap: [number, number];
	/** The weakest onset a cut follows. */
	cutMin: number;
	/** Seconds from the riser to the montage. */
	rise: number;
}

export const KNOBS: Record<Mood, Knobs> = {
	cinematic: {
		presents: 2.8,
		card: [2.4, 3.8],
		premiere: 2.6,
		montage: 8,
		montageMax: 16,
		title: 5.2,
		gap: [1.0, 0.5],
		cutMin: 0.5,
		rise: 2.4
	},
	grit: {
		presents: 2.0,
		card: [1.6, 3.0],
		premiere: 1.8,
		montage: 7.2,
		montageMax: 14,
		title: 4.2,
		gap: [0.6, 0.3],
		cutMin: 0.4,
		rise: 1.6
	},
	bright: {
		presents: 2.4,
		card: [2.2, 3.4],
		premiere: 2.4,
		montage: 8,
		montageMax: 15,
		title: 4.8,
		gap: [0.9, 0.5],
		cutMin: 0.5,
		rise: 2.0
	}
};

/** A trailer's body runs 60 to 90 s: the plan aims past the first and stays under the second. */
export const BODY: [number, number] = [64, 88];

/** The longest a scene plays: the cold open, an act's scene, the button. */
const LONGEST = { open: 6.5, act: 8, button: 5.6 };
/** How far a cut may fall before a line's first word and after its last. */
const LEAD_IN = 0.3;
const LEAD_OUT = 0.4;
/** A J-cut or an L-cut: four frames of one scene's sound under the other's picture. */
const OVERLAP = 4 / FPS;
/** Seconds the button's picture stays on after its last word: the end card closes in over it. */
const BUTTON_HOLD = 0.7;
/** The room a fade needs between a cut and a line: it must never reach a word. */
const CLEAR = 0.08;

/**
 * The stretch of a scene to play, at most `max` seconds, cut in a gap between lines: the run of
 * whole lines with the most speech that fits, a little before its first word and after its last.
 * A single line longer than `max` plays whole.
 */
export function cutWindow(scene: SceneFacts, max: number): [number, number] {
	const { len, lines } = scene;
	if (len <= max + 1e-6) return [0, len];
	if (!lines.length) return [0, frame(max)];
	let best: { from: number; to: number; speech: number } | null = null;
	for (let i = 0; i < lines.length; i++) {
		for (let j = i; j < lines.length; j++) {
			// a cut falls at most LEAD_IN before a line and LEAD_OUT after one, and half way
			// through a gap that is shorter than that
			let from = Math.max(0, lines[i][0] - LEAD_IN);
			if (i) from = Math.max(from, (lines[i - 1][1] + lines[i][0]) / 2);
			let to = Math.min(len, lines[j][1] + LEAD_OUT);
			if (j < lines.length - 1) to = Math.min(to, (lines[j][1] + lines[j + 1][0]) / 2);
			from = Math.floor(from * FPS + 1e-6) / FPS;
			to = Math.min(len, Math.ceil(to * FPS - 1e-6) / FPS);
			if (to - from > max + 1e-6) {
				// the lines only get longer from here; one that does not fit alone plays whole
				if (j === i && !best) best = { from, to, speech: lines[i][1] - lines[i][0] };
				break;
			}
			let speech = 0;
			for (let k = i; k <= j; k++) speech += lines[k][1] - lines[k][0];
			if (!best || speech > best.speech + 1e-6) best = { from, to, speech };
		}
	}
	return best ? [best.from, best.to] : [0, frame(max)];
}

/** Seconds a card of this many characters is held. */
const reading = (chars: number, [lo, hi]: [number, number]) =>
	frame(Math.min(hi, Math.max(lo, 1.2 + 0.055 * chars)));

interface Cut extends SceneFacts {
	scene: number;
	/** The stretch that plays. */
	from: number;
	to: number;
}

const playing = (c: Cut) => c.to - c.from;

const spoken = (c: Cut) => c.lines.filter((l) => l[0] >= c.from - 1e-6 && l[1] <= c.to + 1e-6);

/** How little of a scene's time is speech: the first to go when there are too many scenes. */
const idle = (c: Cut) => playing(c) - 1.5 * spoken(c).reduce((n, [a, b]) => n + b - a, 0);

/** Whether a scene's sound has room for a J-cut at its start (no word under its first frames). */
const roomBefore = (c: Cut) => {
	const first = spoken(c)[0];
	return !first || first[0] - c.from >= OVERLAP + CLEAR;
};
/** Whether it has room for an L-cut at its end. */
const roomAfter = (c: Cut) => {
	const last = spoken(c).at(-1);
	return !last || c.to - last[1] >= OVERLAP + CLEAR;
};

/** The plan for a trailer; `end` is its length, 60 to 90 s whenever the scenes allow. */
export function planTrailer(input: PlanInput): Plan {
	const K = KNOBS[input.mood];

	// the scenes, trimmed to what plays: the cold open is the first, the button the last when it
	// has a line and is short enough, the rest are the acts
	const cut = (i: number, max: number): Cut => {
		const [from, to] = cutWindow(input.scenes[i], max);
		return { ...input.scenes[i], scene: i, from, to };
	};
	const open = cut(0, LONGEST.open);
	const last = input.scenes.length - 1;
	let button: Cut | null = null;
	if (last > 1 && input.scenes[last].lines.length) {
		const c = cut(last, LONGEST.button);
		if (playing(c) <= LONGEST.button + 1) button = c;
	}
	let acts: Cut[] = [];
	for (let i = 1; i < (button ? last : last + 1); i++) acts.push(cut(i, LONGEST.act));

	const holds = input.cards.slice(0, 3).map((chars) => reading(chars, K.card));
	let montage = K.montage;
	const total = () =>
		playing(open) +
		K.presents +
		holds.reduce((n, h) => n + h, 0) +
		(input.premiere ? K.premiere : 0) +
		montage +
		K.title +
		(button ? playing(button) : 0) +
		acts.reduce((n, a) => n + playing(a), 0);

	// too many scenes: the ones with the least speech for their length go first
	while (total() > BODY[1] && acts.length > 1) {
		let worst = 0;
		acts.forEach((a, i) => {
			if (idle(a) > idle(acts[worst])) worst = i;
		});
		acts = acts.filter((_, i) => i !== worst);
	}
	// too few: the montage, then the cards, take the time
	if (total() < BODY[0]) {
		montage = Math.min(K.montageMax, montage + BODY[0] - total());
		const left = BODY[0] - total();
		if (left > 0 && holds.length)
			holds.forEach((_, i) => (holds[i] += Math.min(0.8, left / holds.length)));
	}

	// where the cards fall among the acts: the last one just before the montage
	const slots = holds.map((_, i) =>
		Math.max(
			1,
			holds.length === 1 ? acts.length : Math.floor(((i + 1) * acts.length) / holds.length)
		)
	);
	// the cut between two acts with no card between them is a J-cut or an L-cut, every other one
	const overlap: ('j' | 'l' | null)[] = acts.map(() => null);
	let flip = false;
	acts.forEach((a, i) => {
		const next = acts[i + 1];
		if (!next || slots.includes(i + 1)) {
			flip = false;
			return;
		}
		const order = flip ? ['j', 'l'] : ['l', 'j'];
		const kind = order.find((k) => (k === 'l' ? roomAfter(a) : roomBefore(next)));
		if (kind) {
			overlap[i] = kind as 'j' | 'l';
			flip = !flip;
		}
	});

	const sections: Section[] = [];
	let at = 0;
	const scene = (c: Cut, role: PlannedScene['role'], j: boolean, l: boolean) => {
		let { from, to } = c;
		if (j) from += OVERLAP;
		if (l) to -= OVERLAP;
		const lines = spoken(c).map(([a, b]): [number, number] => [at + (a - from), at + (b - from)]);
		// the button keeps its last frame on after its last word, so the end card's panels, which
		// start closing in before the cut, never close over speech
		const held = role === 'button' && lines.length ? lines[lines.length - 1][1] + BUTTON_HOLD : 0;
		const end = frame(Math.max(at + (to - from), held));
		sections.push({
			kind: 'scene',
			scene: c.scene,
			role,
			at,
			end,
			from,
			to,
			soundFrom: c.from,
			soundTo: c.to,
			lines
		});
		at = end;
	};
	const span = (kind: 'presents' | 'premiere' | 'title', hold: number) => {
		sections.push({ kind, at, end: frame(at + hold) });
		at = frame(at + hold);
	};

	scene(open, 'open', false, false);
	span('presents', K.presents);
	let next = 0;
	const cards = (upTo: number) => {
		for (; next < holds.length && slots[next] <= upTo; next++) {
			sections.push({ kind: 'card', card: next, at, end: frame(at + holds[next]) });
			at = frame(at + holds[next]);
		}
	};
	acts.forEach((a, i) => {
		scene(a, 'act', overlap[i - 1] === 'j', overlap[i] === 'l');
		cards(i + 1);
	});
	cards(Infinity);
	if (input.premiere) span('premiere', K.premiere);

	// the montage on the soundtrack's beats, the title on its peak
	const m0 = at;
	const bedAt = input.bed ? Math.max(0, frame(m0 - K.rise)) : null;
	const onsets: Onset[] = input.bed
		? input.bed.onsets.map((o) => ({ t: (bedAt ?? 0) + o.t, s: o.s }))
		: [];
	let m1 = frame(m0 + montage);
	const peak = onsets
		.filter((o) => o.s >= 0.9 && Math.abs(o.t - m1) <= 0.9 && o.t - m0 >= 3)
		.sort((a, b) => b.s - a.s || Math.abs(a.t - m1) - Math.abs(b.t - m1))[0];
	if (peak) m1 = frame(peak.t);
	sections.push({ kind: 'montage', at: m0, end: m1, cuts: montageCuts(m0, m1, onsets, K) });
	at = m1;
	span('title', K.title);
	if (button) scene(button, 'button', false, false);
	return { sections, end: at, bedAt, riseAt: m0 - K.rise };
}

/**
 * The montage's cut times from `a` to `b`: quicker as it goes, each on the strongest onset near
 * where the next one is due, else on the frame grid, the last a moment before the hit.
 */
export function montageCuts(
	a: number,
	b: number,
	onsets: Onset[],
	K: Pick<Knobs, 'gap' | 'cutMin'>
): number[] {
	const cuts = [a];
	const stop = b - Math.max(0.4, K.gap[1]);
	for (let t = a; ;) {
		const progress = Math.min(1, (t - a) / Math.max(1e-6, b - a));
		const gap = K.gap[0] + (K.gap[1] - K.gap[0]) * progress;
		const near = onsets
			.filter((o) => o.s >= K.cutMin && o.t >= t + gap * 0.7 && o.t <= t + gap * 1.3)
			.sort((x, y) => y.s - x.s || Math.abs(x.t - t - gap) - Math.abs(y.t - t - gap))[0];
		const cutAt = frame(near ? near.t : t + gap);
		if (cutAt > stop + 1e-6 || cutAt <= t) break;
		cuts.push(cutAt);
		t = cutAt;
	}
	return cuts;
}

/** A plan's scenes, in the order they play. */
export const scenesOf = (plan: Plan) =>
	plan.sections.filter((s): s is SceneSection => s.kind === 'scene');

/** Every line of the trailer on its clock, in order: what the music ducks under. */
export const linesOf = (plan: Plan): [number, number][] => scenesOf(plan).flatMap((s) => s.lines);
