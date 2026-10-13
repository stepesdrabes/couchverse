// The old C()UCH TV wordmark (D27, superseded by D33), which the stings still draw: Saira 700
// caps, the O cut into a pair of brackets, TV in a red block and the cursor that typed it. It
// goes with them when the new brand replaces them.
//
// The brackets are drawn, not typed: the O is two superellipses fitted to Saira's. A bracket
// that grows to frame something scales whole, its stroke with it, as the approved reel scaled
// the O's glyph: at any size it is Saira 700's own O, the weight of the word it frames.

import { valueAt, windows, type Move } from '../core/moves';
import type { Timeline } from '../core/timeline';

// Saira 700 in em, measured from the bundled font (a golden test re-measures them): advances
// with the wordmark's -0.004 em tracking, the baseline in a line-height 1 box, the cap height.
const WIDTH = { C: 0.56584, O: 0.69894, U: 0.68572, H: 0.70942, T: 0.58339, V: 0.659 } as const;
export const METRICS = { width: WIDTH, baseline: 0.848, cap: 0.688 } as const;
/** The caps' centre line below the top of a line-height 1 box. */
const CY = METRICS.baseline - METRICS.cap / 2;

// the O as |x/a|^n + |y/b|^n = 1 about its centre, within 0.003 em of the outline
const OUTER = { a: 0.2965, b: 0.3495, n: 3.76 };
const INNER = { a: 0.1215, b: 0.2174, n: 4.9 };

/** Closed, the brackets sit this far apart: an O at bug size, a pair of brackets up close. */
export const SPLIT = 0.05;
const TV_GAP = 0.24;
const TV_PAD = 0.1;
// the red blocks (TV, the cursor) overhang the caps
const BLOCK = { top: 0.07, height: 0.86, radius: 0.07 };
const CURSOR = 0.4;

export interface Span {
	x: number;
	w: number;
}

/** The wordmark's geometry at one size, px from its top left. */
export interface Layout {
	em: number;
	width: number;
	split: number;
	/** The cut between the brackets (the O's centre). */
	cx: number;
	/** The caps' centre line. */
	cy: number;
	baseline: number;
	/** A bracket's width at rest (half the O). */
	half: number;
	capH: number;
	c: Span;
	o: Span;
	uch: Span[];
	/** The TV block, padding included. */
	tv: Span;
	t: Span;
	v: Span;
	restX: number;
	curW: number;
	blockTop: number;
	blockH: number;
	radius: number;
}

export function layout(em: number): Layout {
	const span = (x: number, w: number): Span => ({ x: x * em, w: w * em });
	let x = 0;
	const c = span(x, WIDTH.C);
	x += WIDTH.C + SPLIT / 2;
	const o = span(x, WIDTH.O);
	x += WIDTH.O + SPLIT / 2;
	const restX = x;
	const uch = (['U', 'C', 'H'] as const).map((ch) => {
		const s = span(x, WIDTH[ch]);
		x += WIDTH[ch];
		return s;
	});
	x += TV_GAP;
	const tvX = x;
	const t = span(x + TV_PAD, WIDTH.T);
	const v = span(x + TV_PAD + WIDTH.T, WIDTH.V);
	const tvW = TV_PAD * 2 + WIDTH.T + WIDTH.V;
	return {
		em,
		width: (tvX + tvW) * em,
		split: SPLIT * em,
		cx: o.x + o.w / 2,
		cy: CY * em,
		baseline: METRICS.baseline * em,
		half: OUTER.a * em,
		capH: METRICS.cap * em,
		c,
		o,
		uch,
		tv: span(tvX, tvW),
		t,
		v,
		restX: restX * em,
		curW: CURSOR * em,
		// whole pixels: a block's fractional edge snaps differently on different compositing
		// paths, and the squeeze has to end on the ident's exact first frame
		blockTop: Math.round(BLOCK.top * em),
		blockH: Math.round(BLOCK.height * em),
		radius: BLOCK.radius * em
	};
}

const radius = (s: { a: number; b: number; n: number }, ux: number, uy: number) =>
	Math.pow(Math.pow(Math.abs(ux / s.a), s.n) + Math.pow(Math.abs(uy / s.b), s.n), -1 / s.n);

