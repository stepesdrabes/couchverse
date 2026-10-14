// The trailer's mix: a scene's own sound levelled to one dialogue level, the title's soundtrack
// ridden under it, and the envelopes both follow. The curves are pure functions of the plan, so
// where the music ducks is testable without a browser; every gain move is sample-accurate
// automation on the context's own clock.

import { applyEnv, type Bus, type LoadedBed } from './engine';
import type { Kit } from './kit';
import { integratedLoudness, momentaryPeak } from './loudness';

/** A level in dB as a linear gain; -80 dB and under is silence. */
export const gainOf = (db: number) => (db <= -80 ? 0 : Math.pow(10, db / 20));

/** What dialogue is levelled to (LUFS of its lines, before the engine's own chain). */
export const DIALOGUE_LUFS = -22;
/** Scenes without a line (effects, the score) sit a little under the dialogue. */
export const EFFECTS_LUFS = -24;
/** The soundtrack's reference level (0 dB on its curve), a few dB over the dialogue. */
export const BED_LUFS = -15;
/** A scene's loudest moment may stand this far over what it is levelled to, dB. */
export const OVER = { dialogue: 10, effects: 6 };
/** How far levelling may lift or lower a scene or a soundtrack, dB. */
const REACH: [number, number] = [-14, 14];

interface Measure {
	/** Integrated loudness, LUFS. */
	lufs: number;
	/** The loudest 400 ms, LUFS. */
	peak: number;
}

const measured = new WeakMap<AudioBuffer, Map<string, Measure>>();

/** The loudness of ranges of a decoded buffer, given in seconds into it, heard as one stretch. */
export function loudnessOf(buffer: AudioBuffer, ranges: [number, number][]): Measure {
	const key = ranges.map((r) => r.map((x) => x.toFixed(3)).join('-')).join(',');
	let cache = measured.get(buffer);
	if (!cache) measured.set(buffer, (cache = new Map()));
	let m = cache.get(key);
	if (!m) {
		const rate = buffer.sampleRate;
		const channels = [0, 1].map((c) => {
			const data = buffer.getChannelData(Math.min(c, buffer.numberOfChannels - 1));
			const parts = ranges.map(([a, b]) =>
				data.subarray(
					Math.max(0, Math.round(a * rate)),
					Math.min(data.length, Math.round(b * rate))
				)
			);
			const out = new Float32Array(parts.reduce((n, p) => n + p.length, 0));
			let at = 0;
			for (const p of parts) {
				out.set(p, at);
				at += p.length;
			}
			return out;
		});
		m = { lufs: integratedLoudness(channels, rate), peak: momentaryPeak(channels, rate) };
		cache.set(key, m);
	}
	return m;
}

/**
 * The gain (dB) that brings a measured level to `target`, within reach, and never lets the
 * loudest moment (`peak`, which the caller may measure over more than the levelled stretch) pass
 * `ceiling`; 0 for silence.
 */
export function levelling(
	lufs: number,
	target: number,
	peak = -Infinity,
	ceiling = Infinity
): number {
	if (!Number.isFinite(lufs)) return 0;
	const db = Math.min(target - lufs, Number.isFinite(peak) ? ceiling - peak : Infinity);
	return Math.min(REACH[1], Math.max(REACH[0], db));
}

/** A level at t (dB) on a curve through `[t, dB]` keys: flat before the first, after the last. */
export function dbAt(keys: [number, number][], t: number): number {
	if (!keys.length) return 0;
	if (t <= keys[0][0]) return keys[0][1];
	for (let i = 1; i < keys.length; i++) {
		const [t1, v1] = keys[i];
		if (t > t1) continue;
		const [t0, v0] = keys[i - 1];
		return t1 === t0 ? v1 : v0 + ((v1 - v0) * (t - t0)) / (t1 - t0);
	}
	return keys[keys.length - 1][1];
}

/** How the music gives way to speech. */
export interface Duck {
	/** dB under the music while someone speaks (negative). */
	depth: number;
	/** Seconds the music takes to get out of the way, ending a moment before the line starts. */
	attack: number;
	/** Seconds it takes to come back after the line's last word. */
	release: number;
	/** Seconds before a line it is already down, and after it before it comes back. */
	before: number;
	after: number;
}

/** Twelve dB under every line, down before the first word and back over half a second. */
export const SPEECH: Duck = { depth: -13, attack: 0.22, release: 0.55, before: 0.05, after: 0.12 };

