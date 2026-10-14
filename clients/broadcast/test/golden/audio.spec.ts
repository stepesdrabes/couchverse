import { expect, test } from '@playwright/test';
import { integratedLoudness, normalize, truePeak } from '../../src/audio/loudness';
import type { ClipSpec, PromoSpec, Spec, TrailerSpec } from '../../src/spec';
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
	['break', BREAK, -20],
	['weather (the pad through its scenes, glass, air)', FIXTURES.weather, -18],
	['news (an ostinato per story, the recap, the sign-off)', FIXTURES.news, -18]
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
	['a sting of 8 s', { ...FIXTURES['sting-5'], duration: 8 }, true],
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
	['a short promo without one', FIXTURES['short-logo'], true],
	// the channel tag rings out on the mix's own gain: a trailer rides the mix and a bulletin fades
	// it, and the browser refuses a ring-out curve over either
	['a promo on a bed', FIXTURES['promo-bright'], false],
	['a trailer over a bed', FIXTURES['trailer-cinematic'], false],
	['a trailer scored by the house alone', FIXTURES['trailer-house'], false],
	['a bright trailer', FIXTURES['trailer-bright-premiere'], false],
	['a grit trailer', FIXTURES['trailer-grit'], false],
	['the weather', FIXTURES.weather, true],
	['the news', FIXTURES.news, true]
];

