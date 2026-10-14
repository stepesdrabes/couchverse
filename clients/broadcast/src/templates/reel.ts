// A title's own picture and name on the screens that show it big: its scenes (the footage shots
// cut to the music and cycled, cover-cropped toward their focus, each slowly pushed) and its logo
// (sized by its aspect and never on a box: a screen picks the field the logo reads on, or puts it
// over its picture with a dark logo's colours lifted), with its backdrop and its typed name as the
// fallbacks.

import type { Assets } from '../assets';
import { css, div, img } from '../core/dom';
import { framed, inset } from '../core/framing';
import { lift, reading, reads, SEEN, type Reading } from '../core/logo';
import { edge } from '../core/moves';
import { parseHex, type Rgb } from '../core/palette';
import { FPS } from '../core/timeline';
import type { Crop, Logo, Title } from '../spec';
import type { TL } from './shared';

/** One cut of the scenes: a shot's frames from `from` on, over [a, b). */
export interface SceneCut {
	a: number;
	b: number;
	/** Which of the title's footage shots. */
	shot: number;
	/** The shot's first frame shown. */
	from: number;
}

/**
 * Cuts over [a, b) for shots of `lengths` frames: each as near `want` seconds as the beats allow
 * (a cut lands on the beat nearest that, at least `min` in and no further than its shot has
 * frames), the shots in order and cycled. A shot used again picks up where it stopped, and from
 * its start once too little is left, so a short reel repeats as little as it can; the last cut
 * runs to `b`, holding its shot's last frame for at most a moment. `first` and `cursor` carry on
 * from cuts made before `a` (the shot to start with, the frames each has used).
 */
export function planCuts(
	lengths: number[],
	a: number,
	b: number,
	beats: number[],
	{ min = 1.2, want = 2.4, first = 0, cursor: used = lengths.map(() => 0) } = {}
): SceneCut[] {
	const n = lengths.length;
	if (!n) return [];
	const cursor = [...used];
	const out: SceneCut[] = [];
	let t = a;
	for (let k = first; t < b - 1e-6; k++) {
		const s = k % n;
		if ((lengths[s] - cursor[s]) / FPS < Math.min(min, lengths[s] / FPS)) cursor[s] = 0;
		const left = (lengths[s] - cursor[s]) / FPS;
		let end: number;
		if (b - t <= left + 0.4) end = b;
		else {
			const aim = t + Math.min(want, left);
			const lo = t + Math.min(min, left);
			const on = beats.filter((x) => x >= lo && x <= t + left);
			const near = on.length
				? on.reduce((p, x) => (Math.abs(x - aim) < Math.abs(p - aim) ? x : p))
				: aim;
			end = Math.max(t + 2 / FPS, edge(near));
			// never leave a sliver after it
			if (b - end < min) end = b - t <= left + 0.4 ? b : Math.max(t + 2 / FPS, edge(b - min));
		}
		out.push({ a: t, b: end, shot: s, from: cursor[s] });
		cursor[s] += Math.round((end - t) * FPS);
		t = end;
	}
	return out;
}

export interface SceneOptions {
	/** Seconds each cut dissolves in over, centred on the cut (0: hard cuts). */
	dissolve?: number;
	/** Each cut's push, a scale from and to (linear over the cut). */
	push?: [number, number];
	/** The backdrop's framing when there is no footage: its crop from and to, over [a, b). */
	still?: [Crop, Crop];
}

/**
 * The title's scenes in `parent` over `cuts`, or without footage its backdrop pushed slowly over
 * [a, b). Each cut is a box of its own, shown only while it plays, so only the frame on screen
 * decodes.
 */
export function scenes(
	tl: TL,
	parent: Element,
	title: Title,
	assets: Assets,
	cuts: SceneCut[],
	[a, b]: [number, number],
	{ dissolve = 0, push = [1, 1.04], still }: SceneOptions = {}
): HTMLElement[] {
	const shots = title.footage ?? [];
	if (!shots.length || !cuts.length) {
		const s = framed(parent, assets.picture(title.backdrop));
		const [from, to] = still ?? [inset([0, 0, 1, 1], 0.94), inset([0, 0, 1, 1], 0.86)];
		tl.add(s.img, [{ transform: s.tf(from) }, { transform: s.tf(to) }], a, b - a, 'linear');
		return [s.el];
	}
	return cuts.map((c, i) => {
		const shot = shots[c.shot];
		const fade = i > 0 ? dissolve : 0;
		const inT = c.a - fade / 2;
		const outT = c.b + (i < cuts.length - 1 ? dissolve / 2 : 0);
		const box = div('shot footage', parent);
		const frame = img(null, box);
		if (shot.focus) {
			const [x, y, w, h] = shot.focus;
			const at = `${((x + w / 2) * 100).toFixed(1)}% ${((y + h / 2) * 100).toFixed(1)}%`;
			css(frame, { objectPosition: at, transformOrigin: at });
		}
		tl.footage(frame, shot.frames.slice(c.from), inT);
		tl.add(
			frame,
			[{ transform: `scale(${push[0]})` }, { transform: `scale(${push[1]})` }],
			inT,
			outT - inT,
			'linear'
		);
		tl.show(box, inT, outT);
		if (fade) tl.add(box, [{ opacity: 0 }, { opacity: 1 }], inT, fade, 'sine');
		return box;
	});
}

