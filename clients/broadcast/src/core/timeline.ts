// One piece's clock. Animations are Web Animations created paused with their start folded into
// `delay`, so the picture at t is whatever the browser computes for currentTime = t: seeking is
// setting currentTime, playing is giving every animation one startTime (taken from the audio
// clock). Text and footage that change are pure functions of t (`dynamic`). Sounds are cues
// scheduled against the same zero. Nothing here keeps state between frames.

import type { Bus } from '../audio/engine';
import { easing, type Ease } from './motion';

export const FPS = 25;

/** Where the squeeze's program panel is, in stage pixels, at one moment. */
export interface KeyRect {
	x: number;
	y: number;
	width: number;
	height: number;
	/** Corner radius, stage pixels. */
	radius: number;
	opacity: number;
}

export interface Cue {
	name: string;
	at: number;
}

/** A sound cue: `when` is AudioContext time, `skip` how far into the cue playback starts. */
export type SoundFn = (when: number, bus: Bus, skip: number) => void;

interface SoundCue {
	at: number;
	fn: SoundFn;
	dur: number;
}

/** A sound cue placed on an audio context: the time it plays at, and scheduling it. */
export interface PlacedSound {
	when: number;
	play(): void;
}

interface Dynamic {
	value: (t: number) => string | number;
	apply: (v: string | number, wait: Promise<unknown>[] | null) => void;
	last?: string | number;
}

export interface AddOptions {
	fill?: FillMode;
	iterations?: number;
	direction?: PlaybackDirection;
}

type Frames = Keyframe[];

const values = (f: Keyframe) =>
	JSON.stringify(Object.entries(f).filter(([k]) => k !== 'offset' && k !== 'easing'));

/** One value throughout (a hard cut's visibility, a held pose): it moves nothing. */
const steady = (frames: Frames) => frames.every((f) => values(f) === values(frames[0]));

/**
 * The parts of an iteration (as offsets) where the value changes: between neighbouring
 * keyframes that differ, a hard cut being a part of no length. Null when the offsets are only
 * partly given (the browser spaces the rest).
 */
function moving(frames: Frames): [number, number][] | null {
	const given = frames.filter((f) => typeof f.offset === 'number').length;
	if (given !== 0 && given !== frames.length) return null;
	const offset = (i: number) =>
		given ? Number(frames[i].offset) : frames.length > 1 ? i / (frames.length - 1) : 0;
	const parts: [number, number][] = [];
	for (let i = 1; i < frames.length; i++)
		if (values(frames[i]) !== values(frames[i - 1])) parts.push([offset(i - 1), offset(i)]);
	return parts;
}

export class Timeline {
	readonly root: HTMLElement;
	duration: number;
	readonly anims: Animation[] = [];
	readonly cues: Cue[] = [];
	/**
	 * Named windows of the timeline that render as separate clips (a promo's body and end);
	 * `sound` is how far a clip's sound runs in a break when that is past the clip.
	 */
	parts: Record<string, [number, number]> = {};
	/** Loops seamlessly: the audio render wraps its tail into its start. */
	loop = false;
	/** The squeeze's program panel, when the template has one. */
	keyRect: ((t: number) => KeyRect | null) | null = null;
	/** Where animations move things: [start, end) of each stretch whose value changes. */
	private readonly spans: [number, number][] = [];
	/** Where a value jumps: hard cuts, and steady animations starting or stopping to apply. */
	private readonly edges: number[] = [];
	private readonly sounds: SoundCue[] = [];
	private readonly dynamics: Dynamic[] = [];
	private readonly props = new WeakMap<Element, Set<string>>();
	private docStart: number | null = null;

	constructor(root: HTMLElement, duration: number) {
		this.root = root;
		this.duration = duration;
	}