const num = (v: number) => String(Math.round(v * 100) / 100);

/**
 * One bracket as an SVG path (px), its open edge on (ox, oy): the left half of the O for
 * side -1, the right for 1, `k` times the O's size, stroke and all.
 */
export function bracketPath(em: number, k = 1, side: -1 | 1 = -1, ox = 0, oy = 0): string {
	const n = 40;
	const size = k * em;
	const outer: string[] = [];
	const inner: string[] = [];
	for (let i = 0; i <= 2 * n; i++) {
		const phi = (i / (2 * n)) * Math.PI;
		const ux = Math.sin(phi);
		const uy = -Math.cos(phi);
		const ro = radius(OUTER, ux, uy) * size;
		const ri = radius(INNER, ux, uy) * size;
		outer.push(`${num(ox + side * ro * ux)} ${num(oy + ro * uy)}`);
		inner.push(`${num(ox + side * ri * ux)} ${num(oy + ri * uy)}`);
	}
	return `M${outer.join('L')}L${inner.reverse().join('L')}Z`;
}

export type BracketColor = 'red' | 'black' | 'white';

export interface Bracket {
	svg: SVGSVGElement;
	path: SVGPathElement;
}

/** The wordmark's parts, built absolutely from `layout`; motion is transforms on the parts. */
export interface Wordmark {
	root: HTMLDivElement;
	c: HTMLSpanElement;
	/** U, C, H and TV (and the cursor), which move aside together when the brackets open. */
	rest: HTMLDivElement;
	uch: HTMLSpanElement[];
	tv: HTMLDivElement;
	tvBg: HTMLElement;
	tvT: HTMLSpanElement;
	tvV: HTMLSpanElement;
	cur: HTMLElement;
	/** A copy of the brackets per colour (a piece shows one at a time as the field changes). */
	halves: Partial<Record<BracketColor, { l: Bracket; r: Bracket }>>;
	m: Layout;
}

const SVG_NS = 'http://www.w3.org/2000/svg';

function bracket(parent: Element, m: Layout, side: -1 | 1, color: BracketColor): Bracket {
	const svg = document.createElementNS(SVG_NS, 'svg');
	svg.setAttribute('class', `wm-h c-${color}`);
	svg.setAttribute('width', '1');
	svg.setAttribute('height', '1');
	svg.style.left = `${m.cx + (side * m.split) / 2}px`;
	svg.style.top = `${m.cy}px`;
	const path = document.createElementNS(SVG_NS, 'path');
	path.setAttribute('d', bracketPath(m.em, 1, side));
	svg.appendChild(path);
	parent.appendChild(svg);
	return { svg, path };
}

function el<K extends keyof HTMLElementTagNameMap>(
	tag: K,
	cls: string,
	parent: Element,
	left: number,
	text?: string
): HTMLElementTagNameMap[K] {
	const e = document.createElement(tag);
	e.className = cls;
	e.style.left = `${left}px`;
	if (text) e.textContent = text;
	parent.appendChild(e);
	return e;
}

/**
 * Build a wordmark at font size `size` (px). `colors` are the bracket copies to build; the
 * cursor sits hidden in front of TV until a piece shows it.
 */
export function wordmark(
	parent: Element,
	size: number,
	{ colors = ['red'], cls = '' }: { colors?: BracketColor[]; cls?: string } = {}
): Wordmark {
	const m = layout(size);
	const root = document.createElement('div');
	root.className = `wm ${cls}`.trim();
	Object.assign(root.style, {
		fontSize: `${size}px`,
		width: `${m.width}px`,
		height: `${size}px`
	});
	parent.appendChild(root);
	const c = el('span', 'wm-l', root, m.c.x, 'C');
	const halves: Wordmark['halves'] = {};
	for (const col of colors)
		halves[col] = { l: bracket(root, m, -1, col), r: bracket(root, m, 1, col) };
	const rest = el('div', 'wm-rest', root, m.restX);
	const uch = (['U', 'C', 'H'] as const).map((ch, i) =>
		el('span', 'wm-l', rest, m.uch[i].x - m.restX, ch)
	);
	const tv = el('div', 'wm-tv', rest, m.tv.x - m.restX);
	tv.style.width = `${m.tv.w}px`;
	const tvBg = el('i', 'wm-block wm-tvbg', tv, 0);
	const tvT = el('span', 'wm-l', tv, m.t.x - m.tv.x, 'T');
	const tvV = el('span', 'wm-l', tv, m.v.x - m.tv.x, 'V');
	// the cursor lives with UCH TV, so it moves with them when the brackets open
	const cur = el('i', 'wm-block wm-cur', rest, 0);
	cur.style.width = `${m.curW}px`;
	for (const b of [tvBg, cur])
		Object.assign(b.style, { top: `${m.blockTop}px`, height: `${m.blockH}px` });
	return { root, c, rest, uch, tv, tvBg, tvT, tvV, cur, halves, m };
}

