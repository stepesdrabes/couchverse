// The channel ident: the full cut (10.52 s) is one of four designs (idents/, D35), the short cut
// (4.52 s) the popcorn ending they all close on, alone (short-ident.ts, D34). The framed items
// below are the old variant B's, which the stings still pop in until they move to the new brand.

import { css, div, el, img } from '../core/dom';
import { edge, windows } from '../core/moves';
import type { IdentSpec } from '../spec';
import type { Template, TL } from '../templates/shared';
import { METRICS } from './brackets';
import { buildFullIdent } from './idents';
import { buildShortIdent } from './short-ident';

const COPY = {
	on: 'ON',
	now: 'NOW',
	movies: 'MOVIES',
	series: 'SERIES',
	time: '20:15'
};

export type Item =
	'on' | 'now' | 'movies' | 'series' | 'time' | 'star' | 'play' | 'rgb' | 'couch' | 'poster';

// cap heights (px) per item: the brackets grow to hug them
const CAPS: Record<Item, number> = {
	on: 320,
	now: 270,
	time: 250,
	movies: 175,
	series: 175,
	star: 270,
	play: 270,
	rgb: 200,
	couch: 270,
	poster: 300
};

const STAR =
	'polygon(50% 0%, 61.2% 34.5%, 97.6% 34.5%, 68.2% 55.9%, 79.4% 90.5%, 50% 69.1%, 20.6% 90.5%, 31.8% 55.9%, 2.4% 34.5%, 38.8% 34.5%)';

export interface Framed {
	e: HTMLElement;
	w: number;
	h: number;
}

/** A framed item centred on the stage, sized from its cap height (text: the caps' height). */
export function frameItem(
	layer: Element,
	kind: Item,
	fg: string,
	poster: string | null = null
): Framed {
	const cap = CAPS[kind];
	let e: HTMLElement;
	let w: number;
	let h: number;
	if (kind === 'poster') {
		h = cap * 1.18;
		w = h * (2 / 3);
		e = div('id-poster', layer);
		if (poster) img(null, e, poster);
		// no poster to show: a card in the brand's colours, the couch on it
		else e.classList.add('id-poster-stand');
	} else if (kind === 'couch') {
		h = cap * 0.95;
		w = h * (1046 / 745);
		e = css(div('id-couch', layer), { background: fg });
	} else if (kind === 'play') {
		h = cap;
		w = h * 0.88;
		e = css(div('id-shape', layer), {
			background: fg,
			clipPath: 'polygon(8% 0%, 100% 50%, 8% 100%)'
		});
	} else if (kind === 'star') {
		h = cap * 1.12;
		w = h * 1.05;
		e = css(div('id-shape', layer), { background: fg, clipPath: STAR });
	} else if (kind === 'rgb') {
		h = cap;
		w = h * 1.62;
		e = div('id-rgb', layer);
		for (const c of ['#ff0000', '#00ff00', '#0000ff']) css(el('i', null, e), { background: c });
	} else {
		const size = cap / METRICS.cap;
		e = css(div('id-txt tnum', layer, COPY[kind]), { fontSize: `${size.toFixed(1)}px`, color: fg });
		w = e.offsetWidth;
		h = cap;
		// a text box is line-height 1: its caps' centre sits where the wordmark's does
		css(e, {
			left: `${(960 - w / 2).toFixed(1)}px`,
			top: `${(540 - (METRICS.baseline - METRICS.cap / 2) * size).toFixed(1)}px`
		});
		return { e, w, h };
	}
	css(e, {
		width: `${w}px`,
		height: `${h}px`,
		left: `${(960 - w / 2).toFixed(1)}px`,
		top: `${(540 - h / 2).toFixed(1)}px`
	});
	return { e, w, h };
}

/** Show an item from `a` until `z`, popping in. */
export function popIn(tl: TL, it: Framed, a: number, z: number) {
	windows(tl, it.e, [[a, z]]);
	tl.add(it.e, [{ transform: 'scale(0.82)' }, { transform: 'scale(1)' }], edge(a), 0.1, 'out');
}

export const ident: Template<IdentSpec> = {
	version: 7,
	build(ctx, spec) {
		return spec.cut === 'short' ? buildShortIdent(ctx) : buildFullIdent(ctx, spec);
	}
};
