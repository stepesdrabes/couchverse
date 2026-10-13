// Light on surfaces. A colour is the paint (linear albedo) under two lights: a warm white key
// (the television, the sun) and a red ambient (the room's own bounce, the sky). Through a soft
// shoulder per channel, a pale wall in strong light burns to warm white and falls off through red
// into black; dimming the key leaves the red, never grey.

import type { RGB, Stop } from './art';

const toSrgb = (v: number) => (v <= 0.0031308 ? v * 12.92 : 1.055 * v ** (1 / 2.4) - 0.055);
const toLinear = (v: number) => (v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4);

/** An sRGB hex colour in linear light. */
export function linearOf(hex: string): RGB {
	const n = parseInt(hex.slice(1), 16);
	return [
		toLinear(((n >> 16) & 255) / 255),
		toLinear(((n >> 8) & 255) / 255),
		toLinear((n & 255) / 255)
	];
}

export const KEY = linearOf('#fff2ea');
export const AMBIENT = linearOf('#e01414');

/**
 * The colour of paint `albedo` under `key` of the white light and `amb` of the red. `warm` turns
 * the key toward the red as a lamp does when it is dimmed, so a dimming room never goes grey.
 */
export function lit(albedo: RGB, key: number, amb: number, warm = 0): RGB {
	return albedo.map((a, c) => {
		const k = KEY[c] + (AMBIENT[c] / AMBIENT[0] - KEY[c]) * warm;
		const x = a * (key * k + amb * AMBIENT[c]);
		// identity at the bottom, a smooth shoulder into white at the top
		const y = x < 0.6 ? x : 0.6 + 0.4 * (1 - Math.exp(-(x - 0.6) / 0.4));
		return Math.round(toSrgb(Math.min(1, y)) * 255 * 100) / 100;
	}) as RGB;
}

/**
 * Gradient stops for a pool of light: the white key falls off fast (`key` profile), the red
 * bounce it raises falls off slower (`amb` profile), so the pool goes white, red, then dark.
 */
export function falloff(
	albedo: RGB,
	key: number,
	amb: number,
	keyProfile: (r: number) => number = core,
	ambProfile: (r: number) => number = glow,
	warm = 0,
	n = 20
): Stop[] {
	const out: Stop[] = [];
	for (let k = 0; k <= n; k++) {
		const r = k / n;
		// the light reddens toward its edge, where the room's bounce colours it
		const edge = Math.min(1, warm + (1 - warm) * Math.min(1, r * 1.4));
		out.push([r, lit(albedo, key * keyProfile(r), amb * ambProfile(r), edge)]);
	}
	return out;
}

/** The key's pool: a bright core with a soft shoulder. */
export const core = (r: number) => Math.exp(-11 * r * r);
/** The bounce around it: wide, gone at the edge. */
export const glow = (r: number) => Math.exp(-2.2 * r * r) * (1 - r * r * r);
/** A soft pool: full in the middle, gone at the edge. */
export const pool = (r: number) => Math.exp(-3.2 * r * r) * (1 - r * r * r);
