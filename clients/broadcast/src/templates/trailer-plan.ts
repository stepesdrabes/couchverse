// The trailer's plan: what plays when, from the facts of its scenes and its soundtrack. Pure, so
// the rules (a length of 60 to 90 s in whole frames, cuts that never touch a line, the scenes
// left out when there are too many) are table-tested without a browser; the template only draws
// it.
//
// A trailer is cut the way trailers are: a cold open on a quiet scene, the channel's presence,
// scenes with their lines and the tagline cards between them, a riser into a montage of quick
// cuts on the music's beats, the title on the music's peak, one last short line, the end card.
//
// The montage and the title are cut from picture the trailer has not shown: the title's footage
// shots, the stretches of the scenes before and after what plays of them, and the scenes the plan
// left out. A stretch is played once, forwards: no shot restarts, two shots never take turns, and
// when the material is short the montage is short (or absent) and the other parts breathe.

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
	/** The lengths (seconds) of the title's footage shots, which the montage and the title draw on. */
	shots?: number[];
}

/** Where a picture comes from: one of the title's footage shots, or one of the spec's scenes. */
export interface Source {
	kind: 'shot' | 'scene';
	index: number;
}

/** A take of a source: from `from` seconds into it, on the trailer's clock over [at, end). */
export interface Take {
	src: Source;
	from: number;
	at: number;
	end: number;
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
	| ({ kind: 'montage'; cuts: number[]; takes: Take[] } & Span)
	| ({ kind: 'title'; takes: Take[] } & Span);

export type SceneSection = Extract<Section, { kind: 'scene' }>;

export interface Plan {
	sections: Section[];
	/** The cut to the end card, a whole frame: the body's length. */
	end: number;
	/** Where the soundtrack starts on the trailer's clock; null without one. */
	bedAt: number | null;
	/** The montage's riser starts here. */
	riseAt: number;
	/** The hit: the title's first frame, where the montage ends (or would, without one). */
	hit: number;
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

/** A stretch of a source not yet shown, seconds into it, on the frame grid. */
interface Reel {
	src: Source;
	from: number;
	to: number;
}

/** The least a stretch is worth a take, and the least picture a montage is made of. */
const MIN_REEL = 0.6;
const MIN_MONTAGE = 2;
/** The frames beside a played stretch stay unused: a cut there would show as a jump. */
const BESIDE = 0.2;

/**
 * The picture a trailer has not shown: each of the title's footage shots (`shots`, their lengths
 * in seconds), and of every scene what lies before and after the stretch that plays (`kept`), or
 * all of it when the plan left the scene out.
 */
export function reelsOf(
	shots: number[],
	scenes: SceneFacts[],
	kept: { scene: number; from: number; to: number }[]
): Reel[] {
	const out: Reel[] = shots.map((len, index) => ({
		src: { kind: 'shot', index },
		from: 0,
		to: Math.floor(len * FPS + 1e-6) / FPS
	}));
	scenes.forEach((s, index) => {
		const c = kept.find((k) => k.scene === index);
		const spans: [number, number][] = c
			? [
					[0, c.from - BESIDE],
					[c.to + BESIDE, s.len]
				]
			: [[0, s.len]];
		for (const [a, b] of spans) {
			const from = Math.max(0, Math.ceil(a * FPS - 1e-6)) / FPS;
			const to = Math.floor(b * FPS + 1e-6) / FPS;
			if (to - from >= MIN_REEL - 1e-6) out.push({ src: { kind: 'scene', index }, from, to });
		}
	});
	return out;
}

/**
 * Picture for `need` seconds of the title, taken out of `reels` (which lose it): the longest
 * stretches, each from its end, three at most.
 */
function reserve(reels: Reel[], need: number) {
	const out: { src: Source; from: number; len: number }[] = [];
	for (let left = need; left > 0.5 && out.length < 2;) {
		const best = reels
			.filter((r) => r.to - r.from >= MIN_REEL - 1e-6)
			.reduce<Reel | null>((a, r) => (!a || r.to - r.from > a.to - a.from ? r : a), null);
		if (!best) break;
		const len = Math.min(left, best.to - best.from);
		out.push({ src: best.src, from: best.to - len, len });
		best.to -= len;
		left -= len;
	}
	return out;
}

/**
 * The montage's takes: cut `i` runs from `cuts[i]` to the next (the last to `end`), each a
 * stretch of the reel with the most left, going on where that reel was left and never back. Two
 * reels never take turns (the one before the last is not chosen), and where no other reel has
 * enough left the last one simply goes on (one longer take). When the picture is used up the
 * montage ends where the next cut would have been: the result's `end`.
 */
export function assignTakes(
	cuts: number[],
	end: number,
	reels: Reel[]
): { takes: Take[]; end: number } {
	const cursor = reels.map((r) => r.from);
	const takes: Take[] = [];
	const played: number[] = [];
	for (let i = 0; i < cuts.length; i++) {
		const a = cuts[i];
		const b = i + 1 < cuts.length ? cuts[i + 1] : end;
		const len = b - a;
		const room = (k: number) => reels[k].to - cursor[k] - len >= -1e-6;
		const open = reels.map((_, k) => k).filter((k) => room(k));
		const other = open.filter((k) => k !== played.at(-1) && k !== played.at(-2));
		const k = other.length
			? other.reduce((x, y) => (reels[y].to - cursor[y] > reels[x].to - cursor[x] ? y : x))
			: played.length && open.includes(played[played.length - 1])
				? played[played.length - 1]
				: -1;
		if (k < 0) return { takes, end: a };
		if (k === played.at(-1)) takes[takes.length - 1].end = b;
		else {
			takes.push({ src: reels[k].src, from: cursor[k], at: a, end: b });
			played.push(k);
		}
		cursor[k] += len;
	}
	return { takes, end };
}

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
	// how long the parts that can breathe are held, and the montage (wanted: what the picture left
	// allows is decided below)
	const hold = { presents: K.presents, premiere: K.premiere, title: K.title };
	let montage = K.montage;
	const total = () =>
		playing(open) +
		hold.presents +
		holds.reduce((n, h) => n + h, 0) +
		(input.premiere ? hold.premiere : 0) +
		montage +
		hold.title +
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

