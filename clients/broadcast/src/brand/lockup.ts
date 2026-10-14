// The brand's small pieces on every template: the static lockup, the corner bug and the
// end-of-promo channel tag, all drawn from the one lockup geometry (wordmark.ts); templates only
// call these. Their places are the screens' grid's: the bug's right edge on the title-safe line
// (x 1824) and its top on y 64, the tag's slot ending on x 1824 and y 1008.

import type { Kit } from '../audio/kit';
import { house } from '../audio/voices';
import { div, span, svg } from '../core/dom';
import { edge } from '../core/moves';
import type { Timeline } from '../core/timeline';
import { COUCH, LOGO_GAP, LOGO_H, LOGO_W, POPCORN, SILHOUETTE } from './glyph';
import { END, SONIC, sonicRingOut, sonicTag } from './sonic';
import {
	BRAND,
	CHANNEL,
	geometry,
	lockupSvg,
	PARTS,
	ROW,
	WORD,
	type LockupLayout
} from './wordmark';

export { CHANNEL };

/** The static lockup with the word at `size` px, as an inline SVG. */
export function lockup(
	parent: Element,
	size: number,
	cls = 'lockup',
	layout: LockupLayout = 'row'
): HTMLSpanElement {
	const s = span(cls, parent);
	// our own markup from numbers and constants: no spec text goes in
	s.innerHTML = lockupSvg({ size, layout, label: CHANNEL });
	return s;
}

/**
 * The corner bug: the row lockup (its word at 34 px), a rule, the LIVE marker and the clock (the
 * minute the clip starts). Nothing in it moves (the dot is steady), so the renderer can skip a
 * screen's held frames. `scrim` shades the corner under it, for a screen whose own grade does
 * not.
 */
export function bug(_tl: Timeline, parent: Element, clock: string, { scrim = false } = {}) {
	const b = div('bug', parent);
	if (scrim) div('bug-scrim', b);
	lockup(b, 34, 'bug-wm');
	span('bug-rule', b);
	const live = span('live', b);
	span('live-dot', live);
	span(null, live, 'Live');
	span('clock tnum', b, clock);
	return b;
}

/** The lockup's four parts (the logo, couch, t, v) as SVGs placed from its geometry, to animate. */
export function lockupParts(parent: Element, size: number, layout: LockupLayout = 'row') {
	const g = geometry(size, layout);
	const root = div('lockup-parts', parent);
	Object.assign(root.style, { width: `${g.width}px`, height: `${g.height}px` });
	const part = (at: { x: number; y: number; k: number }, box: number[]) => {
		// a transform's origin is the middle of the part's own box
		const [x0, y0, x1, y1] = box;
		const e = svg(
			'svg',
			{
				width: ((x1 - x0) * at.k).toFixed(2),
				height: ((y1 - y0) * at.k).toFixed(2),
				viewBox: `${x0} ${y0} ${x1 - x0} ${y1 - y0}`
			},
			root
		);
		e.setAttribute('aria-hidden', 'true');
		Object.assign(e.style, {
			position: 'absolute',
			left: `${(at.x + x0 * at.k).toFixed(2)}px`,
			top: `${(at.y + y0 * at.k).toFixed(2)}px`,
			overflow: 'visible'
		});
		return e;
	};
	const logo = part(g.logo, [0, 0, LOGO_W, LOGO_H]);
	// each part's box is its line box, as the idents' spans are: their scale settles about it
	const top = -WORD.baseline * 1000;
	const word = PARTS.map((p, i) => {
		const e = part({ ...g.word, x: g.word.x + p.x * g.word.k }, [0, top, p.advance, top + 1000]);
		svg('path', { d: p.d, fill: i ? BRAND.red : BRAND.white }, e);
		return e;
	});
	return { root, logo, word };
}

/** The popcorn's fall in the tag, from rest 1.1 logo heights up. */
const FALL = 0.42;
/** couch types in half a beat after the landing. */
const HALF = SONIC.at[1];

let tags = 0;

/**
 * The end-of-promo channel tag, the row lockup `height` px tall in the tag slot: the couch fades
 * in, the popcorn drops onto it with gravity and lands at `at` (a soft round impact), then
 * couch, t and v type in on the compact sonic logo, half a beat apart, as every ident ends.
 * Still from `at` + 1.5. `scrim` shades the slot, for a screen whose own grade does not.
 */
export function channelTag(
	tl: Timeline,
	parent: Element,
	at: number,
	sound: Kit,
	{ height = 80, cls = 'tag-lockup', gain = 0.5, scrim = false } = {}
) {
	const wrap = div(cls, parent);
	// the shade comes with the couch
	if (scrim)
		tl.add(div('tag-scrim', wrap), [{ opacity: 0 }, { opacity: 1 }], edge(at - 0.7), 0.4, 'sine');
	const size = height / ROW.logo;
	const { logo, word } = lockupParts(wrap, size);
	// the couch under the popcorn's knockout: a mask, so the knockout shows what is behind
	const id = `tag-knockout-${++tags}`;
	const extent = { x: -400, y: -1600, width: LOGO_W + 800, height: LOGO_H + 2000 };
	const mask = svg('mask', { id, maskUnits: 'userSpaceOnUse', ...extent }, logo);
	svg('rect', { ...extent, fill: '#fff' }, mask);
	const halo = svg('g', {}, mask);
	svg(
		'path',
		{
			d: SILHOUETTE,
			fill: '#000',
			stroke: '#000',
			'stroke-width': LOGO_GAP * 2,
			'stroke-linejoin': 'round'
		},
		halo
	);
	const couch = svg('path', { d: COUCH, fill: BRAND.red, mask: `url(#${id})` }, logo);
	const pop = svg('g', {}, logo);
	svg('path', { d: POPCORN, fill: BRAND.red }, pop);

	tl.add(couch, [{ opacity: 0 }, { opacity: 1 }], edge(at - 0.7), 0.4, 'sine');
	tl.add(pop, [{ opacity: 0 }, { opacity: 1 }], edge(at - FALL), 0.12, 'linear');
	// gravity: a pure function of t, on the popcorn and its knockout alike
	tl.dynamic(
		(t) => {
			const u = Math.min(1, Math.max(0, (t - (at - FALL)) / FALL));
			return u >= 1 ? 'rest' : (-1.1 * LOGO_H * (1 - u * u)).toFixed(2);
		},
		(v) => {
			const tf = v === 'rest' ? '' : `translate(0 ${v})`;
			for (const e of [pop, halo]) e.setAttribute('transform', tf);
		}
	);
	// the approved endings' type-in, scaled to the slot
	const k = size / 184;
	word.forEach((e, i) =>
		tl.add(
			e,
			[
				{
					opacity: 0,
					transform: `translateY(${((i ? 34 : 40) * k).toFixed(2)}px) scale(${i ? 1.2 : 1})`
				},
				{ opacity: 1, transform: 'translateY(0px) scale(1)' }
			],
			edge(at + HALF + SONIC.at[i]),
			i ? 0.26 : 0.34,
			i ? 'snap' : 'out'
		)
	);
	// the cue starts early: the fall breathes in before the landing, and live playback that
	// starts on the tag still hears it. The tag closes its clip: the whole mix rings out with it.
	tl.sound(at - 0.6, (w, b) => {
		const first = w + 0.6 + HALF;
		const until = w + (tl.duration - (at - 0.6)) - END;
		sonicTag(sound, b, first, { gain, until });
		sonicRingOut(house(sound, b), first + SONIC.at[2], until);
	});
	tl.cue('channel tag: the popcorn lands', at);
	return wrap;
}
