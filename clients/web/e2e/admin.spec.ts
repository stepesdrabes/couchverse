import { authFile, draft, expect, movie, series, t, test } from './fixtures';

test.use({ storageState: authFile('admin') });

test('dashboard, library, title editor and settings render', async ({ page }) => {
	const sidebar = page.getByRole('navigation');
	const heading = (name: string) => page.getByRole('heading', { level: 1, name, exact: true });

	await page.goto('/admin');
	await expect(heading(t('admin_nav_overview'))).toBeVisible();
	await expect(page.getByRole('heading', { name: t('admin_live') })).toBeVisible();
	await expect(page.getByRole('heading', { name: t('admin_system') })).toBeVisible();

	await sidebar.getByRole('link', { name: t('admin_nav_library') }).click();
	await expect(heading(t('library_heading'))).toBeVisible();
	const table = page.getByRole('table');
	await expect(table.getByRole('link', { name: movie.name })).toBeVisible();
	await expect(table.getByRole('link', { name: series.name })).toBeVisible();
	await expect(table.getByRole('link', { name: draft.name })).toBeVisible();

	await table.getByRole('link', { name: movie.name }).click();
	await expect(page).toHaveURL(`/admin/library/${movie.id}`);
	await expect(heading(movie.name)).toBeVisible();
	await expect(page.getByRole('heading', { name: t('library_metadata') })).toBeVisible();
	await expect(page.getByLabel(t('library_year'))).toHaveValue('2025');

	await sidebar.getByRole('link', { name: t('admin_nav_settings') }).click();
	await expect(heading(t('settings_heading'))).toBeVisible();

	for (const [link, title] of [
		['admin_nav_users', 'users_heading'],
		['admin_nav_jobs', 'jobs_heading'],
		['admin_nav_ranks', 'admin_nav_ranks']
	]) {
		await sidebar.getByRole('link', { name: t(link) }).click();
		await expect(heading(t(title))).toBeVisible();
	}
});
