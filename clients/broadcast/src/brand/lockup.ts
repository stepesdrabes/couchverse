// The brand on screen: the mark, the lockup (one shared component: end cards, the corner bug,
// the lineup header and the stings all draw it from here), the corner bug and the promo tag.
// A brand redesign replaces this module, brand.css and sonic.ts; templates only call these.

import type { Kit } from '../audio/kit';
import { div, el, span } from '../core/dom';
import { SPRING, spring } from '../core/motion';
import type { Timeline } from '../core/timeline';
import { LOGO, themeTag } from './sonic';

export const CHANNEL = 'Couch TV';

/** The app's LogoMark: the couch on the red rounded square. */
export function mark(parent: Element, size?: number) {
	const m = div('mark', parent);
	if (size) m.style.setProperty('--s', size + 'px');
	el('i', null, m);
	return m;
}

export interface Lockup {
	root: HTMLDivElement;
	mark: HTMLDivElement;
	name: HTMLDivElement;
}

/** The mark beside the channel name; `size` is the mark's edge in stage pixels. */
export function lockup(parent: Element, size = 56): Lockup {
	const root = div('lockup', parent);
	root.style.setProperty('--ls', size + 'px');
	const m = mark(root);
	const name = div('lockup-name', root);
	span(null, name, 'Couch');
	span('tv-pill', name, 'TV');
	return { root, mark: m, name };
}

/** The corner bug: lockup, a breathing LIVE dot and the clock. */
export function bug(tl: Timeline, parent: Element, clock: string) {
	const b = div('bug', parent);
	mark(b);
	span('bug-name', b, CHANNEL);
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

/** A spring from `from` to `to` as keyframes, plus its duration. */
export function springFrames(
	from: number,
	to: number,
	cfg = SPRING.snappy,
	map: (v: number) => string = (v) => `scale(${v})`
) {
	const s = spring(cfg);
	return {
		frames: s.values.map((v) => ({ transform: map(from + (to - from) * v) })),
		dur: s.duration
	};
}

/** The end-of-promo channel tag: the lockup lands on the condensed mnemonic. */
export function channelTag(
	tl: Timeline,
	parent: Element,
	at: number,
	sound: Kit,
	{ size = 54, cls = 'tag-lockup', gain = 0.5 } = {}
) {
	const wrap = div(cls, parent);
	const lk = lockup(wrap, size);
	const sp = springFrames(0.4, 1, SPRING.bouncy);
	tl.add(lk.mark, [{ opacity: 0 }, { opacity: 1 }], at, 0.1, 'linear');
	tl.add(lk.mark, sp.frames, at, sp.dur, 'linear');
	tl.add(
		lk.name,
		[
			{ opacity: 0, transform: 'translateX(-18px)' },
			{ opacity: 1, transform: 'translateX(0)' }
		],
		at + LOGO.tagRhythm[2],
		0.7,
		'enter'
	);
	// the cue starts a beat early so live playback that starts on the tag still hears it
	tl.sound(at - 0.4, (w, b) => themeTag(sound, b, w + 0.4, { gain }));
	tl.cue('channel tag', at);
	return wrap;
}

/** When the tag's final chord lands, relative to its cue. */
export const TAG_SETTLE = LOGO.tagRhythm[3];