/** The footage shots' lengths in frames (none without footage). */
export const shotLengths = (title: Title) => (title.footage ?? []).map((s) => s.frames.length);

// --- the logo ----------------------------------------------------------------------------------

export interface LogoBox {
	/** The widest and the tallest it may be, px. */
	width: number;
	height: number;
	/** The area it takes when neither limit binds, px², so logos of every shape weigh alike. */
	area: number;
}

/** A logo's size in `box`: its aspect kept, the box's area whatever its shape, inside the box. */
export function logoSize(aspect: number, box: LogoBox) {
	let w = Math.sqrt(box.area * aspect);
	let h = w / aspect;
	if (w > box.width) [w, h] = [box.width, box.width / aspect];
	if (h > box.height) [w, h] = [box.height * aspect, box.height];
	return { w: Math.round(w), h: Math.round(h) };
}

/** The logo's pixels drawn at `w` x `h`; null when they cannot be read (another origin). */
function pixels(image: HTMLImageElement, w: number, h: number) {
	const c = document.createElement('canvas');
	c.width = w;
	c.height = h;
	const g = c.getContext('2d', { willReadFrequently: true });
	if (!g) return null;
	try {
		g.drawImage(image, 0, 0, w, h);
		return { c, g, data: g.getImageData(0, 0, w, h) };
	} catch {
		return null;
	}
}

/** A logo that loaded, with its size in `box`: null when the title has none or it did not. */
function loaded(assets: Assets, logo: Logo | undefined, box: LogoBox) {
	if (!logo?.url) return null;
	const image = assets.image(logo.url);
	if (!image || !image.naturalWidth || !image.naturalHeight) return null;
	return { image, url: logo.url, ...logoSize(image.naturalWidth / image.naturalHeight, box) };
}

const readings = new Map<string, Reading | null>();

/** How the logo reads on `field` at its display size; null when its pixels cannot be read. */
function readingOn(image: HTMLImageElement, url: string, field: string, w: number, h: number) {
	const key = `${url}|${w}x${h}|${field}`;
	if (readings.has(key)) return readings.get(key) ?? null;
	const got = pixels(image, w, h);
	const r = got ? reading(got.data, (parseHex(field) ?? [5, 5, 5]) as Rgb) : null;
	readings.set(key, r);
	return r;
}

/**
 * The index of the first of `fields` (hex colours, in the screen's order of preference) the title's
 * logo reads on as it is, else -1. A logo never sits on a box: the screen sets its whole field to
 * the colour it reads on, or puts the logo over its picture. A logo whose pixels cannot be read
 * reads everywhere. Null when there is no logo to place (the title has none, or it did not load):
 * the screen sets the name in type on its own field.
 */
export function readsOnField(
	assets: Assets,
	logo: Logo | undefined,
	fields: string[],
	box: LogoBox
): number | null {
	const l = loaded(assets, logo, box);
	if (!l) return null;
	return fields.findIndex((f) => {
		const r = readingOn(l.image, l.url, f, l.w, l.h);
		return !r || reads(r);
	});
}

const drawn = new Map<string, string | null>();

/**
 * What to draw of the logo on the dark `field`: its own file when it reads there; else, when it
 * has colour to lift, the logo with its colours brighter (hue kept, so a dark red stays Dexter's
 * red); else, when a fair share of it still shows, as it is (a metal logo's dark parts); else
 * null, and the screen sets the name in type.
 */
function drawnOn(image: HTMLImageElement, url: string, field: string, w: number, h: number) {
	const key = `${url}|${w}x${h}|${field}`;
	const known = drawn.get(key);
	if (known !== undefined) return known;
	const got = pixels(image, w, h);
	const bg = (parseHex(field) ?? [5, 5, 5]) as Rgb;
	let result: string | null = url;
	if (got) {
		const before = reading(got.data, bg);
		if (!reads(before)) {
			if (lift(got.data, bg)) {
				got.g.putImageData(got.data, 0, 0);
				result = got.c.toDataURL('image/png');
			} else if (before.fill < SEEN) result = null;
		}
	}
	drawn.set(key, result);
	return result;
}

/**
 * The title's logo for `field`, the hex colour it sits on (night, or the night a grade makes of
 * its picture), sized by `logoSize`; null when the title has none, it did not load or it would not
 * show there: the screen sets the name in type.
 */
export function logoFor(assets: Assets, logo: Logo | undefined, field: string, box: LogoBox) {
	const l = loaded(assets, logo, box);
	if (!l) return null;
	const src = drawnOn(l.image, l.url, field, l.w, l.h);
	return src ? { src, w: l.w, h: l.h } : null;
}

/** `logoFor` as an image in `parent`, drawn in `cls`. */
export function logoImage(
	parent: Element,
	assets: Assets,
	logo: Logo | undefined,
	field: string,
	box: LogoBox,
	cls = 'title-logo'
): { el: HTMLImageElement; w: number; h: number } | null {
	const l = logoFor(assets, logo, field, box);
	if (!l) return null;
	const el = img(cls, parent, l.src);
	css(el, { width: `${l.w}px`, height: `${l.h}px` });
	return { el, w: l.w, h: l.h };
}
