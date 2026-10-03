import { authFile, expect, movie, row, t, test } from './fixtures';

// vera, so the saved language preference never leaks into specs signed in as nora
test.use({ storageState: authFile('vera') });

test('switching to Czech translates the UI and the catalog, and back', async ({ page }) => {
	const nav = page.getByRole('navigation');
	const recent = (lang: 'en' | 'cs') => row(page, t('home_row_recently_added', {}, lang));

	// the choice is saved to the account, so a retry starts from English again
	await page.request.put('/api/v1/me/preferences', { data: { language: 'en' } });
	await page.goto('/');
	await expect(nav.getByRole('link', { name: t('nav_movies') })).toBeVisible();
	await expect(recent('en').getByRole('link', { name: movie.name })).toBeVisible();

	await page.getByRole('button', { name: t('language_label') }).click();
	await page.getByRole('menuitem', { name: 'Čeština' }).click();

	await expect(nav.getByRole('link', { name: t('nav_movies', {}, 'cs') })).toBeVisible();
	await expect(nav.getByRole('link', { name: t('nav_my_list', {}, 'cs') })).toBeVisible();
	await expect(recent('cs').getByRole('link', { name: movie.nameCs })).toBeVisible();

	await recent('cs').getByRole('link', { name: movie.nameCs }).click();
	await expect(page.getByRole('heading', { level: 1, name: movie.nameCs })).toBeVisible();
	await expect(page.getByText(movie.overviewCs)).toBeVisible();
	await expect(page.getByRole('button', { name: t('common_play', {}, 'cs') })).toBeVisible();
	// the title's wordmark in the display language stands in for its name
	const logo = (name: string) => page.getByRole('heading', { level: 1, name }).locator('img');
	await expect(logo(movie.nameCs)).toHaveAttribute('src', new RegExp(movie.logos.cs));

	await page.getByRole('button', { name: t('language_label', {}, 'cs') }).click();
	await page.getByRole('menuitem', { name: 'English' }).click();

	await expect(page.getByRole('heading', { level: 1, name: movie.name })).toBeVisible();
	await expect(page.getByText(movie.overview)).toBeVisible();
	await expect(logo(movie.name)).toHaveAttribute('src', new RegExp(movie.logos.en));
	await expect(nav.getByRole('link', { name: t('nav_movies') })).toBeVisible();
});
