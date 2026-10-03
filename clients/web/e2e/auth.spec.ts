import { authFile, draft, expect, movie, PASSWORD, row, series, t, test } from './fixtures';

test('signs in from a deep link, lands on home and signs out', async ({ page, errors }) => {
	// a deep link starts its data fetch alongside the session check that redirects it
	errors.allow(/status of 401 .*\/api\/v1\/home\b/);
	const visited: string[] = [];
	page.on('framenavigated', (frame) => {
		if (frame === page.mainFrame()) visited.push(frame.url());
	});

	await page.goto('/');
	await expect(page).toHaveURL(/\/login\?next=%2F$/);

	await page.getByLabel(t('login_username')).fill('nora');
	await page.getByLabel(t('login_password')).fill(PASSWORD);
	await page.getByRole('button', { name: t('login_submit') }).click();
	await expect(page).toHaveURL(/\/$/);
	expect(
		visited.filter((url) => url.includes('next=%2Flogin')),
		'a nested login URL'
	).toEqual([]);

	const hero = page.getByRole('region', { name: t('catalog_featured_titles') });
	await expect(hero.getByRole('heading', { level: 1 })).toHaveText(
		new RegExp(`${movie.name}|${series.name}`)
	);
	await expect(hero.getByRole('button', { name: t('common_play') })).toBeVisible();

	const continueWatching = row(page, t('home_row_continue_watching'));
	await expect(continueWatching.getByRole('link', { name: movie.name })).toBeVisible();
	await expect(continueWatching.getByRole('link', { name: series.name })).toBeVisible();

	const recent = row(page, t('home_row_recently_added'));
	await expect(recent.getByRole('link', { name: movie.name })).toBeVisible();
	await expect(recent.getByRole('link', { name: series.name })).toBeVisible();
	await expect(recent.getByRole('link', { name: draft.name })).toHaveCount(0);

	// this ends only the session the form just made, not the one the other specs share
	await page.getByRole('button', { name: t('nav_account_menu') }).click();
	await page.getByRole('menuitem', { name: t('nav_sign_out') }).click();
	await expect(page).toHaveURL(/\/login$/);
	expect((await page.request.get('/api/v1/auth/me')).status()).toBe(401);
});

test('a session that ends while browsing goes back to sign in', async ({ page, errors }) => {
	// the core's next request is refused, whichever it is
	errors.allow(/status of 401 /);
	// vera's saved session stays: this one is her own
	const res = await page.request.post('/api/v1/auth/login', {
		data: { username: 'vera', password: PASSWORD }
	});
	expect(res.ok()).toBe(true);
	await page.goto('/');
	await expect(page.getByRole('region', { name: t('catalog_featured_titles') })).toBeVisible();

	// signed out on another tab or device; the keyboard, so no hover preloads the page early
	await page.request.post('/api/v1/auth/logout');
	await page
		.getByRole('navigation')
		.getByRole('link', { name: t('nav_movies') })
		.focus();
	await page.keyboard.press('Enter');
	await expect(page).toHaveURL(/\/login\?next=%2Fmovies$/);
});

test('a wrong password keeps the visitor on the login page', async ({ page, errors }) => {
	errors.allow(/status of 401 .*\/api\/v1\/auth\/login\b/);
	await page.goto('/login');
	// piet signs in nowhere else, so this attempt cannot trip another spec's rate limit
	await page.getByLabel(t('login_username')).fill('piet');
	await page.getByLabel(t('login_password')).fill('not-the-password');
	await page.getByRole('button', { name: t('login_submit') }).click();

	// the server's code, in the display language
	await expect(page.getByText(t('problem_invalid_credentials'))).toBeVisible();
	await expect(page).toHaveURL(/\/login$/);
	await expect(page.getByLabel(t('login_password'))).toHaveValue('not-the-password');
});

test.describe('signed in', () => {
	test.use({ storageState: authFile('nora') });

	test('a deep link boots the app once', async ({ page }) => {
		let probes = 0;
		page.on('request', (r) => {
			if (new URL(r.url()).pathname === '/api/v1/auth/me') probes++;
		});
		await page.goto('/movies');
		await expect(page.getByRole('heading', { level: 1, name: t('nav_movies') })).toBeVisible();
		await expect(page.getByRole('link', { name: movie.name })).toBeVisible();
		expect(probes).toBe(1);
	});

	test('the login page sends a signed-in member on', async ({ page }) => {
		await page.goto(`/login?next=${encodeURIComponent(`/title/${movie.slug}`)}`);
		await expect(page).toHaveURL(`/title/${movie.slug}`);
		await expect(page.getByRole('heading', { level: 1, name: movie.name })).toBeVisible();
	});
});
