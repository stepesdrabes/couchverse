import { describe, expect, it } from 'vitest';
import { integratedLoudness, normalize, truePeak } from '../../src/audio/loudness';
import { detectOnsets } from '../../src/audio/onsets';
import { decodeWav, encodeWav } from '../../src/audio/wav';

const RATE = 48000;

function sine(freq: number, amp: number, seconds: number) {
	const n = Math.round(seconds * RATE);
	const x = new Float32Array(n);
	for (let i = 0; i < n; i++) x[i] = amp * Math.sin((2 * Math.PI * freq * i) / RATE);
	return x;
}

describe('loudness', () => {
	it('measures a 1 kHz tone as BS.1770 does', () => {
		// a full-scale 997 Hz sine in one channel reads -3.01 LUFS; in both channels +3 dB more
		expect(integratedLoudness([sine(997, 1, 5)], RATE)).toBeCloseTo(-3.01, 1);
		expect(integratedLoudness([sine(997, 1, 5), sine(997, 1, 5)], RATE)).toBeCloseTo(0, 1);
		expect(integratedLoudness([sine(997, 0.1, 5), sine(997, 0.1, 5)], RATE)).toBeCloseTo(-20, 1);
	});

	it('gates silence out', () => {
		const x = new Float32Array(RATE * 10);
		x.set(sine(997, 0.1, 5), 0);
		expect(integratedLoudness([x, x.slice()], RATE)).toBeCloseTo(-20, 0);
		expect(integratedLoudness([new Float32Array(RATE)], RATE)).toBe(-Infinity);
	});

	it('finds inter-sample peaks', () => {
		// a quarter-rate sine sampled at 45 degrees never hits a sample at its peak
		const n = 4800;
		const x = new Float32Array(n);
		for (let i = 0; i < n; i++) x[i] = Math.sin((Math.PI / 2) * i + Math.PI / 4);
		const samplePeak = 20 * Math.log10(Math.max(...x.map(Math.abs)));
		expect(truePeak([x])).toBeGreaterThan(samplePeak + 2);
	});

	it('normalizes to the target under the true-peak ceiling', () => {
		const quiet = [sine(997, 0.05, 4), sine(997, 0.05, 4)];
		const r = normalize(quiet, RATE, -18);
		expect(r.integrated).toBeCloseTo(-18, 1);
		expect(integratedLoudness(quiet, RATE)).toBeCloseTo(-18, 1);
		// a peaky signal stops at -1 dBTP instead
		const peaky = [sine(997, 0.9, 4), sine(997, 0.9, 4)];
		const p = normalize(peaky, RATE, -10);
		expect(p.truePeak).toBeLessThanOrEqual(-1 + 1e-6);
	});
});

describe('wav', () => {
	it('round-trips float samples', () => {
		const ch = [sine(440, 0.5, 0.1), sine(220, 0.25, 0.1)];
		const back = decodeWav(encodeWav(ch, RATE));
		expect(back.sampleRate).toBe(RATE);
		expect(back.channels[0]).toEqual(ch[0]);
		expect(back.channels[1]).toEqual(ch[1]);
	});
});

describe('onsets', () => {
	it('finds clicks on a steady beat', () => {
		const n = RATE * 4;
		const x = new Float32Array(n);
		for (let beat = 0.25; beat < 4; beat += 0.5)
			for (let i = 0; i < 2400; i++)
				x[Math.round(beat * RATE) + i] = 0.8 * Math.exp(-i / 300) * Math.sin(i * 0.3);
		const onsets = detectOnsets({ sampleRate: RATE, left: x, right: x });
		const strong = onsets.filter((o) => o.s > 0.5).map((o) => o.t);
		expect(strong.length).toBeGreaterThanOrEqual(6);
		// within a detection window of a beat
		for (const t of strong)
			expect(Math.abs(t - 0.25 - Math.round((t - 0.25) / 0.5) * 0.5)).toBeLessThan(0.04);
	});
});