	// the picture not yet shown: the title's footage, the stretches of the scenes beside what plays
	// and the scenes left out. The title's picture comes out of it first, the montage takes the rest.
	const kept = [open, ...acts, ...(button ? [button] : [])];
	const reels = reelsOf(input.shots ?? [], input.scenes, kept);
	let pictures = reserve(reels, hold.title);
	if (!pictures.length) {
		// nothing unseen at all: the longest scene plays again under the title's grade
		const c = acts.concat(open).reduce((a, b) => (playing(b) > playing(a) ? b : a));
		pictures = [{ src: { kind: 'scene', index: c.scene }, from: c.from, len: playing(c) }];
	}
	const supply = reels.reduce((n, r) => n + (r.to - r.from), 0);
	const room = supply >= MIN_MONTAGE ? supply : 0;
	montage = Math.min(montage, room);
	// too few scenes: the montage, as far as the picture goes, then everything else takes the time
	if (total() < BODY[0]) montage = Math.min(K.montageMax, room, montage + BODY[0] - total());
	const breathe: [number, (add: number) => void][] = holds.map((_, i) => [
		1.0,
		(add) => (holds[i] += add)
	]);
	breathe.push([0.8, (add) => (hold.presents += add)], [1.6, (add) => (hold.title += add)]);
	if (input.premiere) breathe.push([0.8, (add) => (hold.premiere += add)]);
	for (const [cap, add] of breathe) {
		const left = BODY[0] - total();
		if (left <= 0) break;
		add(Math.min(cap, left));
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
	const span = (kind: 'presents' | 'premiere', dur: number) => {
		sections.push({ kind, at, end: frame(at + dur) });
		at = frame(at + dur);
	};

	scene(open, 'open', false, false);
	span('presents', hold.presents);
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
	if (input.premiere) span('premiere', hold.premiere);

	// the montage on the soundtrack's beats, the title on its peak
	const m0 = at;
	const bedAt = input.bed ? Math.max(0, frame(m0 - K.rise)) : null;
	const onsets: Onset[] = input.bed
		? input.bed.onsets.map((o) => ({ t: (bedAt ?? 0) + o.t, s: o.s }))
		: [];
	// a small pool is cut into longer takes
	const slow = reels.length >= 5 ? 1 : reels.length >= 3 ? 1.4 : 2;
	const pace = { gap: [K.gap[0] * slow, K.gap[1] * slow] as [number, number], cutMin: K.cutMin };
	let m1 = frame(m0 + montage);
	let cuts: number[] = [];
	let takes: Take[] = [];
	if (montage > 0) {
		const peak = onsets
			.filter(
				(o) => o.s >= 0.9 && Math.abs(o.t - m1) <= 0.9 && o.t - m0 >= 3 && o.t - m0 <= room + 1e-6
			)
			.sort((a, b) => b.s - a.s || Math.abs(a.t - m1) - Math.abs(b.t - m1))[0];
		if (peak) m1 = frame(peak.t);
		// the picture may run out before the music's peak: the montage then ends where it does
		for (let tries = 0; tries < 6; tries++) {
			cuts = montageCuts(m0, m1, onsets, pace);
			const got = assignTakes(
				cuts,
				m1,
				reels.map((r) => ({ ...r }))
			);
			takes = got.takes;
			if (got.end >= m1 - 1e-6) break;
			m1 = frame(got.end);
			if (m1 - m0 < MIN_MONTAGE) {
				[m1, cuts, takes] = [m0, [], []];
				break;
			}
		}
		if (m1 > m0) sections.push({ kind: 'montage', at: m0, end: m1, cuts, takes });
	}
	at = m1;
	const hit = at;
	// the title's picture, the reserved stretches in turn (the last holds, slowly pushed, when the
	// picture is short)
	const titleEnd = frame(at + hold.title);
	let t = at;
	const stretches: Take[] = pictures.map((p, i) => {
		const end = i === pictures.length - 1 ? titleEnd : Math.min(titleEnd, frame(t + p.len));
		const take = { src: p.src, from: p.from, at: t, end };
		t = end;
		return take;
	});
	sections.push({ kind: 'title', at, end: titleEnd, takes: stretches });
	at = titleEnd;
	if (button) scene(button, 'button', false, false);
	return { sections, end: at, bedAt, riseAt: m0 - K.rise, hit };
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
