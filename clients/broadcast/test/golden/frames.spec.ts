import { expect, test } from '@playwright/test';
import { METRICS } from '../../src/brand/wordmark';
import type { ClipSpec, PromoSpec, SqueezeSpec } from '../../src/spec';
import { FIXTURES, type FixtureName } from '../fixtures/specs';
import { load, openRender, pageErrors, seek } from './render';

// chosen frames of every template (seconds into the clip)
const GOLDEN: [FixtureName, number[], ('picture' | 'matte')?][] = [
	['ident-full', [1.72, 2.2, 9]],
	['ident-short', [1.12, 3.4]],
	['ident-bare', [2.2]],
	['sting-2', [1.2]],
	['sting-3', [2]],
	['sting-5', [3]],
	['upnext', [2.5, 7.6]],
	['promo-cinematic', [6, 13.5, 19.5]],
	['promo-grit-premiere', [4, 14.6]],
	['promo-bright', [1.5, 10.8, 15.5]],
	['evergreen', [2.5]],
	['strip-bright', [13]],
	['strip-dark', [7.5]],
	['short', [3.5]],
	['lineup', [6]],
	['squeeze-expand', [5]],
	['squeeze-key', [5]],
	['squeeze-key', [2.6], 'matte'],
	['squeeze-frames', [1]],
	['standby', [2]],
	// the longest names and lines: fitted, broken onto lines, never off the frame
	['promo-cinematic-long', [3.4, 7, 16, 20]],
	['promo-grit-long', [4, 6.5, 12.9, 16]],
	['promo-bright-long', [2.4, 5, 8.4, 12.8, 17]],
	['evergreen-long', [3]],
	['upnext-long', [3]],
	['strip-long', [8]],
	['short-long', [4.2]],
	['lineup-long', [4.4, 7]],
	['squeeze-long', [5]],
	['weather', [3]],
	['weather-quiet', [3]],
	['weather-unread', [3]],
	['news', [3]],
	['news-three', [3]],
	// every default city and a warning of each level; the longest headlines
	['weather-long', [3]],
	['news-long', [3]]
];

test.beforeEach(async ({ page }) => openRender(page));
test.afterEach(({ page }) => expect(pageErrors(page)).toEqual([]));

for (const [name, times, layer] of GOLDEN) {
	test(`${name}${layer ? ` (${layer})` : ''}`, async ({ page }) => {
		const info = await load(page, FIXTURES[name]);
		expect(info.missing).toEqual([]);
		expect(Math.abs(info.duration * 25 - Math.round(info.duration * 25))).toBeLessThan(1e-6);
		if (layer) await page.evaluate((l) => window.couchtvRender.setLayer(l), layer);
		for (const t of times) {
			await seek(page, t);
			await expect(page).toHaveScreenshot(`${name}${layer ? `-${layer}` : ''}-${t.toFixed(2)}.png`);
		}
	});
}

test('text stays inside the title-safe width and on the frame', async ({ page }) => {
	for (const [name, times, layer] of GOLDEN) {
		if (layer) continue;
		await load(page, FIXTURES[name]);
		for (const t of times) {
			await seek(page, t);
			const outside = await page.evaluate(() => {
				const stage = document.getElementById('stage')?.shadowRoot?.querySelector('.stage');
				const piece = stage?.querySelector('.piece');
				if (!stage || !piece) return ['no piece'];
				const out: string[] = [];
				const range = document.createRange();
				const walker = document.createTreeWalker(piece, NodeFilter.SHOW_TEXT);
				for (let n = walker.nextNode(); n; n = walker.nextNode()) {
					const el = n.parentElement;
					// the corner bug sits in the action-safe corner by design
					if (!el || !n.textContent?.trim() || el.closest('.bug')) continue;
					if (getComputedStyle(el).visibility === 'hidden') continue;
					let opacity = 1;
					const clips: DOMRect[] = [];
					for (let a: Element | null = el; a && a !== piece; a = a.parentElement) {
						const s = getComputedStyle(a);
						opacity *= parseFloat(s.opacity);
						if (s.overflow !== 'visible') clips.push(a.getBoundingClientRect());
					}
					// only what reads: the tail of a fade does not
					if (opacity < 0.5) continue;
					range.selectNodeContents(n);
					for (const r of range.getClientRects()) {
						const box = clips.reduce(
							(b, c) => ({
								l: Math.max(b.l, c.left),
								r: Math.min(b.r, c.right),
								t: Math.max(b.t, c.top),
								b: Math.min(b.b, c.bottom)
							}),
							{ l: r.left, r: r.right, t: r.top, b: r.bottom }
						);
						if (box.r - box.l < 1 || box.b - box.t < 1) continue;
						if (box.l < 95.5 || box.r > 1824.5 || box.t < 0 || box.b > 1080)
							out.push(`${n.textContent} [${Math.round(box.l)}..${Math.round(box.r)}]`);
					}
				}
				return out;
			});
			expect(outside, `${name} at ${t}`).toEqual([]);
		}
	}
});

test('versions name every template as load reports it', async ({ page }) => {
	const versions = await page.evaluate(() => window.couchtvRender.versions());
	for (const name of ['ident-short', 'upnext', 'promo-bright', 'lineup', 'standby'] as const) {
		const spec = FIXTURES[name];
		expect((await load(page, spec)).version, name).toBe(versions[spec.template]);
	}
});

