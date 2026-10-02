// Spatial navigation for the TV remote: arrow keys move focus to the geometrically
// nearest focusable element in that direction. Pages opt into tweaks with attributes:
//   data-tv-autofocus  where focus lands when a page opens
//   data-tv-pin        fixed chrome (nav bar, music bar) that the page scrolls under
//   data-tv-layer      a custom overlay that confines focus like a bits-ui dialog
//   data-tv-skip       never focused by the remote

export type Direction = 'up' | 'down' | 'left' | 'right';

export const DIRECTIONS: Record<string, Direction> = {
	ArrowUp: 'up',
	ArrowDown: 'down',
	ArrowLeft: 'left',
	ArrowRight: 'right'
};

const FOCUSABLE = [
	'a[href]',
	'button:not([disabled])',
	'input:not([disabled]):not([type="hidden"])',
	'select:not([disabled])',
	'textarea:not([disabled])',
	'[tabindex]:not([tabindex="-1"])'
].join(',');

// overlays that own the remote while open: the roles bits-ui renders, plus custom panels
const LAYER = '[role="dialog"],[role="alertdialog"],[role="menu"],[role="listbox"],[data-tv-layer]';
const PIN = '[data-tv-pin]';

// candidates this much farther than the nearest one still belong to its row
const ROW_SLACK = 24;
// how much farther than the nearest row a target straight ahead may be and still win
const BEAM_REACH = 120;

interface Box {
	el: HTMLElement;
	top: number;
	bottom: number;
	left: number;
	right: number;
	pin: Element | null;
}

const isVertical = (dir: Direction) => dir === 'up' || dir === 'down';

// whether an animation (a Svelte transition or a CSS keyframe one) is running on `el`
// or an ancestor, e.g. a page fading in on arrival
function animating(el: HTMLElement): boolean {
	return document.getAnimations().some((a) => {
		const target = a.effect instanceof KeyframeEffect ? a.effect.target : null;
		return !!target?.contains(el);
	});
}

// Transparent elements are not targets (hover-revealed controls), unless they are
// only transparent because they are still fading in.
function shown(el: HTMLElement): boolean {
	// checkVisibility is missing on the oldest TV Chromium builds
	if (typeof el.checkVisibility !== 'function') return true;
	if (el.checkVisibility({ checkOpacity: true, checkVisibilityCSS: true })) return true;
	return el.checkVisibility({ checkVisibilityCSS: true }) && animating(el);
}

function toBox(el: HTMLElement): Box {
	const r = el.getBoundingClientRect();
	return { el, top: r.top, bottom: r.bottom, left: r.left, right: r.right, pin: el.closest(PIN) };
}

// the element's box if the remote may focus it (rendered, visible, not opted out)
function measure(el: HTMLElement): Box | null {
	if (el.closest('[inert],[data-tv-skip]')) return null;
	const b = toBox(el);
	return b.right > b.left && b.bottom > b.top && shown(el) ? b : null;
}

function boxes(root: ParentNode): Box[] {
	const out: Box[] = [];
	for (const el of root.querySelectorAll<HTMLElement>(FOCUSABLE)) {
		const b = measure(el);
		if (b) out.push(b);
	}
	return out;
}

const onScreen = (b: Box) =>
	b.bottom > 0 && b.top < innerHeight && b.right > 0 && b.left < innerWidth;

function span(b: Box, horizontal: boolean): [number, number] {
	return horizontal ? [b.left, b.right] : [b.top, b.bottom];
}

const overlap = (a: [number, number], b: [number, number]) =>
	Math.min(a[1], b[1]) - Math.max(a[0], b[0]);

