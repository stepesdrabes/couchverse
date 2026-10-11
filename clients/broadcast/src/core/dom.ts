// DOM building blocks for templates. Text from a spec only ever goes in as text nodes.

import type { Timeline } from './timeline';
import type { Ease } from './motion';

export function el<K extends keyof HTMLElementTagNameMap>(
	tag: K,
	cls?: string | null,
	parent?: Element | null,
	text?: string | null
): HTMLElementTagNameMap[K] {
	const e = document.createElement(tag);
	if (cls) e.className = cls;
	if (text != null) e.textContent = text;
	if (parent) parent.appendChild(e);
	return e;
}

export function div(cls?: string | null, parent?: Element | null, text?: string | null) {
	return el('div', cls, parent, text);
}

export function span(cls?: string | null, parent?: Element | null, text?: string | null) {
	return el('span', cls, parent, text);
}

/**
 * An image that rasterizes with the frame it first appears in: `sync` keeps the browser from
 * painting it a frame later (checker-imaging), which would make a frame depend on timing.
 */
export function img(cls: string | null, parent: Element | null, src?: string): HTMLImageElement {
	const i = el('img', cls, parent);
	i.decoding = 'sync';
	i.alt = '';
	if (src) i.src = src;
	return i;
}

export function css<T extends HTMLElement>(e: T, styles: Partial<CSSStyleDeclaration>): T {
	Object.assign(e.style, styles);
	return e;
}

export function svg<K extends keyof SVGElementTagNameMap>(
	tag: K,
	attrs: Record<string, string | number>,
	parent?: Element | null
): SVGElementTagNameMap[K] {
	const e = document.createElementNS('http://www.w3.org/2000/svg', tag);
	for (const [k, v] of Object.entries(attrs)) e.setAttribute(k, String(v));
	if (parent) parent.appendChild(e);
	return e;
}

/** Text split into characters (inline-blocks). Spaces become fixed-width spacers. */
export function chars(parent: Element, text: string, cls?: string) {
	const root = span(cls, parent);
	const items: HTMLSpanElement[] = [];
	for (const c of text) {
		if (c === ' ') {
			span('sp', root);
			continue;
		}
		items.push(span('ch', root, c));
	}
	return { root, items };
}

/** Text split into words, each inside its own clip mask (for rise-from-the-edge reveals). */
export function words(parent: Element, text: string, cls?: string) {
	const root = span(cls, parent);
	const items: HTMLSpanElement[] = [];
	text.split(' ').forEach((w, i) => {
		if (i) root.appendChild(document.createTextNode(' '));
		const m = span('mask', root);
		items.push(span('w', m, w));
	});
	return { root, items };
}

/** Fit a single line into maxWidth by scaling its font size down (long titles). */
export function fit<T extends HTMLElement>(e: T, maxWidth: number): T {
	const w = e.scrollWidth;
	if (w > maxWidth) {
		const fs = parseFloat(getComputedStyle(e).fontSize);
		e.style.fontSize = Math.floor((fs * maxWidth) / w) + 'px';
	}
	return e;
}

export interface RiseOptions {
	stagger?: number;
	dur?: number;
	from?: string;
	ease?: Ease;
	fade?: boolean;
}

export function rise(tl: Timeline, items: Element[], at: number, o: RiseOptions = {}) {
	const { stagger = 0.07, dur = 0.9, from = '105%', ease = 'enter', fade = true } = o;
	items.forEach((it, i) => {
		const f = fade
			? [
					{ transform: `translateY(${from})`, opacity: 0 },
					{ transform: 'translateY(0)', opacity: 1 }
				]
			: [{ transform: `translateY(${from})` }, { transform: 'translateY(0)' }];
		tl.add(it, f, at + i * stagger, dur, ease);
	});
	return at + (items.length - 1) * stagger + dur;
}

export function fadeUp(
	tl: Timeline,
	e: Element,
	at: number,
	{ dur = 0.8, y = 18, ease = 'enter' as Ease } = {}
) {
	tl.add(
		e,
		[
			{ opacity: 0, transform: `translateY(${y}px)` },
			{ opacity: 1, transform: 'translateY(0)' }
		],
		at,
		dur,
		ease
	);
}

export function fadeOut(
	tl: Timeline,
	e: Element,
	at: number,
	{ dur = 0.45, y = -12, x = 0, scale = 1, ease = 'exit' as Ease } = {}
) {
	tl.add(
		e,
		[
			{ opacity: 1, transform: 'translate(0, 0) scale(1)' },
			{ opacity: 0, transform: `translate(${x}px, ${y}px) scale(${scale})` }
		],
		at,
		dur,
		ease
	);
}

/**
 * A light sweep across text: a window moving over a bright copy of the text that moves the
 * other way, so the copy stays registered. Two transforms, no paint per frame.
 */
export function glint(
	tl: Timeline,
	host: HTMLElement,
	at: number,
	dur = 0.9,
	{ band = 0.32, ease = 'inOut' as Ease } = {}
) {
	host.classList.add('glint-host');
	const w = host.offsetWidth;
	const bw = Math.max(80, w * band);
	const wrap = span('glint-wrap', host);
	const win = css(span('glint-band', wrap), { width: bw + 'px' });
	const copy = host.cloneNode(true) as HTMLElement;
	copy.querySelector('.glint-wrap')?.remove();
	copy.className = (host.className + ' glint-copy').replace('glint-host', '');
	copy.removeAttribute('style');
	css(copy, { width: w + 'px', position: 'absolute', margin: '0' });
	win.appendChild(copy);
	tl.add(
		win,
		[{ transform: `translateX(${-bw}px)` }, { transform: `translateX(${w}px)` }],
		at,
		dur,
		ease
	);
	tl.add(
		copy,
		[{ transform: `translateX(${bw}px)` }, { transform: `translateX(${-w}px)` }],
		at,
		dur,
		ease
	);
	tl.show(win, at, at + dur);
	return win;
}