test('a frame is a function of t alone', async ({ page }) => {
	for (const [name, t] of [
		['promo-cinematic', 6.2],
		['lineup', 4.4],
		['squeeze-frames', 0.6]
	] as const) {
		await load(page, FIXTURES[name]);
		await seek(page, t);
		const first = await page.screenshot();
		await seek(page, t + 2.3);
		await seek(page, 0);
		await seek(page, t);
		expect((await page.screenshot()).equals(first), name).toBe(true);
	}
});

test("the renderer's step shows the frame seek does and skips held ones", async ({ page }) => {
	const info = await load(page, FIXTURES['ident-short']);
	const frames = Math.round(info.duration * 25);
	const held = await page.evaluate((n) => {
		for (let i = 1; i < n; i++)
			if (window.couchtvRender.isStatic((i - 1) / 25, i / 25)) return i / 25;
		return null;
	}, frames);
	// the lockup holds before the short ident ends
	expect(held).not.toBeNull();
	if (held === null) return;
	expect(await page.evaluate((t) => window.couchtvRender.advance(t, t - 0.04), held)).toBe(false);
	for (const t of [0.6, held]) {
		await seek(page, t);
		const seeked = await page.screenshot();
		await seek(page, 0);
		expect(await page.evaluate((x) => window.couchtvRender.advance(x, -1), t)).toBe(true);
		expect((await page.screenshot()).equals(seeked), `t=${t}`).toBe(true);
	}
});

test('a promo body does not depend on the airing', async ({ page }) => {
	for (const mood of ['promo-cinematic', 'promo-grit-premiere', 'promo-bright'] as const) {
		const base = FIXTURES[mood] as PromoSpec;
		const shots: Buffer[] = [];
		for (const airing of [
			base.airing,
			{ eyebrow: 'Every Saturday', day: 'Saturday', time: '23:45' }
		]) {
			const info = await load(page, { ...base, part: 'body', airing } satisfies ClipSpec);
			await seek(page, info.duration - 0.04);
			shots.push(await page.screenshot());
		}
		expect(shots[0].equals(shots[1]), mood).toBe(true);
	}
});

test('the end card follows the body on the next frame', async ({ page }) => {
	const base = FIXTURES['promo-cinematic'] as PromoSpec;
	const full = await load(page, { ...base, part: 'full' });
	const body = await load(page, { ...base, part: 'body' });
	const end = await load(page, { ...base, part: 'end' });
	expect(body.duration + end.duration).toBeCloseTo(full.duration, 6);
	expect(end.duration).toBeCloseTo(4, 6);
	await seek(page, 0);
	const endFirst = await page.screenshot();
	await load(page, { ...base, part: 'full' });
	await seek(page, body.duration);
	expect((await page.screenshot()).equals(endFirst)).toBe(true);
});

test('the key rectangle follows the program panel', async ({ page }) => {
	await load(page, FIXTURES['squeeze-key']);
	for (const t of [0.5, 2.4, 2.9, 5, 9.6, 10.0]) {
		await seek(page, t);
		const { rect, box } = await page.evaluate((x) => {
			const panel = document.getElementById('stage')?.shadowRoot?.querySelector('.program');
			const r = panel?.getBoundingClientRect();
			return {
				rect: window.couchtvRender.keyRect(x),
				box: r ? { x: r.x, y: r.y, width: r.width, height: r.height } : null
			};
		}, t);
		expect(rect, `t=${t}`).not.toBeNull();
		if (!rect || !box) continue;
		for (const k of ['x', 'y', 'width', 'height'] as const)
			expect(Math.abs(rect[k] - box[k]), `${k} at ${t}`).toBeLessThan(1.5);
	}
	expect(await page.evaluate(() => window.couchtvRender.keyRect(10.56))).toBeNull();
});

test("the squeeze hands over on the short ident's first frame", async ({ page }) => {
	const squeeze: SqueezeSpec = {
		...(FIXTURES['squeeze-expand'] as SqueezeSpec),
		handover: 'cursor'
	};
	const info = await load(page, squeeze);
	expect(info.duration).toBeCloseTo(10.6, 6);
	await seek(page, info.duration - 0.04);
	const last = await page.screenshot();
	await load(page, FIXTURES['ident-short']);
	await seek(page, 0);
	expect((await page.screenshot()).equals(last)).toBe(true);
});

test('the wordmark is laid out from the bundled display face', async ({ page }) => {
	await load(page, FIXTURES['ident-short']);
	const measured = await page.evaluate(() => {
		const g = document.createElement('canvas').getContext('2d') as CanvasRenderingContext2D;
		g.font = '700 1000px "Couch TV Display"';
		const width = Object.fromEntries(
			['C', 'O', 'U', 'H', 'T', 'V'].map((ch) => [ch, g.measureText(ch).width / 1000 - 0.004])
		);
		const box = document.createElement('div');
		box.style.cssText = 'position:absolute;font:700 1000px "Couch TV Display";line-height:1';
		const probe = document.createElement('span');
		probe.style.cssText = 'display:inline-block;width:0;height:0';
		box.append('O', probe);
		document.body.append(box);
		const baseline = (probe.getBoundingClientRect().top - box.getBoundingClientRect().top) / 1000;
		box.remove();
		return { width, baseline };
	});
	for (const [ch, w] of Object.entries(METRICS.width))
		expect(Math.abs(measured.width[ch] - w), ch).toBeLessThan(5e-4);
	expect(measured.baseline).toBeCloseTo(METRICS.baseline, 3);
});
