// A title's own picture and name on the screens that show it big: its scenes (the footage shots
// cut to the music and cycled, cover-cropped toward their focus, each slowly pushed) and its logo
// (sized by its aspect, made to read on the field it sits on), with its backdrop and its typed
// name as the fallbacks.

import type { Assets } from '../assets';
import { css, div, img } from '../core/dom';
import { framed, inset } from '../core/framing';
import { edge } from '../core/moves';
import { contrast, fromHsl, parseHex, toHsl, type Rgb } from '../core/palette';
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

/** A logo's colour reads on its field from 3:1 (display size); a redrawn one aims at 4.5:1. */
const READS = 3;
const AIM = 4.5;

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

const reading = new Map<string, boolean>();

/**
 * Whether the logo reads on `field` as it is: most of it, or its outline (the opaque pixels at
 * its edge), stands off the field, so a yellow logo outlined in black reads on night by its fill
 * and on paper by its outline. True when its pixels cannot be read.
 */
function readsOn(image: HTMLImageElement, url: string, field: string, w: number, h: number) {
	const key = `${url}|${w}x${h}|${field}`;
	const known = reading.get(key);
	if (known !== undefined) return known;
	const bg = (parseHex(field) ?? [5, 5, 5]) as Rgb;
	const got = pixels(image, w, h);
	let ok = true;
	if (got) {
		const px = got.data.data;
		const opaque = (i: number) => px[i * 4 + 3] >= 128;
		let solid = 0;
		let fine = 0;
		let rim = 0;
		let rimFine = 0;
		for (let y = 0; y < h; y++)
			for (let x = 0; x < w; x++) {
				const i = y * w + x;
				if (!opaque(i)) continue;
				solid++;
				const good = contrast([px[i * 4], px[i * 4 + 1], px[i * 4 + 2]], bg) >= READS;
				if (good) fine++;
				const atEdge =
					x === 0 ||
					y === 0 ||
					x === w - 1 ||
					y === h - 1 ||
					!opaque(i - 1) ||
					!opaque(i + 1) ||
					!opaque(i - w) ||
					!opaque(i + w);
				if (atEdge) {
					rim++;
					if (good) rimFine++;
				}
			}
		ok = !solid || fine / solid >= 0.5 || rimFine / Math.max(1, rim) >= 0.6;
	}
	reading.set(key, ok);
	return ok;
}

const redrawn = new Map<string, string>();

/**
 * The logo redrawn to read on `field`: every colour walks its lightness away from the field until
 * it reads (a dark red lifts to a brighter red), the neutral ones invert (a black logo made for
 * paper is white on night). A pure function of the image's pixels; the image as it is when they
 * cannot be read.
 */
function redrawFor(image: HTMLImageElement, url: string, field: string, w: number, h: number) {
	const key = `${url}|${w}x${h}|${field}`;
	const known = redrawn.get(key);
	if (known) return known;
	const bg = (parseHex(field) ?? [5, 5, 5]) as Rgb;
	const got = pixels(image, w, h);
	let result = url;
	if (got) {
		const px = got.data.data;
		const dark = bg[0] + bg[1] + bg[2] < 384;
		const walked = new Map<number, Rgb>();
		for (let i = 0; i < px.length; i += 4) {
			if (!px[i + 3]) continue;
			const k = (px[i] << 16) | (px[i + 1] << 8) | px[i + 2];
			let to = walked.get(k);
			if (!to) {
				to = walk([px[i], px[i + 1], px[i + 2]], bg, dark);
				walked.set(k, to);
			}
			[px[i], px[i + 1], px[i + 2]] = to;
		}
		got.g.putImageData(got.data, 0, 0);
		result = got.c.toDataURL('image/png');
	}
	redrawn.set(key, result);
	return result;
}

/** A colour moved in lightness away from `bg` until it reads at AIM; neutral colours invert. */
function walk(rgb: Rgb, bg: Rgb, dark: boolean): Rgb {
	if (contrast(rgb, bg) >= AIM) return rgb;
	const [h, s, l] = toHsl(rgb);
	if (s < 0.12) {
		const flipped = fromHsl([h, s, 1 - l]);
		if (contrast(flipped, bg) >= AIM) return flipped;
	}
	for (let step = 1; step <= 100; step++) {
		const to = fromHsl([h, s, Math.max(0, Math.min(1, l + (dark ? step : -step) / 100))]);
		if (contrast(to, bg) >= AIM) return to;
	}
	return dark ? [255, 255, 255] : [0, 0, 0];
}

export interface Plate {
	/** The plate's colour: a field the logo may read on where it does not on its own. */
	field: string;
	/** Its padding around the logo, px: [vertical, horizontal]. */
	pad: [number, number];
}

/**
 * The title's logo as an image (sized by `logoSize`) that reads on `field`, a hex colour (the
 * night or paper it sits on): as it is when it reads there; else, given a `plate`, as it is on a
 * plate of the plate's field when it reads on that (a yellow logo made for dark backgrounds, on
 * night inside a paper panel), smaller by the plate's padding; else redrawn for the field. Null
 * when the title has none or it did not load: the screen sets the name in type.
 */
export function logoImage(
	parent: Element,
	assets: Assets,
	logo: Logo | undefined,
	field: string,
	box: LogoBox,
	{ cls = 'title-logo', plate }: { cls?: string; plate?: Plate } = {}
): { el: HTMLImageElement; w: number; h: number; plate: boolean } | null {
	if (!logo?.url) return null;
	const image = assets.image(logo.url);
	if (!image || !image.naturalWidth || !image.naturalHeight) return null;
	const aspect = image.naturalWidth / image.naturalHeight;
	let { w, h } = logoSize(aspect, box);
	let src = logo.url;
	let plated = false;
	if (!readsOn(image, logo.url, field, w, h)) {
		if (plate && readsOn(image, logo.url, plate.field, w, h)) {
			const [py, px] = plate.pad;
			({ w, h } = logoSize(aspect, {
				width: box.width - 2 * px,
				height: box.height - 2 * py,
				area: box.area * 0.8
			}));
			plated = true;
		} else src = redrawFor(image, logo.url, field, w, h);
	}
	const el = img(cls, parent, src);
	css(el, { width: `${w}px`, height: `${h}px` });
	return { el, w, h, plate: plated };
}
