// Easing, keyframe tracks and colour mixing for the morph ident. Every value here is a pure
// function of t, so the picture can be sampled at any sub-frame time (the motion blur does).

import { bezierPoints, cubicBezierAt, type Ease } from '../../../core/motion';

export type Bezier = [number, number, number, number];
export type Curve = Ease | Bezier | ((p: number) => number);

/** The curves the morphs use, beyond the package's named ones. */
export const CURVE = {
	/** The morphs' main move: a soft start, a long gentle arrival. */
	glide: [0.6, 0, 0.16, 1] as Bezier,
	/** Even both ways, for drifts and colour changes. */
	sine: [0.37, 0, 0.63, 1] as Bezier,
	inOut: [0.65, 0, 0.35, 1] as Bezier,
	/** A fast start that settles for a long time (type, arrivals). */
	expo: [0.16, 1, 0.3, 1] as Bezier,
	out: [0.25, 0.8, 0.3, 1] as Bezier,
	/** Accelerating away (the ground falling). */
	in: [0.55, 0, 0.85, 0.35] as Bezier
};

const points = new Map<string, Bezier>();

export const clamp01 = (v: number) => (v < 0 ? 0 : v > 1 ? 1 : v);
export const lerp = (a: number, b: number, k: number) => a + (b - a) * k;
/** 0..1 across [a, b]. */
export const span = (t: number, a: number, b: number) => clamp01((t - a) / (b - a));

export function ease(c: Curve, p: number): number {
	const x = clamp01(p);
	if (typeof c === 'function') return c(x);
	if (Array.isArray(c)) return cubicBezierAt(c, x);
	let pts = points.get(c);
	if (!pts) points.set(c, (pts = bezierPoints(c)));
	return cubicBezierAt(pts, x);
}

/** A key: at t the value is v; the curve shapes the way to the next key. */
export type Key<T> = [t: number, v: T, c?: Curve];

export function track<T>(keys: Key<T>[], mix: (a: T, b: T, k: number) => T): (t: number) => T {
	return (t) => {
		if (t <= keys[0][0]) return keys[0][1];
		for (let i = 1; i < keys.length; i++) {
			if (t < keys[i][0]) {
				const [t0, v0, c = CURVE.inOut] = keys[i - 1];
				const [t1, v1] = keys[i];
				return mix(v0, v1, ease(c, (t - t0) / (t1 - t0)));
			}
		}
		return keys[keys.length - 1][1];
	};
}

export const num = (keys: Key<number>[]) => track(keys, lerp);

// --- colour: mixed in OKLab, so red to black passes through deep reds, not grey -----------

export type Lab = [number, number, number];

const toLinear = (c: number) => (c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4);
const toGamma = (c: number) => (c <= 0.0031308 ? 12.92 * c : 1.055 * c ** (1 / 2.4) - 0.055);

export function hexLab(hex: string): Lab {
	const n = parseInt(hex.slice(1), 16);
	const r = toLinear(((n >> 16) & 255) / 255);
	const g = toLinear(((n >> 8) & 255) / 255);
	const b = toLinear((n & 255) / 255);
	const l = Math.cbrt(0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b);
	const m = Math.cbrt(0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b);
	const s = Math.cbrt(0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b);
	return [
		0.2104542553 * l + 0.793617785 * m - 0.0040720468 * s,
		1.9779984951 * l - 2.428592205 * m + 0.4505937099 * s,
		0.0259040371 * l + 0.7827717662 * m - 0.808675766 * s
	];
}

/** OKLab to sRGB 0..255 floats (unrounded: the renderer dithers). */
export function labRgb([L, a, b]: Lab): [number, number, number] {
	const l = (L + 0.3963377774 * a + 0.2158037573 * b) ** 3;
	const m = (L - 0.1055613458 * a - 0.0638541728 * b) ** 3;
	const s = (L - 0.0894841775 * a - 1.291485548 * b) ** 3;
	const r = 4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s;
	const g = -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s;
	const bb = -0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s;
	return [r, g, bb].map((c) => 255 * toGamma(Math.max(0, Math.min(1, c)))) as [
		number,
		number,
		number
	];
}

export const mixLab = (x: Lab, y: Lab, k: number): Lab => [
	lerp(x[0], y[0], k),
	lerp(x[1], y[1], k),
	lerp(x[2], y[2], k)
];

export function css(lab: Lab, alpha = 1): string {
	const [r, g, b] = labRgb(lab).map((v) => Math.round(v));
	return alpha >= 1 ? `rgb(${r} ${g} ${b})` : `rgb(${r} ${g} ${b} / ${alpha.toFixed(4)})`;
}

/** A colour track: keys in hex, mixed in OKLab. */
export function color(keys: Key<string>[]): (t: number) => Lab {
	return track(
		keys.map(([t, v, c]): Key<Lab> => [t, hexLab(v), c]),
		mixLab
	);
}
