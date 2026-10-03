import AxeBuilder from '@axe-core/playwright';
import type { Page } from '@playwright/test';
import { authFile, expect, movie, t, test } from './fixtures';

// WCAG 2.1 A and AA, as axe checks them on the rendered page
const TAGS = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'];

/** fails on any serious or critical violation, listing each rule with the elements it hit */
async function audit(page: Page) {
	// an element still fading in reads as low contrast
	await page.waitForFunction(() =>
		document
			.getAnimations()
			.every((a) => a.playState !== 'running' || a.effect?.getTiming().iterations === Infinity)
	);
	const results = await new AxeBuilder({ page }).withTags(TAGS).analyze();
	const blocking = results.violations
		.filter((v) => v.impact === 'serious' || v.impact === 'critical')
		.map((v) => `${v.id}: ${v.help} (${v.nodes.map((n) => n.target.join(' ')).join(', ')})`);
	expect(blocking, `accessibility violations on ${page.url()}`).toEqual([]);
}

test('the sign-in page is accessible', async ({ page, errors }) => {
	errors.allow(/status of 401 /);
	await page.goto('/login');
	await expect(page.getByLabel(t('login_username'))).toBeVisible();
	await audit(page);
});

test.describe('viewer pages', () => {
	test.use({ storageState: authFile('nora') });

	test('home is accessible', async ({ page }) => {
		await page.goto('/');
		await expect(page.getByRole('region', { name: t('catalog_featured_titles') })).toBeVisible();
		await audit(page);
	});

	for (const [path, heading] of [
		['/movies', 'nav_movies'],
		['/genres', 'nav_genres'],
		['/my-list', 'nav_my_list']
	] as const) {
		test(`${path} is accessible`, async ({ page }) => {
			await page.goto(path);
			await expect(page.getByRole('heading', { level: 1, name: t(heading) })).toBeVisible();
			await audit(page);
		});
	}

	test('a title page is accessible', async ({ page }) => {
		await page.goto(`/title/${movie.slug}`);
		await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
		await audit(page);
	});

	test('search is accessible', async ({ page }) => {
		await page.goto('/search');
		await page.getByRole('searchbox', { name: t('nav_search') }).fill('glass');
		await expect(page.getByRole('link', { name: movie.name }).first()).toBeVisible();
		await audit(page);
	});

	// one audit each: a profile is the largest page there is, and WebKit takes most of a
	// test's time to audit it
	test('a profile is accessible', async ({ page }) => {
		await page.goto('/u/nora');
		await expect(page.getByRole('heading', { level: 1 }).first()).toBeVisible();
		await audit(page);
	});

	test('the leaderboard is accessible', async ({ page }) => {
		await page.goto('/leaderboard');
		await expect(page.getByRole('table')).toBeVisible();
		await audit(page);
	});
});

test.describe('admin pages', () => {
	test.use({ storageState: authFile('admin') });

	for (const [path, heading] of [
		['/admin', 'admin_nav_overview'],
		['/admin/library', 'library_heading'],
		['/admin/settings', 'settings_heading'],
		['/admin/users', 'users_heading'],
		['/admin/jobs', 'jobs_heading']
	] as const) {
		test(`${path} is accessible`, async ({ page }) => {
			await page.goto(path);
			await expect(page.getByRole('heading', { level: 1, name: t(heading) })).toBeVisible();
			await audit(page);
		});
	}
});
