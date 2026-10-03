import { authFile, draft, expect, movie, series, t, test } from './fixtures';

test.use({ storageState: authFile('nora') });

test('movies and series pages list their titles', async ({ page }) => {
	await page.goto('/movies');
	await expect(page.getByRole('heading', { level: 1, name: t('nav_movies') })).toBeVisible();
	await expect(page.getByRole('link', { name: movie.name })).toBeVisible();
	await expect(page.getByRole('link', { name: series.name })).toHaveCount(0);
	await expect(page.getByRole('link', { name: draft.name })).toHaveCount(0);

	await page.getByRole('link', { name: t('nav_series'), exact: true }).click();
	await expect(page.getByRole('heading', { level: 1, name: t('nav_series') })).toBeVisible();
	await expect(page.getByRole('link', { name: series.name })).toBeVisible();
	await expect(page.getByRole('link', { name: movie.name })).toHaveCount(0);
});

test('a genre lists the titles tagged with it', async ({ page }) => {
	await page.goto('/');
	await page.getByRole('link', { name: t('nav_genres'), exact: true }).click();
	await expect(page.getByRole('heading', { level: 1, name: t('nav_genres') })).toBeVisible();

	await page.getByRole('link', { name: 'Science Fiction' }).click();
	await expect(page).toHaveURL('/genres/Science%20Fiction');
	await expect(page.getByRole('heading', { level: 1, name: 'Science Fiction' })).toBeVisible();
	await expect(page.getByRole('link', { name: movie.name })).toBeVisible();
	await expect(page.getByRole('link', { name: series.name })).toBeVisible();

	await page.goBack();
	await page.getByRole('link', { name: 'Drama' }).click();
	await expect(page.getByRole('heading', { level: 1, name: 'Drama' })).toBeVisible();
	await expect(page.getByRole('link', { name: movie.name })).toBeVisible();
	await expect(page.getByRole('link', { name: series.name })).toHaveCount(0);
});

test('search finds a title and opens its page', async ({ page }) => {
	await page.goto('/search');
	await page.getByPlaceholder(t('catalog_search_placeholder')).fill('glass');
	await page.getByRole('link', { name: movie.name }).click();

	await expect(page).toHaveURL(`/title/${movie.slug}`);
	await expect(page.getByRole('heading', { level: 1, name: movie.name })).toBeVisible();
	await expect(page.getByText(movie.overview)).toBeVisible();
	// nora stopped part-way through it (the couch spec may move her position on)
	const resume = t('catalog_resume_from', { time: '' }).trim();
	await expect(page.getByRole('button', { name: resume })).toBeVisible();
});

test('a series page lists its seasons and episodes', async ({ page }) => {
	await page.goto(`/title/${series.slug}`);
	await expect(page.getByRole('heading', { level: 1, name: series.name })).toBeVisible();
	await expect(page.getByRole('heading', { name: t('catalog_episodes') })).toBeVisible();

	// only episodes with a playable file are listed, which leaves season 2 out
	await expect(page.getByRole('link', { name: /Pilot/ })).toBeVisible();
	await expect(page.getByRole('link', { name: /Interference/ })).toBeVisible();
	await expect(page.getByText('Rebroadcast')).toHaveCount(0);
});

test('My List adds and removes a title', async ({ page }) => {
	const myList = page.getByRole('navigation').getByRole('link', { name: t('nav_my_list') });
	const toggle = page.getByRole('button', { name: t('nav_my_list') });

	// a retry must not inherit the add a failed attempt made
	await page.request.delete(`/api/v1/me/watchlist/${series.id}`);
	await page.goto(`/title/${series.slug}`);
	await expect(toggle).toHaveAttribute('aria-pressed', 'false');
	await toggle.click();
	await expect(toggle).toHaveAttribute('aria-pressed', 'true');

	await myList.click();
	await expect(page.getByRole('heading', { level: 1, name: t('nav_my_list') })).toBeVisible();
	await expect(page.getByRole('link', { name: movie.name })).toBeVisible();

	// Back on the title, its refetch (the hover preload's too) is held until after the
	// toggle below, as on a slow server: the answer it carries, still listed, lands last
	// and must not undo the removal.
	const isTitle = (url: URL) => url.pathname === `/api/v1/titles/${series.slug}`;
	let release = () => {};
	const held = new Promise<void>((resolve) => (release = resolve));
	const answered: Promise<unknown>[] = [];
	let fetched = () => {};
	const answerTaken = new Promise<void>((resolve) => (fetched = resolve));
	await page.route(isTitle, async (route) => {
		answered.push(page.waitForEvent('requestfinished', (r) => r === route.request()));
		const response = await route.fetch();
		fetched();
		await held;
		await route.fulfill({ response });
	});
	await page.getByRole('link', { name: series.name }).click();
	await answerTaken;
	await expect(toggle).toHaveAttribute('aria-pressed', 'true');
	await toggle.click();
	await expect(toggle).toHaveAttribute('aria-pressed', 'false');
	release();
	await Promise.all(answered);
	await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(resolve)));
	await expect(toggle).toHaveAttribute('aria-pressed', 'false');
	await page.unrouteAll();

	await myList.click();
	await expect(page.getByRole('link', { name: movie.name })).toBeVisible();
	await expect(page.getByRole('link', { name: series.name })).toHaveCount(0);
});

test.describe(() => {
	// nora's list belongs to the test above
	test.use({ storageState: authFile('otto') });

	test('a title page catches up with a list change made elsewhere', async ({ page }) => {
		const toggle = page.getByRole('button', { name: t('nav_my_list') });
		await page.request.delete(`/api/v1/me/watchlist/${movie.id}`);
		await page.goto(`/title/${movie.slug}`);
		await expect(toggle).toHaveAttribute('aria-pressed', 'false');

		// added on another device while this page's copy sits in the cache
		await page.request.put(`/api/v1/me/watchlist/${movie.id}`);
		await page
			.getByRole('navigation')
			.getByRole('link', { name: t('nav_my_list') })
			.click();
		await page.getByRole('link', { name: movie.name }).click();
		// the stale copy paints first, then the refetch
		await expect(toggle).toHaveAttribute('aria-pressed', 'true');
		await page.request.delete(`/api/v1/me/watchlist/${movie.id}`);
	});
});