	/**
	 * Animate `target` from `at` for `dur` seconds. The first animation of a property on an
	 * element fills both ways (it owns the element's state before its cue); later ones only
	 * fill forwards, so cues stack in time order without fighting.
	 */
	add(
		target: Element | null | undefined,
		frames: Frames,
		at: number,
		dur: number,
		ease: Ease = 'enter',
		opts: AddOptions = {}
	): Animation | null {
		if (!target) return null;
		const keys = new Set<string>();
		for (const f of frames)
			for (const k of Object.keys(f)) if (k !== 'offset' && k !== 'easing') keys.add(k);
		let seen = this.props.get(target);
		if (!seen) this.props.set(target, (seen = new Set()));
		const again = [...keys].some((p) => seen.has(p));
		keys.forEach((p) => seen.add(p));
		const iterations = opts.iterations ?? 1;
		const ms = Math.max(1, dur * 1000);
		const fill = opts.fill ?? (again ? 'forwards' : 'both');
		const a = target.animate(frames, {
			duration: ms,
			delay: at * 1000,
			fill,
			easing: easing(ease),
			iterations,
			direction: opts.direction ?? 'normal'
		});
		a.pause();
		this.anims.push(a);
		const end = at + (ms / 1000) * iterations;
		// with a linear timing (whole-piece `motion` and `windows`), a held keyframe holds its
		// value exactly, so only the parts in between move
		const parts = steady(frames)
			? []
			: easing(ease) === 'linear' && !opts.direction
				? moving(frames)
				: null;
		if (!parts) this.spans.push([at, end]);
		else {
			const loops = values(frames[0]) !== values(frames[frames.length - 1]);
			for (let k = 0; k < iterations; k++) {
				const from = at + (k * ms) / 1000;
				if (k > 0 && loops) this.edges.push(from);
				for (const [p, q] of parts) {
					if (q > p) this.spans.push([from + (p * ms) / 1000, from + (q * ms) / 1000]);
					else this.edges.push(from + (p * ms) / 1000);
				}
			}
			if (fill !== 'backwards' && fill !== 'both') this.edges.push(at);
			if (fill !== 'forwards' && fill !== 'both') this.edges.push(end);
		}
		return a;
	}

	/** Visible only within [a, b): hard cuts. Uses visibility, so opacity stays free. */
	show(target: HTMLElement, a: number, b: number) {
		target.style.visibility = 'hidden';
		return this.add(
			target,
			[{ visibility: 'visible' }, { visibility: 'visible' }],
			a,
			b - a,
			'linear',
			{ fill: 'none' }
		);
	}

	/** A value that changes with t (a clock, a counter, a footage frame). */
	dynamic(
		value: (t: number) => string | number,
		apply: (v: string | number, wait: Promise<unknown>[] | null) => void
	) {
		this.dynamics.push({ value, apply });
	}

	text(target: HTMLElement, value: (t: number) => string) {
		this.dynamic(value, (v) => (target.textContent = String(v)));
	}

	/**
	 * Footage: the frame for t of a 25 fps sequence that starts at `at` (held at its ends). With a
	 * `window` the image holds a frame only within it (a take and a moment around it) and none
	 * outside: a long piece with dozens of takes would otherwise decode every one of them on
	 * every frame it renders.
	 */
	footage(img: HTMLImageElement, frames: string[], at: number, window?: [number, number]) {
		const index = (t: number) =>
			window && (t < window[0] || t > window[1])
				? -1
				: Math.max(0, Math.min(frames.length - 1, Math.floor((t - at) * FPS + 1e-6)));
		this.dynamic(index, (v, wait) => {
			const i = Number(v);
			if (i < 0) {
				img.removeAttribute('src');
				return;
			}
			img.src = frames[i];
			if (wait) wait.push(img.decode().catch(() => undefined));
			else prefetch(frames, i + 1);
		});
	}

	sound(at: number, fn: SoundFn, dur = 0) {
		this.sounds.push({ at, fn, dur });
	}

	cue(name: string, at: number) {
		this.cues.push({ name, at });
		return at;
	}

	cueAt(t: number): Cue | null {
		let cur: Cue | null = null;
		for (const c of this.cues) if (c.at <= t + 1e-3 && (!cur || c.at >= cur.at)) cur = c;
		return cur;
	}

