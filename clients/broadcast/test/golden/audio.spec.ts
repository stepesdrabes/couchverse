import { expect, test } from '@playwright/test';
import { integratedLoudness, normalize, truePeak } from '../../src/audio/loudness';
import type { ClipSpec, PromoSpec, Spec } from '../../src/spec';
import { BREAK, FIXTURES } from '../fixtures/specs';
import { load, openRender, pageErrors, renderTwice, sound } from './render';

// Two renders differ by float noise only: under 1e-5 on macOS, but Linux Chromium's
// convolution reverb is not bit-exact between renders (up to about 1.5e-4), so the bar is
// -60 dBFS, far below anything audible under a soundtrack.
const INAUDIBLE = 1e-3;

test.beforeEach(async ({ page }) => openRender(page));
test.afterEach(({ page }) => expect(pageErrors(page)).toEqual([]));

const CASES: [string, Spec, number][] = [
	['ident-journey', FIXTURES['ident-journey'], -18],
	['ident-cinematic', FIXTURES['ident-cinematic'], -18],
	['ident-morph', FIXTURES['ident-morph'], -18],
	['ident-assemble', FIXTURES['ident-assemble'], -18],
	['ident-short (the house instruments and hall)', FIXTURES['ident-short'], -18],
	['sting-5', { ...FIXTURES['sting-5'], loudness: -20 }, -20],
	['sting-10', FIXTURES['sting-10'], -18],
	['promo-bright (bed, beats)', FIXTURES['promo-bright'], -18],
	['strip-dark (bed, the days on its beats)', FIXTURES['strip-dark'], -18],
	['short-logo (the house music, no bed)', FIXTURES['short-logo'], -18],
	['squeeze-expand', { ...FIXTURES['squeeze-expand'], loudness: -23 }, -23],
	['break', BREAK, -20]
];

for (const [name, spec, target] of CASES) {
	test(`${name}: two renders agree and hit the loudness target`, async ({ page }) => {
		const info = await load(page, spec);
		const out = await renderTwice(page);
		expect(out.length).toBe(true);
		expect(out.frames).toBe(Math.round(info.duration * 48000));
		expect(out.maxDiff).toBeLessThan(INAUDIBLE);
		expect(out.report?.integrated).toBeCloseTo(target, 1);
		expect(out.report?.truePeak).toBeLessThanOrEqual(-1 + 1e-6);
	});
}

const RATE = 48000;
const level = (ch: Float32Array[], from: number, to: number) => {
	let e = 0;
	for (const c of ch) for (let i = from; i < to; i++) e += c[i] * c[i];
	return 10 * Math.log10(e / (ch.length * (to - from)) + 1e-30);
};
const slice = (ch: Float32Array[], from: number, to: number) => ch.map((c) => c.slice(from, to));

test("an item's sound is the clip's own at the engine's level, over where the clip ends", async ({
	page
}) => {
	const info = await load(page, FIXTURES.upnext);
	const own = await sound(page, 'renderAudio');
	const gain = (await page.evaluate(() => window.couchtvRender.audioReport()))!.gainDb;
	const item = await sound(page, 'renderItemAudio');
	const end = Math.round(info.soundDuration * RATE);
	expect(info.soundDuration).toBe(info.duration);
	expect(own[0].length).toBe(end);
	// the same sound at the engine's level (a render from 0 opens its compressors cold, so the
	// first half second is left out), with nothing ringing on under what follows
	const ownLoudness = integratedLoudness(slice(own, RATE / 2, end), RATE);
	const itemLoudness = integratedLoudness(slice(item, RATE / 2, end), RATE);
	expect(Math.abs(ownLoudness - (itemLoudness + gain))).toBeLessThan(0.05);
	expect(item[0].length).toBeLessThan(end + RATE / 10);
});

// A clip's sound is over before its last frame, with no step: it dies away (a slot's edge and
// the break's end cut whatever still sounds). The last 50 ms are silence and nothing rings on
// past the clip as an item. Where the house plays alone (`steady`) the 100 ms envelope of the
// last second also only falls (never up more than 3 dB: no swell, no stray note); a title's bed
// has beats of its own.
const ENDINGS: [string, Spec, boolean][] = [
	['the shortest sting', FIXTURES['sting-5'], true],
	['a sting of 6.2 s', { ...FIXTURES['sting-5'], duration: 6.2 }, true],
	['the longest sting', FIXTURES['sting-10'], true],
	['the short ident', FIXTURES['ident-short'], true],
	['up next over a bed', FIXTURES.upnext, false],
	['up next without one', FIXTURES['upnext-logo'], true],
	['the lineup', FIXTURES.lineup, true],
	['the squeeze into up next', FIXTURES['squeeze-expand'], false],
	['the squeeze into the short ident', { ...FIXTURES['squeeze-expand'], handover: 'ident' }, false],
	['the evergreen end card', FIXTURES.evergreen, false],
	['a strip promo', FIXTURES['strip-bright'], false],
	['a short promo on a bed', FIXTURES.short, false],
	['a short promo without one', FIXTURES['short-logo'], true]
];

