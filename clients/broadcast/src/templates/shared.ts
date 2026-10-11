// What templates share: the context they build in and small composites (glows, grain, shake).

import type { Assets } from '../assets';
import { grainTexture } from '../assets';
import type { Kit } from '../audio/kit';
import { css, div } from '../core/dom';
import { palette, pickAccent, type Palette } from '../core/palette';
import { rng } from '../core/rng';
import { Timeline } from '../core/timeline';
import type { Title } from '../spec';

export type TL = Timeline;

export const timeline = (root: HTMLElement, duration: number): TL => new Timeline(root, duration);

export interface Ctx {
	/** The piece's layer on the stage (1920x1080). */
	root: HTMLElement;
	sound: Kit;
	assets: Assets;
}

export interface Template<S> {
	/** Bumped on every change to the template's picture or sound: part of a clip's cache key. */
	version: number;
	build: (ctx: Ctx, spec: S) => TL;
}

export function titlePalette(t: Title): Palette {
	return palette(pickAccent(t.accents.backdrop, t.accents.poster));
}

/** A soft radial light (GlowBackdrop without the blur: the gradient alone is soft enough). */
export function glow(
	parent: Element,
	kind: 'accent' | 'violet' | 'teal' | 'red',
	x: number,
	y: number,
	r: number
) {
	return css(div(`glow ${kind}`, parent), {
		left: x - r + 'px',
		top: y - r + 'px',
		width: 2 * r + 'px',
		height: 2 * r + 'px'
	});
}

/** A short decaying jitter on a wrapper (for hits). */
export function shake(
	tl: TL,
	target: Element,
	at: number,
	{ dur = 0.35, amp = 14, seed = 1 } = {}
) {
	const r = rng(seed);
	const n = 9;
	const frames: Keyframe[] = [];
	for (let i = 0; i <= n; i++) {
		const k = 1 - i / n;
		const x = i === n ? 0 : (r() * 2 - 1) * amp * k;
		const y = i === n ? 0 : (r() * 2 - 1) * amp * k;
		frames.push({ transform: `translate(${x.toFixed(1)}px, ${y.toFixed(1)}px)` });
	}
	tl.add(target, frames, at, dur, 'linear');
}

/** Film grain: a noise tile jumped to a new offset in steps (composited, no paint). */
export function grain(tl: TL, parent: Element, opacity: number, seed = 5) {
	const g = css(div('grain', parent), { opacity: String(opacity) });
	g.style.backgroundImage = `url(${grainTexture()})`;
	const r = rng(seed);
	const n = 12;
	const frames: Keyframe[] = [];
	for (let i = 0; i < n; i++)
		frames.push({ transform: `translate(${Math.round(r() * 256)}px, ${Math.round(r() * 256)}px)` });
	tl.add(g, frames, 0, 0.5, `steps(${n}, jump-none)`, {
		iterations: Math.ceil(tl.duration / 0.5) + 1
	});
	return g;
}

/** A black layer fading in at the end, so every piece hands over on black. */
export function outToBlack(tl: TL, parent: Element, dur = 0.75) {
	const blk = css(div('fill', parent), { background: '#000' });
	tl.add(blk, [{ opacity: 0 }, { opacity: 1 }], tl.duration - dur, dur, 'sine');
	return blk;
}

/** `#rrggbb` as the `r g b` triple CSS colour functions take. */
export function rgbOf(hex: string) {
	return (hex.match(/\w\w/g) ?? ['0', '0', '0']).map((h) => parseInt(h, 16)).join(' ');
}

export const crop = <T>(value: T | undefined, fallback: T): T => value ?? fallback;
