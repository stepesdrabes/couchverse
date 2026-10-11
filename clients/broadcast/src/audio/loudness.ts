// Loudness per ITU-R BS.1770-4 (K-weighting, 400 ms blocks with 75% overlap, absolute and
// relative gates) and a 4x oversampled true-peak estimate. Pure functions over sample arrays,
// so a render's level is as deterministic as its samples.

type Coeffs = { b: [number, number, number]; a: [number, number, number] };

/** The two K-weighting biquads for a sample rate (the libebur128 derivation). */
export function kWeighting(rate: number): [Coeffs, Coeffs] {
	let f0 = 1681.974450955533;
	const G = 3.999843853973347;
	let Q = 0.7071752369554196;
	let K = Math.tan((Math.PI * f0) / rate);
	const Vh = Math.pow(10, G / 20);
	const Vb = Math.pow(Vh, 0.4996667741545416);
	let a0 = 1 + K / Q + K * K;
	const shelf: Coeffs = {
		b: [
			(Vh + (Vb * K) / Q + K * K) / a0,
			(2 * (K * K - Vh)) / a0,
			(Vh - (Vb * K) / Q + K * K) / a0
		],
		a: [1, (2 * (K * K - 1)) / a0, (1 - K / Q + K * K) / a0]
	};
	f0 = 38.13547087602444;
	Q = 0.5003270373238773;
	K = Math.tan((Math.PI * f0) / rate);
	a0 = 1 + K / Q + K * K;
	const highpass: Coeffs = {
		b: [1, -2, 1],
		a: [1, (2 * (K * K - 1)) / a0, (1 - K / Q + K * K) / a0]
	};
	return [shelf, highpass];
}

function biquad(x: Float32Array, { b, a }: Coeffs): Float32Array {
	const y = new Float32Array(x.length);
	let x1 = 0,
		x2 = 0,
		y1 = 0,
		y2 = 0;
	for (let i = 0; i < x.length; i++) {
		const v = b[0] * x[i] + b[1] * x1 + b[2] * x2 - a[1] * y1 - a[2] * y2;
		x2 = x1;
		x1 = x[i];
		y2 = y1;
		y1 = v;
		y[i] = v;
	}
	return y;
}

/** Integrated loudness in LUFS; -Infinity for silence. */
export function integratedLoudness(channels: Float32Array[], rate: number): number {
	const [shelf, hp] = kWeighting(rate);
	const weighted = channels.map((c) => biquad(biquad(c, shelf), hp));
	const block = Math.round(rate * 0.4);
	const step = Math.round(rate * 0.1);
	const len = weighted[0]?.length ?? 0;
	const powers: number[] = [];
	for (let s = 0; s + block <= len; s += step) {
		let p = 0;
		for (const w of weighted) {
			let sum = 0;
			for (let i = s; i < s + block; i++) sum += w[i] * w[i];
			p += sum / block;
		}
		powers.push(p);
	}
	const lufs = (p: number) => -0.691 + 10 * Math.log10(p);
	const abs = powers.filter((p) => lufs(p) > -70);
	if (!abs.length) return -Infinity;
	const mean = (xs: number[]) => xs.reduce((s, x) => s + x, 0) / xs.length;
	const relGate = lufs(mean(abs)) - 10;
	const rel = abs.filter((p) => lufs(p) > relGate);
	return lufs(mean(rel.length ? rel : abs));
}

// a 4x interpolator: a Hann-windowed sinc, 12 taps per phase
const PHASES = 4;
const TAPS = 12;
const FIR: number[][] = Array.from({ length: PHASES }, (_, p) =>
	Array.from({ length: TAPS }, (_, k) => {
		const n = k * PHASES + p;
		const center = (TAPS * PHASES - 1) / 2;
		const x = (n - center) / PHASES;
		const sinc = x === 0 ? 1 : Math.sin(Math.PI * x) / (Math.PI * x);
		const w = 0.5 - 0.5 * Math.cos((2 * Math.PI * (n + 0.5)) / (TAPS * PHASES));
		return sinc * w;
	})
);

/** The true peak (dBTP) of the channels, estimated by 4x oversampling. */
export function truePeak(channels: Float32Array[]): number {
	let peak = 0;
	for (const c of channels) {
		for (let i = 0; i < c.length; i++) {
			const v = Math.abs(c[i]);
			if (v > peak) peak = v;
		}
		for (let i = TAPS; i < c.length; i++) {
			for (let p = 1; p < PHASES; p++) {
				const h = FIR[p];
				let s = 0;
				for (let k = 0; k < TAPS; k++) s += h[k] * c[i - k];
				const v = Math.abs(s);
				if (v > peak) peak = v;
			}
		}
	}
	return 20 * Math.log10(peak + 1e-12);
}

export interface LoudnessReport {
	/** Integrated loudness before normalization, LUFS. */
	measured: number;
	/** The gain applied, dB. */
	gainDb: number;
	/** Integrated loudness after normalization, LUFS. */
	integrated: number;
	/** True peak after normalization, dBTP. */
	truePeak: number;
}

/**
 * Scale the channels in place to `target` LUFS, never letting the true peak pass `ceiling`
 * dBTP (a quieter render beats a clipped one).
 */
export function normalize(
	channels: Float32Array[],
	rate: number,
	target: number,
	ceiling = -1
): LoudnessReport {
	const measured = integratedLoudness(channels, rate);
	const peak = truePeak(channels);
	let gainDb = isFinite(measured) ? target - measured : 0;
	if (peak + gainDb > ceiling) gainDb = ceiling - peak;
	const g = Math.pow(10, gainDb / 20);
	for (const c of channels) for (let i = 0; i < c.length; i++) c[i] *= g;
	return {
		measured,
		gainDb,
		integrated: isFinite(measured) ? measured + gainDb : measured,
		truePeak: peak + gainDb
	};
}
