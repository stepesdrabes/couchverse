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
/** A lift keeps colour: only pixels at least this saturated move, and never past this lightness. */
const CHROMA = 0.2;
const CEILING = 0.78;

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

/** A colour's lightness lifted until it reads on `bg` at the aim, its hue and saturation kept. */
function lifted(rgb: Rgb, bg: Rgb): Rgb {
	if (contrast(rgb, bg) >= AIM) return rgb;
	const [h, s, l] = toHsl(rgb);
	let to = rgb;
	for (let step = 1; l + step / 100 <= CEILING; step++) {
		to = fromHsl([h, s, l + step / 100]);
		if (contrast(to, bg) >= AIM) return to;
	}
	return to;
}

/**
 * The coloured pixels lifted in lightness until they read on the dark `bg` (a dark red becomes a
 * brighter red), in place. Greys, blacks and whites stay as drawn: a logo made for paper is not
 * inverted, and an outline stays an outline. True when any pixel changed.
 */
export function lift(px: Pixels, bg: Rgb): boolean {
	const moved = new Map<number, Rgb>();
	let changed = false;
	for (let i = 0; i < px.width * px.height; i++) {
		if (!px.data[i * 4 + 3]) continue;
		const c = colour(px, i);
		if (toHsl(c)[1] < CHROMA) continue;
		const key = (c[0] << 16) | (c[1] << 8) | c[2];
		let to = moved.get(key);
		if (!to) {
			to = lifted(c, bg);
			moved.set(key, to);
		}
		if (to[0] !== c[0] || to[1] !== c[1] || to[2] !== c[2]) {
			[px.data[i * 4], px.data[i * 4 + 1], px.data[i * 4 + 2]] = to;
			changed = true;
		}
	}
	return changed;
}