/**
 * A line of speech, `[from, to]` seconds, and optionally how many dB quieter than the dialogue's
 * level it stayed (a quiet scene the levelling could not lift): the music goes down that much more.
 */
export type Line = [from: number, to: number, quieter?: number];

/**
 * The keys of a duck over `lines`: lines whose ramps would meet share one stretch held down
 * (never back up between them), so the music never pumps between two close lines; within a
 * stretch the depth follows each line's own (a quieter line takes it deeper), ramping between.
 */
export function duckKeys(lines: Line[], d: Duck = SPEECH): [number, number][] {
	// each line's span, those that overlap one another as one
	const spans: [number, number, number][] = [];
	for (const [a, b, q = 0] of [...lines].sort((x, y) => x[0] - y[0])) {
		const [from, to] = [a - d.before, b + d.after];
		const last = spans[spans.length - 1];
		if (last && from <= last[1]) {
			last[1] = Math.max(last[1], to);
			last[2] = Math.max(last[2], q);
		} else spans.push([from, to, q]);
	}
	const keys: [number, number][] = [];
	spans.forEach(([a, b, q], i) => {
		const before = spans[i - 1];
		const joined = before && a - d.attack < before[1] + d.release;
		if (!joined) keys.push([a - d.attack, 0]);
		keys.push([a, d.depth - q], [b, d.depth - q]);
		const next = spans[i + 1];
		if (!(next && next[0] - d.attack < b + d.release)) keys.push([b + d.release, 0]);
	});
	return keys;
}

/**
 * A level curve as automation points `[t, gain]`: `base` (dB keys) plus the duck under `lines`,
 * every `step` seconds and at every key, between `from` and `to`. A point's gain is linear
 * between neighbours, so the step keeps the dB ramps smooth.
 */
export function ride(
	base: [number, number][],
	lines: Line[],
	from: number,
	to: number,
	{ duck = SPEECH, step = 0.02 } = {}
): [number, number][] {
	const ducks = duckKeys(lines, duck);
	const times = new Set<number>();
	for (let t = from; t < to; t += step) times.add(+t.toFixed(4));
	times.add(to);
	for (const [t] of [...base, ...ducks]) if (t > from && t < to) times.add(+t.toFixed(4));
	return [...times]
		.sort((a, b) => a - b)
		.map((t): [number, number] => [t, gainOf(dbAt(base, t) + dbAt(ducks, t))]);
}

export interface SceneSound {
	loaded: LoadedBed;
	/** The stretch of the scene's sound that plays, seconds into it. */
	from: number;
	to: number;
	/** Linear gain after levelling. */
	gain: number;
	fadeIn: number;
	fadeOut: number;
}

/**
 * A scene's own sound on the voice bus, from `when` (`skip` seconds into it when playback starts
 * late). Its edges are short fades, so no cut clicks, and a fade never reaches a word: the plan
 * keeps the lines clear of both.
 */
export function playScene(k: Kit, b: Bus, when: number, s: SceneSound, skip = 0) {
	const length = s.to - s.from;
	if (length <= 0 || skip >= length) return;
	const fin = Math.min(s.fadeIn, length / 2);
	const fout = Math.min(s.fadeOut, length / 2);
	const src = k.ctx.createBufferSource();
	src.buffer = s.loaded.buffer;
	const env = k.gain(0);
	src.connect(env).connect(b.voice);
	applyEnv(
		env.gain,
		[
			[0, 0],
			[fin, s.gain],
			[length - fout, s.gain],
			[length, 0]
		],
		when,
		skip
	);
	src.start(when, s.loaded.in + s.from + skip);
	src.stop(when + (length - skip) + 0.02);
	k.track(b, src);
}

/**
 * The soundtrack on the music bus along `points` (`[t, gain]` from the cue's start), playing
 * `length` seconds of its excerpt from `from`.
 */
export function playRidden(
	k: Kit,
	b: Bus,
	when: number,
	bed: LoadedBed,
	{ from, length, points }: { from: number; length: number; points: [number, number][] },
	skip = 0
) {
	if (length <= 0 || skip >= length) return;
	const src = k.ctx.createBufferSource();
	src.buffer = bed.buffer;
	const env = k.gain(0);
	src.connect(env).connect(b.music);
	applyEnv(env.gain, points, when, skip);
	src.start(when, bed.in + from + skip);
	src.stop(when + (length - skip) + 0.05);
	k.track(b, src);
}
