import type { Onset } from './engine';

/** The samples a detector reads: two channels (or one, twice) and a rate. */
export interface PcmView {
	sampleRate: number;
	left: Float32Array;
	right: Float32Array;
}

/**
 * Onset detection over `[from, from + length)` seconds: high-frequency-weighted log energy, its
 * positive flux against a short history, an adaptive threshold and peak picking. Strengths are
 * normalized to the 90th percentile, so templates can ask for "strong" hits portably. Times
 * are relative to `from`.
 */
export function detectOnsets(pcm: PcmView, from = 0, length = Infinity): Onset[] {
	const sr = pcm.sampleRate;
	const start = Math.max(0, Math.floor(from * sr));
	const stop = Math.min(pcm.left.length, start + Math.floor(Math.min(length, 1e6) * sr));
	const a = pcm.left;
	const b = pcm.right;
	const hop = Math.round((512 * sr) / 44100);
	const win = hop * 2;
	const n = Math.max(0, Math.floor((stop - start - win) / hop));
	const e = new Float32Array(n);
	for (let i = 0; i < n; i++) {
		let s = 0;
		let prev = 0;
		const o = start + i * hop;
		for (let j = 0; j < win; j++) {
			const x = (a[o + j] + b[o + j]) * 0.5;
			const d = x - prev;
			prev = x;
			s += d * d * 0.75 + x * x * 0.25;
		}
		e[i] = Math.log(1e-8 + s / win);
	}
	const flux = new Float32Array(n);
	for (let i = 3; i < n; i++)
		flux[i] = Math.max(0, e[i] - Math.max(e[i - 1], (e[i - 2] + e[i - 3]) / 2));
	const out: Onset[] = [];
	const W = 16;
	for (let i = 1; i < n - 1; i++) {
		let m = 0;
		let c = 0;
		for (let j = Math.max(0, i - W); j < Math.min(n, i + W); j++) {
			m += flux[j];
			c++;
		}
		m /= c;
		let isMax = true;
		for (let j = Math.max(0, i - 5); j <= Math.min(n - 1, i + 5); j++)
			if (flux[j] > flux[i]) isMax = false;
		if (isMax && flux[i] > m * 1.6 + 0.08) out.push({ t: (i * hop) / sr, s: flux[i] });
	}
	const sorted = out.map((o) => o.s).sort((x, y) => x - y);
	const p90 = sorted[Math.floor(sorted.length * 0.9)] || 1;
	return out.map((o) => ({ t: +o.t.toFixed(3), s: +Math.min(1.5, o.s / p90).toFixed(3) }));
}

/** The strongest onset near t (weighted by distance), or t when nothing qualifies. */
export function snapper(onsets: Onset[]) {
	return (t: number, win = 0.6, min = 0.4) => {
		let best = t;
		let score = -1;
		for (const o of onsets) {
			const d = Math.abs(o.t - t);
			if (d > win || o.s < min) continue;
			const sc = o.s * (1 - (0.6 * d) / win);
			if (sc > score) {
				score = sc;
				best = o.t;
			}
		}
		return best;
	};
}

/** Onsets in (a, b) at least `gap` apart and `min` strong: cut points that follow the music. */
export function beatsIn(
	onsets: Onset[],
	a: number,
	b: number,
	gap: number,
	min: number,
	tail = gap * 0.6
) {
	const out: number[] = [];
	let last = a;
	for (const o of onsets) {
		if (o.t <= last + gap || o.t >= b - tail || o.s < min) continue;
		out.push(o.t);
		last = o.t;
	}
	return out;
}
