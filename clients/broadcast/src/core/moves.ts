// Whole-piece animations built from lists. `motion` holds a value between moves (a move of 0 s
// is a hard cut) and `windows` shows an element over a union of time windows; each is one
// animation per element and property for the whole timeline, so cues never fight. Their times
// sit on frame edges: half a frame before the frame nearest the musical time, so float error
// can never move a cut to the neighbouring frame.

import { bezierPoints, cubicBezierAt, easing, type Ease } from './motion';
import { FPS, type Timeline } from './timeline';

export interface Move<T> {
	at: number;
	/** Seconds; 0 (the default) is a hard cut. */
	dur?: number;
	to: T;
	ease?: Ease;
}

/** The frame edge before the frame nearest t: a cut placed there shows from that frame on. */
export const edge = (t: number) => (Math.round(t * FPS) - 0.5) / FPS;

interface Key<T> {
	t: number;
	v: T;
	/** The easing from this key to the next. */
	ease: Ease;
}

/** The keys `motion` animates through, shared with `valueAt` so both agree on every frame. */
function keys<T>(v0: T, moves: Move<T>[], D: number): Key<T>[] {
	const out: Key<T>[] = [{ t: 0, v: v0, ease: 'linear' }];
	let cur = v0;
	for (const mv of [...moves].sort((a, b) => a.at - b.at)) {
		const a = Math.max(0, edge(mv.at));
		const z = a + Math.max(0.001, mv.dur ?? 0);
		out.push({ t: a, v: cur, ease: mv.ease ?? 'linear' });
		out.push({ t: Math.min(D, z), v: mv.to, ease: 'linear' });
		cur = mv.to;
	}
	out.push({ t: D, v: cur, ease: 'linear' });
	for (let i = 1; i < out.length; i++) out[i].t = Math.min(D, Math.max(out[i].t, out[i - 1].t));
	return out;
}

/** Animate `target` through `moves` from `v0`; `fmt` turns a value into a keyframe. */
export function motion<T>(
	tl: Timeline,
	target: Element,
	v0: T,
	moves: Move<T>[],
	fmt: (v: T) => Keyframe
) {
	const D = tl.duration;
	const frames = keys(v0, moves, D).map((k) => ({
		offset: k.t / D,
		easing: easing(k.ease),
		...fmt(k.v)
	}));
	return tl.add(target, frames, 0, D, 'linear');
}

/** The value `motion` shows at t, for what is computed rather than animated (a path). */
export function valueAt<T>(
	v0: T,
	moves: Move<T>[],
	D: number,
	t: number,
	mix: (a: T, b: T, k: number) => T
): T {
	const ks = keys(v0, moves, D);
	for (let i = ks.length - 1; i >= 0; i--) {
		const k = ks[i];
		if (t < k.t) continue;
		const next = ks[i + 1];
		if (!next || next.t <= k.t) return k.v;
		const p = cubicBezierAt(bezierPoints(k.ease), (t - k.t) / (next.t - k.t));
		return mix(k.v, next.v, p);
	}
	return v0;
}

/** Visible over the union of `list`'s [from, to) windows, as hard opacity steps. */
export function windows(tl: Timeline, target: Element, list: [number, number][]) {
	const D = tl.duration;
	const ws = list
		.map(([a, b]): [number, number] => [edge(a), edge(b)])
		.filter(([a, b]) => b > a)
		.sort((x, y) => x[0] - y[0]);
	const merged: [number, number][] = [];
	for (const [a, b] of ws) {
		const last = merged[merged.length - 1];
		if (last && a <= last[1]) last[1] = Math.max(last[1], b);
		else merged.push([a, b]);
	}
	const at = (t: number) => Math.min(1, Math.max(0, t / D));
	const on0 = merged.length > 0 && merged[0][0] <= 0;
	const frames: Keyframe[] = [{ offset: 0, opacity: on0 ? 1 : 0 }];
	for (const [a, b] of merged) {
		if (a > 0) frames.push({ offset: at(a), opacity: 0 }, { offset: at(a), opacity: 1 });
		if (b < D) frames.push({ offset: at(b), opacity: 1 }, { offset: at(b), opacity: 0 });
	}
	frames.push({ offset: 1, opacity: frames[frames.length - 1].opacity });
	return tl.add(target, frames, 0, D, 'linear');
}

/** `list` with the `cuts` windows taken out. */
export function minus(list: [number, number][], cuts: [number, number][]): [number, number][] {
	let out = list;
	for (const [x, y] of cuts) {
		const next: [number, number][] = [];
		for (const [a, b] of out) {
			if (y <= a || x >= b) next.push([a, b]);
			else {
				if (a < x) next.push([a, x]);
				if (y < b) next.push([y, b]);
			}
		}
		out = next;
	}
	return out;
}
