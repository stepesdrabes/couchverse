import { authFile, expect, t, test } from './fixtures';

test.use({ storageState: authFile('nora') });

test('profile shows the member with their markdown bio', async ({ page }) => {
	await page.goto('/profile');
	await expect(page.getByRole('heading', { level: 1, name: 'Nora' })).toBeVisible();
	await expect(page.getByText('@nora')).toBeVisible();
	await expect(page.getByText('everything', { exact: true })).toHaveJSProperty('tagName', 'STRONG');

	await page.goto('/u/nora');
	await expect(page.getByRole('heading', { level: 1, name: 'Nora' })).toBeVisible();
});

test('leaderboard lists public members only', async ({ page }) => {
	await page.goto('/');
	await page.getByRole('link', { name: t('nav_leaderboard') }).click();
	await expect(
		page.getByRole('heading', { level: 1, name: t('leaderboard_heading') })
	).toBeVisible();

	const table = page.getByRole('table');
	await expect(table.getByRole('link', { name: 'Nora' })).toBeVisible();
	await expect(table.getByText('@nora')).toBeVisible();
	await expect(page.getByText('@piet')).toHaveCount(0);
});

test('a private member has no public profile', async ({ page, errors }) => {
	errors.allow(/status of 404 .*\/api\/v1\/users\/piet\/profile\b/);
	await page.goto('/u/piet');
	await expect(page.getByText(t('profiles_not_found'))).toBeVisible();
	await expect(page.getByRole('heading', { name: 'Piet' })).toHaveCount(0);
});
