// Whether a title's logo reads on a field, and the one change a logo may take to read on night.
// Pure over the logo's pixels, so the screens place a logo the same way on every machine.

import { contrast, fromHsl, toHsl, type Rgb } from './palette';

/** A logo's pixels as `getImageData` gives them (`ImageData` fits): RGBA, row by row. */
export interface Pixels {
	data: Uint8ClampedArray;
	width: number;
	height: number;
}

/** A colour reads on its field from 3:1 (display size); a lifted one aims at 4.5:1. */
export const READS = 3;
const AIM = 4.5;
/** The share of a logo's pixels, or of its outline, that must read for the logo to read. */
const FILL = 0.55;
const RIM = 0.6;
/** Below this share of readable pixels a logo is not there at all (black on night). */
export const SEEN = 0.25;
/**
 * A lift keeps colour: only pixels whose channels are at least this far apart (max less min, of
 * 255) are coloured, and none is lifted past this lightness. Chroma, not HSL saturation: a
 * near-black pixel with a cool tint, or a metal logo's brown shading, is saturated in HSL and
 * grey to the eye. The lift is only tried on a logo that is mostly colour, as a dark red wordmark
 * is.
 */
const CHROMA = 48;
const COLOURED = 0.5;
const CEILING = 0.78;
/** The share of the coloured pixels, darkest first, that the lift brings to the aim. */
const DARKEST = 0.1;

export interface Reading {
	/** The share of its opaque pixels that read on the field. */
	fill: number;
	/** The same for its outline, the opaque pixels at its edge. */
	rim: number;
}

const opaque = (px: Pixels, i: number) => px.data[i * 4 + 3] >= 128;
const colour = (px: Pixels, i: number): Rgb => [
	px.data[i * 4],
	px.data[i * 4 + 1],
	px.data[i * 4 + 2]
];

/** How a logo's pixels read on `bg`. */
export function reading(px: Pixels, bg: Rgb): Reading {
	const { width: w, height: h } = px;
	let solid = 0;
	let fine = 0;
	let rim = 0;
	let rimFine = 0;
	for (let y = 0; y < h; y++)
		for (let x = 0; x < w; x++) {
			const i = y * w + x;
			if (!opaque(px, i)) continue;
			solid++;
			const good = contrast(colour(px, i), bg) >= READS;
			if (good) fine++;
			const atEdge =
				x === 0 ||
				y === 0 ||
				x === w - 1 ||
				y === h - 1 ||
				!opaque(px, i - 1) ||
				!opaque(px, i + 1) ||
				!opaque(px, i - w) ||
				!opaque(px, i + w);
			if (atEdge) {
				rim++;
				if (good) rimFine++;
			}
		}
	if (!solid) return { fill: 1, rim: 1 };
	return { fill: fine / solid, rim: rimFine / Math.max(1, rim) };
}

/**
 * Whether the logo reads: most of it does, or its outline does (a yellow logo outlined in black
 * reads on paper by its outline, on night by its fill).
 */
export const reads = (r: Reading) => r.fill >= FILL || r.rim >= RIM;

const chroma = ([r, g, b]: Rgb) => Math.max(r, g, b) - Math.min(r, g, b);

/** The lightness of the darkest tenth of `colours`: the pixels the lift is measured by. */
function darkest(colours: Rgb[]): Rgb {
	return colours.map((c) => ({ c, l: toHsl(c)[2] })).sort((a, b) => a.l - b.l)[
		Math.floor((colours.length - 1) * DARKEST)
	].c;
}

/**
 * The lightness the darkest tenth of the coloured pixels must reach to read on `bg` at the aim,
 * its hue and saturation kept; null when it needs no lifting.
 */
function target(colours: Rgb[], bg: Rgb): number | null {
	const dark = darkest(colours);
	if (contrast(dark, bg) >= AIM) return null;
	const [h, s, l] = toHsl(dark);
	for (let step = 1; l + step / 100 <= CEILING; step++)
		if (contrast(fromHsl([h, s, l + step / 100]), bg) >= AIM) return l + step / 100;
	return CEILING;
}

/**
 * The coloured pixels lifted in lightness until they read on the dark `bg` (a dark red becomes a
 * brighter red), in place, and only in a logo that is mostly colour. Every coloured pixel moves
 * toward the ceiling by the same share, so the shading a logo has stays in order and in
 * proportion rather than all of it landing on one colour; hue and saturation are kept. Greys,
 * blacks, whites and the faintly tinted stay as drawn: a logo made for paper is not inverted, and
 * an outline stays an outline. True when any pixel changed.
 */
export function lift(px: Pixels, bg: Rgb): boolean {
	const colours: Rgb[] = [];
	let opaques = 0;
	for (let i = 0; i < px.width * px.height; i++) {
		if (!px.data[i * 4 + 3]) continue;
		opaques++;
		const c = colour(px, i);
		if (chroma(c) >= CHROMA) colours.push(c);
	}
	if (!colours.length || colours.length < COLOURED * opaques) return false;
	const to = target(colours, bg);
	if (to === null) return false;
	// the share of the way to the ceiling is the darkest tenth's, for every coloured pixel
	const from = toHsl(darkest(colours))[2];
	const share = (to - from) / Math.max(1e-6, CEILING - from);
	const moved = new Map<number, Rgb>();
	let changed = false;
	for (let i = 0; i < px.width * px.height; i++) {
		if (!px.data[i * 4 + 3]) continue;
		const c = colour(px, i);
		if (chroma(c) < CHROMA) continue;
		const key = (c[0] << 16) | (c[1] << 8) | c[2];
		let next = moved.get(key);
		if (!next) {
			const [h, s, l] = toHsl(c);
			next = fromHsl([h, s, l < CEILING ? l + (CEILING - l) * share : l]);
			moved.set(key, next);
		}
		if (next[0] !== c[0] || next[1] !== c[1] || next[2] !== c[2]) {
			[px.data[i * 4], px.data[i * 4 + 1], px.data[i * 4 + 2]] = next;
			changed = true;
		}
	}
	return changed;
}

/** What to draw of a logo on a dark field: its own pixels, a lift of them, or the name in type. */
export type Draw = 'as is' | 'lifted' | 'name';

/**
 * How a logo is drawn on the dark `bg`: as it is when it reads there; else lifted (in place) when
 * the lift makes it read; else as it is while a quarter of it still shows (a metal logo's dark
 * parts melt into the night, the rest carries it); else the name in type.
 */
export function onNight(px: Pixels, bg: Rgb): Draw {
	const before = reading(px, bg);
	if (reads(before)) return 'as is';
	// (an ImageData's size is not an own property: spreading it would lose it)
	const trial = { width: px.width, height: px.height, data: new Uint8ClampedArray(px.data) };
	if (lift(trial, bg) && reads(reading(trial, bg))) {
		px.data.set(trial.data);
		return 'lifted';
	}
	return before.fill >= SEEN ? 'as is' : 'name';
}
