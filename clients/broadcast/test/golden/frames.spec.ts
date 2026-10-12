import { expect, test } from '@playwright/test';
import type { ClipSpec, PromoSpec } from '../../src/spec';
import { FIXTURES, type FixtureName } from '../fixtures/specs';
import { load, openRender, pageErrors, seek } from './render';

// chosen frames of every template (seconds into the clip)
const GOLDEN: [FixtureName, number[], ('picture' | 'matte')?][] = [
	['ident-full', [6.8]],
	['ident-short', [3]],
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
	['standby', [2]]
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
	expect(await page.evaluate(() => window.couchtvRender.keyRect(10.9))).toBeNull();
});
