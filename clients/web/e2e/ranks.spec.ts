import { authFile, expect, PASSWORD, t, test } from './fixtures';

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

// a 1x1 PNG, for a profile picture
const PNG = Buffer.from(
	'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==',
	'base64'
);

test.describe(() => {
	test.use({ storageState: authFile('admin') });

	test('the profile editor saves through the core and the change shows everywhere', async ({
		page,
		browser,
		errors
	}) => {
		errors.allow(/status of 400 .*\/api\/v1\/me\/password\b/);
		// a member of their own: other specs, running alongside, show the seeded members' names
		const username = `editor-${test.info().project.name}-${test.info().retry}`;
		const created = await page.request.post('/api/v1/admin/users', {
			data: { username, password: PASSWORD }
		});
		expect(created.ok()).toBe(true);

		const context = await browser.newContext({ storageState: { cookies: [], origins: [] } });
		errors.watch(context);
		const member = await context.newPage();
		const login = await member.request.post('/api/v1/auth/login', {
			data: { username, password: PASSWORD }
		});
		expect(login.ok()).toBe(true);

		const name = `Editor ${test.info().project.name} ${test.info().retry}`;
		await member.goto('/profile');
		await member.getByRole('button', { name: t('profiles_edit_profile') }).click();
		const dialog = member.getByRole('dialog');
		await dialog.getByLabel(t('profile_display_name')).fill(name);
		await dialog.getByRole('button', { name: t('common_save') }).click();
		await expect(member.getByText(t('profile_saved'))).toBeVisible();
		await expect(member.getByRole('heading', { level: 1, name })).toBeVisible();

		// the picked picture stays in the page; the core asks for its upload by a handle
		await member.getByRole('button', { name: t('profiles_edit_profile') }).click();
		await member
			.locator('input[type="file"]')
			.first()
			.setInputFiles({ name: 'me.png', mimeType: 'image/png', buffer: PNG });
		await expect(member.getByText(t('profile_picture_updated'))).toBeVisible();
		// the nav shows the new name and picture as well
		const menu = member.getByRole('button', { name: t('nav_account_menu') });
		await expect(menu.getByRole('img', { name })).toHaveAttribute('src', /\/api\/v1\/artwork\//);
		await dialog.getByRole('button', { name: t('common_cancel') }).click();

		// a refused change says why, in the display language
		await member.getByRole('button', { name: t('profile_password_heading') }).click();
		await dialog
			.getByLabel(t('profile_current_password'), { exact: true })
			.fill('not-the-password');
		await dialog.getByLabel(t('profile_new_password'), { exact: true }).fill('long-enough');
		await dialog.getByLabel(t('profile_confirm_password'), { exact: true }).fill('long-enough');
		await dialog.getByRole('button', { name: t('profile_password_heading') }).click();
		await expect(member.getByText(t('problem_invalid_password'))).toBeVisible();
		await context.close();
	});

	test('a badge earned between visits celebrates on the next one', async ({
		page,
		browser,
		errors
	}) => {
		// a member of their own, whose first visit is the first look at their badges
		const username = `badge-${test.info().project.name}-${test.info().retry}`;
		const created = await page.request.post('/api/v1/admin/users', {
			data: { username, password: PASSWORD }
		});
		expect(created.ok()).toBe(true);

		const context = await browser.newContext({ storageState: { cookies: [], origins: [] } });
		errors.watch(context);
		const member = await context.newPage();
		const login = await member.request.post('/api/v1/auth/login', {
			data: { username, password: PASSWORD }
		});
		expect(login.ok()).toBe(true);
		// a first picture, uploaded from another device
		const avatar = await member.request.post('/api/v1/me/avatar', {
			multipart: { file: { name: 'me.png', mimeType: 'image/png', buffer: PNG } }
		});
		expect(avatar.ok()).toBe(true);

		await member.goto('/');
		await expect(member.locator('.achievement-celebrate')).toContainText(
			t('achievement_avatar_set_name')
		);
		await context.close();
	});
});
