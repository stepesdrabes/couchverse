// What the bulletins share: the night with a still dusk under it, the header (the title over its
// label in a red block, both revealed from behind their own edges and wiped away at a change of
// scene), the channel's lockup in the corner where the bug sits on the other screens, the
// credits and the exit to night. A bulletin is bound to its slot late and may air in any slot
// while it is valid (D38), so it has no clock to show: the corner carries the lockup alone.
// Between their moves nothing moves, so held frames render as one capture; their sound starts
// and ends inside the clip (bulletinEnd), since a slot cuts on both sides.

import { grainTexture } from '../assets';
import { house, type House } from '../audio/voices';
import type { Kit } from '../audio/kit';
import type { Bus } from '../audio/engine';
import { channelTag, lockup } from '../brand/lockup';
import { css, div, span } from '../core/dom';
import { fitText } from '../core/fit';
import { edge } from '../core/moves';
import type { TL } from './shared';

/** 75 bpm, the house tempo: a beat is 20 frames. */
export const BEAT = 0.8;

/** The nearest beat, so a move lands on the music's grid. */
export const onBeat = (t: number) => Math.round(t / BEAT) * BEAT;

/** `A, B and C`. */
export const list = (names: string[]) =>
	names.length < 2 ? names.join('') : `${names.slice(0, -1).join(', ')} and ${names.at(-1)}`;

/**
 * Whole beats for parts that each want `w` beats and stay within `[min, max]`, together exactly
 * `total`: the wants scaled by one factor, each held to its bounds, the beats that rounding
 * leaves going to the parts that lost the most to it. Bounds that cannot hold (all minimums
 * over the total, all maximums under it) give way proportionally.
 */
export function share(parts: { w: number; min: number; max: number }[], total: number): number[] {
	const sum = (f: (p: { w: number; min: number; max: number }) => number) =>
		parts.reduce((a, p) => a + f(p), 0);
	const least = sum((p) => p.min);
	const most = sum((p) => p.max);
	let beats: number[];
	if (least >= total) beats = parts.map((p) => (p.min * total) / least);
	else if (most <= total) beats = parts.map((p) => (p.max * total) / most);
	else {
		const at = (k: number) => parts.map((p) => Math.min(p.max, Math.max(p.min, p.w * k)));
		let [lo, hi] = [0, 1000];
		for (let i = 0; i < 60; i++) {
			const mid = (lo + hi) / 2;
			if (at(mid).reduce((a, b) => a + b, 0) < total) lo = mid;
			else hi = mid;
		}
		beats = at((lo + hi) / 2);
	}
	const whole = beats.map(Math.floor);
	let left = total - whole.reduce((a, b) => a + b, 0);
	const order = beats.map((b, i) => [b - whole[i], i]).sort((a, b) => b[0] - a[0]);
	for (const [, i] of order) {
		if (left <= 0) break;
		whole[i]++;
		left--;
	}
	return whole;
}

/** Past this, a fitted line is as wide as the stage's title-safe width allows from x 96. */
export const SAFE_RIGHT = 1824;

/** The night and its dusk: up with the first moves, down with the last. */
export function field(
	tl: TL,
	root: HTMLElement,
	dusk: { x: number; y: number; rx: number; ry: number; a: number }
) {
	const d = css(div('bl-dusk', root), {});
	for (const [k, v] of Object.entries({
		'--dusk-x': `${dusk.x}px`,
		'--dusk-y': `${dusk.y}px`,
		'--dusk-rx': `${dusk.rx}px`,
		'--dusk-ry': `${dusk.ry}px`,
		'--dusk-a': String(dusk.a)
	}))
		d.style.setProperty(k, v);
	const g = div('bl-grain', root);
	g.style.backgroundImage = `url(${grainTexture()})`;
	const D = tl.duration;
	for (const e of [d, g]) {
		tl.add(e, [{ opacity: 0 }, { opacity: e === d ? dusk.a : 0.035 }], 0, 0.9, 'sine');
		tl.add(e, [{ opacity: e === d ? dusk.a : 0.035 }, { opacity: 0 }], D - 0.5, 0.45, 'sine');
	}
}

export interface Head {
	root: HTMLElement;
	title: HTMLElement;
	label: HTMLElement;
	block: HTMLElement;
	text: HTMLElement;
}

/**
 * A header, still hidden: the title (display, 88 px) over its label in a red block, the label's
 * parts parted by small squares.
 */
