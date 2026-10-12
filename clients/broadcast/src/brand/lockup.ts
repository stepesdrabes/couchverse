// The brand's small pieces on every template: the static lockup, the corner bug and the
// end-of-promo channel tag. All of them draw the one wordmark (wordmark.ts); templates only
// call these.

import type { Kit } from '../audio/kit';
import { div, span } from '../core/dom';
import { motion, windows } from '../core/moves';
import type { Timeline } from '../core/timeline';
import { HOOK, themeTag } from './theme';
import { CHANNEL, cursorAt, geometry, lockupSvg, wordmark } from './wordmark';

export { CHANNEL };

/** The static lockup at font size `size` (px), as an inline SVG. */
export function lockup(parent: Element, size: number, cls = 'lockup'): HTMLSpanElement {
	const s = span(cls, parent);
	// our own markup from numbers and constants: no spec text goes in
	s.innerHTML = lockupSvg({ size, label: CHANNEL });
	return s;
}

/** The corner bug: the lockup, a breathing LIVE dot and the clock. */
export function bug(tl: Timeline, parent: Element, clock: string) {
	const b = div('bug', parent);
	lockup(b, 27, 'bug-wm');
	span('bug-sep', b);
	const live = span('live', b);
	const dot = span('live-dot', live);
	span(null, live, 'Live');
	span('bug-sep', b);
	span('clock tnum', b, clock);
	// iterations rather than an infinite loop, so seeking still lands right
	tl.add(dot, [{ opacity: 1 }, { opacity: 0.35 }, { opacity: 1 }], 0, 1.6, 'sine', {
		iterations: Math.ceil(tl.duration / 1.6)
	});
	return b;
}

/**
 * The end-of-promo channel tag: the wordmark lands on the hook, condensed. It appears closed
 * on the first note, the brackets open a little on the next three (widest on the top note) and
 * TV drops into its block on the long last note, as the ident does.
 */
export function channelTag(
	tl: Timeline,
	parent: Element,
	at: number,
	sound: Kit,
	{ size = 62, cls = 'tag-lockup', gain = 0.5 } = {}
) {
	const wrap = div(cls, parent);
	const wm = wordmark(wrap, size, { cls: 'wm-tag' });
	const m = wm.m;
	const R = HOOK.tagAt.map((r) => at + r);
	tl.add(
		wrap,
		[
			{ opacity: 0, transform: 'scale(0.92)' },
			{ opacity: 1, transform: 'scale(1)' }
		],
		at - 0.02,
		0.2,
		'snap'
	);
	const moves = (
		[
			[R[1], 0.2],
			[R[2], 0.42],
			[R[3], 0.22]
		] as const
	).flatMap(([t, w]) => [
		{ at: t, dur: 0.05, to: m.em * w + m.split, ease: 'out' },
		{ at: t + 0.08, dur: 0.1, to: m.split, ease: 'inOut' }
	]);
	const half = wm.halves.red;
	if (half) {
		motion(tl, half.l.svg, m.split, moves, (g) => ({ transform: geometry(m, g).l }));
		motion(tl, half.r.svg, m.split, moves, (g) => ({ transform: geometry(m, g).r }));
	}
	motion(tl, wm.c, m.split, moves, (g) => ({ transform: geometry(m, g).c }));
	motion(tl, wm.rest, m.split, moves, (g) => ({ transform: geometry(m, g).rest }));
	motion(
		tl,
		wm.cur,
		cursorAt(m, m.tv.x),
		[{ at: R[4], dur: 0.12, to: cursorAt(m, m.tv.x, m.tv.w), ease: 'snap' }],
		(v) => ({ transform: v })
	);
	const D = tl.duration;
	windows(tl, wm.cur, [[0, R[4] + 0.12]]);
	windows(tl, wm.tvBg, [[R[4] + 0.12, D + 1]]);
	windows(tl, wm.tvT, [[R[4], D + 1]]);
	windows(tl, wm.tvV, [[R[4] + 0.06, D + 1]]);
	// the cue starts early: the tag's swell leads into the first note, and live playback that
	// starts on the tag still hears it
	tl.sound(at - 0.4, (w, b) => themeTag(sound, b, w + 0.4, { gain }));
	tl.cue('channel tag', at);
	return wrap;
}
