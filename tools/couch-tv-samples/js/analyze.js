// Debug metering for offline renders: BS.1770 loudness (K-weighted, gated), sample peak,
// short-term loudness over time and the long-term spectrum in mix-engineering bands. Used by
// reel.analyze(); never part of a piece.

/** RBJ biquad coefficients (normalized by a0). */
function biquad(type, f0, q, gainDb, sr) {
	const A = Math.pow(10, gainDb / 40);
	const w = (2 * Math.PI * f0) / sr;
	const cs = Math.cos(w);
	const al = Math.sin(w) / (2 * q);
	let b0, b1, b2, a0, a1, a2;
	if (type === 'highshelf') {
		const s = 2 * Math.sqrt(A) * al;
		b0 = A * (A + 1 + (A - 1) * cs + s);
		b1 = -2 * A * (A - 1 + (A + 1) * cs);
		b2 = A * (A + 1 + (A - 1) * cs - s);
		a0 = A + 1 - (A - 1) * cs + s;
		a1 = 2 * (A - 1 - (A + 1) * cs);
		a2 = A + 1 - (A - 1) * cs - s;
	} else {
		b0 = (1 + cs) / 2;
		b1 = -(1 + cs);
		b2 = (1 + cs) / 2;
		a0 = 1 + al;
		a1 = -2 * cs;
		a2 = 1 - al;
	}
	return [b0 / a0, b1 / a0, b2 / a0, a1 / a0, a2 / a0];
}

function filt(x, [b0, b1, b2, a1, a2]) {
	const y = new Float32Array(x.length);
	let x1 = 0, x2 = 0, y1 = 0, y2 = 0;
	for (let i = 0; i < x.length; i++) {
		const v = b0 * x[i] + b1 * x1 + b2 * x2 - a1 * y1 - a2 * y2;
		x2 = x1;
		x1 = x[i];
		y2 = y1;
		y1 = v;
		y[i] = v;
	}
	return y;
}

/** K-weighted channels (the BS.1770 pre-filter, re-derived for any sample rate). */
function kWeight(ch, sr) {
	const shelf = biquad('highshelf', 1681.974450955533, 0.7071752369554196, 3.999843853973347, sr);
	const hp = biquad('highpass', 38.13547087602444, 0.5003270373238773, 0, sr);
	return ch.map((x) => filt(filt(x, shelf), hp));
}

const lufsOf = (ms) => -0.691 + 10 * Math.log10(ms + 1e-15);

export function loudness(buffer) {
	const sr = buffer.sampleRate;
	const chans = [];
	for (let c = 0; c < buffer.numberOfChannels; c++) chans.push(buffer.getChannelData(c));
	const k = kWeight(chans, sr);
	const block = (len, hop) => {
		const out = [];
		for (let i = 0; i + len <= k[0].length; i += hop) {
			let s = 0;
			for (const ch of k) for (let j = i; j < i + len; j++) s += ch[j] * ch[j];
			out.push({ t: i / sr, ms: s / len });
		}
		return out;
	};
	const mom = block(Math.round(0.4 * sr), Math.round(0.1 * sr));
	const abs = mom.filter((b) => lufsOf(b.ms) > -70);
	const rel = abs.length ? lufsOf(abs.reduce((a, b) => a + b.ms, 0) / abs.length) - 10 : -70;
	const gated = abs.filter((b) => lufsOf(b.ms) > rel);
	const integrated = gated.length ? lufsOf(gated.reduce((a, b) => a + b.ms, 0) / gated.length) : null;
	const st = block(Math.round(3 * sr), Math.round(0.25 * sr));
	let peak = 0;
	for (const ch of chans) for (let i = 0; i < ch.length; i++) peak = Math.max(peak, Math.abs(ch[i]));
	return {
		peakDb: +(20 * Math.log10(peak + 1e-12)).toFixed(2),
		integratedLufs: integrated == null ? null : +integrated.toFixed(1),
		maxMomentaryLufs: +Math.max(...mom.map((b) => lufsOf(b.ms))).toFixed(1),
		maxShortTermLufs: st.length ? +Math.max(...st.map((b) => lufsOf(b.ms))).toFixed(1) : null,
		// momentary loudness every 0.5 s, to read the shape of a piece
		curve: mom.filter((_, i) => i % 5 === 0).map((b) => [+b.t.toFixed(1), +lufsOf(b.ms).toFixed(1)])
	};
}

function fft(re, im) {
	const n = re.length;
	for (let i = 1, j = 0; i < n; i++) {
		let bit = n >> 1;
		for (; j & bit; bit >>= 1) j ^= bit;
		j ^= bit;
		if (i < j) {
			[re[i], re[j]] = [re[j], re[i]];
			[im[i], im[j]] = [im[j], im[i]];
		}
	}
	for (let len = 2; len <= n; len <<= 1) {
		const ang = (-2 * Math.PI) / len;
		const wr = Math.cos(ang);
		const wi = Math.sin(ang);
		for (let i = 0; i < n; i += len) {
			let cr = 1, ci = 0;
			for (let j = 0; j < len / 2; j++) {
				const ar = re[i + j + len / 2] * cr - im[i + j + len / 2] * ci;
				const ai = re[i + j + len / 2] * ci + im[i + j + len / 2] * cr;
				re[i + j + len / 2] = re[i + j] - ar;
				im[i + j + len / 2] = im[i + j] - ai;
				re[i + j] += ar;
				im[i + j] += ai;
				const t = cr * wr - ci * wi;
				ci = cr * wi + ci * wr;
				cr = t;
			}
		}
	}
}

export const BANDS = [
	['sub', 20, 60],
	['bass', 60, 200],
	['lowmid', 200, 400],
	['mid', 400, 2000],
	['presence', 2000, 5000],
	['air', 5000, 16000]
];

/** Long-term power spectrum summed into bands, in dB relative to the total (from `from` s). */
export function spectrum(buffer, from = 0, to = Infinity) {
	const sr = buffer.sampleRate;
	const N = 4096;
	const a = buffer.getChannelData(0);
	const b = buffer.numberOfChannels > 1 ? buffer.getChannelData(1) : a;
	const win = new Float32Array(N).map((_, i) => 0.5 - 0.5 * Math.cos((2 * Math.PI * i) / (N - 1)));
	const pow = new Float64Array(N / 2);
	const i0 = Math.floor(from * sr);
	const i1 = Math.min(a.length, Math.floor(to * sr));
	for (let o = i0; o + N <= i1; o += N / 2) {
		const re = new Float64Array(N);
		const im = new Float64Array(N);
		for (let i = 0; i < N; i++) re[i] = ((a[o + i] + b[o + i]) / 2) * win[i];
		fft(re, im);
		for (let k = 0; k < N / 2; k++) pow[k] += re[k] * re[k] + im[k] * im[k];
	}
	const total = pow.reduce((x, y) => x + y, 0) + 1e-20;
	const out = {};
	for (const [name, lo, hi] of BANDS) {
		let s = 0;
		for (let k = Math.ceil((lo * N) / sr); k < Math.min(N / 2, (hi * N) / sr); k++) s += pow[k];
		out[name] = +(10 * Math.log10(s / total + 1e-20)).toFixed(1);
	}
	return out;
}