export function head(parent: HTMLElement, title: string, label: string[]): Head {
	const root = div('bl-head', parent);
	const t = div('bl-title', div('bl-clip', root), title);
	fitText(t, { width: 760, min: 60 });
	const l = div('bl-label', root);
	const block = div('bl-label-block', l);
	const text = span(null, div('bl-clip', l));
	label.forEach((part, i) => {
		if (i) span('bl-sep', text);
		span(null, text, part);
	});
	fitText(text, { width: 700, min: 22 });
	return { root, title: t, label: l, block, text };
}

// a clip's content parked under its edge: the clip's padding is 0.34 em of the line's height
const BELOW = 'translateY(140%)';
const ABOVE = 'translateY(-140%)';

/** In from behind their edges: the title rises, the red block wipes open, its label rises. */
export function headIn(tl: TL, h: Head, at: number) {
	tl.add(h.title, [{ transform: BELOW }, { transform: 'translateY(0)' }], at, 0.8, 'glide');
	tl.add(
		h.block,
		[{ transform: 'translateX(0px) scaleX(0)' }, { transform: 'translateX(0px) scaleX(1)' }],
		at + 0.16,
		0.5,
		'glide'
	);
	tl.add(h.text, [{ transform: BELOW }, { transform: 'translateY(0)' }], at + 0.28, 0.6, 'glide');
}

/** Away over their edges: the lines lift out, the red block closes towards its right end. */
export function headOut(tl: TL, h: Head, at: number) {
	tl.add(h.title, [{ transform: 'translateY(0)' }, { transform: ABOVE }], at, 0.38, 'leave');
	tl.add(h.text, [{ transform: 'translateY(0)' }, { transform: ABOVE }], at + 0.04, 0.32, 'leave');
	const w = h.label.offsetWidth;
	tl.add(
		h.block,
		[{ transform: 'translateX(0px) scaleX(1)' }, { transform: `translateX(${w}px) scaleX(0)` }],
		at + 0.12,
		0.36,
		'move'
	);
}

/** The channel's lockup where the bug sits (its word at the bug's 34 px), fading in at `at`. */
export function mark(tl: TL, parent: HTMLElement, at: number) {
	const m = div('bl-mark', parent);
	lockup(m, 34, 'bl-lockup');
	tl.add(m, [{ opacity: 0 }, { opacity: 1 }], at, 0.6, 'sine');
	return m;
}

/** A credit line along the foot: the sources' credits parted by small squares. */
export function credits(parent: HTMLElement, items: string[]) {
	const c = div('bl-credit', parent);
	items.forEach((t, i) => {
		if (i) span('bl-sep', c);
		span(null, c, t);
	});
	fitText(c, { width: SAFE_RIGHT - 96, min: 16 });
	return c;
}

/** `els` lift away into the night from `at`, 30 ms apart. */
export function lift(tl: TL, els: Element[], at: number) {
	els.forEach((e, i) =>
		tl.add(
			e,
			[
				{ opacity: 1, transform: 'translateY(0px)' },
				{ opacity: 0, transform: 'translateY(-14px)' }
			],
			at + i * 0.03,
			0.4,
			'leave'
		)
	);
}

/**
 * The exit: everything left lifts away into the night over the last half second, the last of
 * it gone by the last frame (D less a frame), so the clip ends on night.
 */
export function exit(tl: TL, els: Element[]) {
	lift(tl, els, tl.duration - 0.04 - 0.4 - 0.03 * (els.length - 1));
}

/**
 * The channel's sign-off: the corner's lockup gives way to the tag in the foot's corner, whose
 * popcorn lands at `at` and whose couch, t and v type in on the compact sonic logo, as every
 * promo ends. The tag's sound rings out well before the sound's fade (see FADE).
 */
export function signOff(tl: TL, corner: Element, at: number, sound: Kit) {
	tl.add(corner, [{ opacity: 1 }, { opacity: 0 }], edge(at - 0.7), 0.4, 'sine');
	return channelTag(tl, tl.root, at, sound, { gain: 0.5 });
}

/** How long the bulletins' sound takes to fade out, ending this long before the last frame. */
export const FADE = { dur: 1.6, before: 0.2 };

/**
 * The bulletin's house mix (`start` the clip's start on the context's clock) with the whole of
 * it faded to silence before the clip ends, on a raised cosine (gentle, then gone): a slot cuts
 * the sound at both of its edges, so the hall must have rung out by the last frame. Called once,
 * by the cue that starts the score.
 */
export function bulletinHouse(k: Kit, b: Bus, start: number, D: number): House {
	const h = house(k, b);
	const curve = new Float32Array(64).map((_, i) => 0.5 + 0.5 * Math.cos((Math.PI * i) / 63));
	h.out.gain.setValueCurveAtTime(curve, start + D - FADE.before - FADE.dur, FADE.dur);
	return h;
}
