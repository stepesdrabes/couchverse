import { expect, test } from '@playwright/test';
import type { Spec } from '../../src/spec';
import { BREAK, FIXTURES } from '../fixtures/specs';
import { load, openRender, pageErrors, renderTwice } from './render';

test.beforeEach(async ({ page }) => openRender(page));
test.afterEach(({ page }) => expect(pageErrors(page)).toEqual([]));

const CASES: [string, Spec, number][] = [
	['ident-full', FIXTURES['ident-full'], -18],
	['promo-bright (bed, beats)', FIXTURES['promo-bright'], -18],
	['squeeze-expand', { ...FIXTURES['squeeze-expand'], loudness: -23 }, -23],
	['break', BREAK, -20]
];

for (const [name, spec, target] of CASES) {
	test(`${name}: two renders agree and hit the loudness target`, async ({ page }) => {
		const info = await load(page, spec);
		const out = await renderTwice(page);
		expect(out.length).toBe(true);
		expect(out.frames).toBe(Math.round(info.duration * 48000));
		// float noise between renders, nothing audible
		expect(out.maxDiff).toBeLessThan(1e-5);
		expect(out.report?.integrated).toBeCloseTo(target, 1);
		expect(out.report?.truePeak).toBeLessThanOrEqual(-1 + 1e-6);
	});
}

test('the standby loop has no seam', async ({ page }) => {
	await load(page, FIXTURES.standby);
	const out = await renderTwice(page);
	expect(out.maxDiff).toBeLessThan(1e-5);
	expect(out.seam).toBeLessThan(out.meanStep * 4);
});
