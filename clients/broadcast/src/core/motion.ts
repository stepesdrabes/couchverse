// Motion: the design tokens' curves and springs, plus the broadcast additions. Springs are
// sampled into keyframes, so a spring is an ordinary seekable animation.

import { curves, springs } from '../generated/tokens';

const bezier = (p: readonly number[]) => `cubic-bezier(${p.join(', ')})`;

export const EASE = {
	standard: bezier(curves.standard.bezier),
	enter: bezier(curves.enter.bezier),
	exit: 'cubic-bezier(0.7, 0, 0.84, 0)',
	inOut: 'cubic-bezier(0.65, 0, 0.35, 1)',
	sine: 'cubic-bezier(0.37, 0, 0.63, 1)',
	out: 'cubic-bezier(0.22, 1, 0.36, 1)',
	in: 'cubic-bezier(0.55, 0, 1, 0.45)',
	snap: 'cubic-bezier(0.05, 0.9, 0.1, 1)',
	pop: 'cubic-bezier(0.34, 1.56, 0.64, 1)',
	linear: 'linear'
} as const;

export type EaseName = keyof typeof EASE;

/** A named curve or any CSS easing function. */
export type Ease = EaseName | (string & {});

export const easing = (e: Ease): string => (e in EASE ? EASE[e as EaseName] : e);

export type SpringConfig = { readonly response: number; readonly damping: number };

export const SPRING: Record<keyof typeof springs, SpringConfig> = springs;

/** A damped spring from 0 to 1 sampled as progress values, plus its settle time (seconds). */
export function spring({ response, damping }: SpringConfig, samples = 28) {
	const w0 = (2 * Math.PI) / response;
	const zeta = damping;
	const wd = w0 * Math.sqrt(1 - zeta * zeta);
	const settle = Math.log(1000) / (zeta * w0);
	const values: number[] = [];
	for (let i = 0; i <= samples; i++) {
		const t = (i / samples) * settle;
		const env = Math.exp(-zeta * w0 * t);
		values.push(1 - env * (Math.cos(wd * t) + ((zeta * w0) / wd) * Math.sin(wd * t)));
	}
	values[samples] = 1;
	return { values, duration: settle };
}

/** A spring from `from` to `to` as transform keyframes, plus its duration. */
export function springFrames(
	from: number,
	to: number,
	cfg: SpringConfig = SPRING.snappy,
	map: (v: number) => string = (v) => `scale(${v})`
) {
	const s = spring(cfg);
	return {
		frames: s.values.map((v) => ({ transform: map(from + (to - from) * v) })),
		dur: s.duration
	};
}

/**
 * The progress of a cubic-bezier easing at x (0..1), solved like the browser does, so a
 * template can compute a value it also animates (the squeeze's key rectangle).
 */
export function cubicBezierAt(p: readonly [number, number, number, number], x: number): number {
	const [x1, y1, x2, y2] = p;
	if (x <= 0) return 0;
	if (x >= 1) return 1;
	const cx = 3 * x1;
	const bx = 3 * (x2 - x1) - cx;
	const ax = 1 - cx - bx;
	const cy = 3 * y1;
	const by = 3 * (y2 - y1) - cy;
	const ay = 1 - cy - by;
	const sx = (t: number) => ((ax * t + bx) * t + cx) * t;
	const dx = (t: number) => (3 * ax * t + 2 * bx) * t + cx;
	let t = x;
	for (let i = 0; i < 8; i++) {
		const err = sx(t) - x;
		if (Math.abs(err) < 1e-7) break;
		const d = dx(t);
		if (Math.abs(d) < 1e-6) break;
		t -= err / d;
	}
	if (t < 0 || t > 1 || Math.abs(sx(t) - x) > 1e-5) {
		// bisection when Newton wanders off (flat ends)
		let lo = 0;
		let hi = 1;
		t = x;
		for (let i = 0; i < 40; i++) {
			if (sx(t) < x) lo = t;
			else hi = t;
			t = (lo + hi) / 2;
		}
	}
	return ((ay * t + by) * t + cy) * t;
}

/** The control points of a cubic-bezier easing (`linear` is the identity). */
export function bezierPoints(e: Ease): [number, number, number, number] {
	const css = easing(e);
	if (css === 'linear') return [0, 0, 1, 1];
	const m = /^cubic-bezier\(([^)]+)\)$/.exec(css);
	if (!m) throw new Error(`not a cubic-bezier easing: ${css}`);
	const p = m[1].split(',').map(Number);
	return [p[0], p[1], p[2], p[3]];
}