for (const [name, spec, steady] of ENDINGS) {
	test(`${name}: its sound is over before its last frame`, async ({ page }) => {
		// the trailers and the bulletins are long, and rendered twice
		test.slow();
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

// A long sting holds its chord through the hold: the sound is never gone from the lockup that holds
// still, and falls over the last two seconds or so (the 400 ms loudness stays above -35 LUFS, the
// clip at -18, until two seconds before its end).
for (const seconds of [7, 8, 9, 10]) {
	test(`a sting of ${seconds} s holds its chord to two seconds before its end`, async ({
		page
	}) => {
		const info = await load(page, { ...FIXTURES['sting-5'], duration: seconds });
		const own = await sound(page, 'renderAudio');
		const end = Math.round(info.duration * RATE);
		const window = slice(own, end - Math.round(2.4 * RATE), end - 2 * RATE);
		expect(integratedLoudness(window, RATE)).toBeGreaterThan(-35);
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

// How much a pitch jumps (dB) across `at`: its power in the 200 ms after the onset against the
// 200 ms before it, by a windowed Goertzel sum.
const jump = (mono: Float32Array, hz: number, at: number) => {
	const power = (from: number, to: number) => {
		const a = Math.round(from * RATE);
		const n = Math.round((to - from) * RATE);
		let re = 0;
		let im = 0;
		for (let i = 0; i < n; i++) {
			const w = 0.5 - 0.5 * Math.cos((2 * Math.PI * i) / n);
			const phase = (2 * Math.PI * hz * i) / RATE;
			re += mono[a + i] * w * Math.cos(phase);
			im -= mono[a + i] * w * Math.sin(phase);
		}
		return (re * re + im * im) / (n * n);
	};
	return 10 * Math.log10(power(at + 0.02, at + 0.22) / power(at - 0.25, at - 0.05));
};

// A break that carries both bulletins plays the news and, some 40 s later, the weather: the sonic
// logo's motif (A4 E5 D5, struck on the beats after the tag lands) ends the news only, and the
// weather's tag lands on its own closing chord.
test("the news's tag sounds the motif and the weather's does not", async ({ page }) => {
	test.slow();
	const struck: Record<string, number> = {};
	for (const name of ['news', 'weather'] as const) {
		const info = await load(page, FIXTURES[name]);
		const land = info.cues.find((c) => c.name.startsWith('channel tag'))?.at ?? NaN;
		const [l, r] = await sound(page, 'renderAudio');
		const mono = l.map((v, i) => (v + r[i]) / 2);
		// A4 on the first beat after the landing and D5 on the last, E5 between
		const notes: [number, number][] = [
			[440, 0.4],
			[659.26, 0.8],
			[587.33, 1.2]
		];
		const jumps = notes.map(([hz, dt]) => jump(mono, hz, land + dt));
		struck[name] = jumps.filter((j, i) => i !== 1 && j >= 8).length;
	}
	expect(struck.news).toBe(2);
	expect(struck.weather).toBeLessThan(2);
});

// A pad chord once ticked: an A suspended (A2 E3 D4 E4) stepped by a twentieth of the music's
// level 4.4 s in, at that moment whatever the voices' seeds, and a spectrogram hardly shows it.
// The largest second difference of a bulletin's mix stays within the music's own roughness.
test("a bulletin's score has no clicks: no step stands out of the music's own texture", async ({
	page
}) => {
	for (const name of ['weather', 'news'] as const) {
		await load(page, FIXTURES[name]);
		const [l, r] = await sound(page, 'renderAudio');
		let step = 0;
		let power = 0;
		for (let i = 2; i < l.length; i++) {
			const m = (l[i] + r[i]) / 2;
			step = Math.max(
				step,
				Math.abs(m - ((l[i - 1] + r[i - 1]) / 2) * 2 + (l[i - 2] + r[i - 2]) / 2)
			);
			power += m * m;
		}
		expect(step / Math.sqrt(power / l.length), name).toBeLessThan(0.33);
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

// the biggest step between two samples of either channel in [from, to)
const step = (ch: Float32Array[], from: number, to: number) => {
	let max = 0;
	for (const c of ch)
		for (let i = Math.max(1, from); i < Math.min(c.length, to); i++)
			max = Math.max(max, Math.abs(c[i] - c[i - 1]));
	return max;
};

test('a trailer sounds the same twice, at its loudness, with clean cuts and a quiet end', async ({
	page
}) => {
	// two long renders of each
	test.slow();
	for (const name of ['trailer-cinematic', 'trailer-house'] as const) {
		const info = await load(page, FIXTURES[name]);
		const own = await sound(page, 'renderAudio');
		const report = await page.evaluate(() => window.couchtvRender.audioReport());
		const again = await sound(page, 'renderAudio');
		expect(own[0].length, name).toBe(Math.round(info.duration * RATE));
		let diff = 0;
		for (let c = 0; c < 2; c++)
			for (let i = 0; i < own[c].length; i++)
				diff = Math.max(diff, Math.abs(own[c][i] - again[c][i]));
		expect(diff, name).toBeLessThan(INAUDIBLE);
		expect(report?.integrated, name).toBeCloseTo(-18, 0);
		expect(report?.truePeak, name).toBeLessThanOrEqual(-1 + 1e-6);
		// no click where one scene's sound meets the next: the step at a cut is what the sound
		// around it does, not more than a few times that
		const ms3 = Math.round(0.003 * RATE);
		for (const cue of info.cues.filter((c) =>
			['scene', 'the cold open', 'the button'].includes(c.name)
		)) {
			const at = Math.round(cue.at * RATE);
			const around: number[] = [];
			for (let k = -50; k < 50; k++)
				if (k < -1 || k > 0) around.push(step(own, at + k * ms3, at + (k + 1) * ms3));
			around.sort((a, b) => a - b);
			const typical = around[Math.floor(around.length / 2)];
			expect(step(own, at - ms3, at + ms3), `${name} cut at ${cue.at}`).toBeLessThanOrEqual(
				4 * typical + 0.002
			);
		}
		// it opens from silence, and rings out: the last 100 ms is well under the body
		const frames = own[0].length;
		expect(Math.abs(own[0][0]) + Math.abs(own[1][0]), name).toBeLessThan(1e-3);
		const body = level(own, 5 * RATE, frames - 5 * RATE);
		expect(body - level(own, frames - RATE / 10, frames), name).toBeGreaterThan(12);
	}
});

// The review found the trailer's music 24 dB under its dialogue, no musical line for the first
// half (the soundtrack came in 35 to 50 s on, the house's underscore sat 24 dB under the lines and
// fell to -26..-33 LUFS at the cards and the channel's moment) and the title's hit 5 LU over the
// loudness target. The music alone (the scenes' sound muted) now never falls out of hearing from
// the channel's moment to the hit, and the hit stays within 4 LU of the target.
test('a trailer has a musical line under its story and a hit within 4 LU of its loudness', async ({
	page
}) => {
	test.slow();
	const short = (ch: Float32Array[], at: number) =>
		integratedLoudness(slice(ch, Math.round(at * RATE), Math.round((at + 3) * RATE)), RATE);
	for (const name of ['trailer-cinematic', 'trailer-grit'] as const) {
		const spec = FIXTURES[name] as TrailerSpec;
		const muted: TrailerSpec = {
			...spec,
			scenes: spec.scenes.map((s) => ({ ...s, sound: { ...s.sound, gain: 0 } }))
		};
		const info = await load(page, spec);
		const full = await sound(page, 'renderAudio');
		const cue = (n: string) => info.cues.find((c) => c.name === n)?.at ?? NaN;
		await load(page, muted);
		const music = await sound(page, 'renderAudio');
		// the music alone, three seconds at a time (normalized to -18 like the trailer; it fell to
		// -44 between the cards before)
		const levels: number[] = [];
		for (let t = cue('the channel') + 1.5; t <= cue('the title') - 3; t++)
			levels.push(short(music, t));
		expect(Math.min(...levels), name).toBeGreaterThan(-34);
		// the loudest three seconds of the whole trailer, which is the hit
		let loudest = -Infinity;
		for (let t = 0; t + 3 <= full[0].length / RATE; t++)
			loudest = Math.max(loudest, short(full, t));
		expect(loudest, name).toBeLessThanOrEqual(-18 + 4.2);
	}
});

test('a trailer sounds the same whatever airing its end card shows', async ({ page }) => {
	test.slow();
	const base = FIXTURES['trailer-grit'] as TrailerSpec;
	const renders: Float32Array[][] = [];
	for (const spec of [
		{ ...base, part: 'body' },
		{ ...base, part: 'full' },
		{
			...base,
			part: 'full',
			airing: { eyebrow: 'Every Saturday', day: 'Saturday', time: '23:45' }
		}
	] satisfies ClipSpec[]) {
		await load(page, spec, { sound: true });
		renders.push(await sound(page, 'renderItemAudio'));
	}
	// the body's sound in a break is its full trailer's, whenever that airs
	for (const r of renders.slice(1)) {
		expect(r[0].length).toBe(renders[0][0].length);
		let diff = 0;
		for (let c = 0; c < 2; c++)
			for (let i = 0; i < r[c].length; i++)
				diff = Math.max(diff, Math.abs(r[c][i] - renders[0][c][i]));
		expect(diff).toBeLessThan(INAUDIBLE);
	}
});