// edge-to-edge distance from `from` to `to` along `dir`, or null when `to` is not
// that way (its centre must be past ours, overlapping us by under half the smaller box)
function distance(from: Box, to: Box, dir: Direction): number | null {
	const [f0, f1] = span(from, !isVertical(dir));
	const [t0, t1] = span(to, !isVertical(dir));
	const forward = dir === 'down' || dir === 'right';
	if (forward ? t0 + t1 <= f0 + f1 : t0 + t1 >= f0 + f1) return null;
	if (overlap([f0, f1], [t0, t1]) > Math.min(f1 - f0, t1 - t0) / 2) return null;
	return Math.max(0, forward ? t0 - f1 : f0 - t1);
}

// how far `to` sits off our line of travel (0 when the two overlap across it)
function offLine(from: Box, to: Box, dir: Direction): number {
	return Math.max(0, -overlap(span(from, isVertical(dir)), span(to, isVertical(dir))));
}

// centre misalignment across the line of travel
function centreGap(from: Box, to: Box, dir: Direction): number {
	const [a0, a1] = span(from, isVertical(dir));
	const [b0, b1] = span(to, isVertical(dir));
	return Math.abs(a0 + a1 - b0 - b1) / 2;
}

type Scored = { b: Box; d: number };

function closest(list: Scored[], from: Box, dir: Direction): Box {
	return list.reduce((best, s) =>
		s.d < best.d || (s.d === best.d && centreGap(from, s.b, dir) < centreGap(from, best.b, dir))
			? s
			: best
	).b;
}

function pick(from: Box, pool: Box[], dir: Direction): Box | null {
	const scored: Scored[] = [];
	for (const b of pool) {
		const d = distance(from, b, dir);
		if (d !== null) scored.push({ b, d });
	}
	if (!scored.length) return null;

	// sideways moves stay in the row; its end is a wall rather than a diagonal jump
	if (!isVertical(dir)) {
		const row = scored.filter(({ b }) => offLine(from, b, dir) === 0);
		return row.length ? closest(row, from, dir) : null;
	}

	const nearest = Math.min(...scored.map((s) => s.d));
	const reach = nearest + Math.max(from.bottom - from.top, BEAM_REACH);
	const beam = scored.filter(({ b, d }) => offLine(from, b, dir) === 0 && d <= reach);
	if (beam.length) return closest(beam, from, dir);
	// nothing straight ahead: the nearest row, at its best-aligned element
	const row = scored.filter(({ d }) => d <= nearest + ROW_SLACK);
	return row.reduce((best, s) =>
		centreGap(from, s.b, dir) < centreGap(from, best.b, dir) ? s : best
	).b;
}

// Fixed chrome is reached by leaving the page past its edge, whatever the page's scroll
// position (it may still be animating): up finds the chrome along the top of the
// screen, down the chrome along the bottom.
function toChrome(from: Box, chrome: Box[], dir: Direction): Box | null {
	const side = chrome.filter((b) => b.top + b.bottom < innerHeight === (dir === 'up'));
	if (!side.length) return null;
	return side.reduce((best, b) =>
		centreGap(from, b, dir) < centreGap(from, best, dir) ? b : best
	);
}

/** The next element to focus from `from` in direction `dir`, searching within `root`. */
export function findNext(from: HTMLElement, dir: Direction, root: ParentNode): HTMLElement | null {
	const all = boxes(root).filter((b) => b.el !== from);
	const src = toBox(from);
	// Fixed chrome never competes with the page scrolling under it: moves stay within
	// the page or within one pinned group, and only a vertical move with nothing left
	// that way crosses over. Out of the chrome, only the page on screen is in reach.
	const hit = pick(
		src,
		all.filter((b) => b.pin === src.pin),
		dir
	);
	if (hit || !isVertical(dir)) return hit?.el ?? null;
	const otherChrome = all.filter((b) => b.pin && b.pin !== src.pin);
	const intoPage = src.pin
		? pick(
				src,
				all.filter((b) => !b.pin && onScreen(b)),
				dir
			)
		: null;
	return (intoPage ?? toChrome(src, otherChrome, dir))?.el ?? null;
}