	/** Show the frame at t; resolves when every footage frame it needs is decoded. */
	seek(t: number): Promise<void> {
		this.docStart = null;
		for (const a of this.anims) {
			a.pause();
			a.currentTime = t * 1000;
		}
		const wait: Promise<unknown>[] = [];
		this.updateDynamics(t, wait);
		return Promise.all(wait).then(() => undefined);
	}

	/** Start at document time `docStart` (ms) from timeline offset `offset` (s). */
	play(docStart: number, offset = 0) {
		this.docStart = docStart - offset * 1000;
		for (const a of this.anims) a.startTime = this.docStart;
	}

	pause() {
		for (const a of this.anims) a.pause();
		this.docStart = null;
	}

	cancel() {
		for (const a of this.anims) a.cancel();
		this.anims.length = 0;
	}

	/** The timeline time at document time `now` while playing; null when frozen. */
	time(now = Number(document.timeline.currentTime ?? 0)): number | null {
		if (this.docStart == null) return null;
		return (now - this.docStart) / 1000;
	}

	/** Bring the dynamic values up to t (every frame while playing). */
	update(t: number) {
		this.updateDynamics(t, null);
	}

	private updateDynamics(t: number, wait: Promise<unknown>[] | null) {
		for (const d of this.dynamics) {
			const v = d.value(t);
			if (v !== d.last || wait) {
				d.last = v;
				d.apply(v, wait);
			}
		}
	}

	/**
	 * Whether the frames at t0 and t1 (t0 < t1) can differ: some animation moves something in
	 * between, a steady one starts or stops applying, or some dynamic value changes.
	 * Conservative: an animation whose keyframes differ counts as motion all through.
	 */
	changes(t0: number, t1: number): boolean {
		for (const [a, b] of this.spans) if (a <= t1 && b > t0) return true;
		for (const e of this.edges) if (e > t0 && e <= t1) return true;
		for (const d of this.dynamics) if (d.value(t0) !== d.value(t1)) return true;
		return false;
	}

	/**
	 * Schedule every sound cue that is still ahead of `offset` (or still sounding, for cues
	 * with a duration). `reseed` runs before each cue with its index, so a cue's noise is the
	 * same wherever playback starts.
	 */
	scheduleSounds(ctxStart: number, offset: number, bus: Bus, reseed?: (index: number) => void) {
		for (const s of this.soundsFrom(ctxStart, offset, bus, reseed)) s.play();
	}

	/** `scheduleSounds` one cue at a time: when each plays, and the call that schedules it. */
	soundsFrom(
		ctxStart: number,
		offset: number,
		bus: Bus,
		reseed?: (index: number) => void
	): PlacedSound[] {
		const out: PlacedSound[] = [];
		this.sounds.forEach((s, i) => {
			const inside = s.dur > 0 && offset > s.at && offset < s.at + s.dur;
			if (s.at < offset - 0.03 && !inside) return;
			const skip = Math.max(0, offset - s.at);
			const when = ctxStart + Math.max(0, s.at - offset);
			out.push({
				when,
				play: () => {
					reseed?.(i);
					try {
						s.fn(when, bus, skip);
					} catch (err) {
						// one cue failing must not silence the rest, but it is a bug: an error, so the
						// tests (and the sidecar's log) see it
						console.error('sound cue failed', err);
					}
				}
			});
		});
		return out;
	}
}

const prefetched = new Set<string>();

/** Warm the cache for the next footage frames while playing live. */
function prefetch(frames: string[], from: number) {
	for (let i = from; i < Math.min(frames.length, from + 12); i++) {
		const url = frames[i];
		if (prefetched.has(url)) continue;
		prefetched.add(url);
		if (prefetched.size > 2000) prefetched.clear();
		const img = new Image();
		img.decoding = 'async';
		img.src = url;
	}
}

/** Round a time to the frame grid. */
export const frame = (t: number) => Math.round(t * FPS) / FPS;