for (const [name, spec, steady] of ENDINGS) {
	test(`${name}: its sound is over before its last frame`, async ({ page }) => {
		const info = await load(page, spec);
		const own = await sound(page, 'renderAudio');
		const end = Math.round(info.duration * RATE);
		expect(own[0].length).toBe(end);
		expect(level(own, end - RATE / 20, end)).toBeLessThan(-60);
		expect(Math.max(Math.abs(own[0][end - 1]), Math.abs(own[1][end - 1]))).toBeLessThan(1e-3);
		let prev = Infinity;
		for (let from = end - RATE; steady && from + RATE / 10 <= end; from += RATE / 10) {
			const now = level(own, from, from + RATE / 10);
			expect(now - Math.min(prev, now), `${from / RATE} s`).toBeLessThan(3);
			prev = now;
		}
		// as a break item nothing is left to ring on under what follows
		await load(page, spec, { sound: true });
		const item = await sound(page, 'renderItemAudio');
		expect(item[0].length).toBeLessThan(end + RATE / 10);
	});
}

test("a bulletin's sound starts and ends inside it: its slot cuts on both sides", async ({
	page
}) => {
	for (const name of ['weather', 'news'] as const) {
		const info = await load(page, FIXTURES[name]);
		const own = await sound(page, 'renderAudio');
		const end = Math.round(info.duration * RATE);
		expect(own[0].length).toBe(end);
		// silent at its first sample, and 50 dB under its body in its last 100 ms
		expect(Math.abs(own[0][0]) + Math.abs(own[1][0])).toBeLessThan(1e-4);
		const body = level(own, RATE, end - 2 * RATE);
		expect(body - level(own, end - RATE / 10, end), name).toBeGreaterThan(50);
	}
});

test('a promo sounds the same whatever airing its end card shows', async ({ page }) => {
	// nine promo renders
	test.slow();
	for (const mood of ['promo-cinematic', 'promo-grit-premiere', 'promo-bright'] as const) {
		const base = FIXTURES[mood] as PromoSpec;
		const renders: Float32Array[][] = [];
		const specs: ClipSpec[] = [
			{ ...base, part: 'body' },
			{ ...base, part: 'full' },
			{
				...base,
				part: 'full',
				airing: { eyebrow: 'Every Saturday', day: 'Saturday', time: '23:45' }
			}
		];
		for (const spec of specs) {
			await load(page, spec, { sound: true });
			renders.push(await sound(page, 'renderItemAudio'));
		}
		// the body's sound in a break is its full promo's, whenever that airs
		for (const r of renders.slice(1)) {
			expect(r[0].length, mood).toBe(renders[0][0].length);
			let diff = 0;
			for (let c = 0; c < 2; c++)
				for (let i = 0; i < r[c].length; i++)
					diff = Math.max(diff, Math.abs(r[c][i] - renders[0][c][i]));
			expect(diff, mood).toBeLessThan(INAUDIBLE);
		}
	}
});

test('items rendered alone and mixed sound like the break rendered in one pass', async ({
	page
}) => {
	const info = await load(page, BREAK);
	const whole = await sound(page, 'renderAudio');
	const frames = whole[0].length;
	expect(frames).toBe(Math.round(info.duration * RATE));
	const mix = [new Float32Array(frames), new Float32Array(frames)];
	const starts = BREAK.items.map((it) => Math.round(it.at * RATE));
	for (const [i, it] of BREAK.items.entries()) {
		const item = await load(page, it.spec, { sound: true });
		const next = starts[i + 1] ?? frames;
		expect(Math.round(item.soundDuration * RATE)).toBe(next - starts[i]);
		const ch = await sound(page, 'renderItemAudio');
		for (let c = 0; c < 2; c++)
			for (let k = 0; k < ch[c].length && starts[i] + k < frames; k++)
				mix[c][starts[i] + k] += ch[c][k];
	}
	const report = normalize(mix, RATE, BREAK.loudness ?? -18);
	// the same loudness, item by item and moment by moment, and the same peak: the glue and
	// the limiter now run per item, and a voice's phase may land a sample apart, as it did
	// wherever a break put it. (The one render opened its compressors cold, so its first
	// second is left out.)
	expect(report.integrated).toBeCloseTo(-20, 3);
	expect(Math.abs(integratedLoudness(whole, RATE) - report.integrated)).toBeLessThan(0.1);
	expect(Math.abs(truePeak(whole) - report.truePeak)).toBeLessThan(0.2);
	// (an item ending in a decay has blocks at the loudness gate's edge, which one render lets
	// through and the other not: 0.2 LU for a whole item, the moments below hold it closer)
	starts.forEach((from, i) => {
		const to = starts[i + 1] ?? frames;
		const a = integratedLoudness(slice(whole, from, to), RATE);
		const b = integratedLoudness(slice(mix, from, to), RATE);
		expect(Math.abs(a - b), `item ${i}`).toBeLessThan(0.2);
	});
	for (let from = RATE; from + 0.4 * RATE <= frames; from += RATE / 10) {
		const a = integratedLoudness(slice(whole, from, from + 0.4 * RATE), RATE);
		const b = integratedLoudness(slice(mix, from, from + 0.4 * RATE), RATE);
		if (a > report.integrated - 20) expect(Math.abs(a - b), `${from / RATE} s`).toBeLessThan(0.5);
	}
});

test('the standby loop has no seam', async ({ page }) => {
	await load(page, FIXTURES.standby);
	const out = await renderTwice(page);
	expect(out.maxDiff).toBeLessThan(INAUDIBLE);
	expect(out.seam).toBeLessThan(out.meanStep * 4);
});