/**
 * Transforms that open the brackets to an inner gap `g` px (between the open edges), moved by
 * (dx, dy); a bracket `k` times the O's size grows outwards from its open edge, so the letters
 * move aside by its extra width to clear it.
 */
export function geometry(m: Layout, g: number, { k = 1, dx = 0, dy = 0 } = {}) {
	const d = (g - m.split) / 2;
	const e = (k - 1) * m.half;
	const at = (x: number) => `translate(${(x + dx).toFixed(2)}px, ${dy.toFixed(2)}px)`;
	return {
		d,
		l: at(-d),
		r: at(d),
		c: `translateX(${(-d - e).toFixed(2)}px)`,
		rest: `translateX(${(d + e).toFixed(2)}px)`
	};
}

/** The brackets' state: the gap between their open edges, their scale and offset (px). */
export interface Frame {
	g: number;
	k: number;
	dx: number;
	dy: number;
}

const mixFrame = (a: Frame, b: Frame, p: number): Frame => ({
	g: a.g + (b.g - a.g) * p,
	k: a.k + (b.k - a.k) * p,
	dx: a.dx + (b.dx - a.dx) * p,
	dy: a.dy + (b.dy - a.dy) * p
});

/**
 * Drive the brackets through `moves` from `v0`: their shape and place are computed per frame
 * (a pure function of t) and set on every colour's copy; `win` says when each copy shows.
 */
export function animateBrackets(
	tl: Timeline,
	wm: Wordmark,
	v0: Frame,
	moves: Move<Frame>[],
	win: Partial<Record<BracketColor, [number, number][]>>
) {
	const m = wm.m;
	const copies = Object.entries(wm.halves).map(([c, h]) => {
		windows(tl, h.l.svg, win[c as BracketColor] ?? []);
		windows(tl, h.r.svg, win[c as BracketColor] ?? []);
		return h;
	});
	tl.dynamic(
		(t) => {
			const v = valueAt(v0, moves, tl.duration, t, mixFrame);
			const d = (v.g - m.split) / 2;
			return `${bracketPath(m.em, v.k, -1, v.dx - d, v.dy)}|${bracketPath(m.em, v.k, 1, v.dx + d, v.dy)}`;
		},
		(v) => {
			const [l, r] = String(v).split('|');
			for (const h of copies) {
				h.l.path.setAttribute('d', l);
				h.r.path.setAttribute('d', r);
			}
		}
	);
}

/** The cursor block's transform: its left edge at x (px in the wordmark), `w` wide. */
export function cursorAt(m: Layout, x: number, w = m.curW): string {
	return `translateX(${(x - m.restX).toFixed(2)}px) scaleX(${(w / m.curW).toFixed(4)})`;
}

/**
 * Where a wordmark centred on (x, y) has its top left: its box centred, as the approved reel
 * placed it (the caps sit 0.004 em below the box's centre).
 */
export const placeAt = (m: Layout, x: number, y: number) => ({
	x: Math.round((x - m.width / 2) * 100) / 100,
	y: Math.round((y - m.em / 2) * 100) / 100
});

/** Centre the wordmark on (x, y) of its parent; returns its top left. */
export function centre(wm: Wordmark, x: number, y: number): { x: number; y: number } {
	const p = placeAt(wm.m, x, y);
	wm.root.classList.add('wm-abs');
	wm.root.style.left = `${p.x}px`;
	wm.root.style.top = `${p.y}px`;
	return p;
}