/**
 * Where focus lands when nothing is focused: a `data-tv-autofocus` element, else the
 * first thing in the page on screen, else (if allowed) the chrome.
 */
export function defaultTarget(root: ParentNode, allowChrome = true): HTMLElement | null {
	const all = boxes(root);
	const marked = all.find((b) => b.el.hasAttribute('data-tv-autofocus'));
	const first = marked ?? all.find((b) => !b.pin && onScreen(b));
	return (first ?? (allowChrome ? all[0] : undefined))?.el ?? null;
}

/** Whether `el` is something the remote could focus right now. */
export function focusable(el: Element | null): el is HTMLElement {
	return (
		el instanceof HTMLElement && el.isConnected && el.matches(FOCUSABLE) && measure(el) !== null
	);
}

/** The open overlay focus is confined to, or null when none is open. */
export function activeLayer(): HTMLElement | null {
	const owner = document.activeElement?.closest<HTMLElement>(LAYER);
	if (owner) return owner;
	const open = [...document.querySelectorAll<HTMLElement>(LAYER)].filter(shown);
	return open.at(-1) ?? null;
}

const scrolls = (overflow: string) => overflow === 'auto' || overflow === 'scroll';

// how far to scroll so [start, end] fits within [min, max]
function nudge(start: number, end: number, min: number, max: number): number {
	if (start < min) return start - min;
	if (end > max) return end - max;
	return 0;
}

/**
 * Focus `el` and bring it into view: real scroll containers (carousel rows, menu lists)
 * just enough to show it, the page so it sits mid-screen. Not scrollIntoView, which
 * also scrolls overflow-hidden boxes such as the hero and shifts what they clip.
 */
export function focusTarget(el: HTMLElement, block: 'center' | 'nearest' = 'center') {
	el.focus({ preventScroll: true });
	const reduced = matchMedia('(prefers-reduced-motion: reduce)').matches;
	const behavior: ScrollBehavior = reduced ? 'instant' : 'smooth';
	const r = el.getBoundingClientRect();
	for (let p = el.parentElement; p && p !== document.body; p = p.parentElement) {
		const s = getComputedStyle(p);
		const box = p.getBoundingClientRect();
		const padLeft = parseFloat(s.scrollPaddingLeft) || 0;
		const padRight = parseFloat(s.scrollPaddingRight) || 0;
		const left =
			scrolls(s.overflowX) && p.scrollWidth > p.clientWidth
				? nudge(r.left, r.right, box.left + padLeft, box.right - padRight)
				: 0;
		const top =
			scrolls(s.overflowY) && p.scrollHeight > p.clientHeight
				? nudge(r.top, r.bottom, box.top, box.bottom)
				: 0;
		if (left || top) p.scrollBy({ left, top, behavior });
	}
	// fixed chrome and overlays stay put while the page scrolls under them
	if (el.closest(`${PIN},${LAYER}`)) return;
	if (block === 'center' || r.top < 0 || r.bottom > innerHeight) {
		window.scrollBy({ top: (r.top + r.bottom - innerHeight) / 2, behavior });
	}
}

/** Whether an arrow key should leave the focused form field rather than edit it. */
export function leavesField(el: Element, dir: Direction): boolean {
	if (el instanceof HTMLTextAreaElement) {
		const backward = dir === 'left' || dir === 'up';
		return (
			el.selectionStart === el.selectionEnd &&
			(backward ? el.selectionStart === 0 : el.selectionEnd === el.value.length)
		);
	}
	if (!(el instanceof HTMLInputElement)) return true;
	// left/right adjust a slider
	if (el.type === 'range') return isVertical(dir);
	if (isVertical(dir)) return true;
	// types without a caret (email, number) let go straight away
	if (el.selectionStart === null) return true;
	if (el.selectionStart !== el.selectionEnd) return false;
	return dir === 'left' ? el.selectionStart === 0 : el.selectionStart === el.value.length;
}
