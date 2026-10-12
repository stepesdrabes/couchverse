// The Couch TV wordmark: C()UCH TV. The O of COUCH is one glyph cut down the middle, so it
// reads as an O at any size and opens into a pair of brackets: the couch's armrests, holding
// whatever is on. TV sits in a red block, the terminal cursor that typed it.
//
// Everything here is built once and measured; motion is transforms on the parts, so a piece
// can open the brackets, frame something and snap them shut on any clock.

import { el, css } from './core.js';

// the O halves sit apart by this much when closed: an O at bug size, brackets up close
export const SPLIT = 0.05;
// cap box of Saira in a line-height-1 box: caps from 0.155em to 0.845em (centred on 0.5em)
export const CAP = { top: 0.155, height: 0.69 };
// red blocks (cursor, TV) overhang the caps by this much
const PAD = 0.085;

let inkCache = null;
/** The O's ink half-width and stem, in em (canvas metrics, so it follows the real font). */
function inkMetrics() {
	if (inkCache) return inkCache;
	const c = document.createElement('canvas').getContext('2d');
	c.font = '700 1000px Saira';
	const m = c.measureText('O');
	const adv = m.width / 1000;
	const left = m.actualBoundingBoxLeft / 1000;
	const right = m.actualBoundingBoxRight / 1000;
	inkCache = { adv, half: (left + right) / 2, centre: (right - left) / 2 };
	return inkCache;
}

/**
 * Build a wordmark. `colors` are the bracket copies to build (a piece shows one at a time
 * when the field behind changes colour). Returns the parts and their metrics
 * in the root's coordinates (px).
 */
export function wordmark(parent, size, { colors = ['red'], plainO = false, cls = '' } = {}) {
	const root = el('div', `wm ${cls}`.trim(), parent);
	root.style.fontSize = size + 'px';
	root.style.setProperty('--split', SPLIT + 'em');
	const c = el('span', 'wm-l', root, 'C');
	const o = el('span', 'wm-o', root);
	el('span', 'wm-ghost', o, 'O');
	const halves = {};
	for (const col of colors) {
		halves[col] = { l: el('span', `wm-h wm-hl c-${col}`, o, 'O'), r: el('span', `wm-h wm-hr c-${col}`, o, 'O') };
	}
	const plain = plainO ? { white: el('span', 'wm-h wm-plain c-white', o, 'O'), black: el('span', 'wm-h wm-plain c-black', o, 'O') } : null;
	const rest = el('span', 'wm-rest', root);
	const uch = ['U', 'C', 'H'].map((ch) => el('span', 'wm-l', rest, ch));
	const tv = el('span', 'wm-tv', rest);
	const tvBg = el('i', 'wm-block wm-tvbg', tv);
	const tvT = el('span', 'wm-l', tv, 'T');
	const tvV = el('span', 'wm-l', tv, 'V');
	// the cursor lives with UCH TV, so it moves with them when the brackets open
	const cur = el('i', 'wm-block wm-cur', rest);

	const em = size;
	const box = (e) => {
		let x = 0;
		for (let n = e; n && n !== root; n = n.offsetParent) x += n.offsetLeft;
		return { x, w: e.offsetWidth };
	};
	const ink = inkMetrics();
	const ob = box(o);
	const m = {
		em,
		width: root.offsetWidth,
		split: SPLIT * em,
		// the O's centre, and the bracket halves' open edges at rest
		cx: ob.x + ob.w / 2,
		half: ink.half * em,
		capTop: CAP.top * em,
		capH: CAP.height * em,
		cy: 0.5 * em,
		c: box(c),
		o: ob,
		uch: uch.map(box),
		tv: box(tv),
		t: box(tvT),
		v: box(tvV),
		restX: box(rest).x,
		curW: 0.4 * em,
		blockTop: (CAP.top - PAD) * em,
		blockH: (CAP.height + 2 * PAD) * em
	};
	css(cur, { width: m.curW + 'px' });
	return { root, c, o, halves, plain, rest, uch, tv, tvBg, tvT, tvV, cur, m };
}

/**
 * Transforms that open the brackets to an inner gap `g` px (between the open edges), scaled
 * by `k` about the cap centre and moved by (dx, dy). The letters move aside in-line.
 */
export function geometry(m, g, { k = 1, dx = 0, dy = 0 } = {}) {
	const d = (g - m.split) / 2;
	// a bracket scaled up grows outwards from its open edge: the letters clear it
	const e = (k - 1) * m.half;
	const tf = (x) => `translate(${(x + dx).toFixed(2)}px, ${dy.toFixed(2)}px) scale(${k.toFixed(4)})`;
	return {
		l: tf(-d),
		r: tf(d),
		c: `translateX(${(-d - e).toFixed(2)}px)`,
		rest: `translateX(${(d + e).toFixed(2)}px)`
	};
}

/** The cursor block at x (px in the wordmark, its left edge), optionally stretched to width w. */
export function cursorAt(m, x, w = m.curW) {
	return `translateX(${(x - m.restX).toFixed(2)}px) scaleX(${(w / m.curW).toFixed(4)})`;
}

/** Put the wordmark's cap centre on (x, y) of its parent, horizontally centred as a whole. */
export function centre(wm, x, y) {
	css(wm.root, { left: (x - wm.m.width / 2).toFixed(1) + 'px', top: (y - wm.m.cy).toFixed(1) + 'px' });
	return wm;
}

/** A static lockup (bug, lineup header, end cards): closed brackets, TV in its block. */
export function lockupStatic(parent, size, cls = '') {
	const wm = wordmark(parent, size, { cls: `wm-static ${cls}`.trim() });
	wm.cur.remove();
	return wm;
}
