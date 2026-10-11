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

export class Timeline {
	readonly root: HTMLElement;
	duration: number;
	readonly anims: Animation[] = [];
	readonly cues: Cue[] = [];
	/** Named windows of the timeline that render as separate clips (a promo's body and end). */
	parts: Record<string, [number, number]> = {};
	/** Loops seamlessly: the audio render wraps its tail into its start. */
	loop = false;
	/** The squeeze's program panel, when the template has one. */
	keyRect: ((t: number) => KeyRect | null) | null = null;
	private readonly spans: [number, number][] = [];
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
		const a = target.animate(frames, {
			duration: ms,
			delay: at * 1000,
			fill: opts.fill ?? (again ? 'forwards' : 'both'),
			easing: easing(ease),
			iterations,
			direction: opts.direction ?? 'normal'
		});
		a.pause();
		this.anims.push(a);
		this.spans.push([at, at + (ms / 1000) * iterations]);
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

	/** Footage: the frame for t of a 25 fps sequence that starts at `at` (held at its ends). */
	footage(img: HTMLImageElement, frames: string[], at: number) {
		const index = (t: number) =>
			Math.max(0, Math.min(frames.length - 1, Math.floor((t - at) * FPS + 1e-6)));
		this.dynamic(index, (v, wait) => {
			const i = Number(v);
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
	 * Whether the frames at t0 and t1 (t0 < t1) can differ: some animation is active in
	 * between or some dynamic value changes. Conservative: a held keyframe counts as motion.
	 */
	changes(t0: number, t1: number): boolean {
		for (const [a, b] of this.spans) if (a <= t1 && b > t0) return true;
		for (const d of this.dynamics) if (d.value(t0) !== d.value(t1)) return true;
		return false;
	}

	/**
	 * Schedule every sound cue that is still ahead of `offset` (or still sounding, for cues
	 * with a duration). `reseed` runs before each cue with its index, so a cue's noise is the
	 * same wherever playback starts.
	 */
	scheduleSounds(ctxStart: number, offset: number, bus: Bus, reseed?: (index: number) => void) {
		this.sounds.forEach((s, i) => {
			const inside = s.dur > 0 && offset > s.at && offset < s.at + s.dur;
			if (s.at < offset - 0.03 && !inside) return;
			const skip = Math.max(0, offset - s.at);
			reseed?.(i);
			try {
				s.fn(ctxStart + Math.max(0, s.at - offset), bus, skip);
			} catch (err) {
				console.warn('sound cue failed', err);
			}
		});
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
