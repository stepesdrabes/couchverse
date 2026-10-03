import { authFile, expect, MEDIA_TIMEOUT, movie, row, t, test, videoTime } from './fixtures';

// otto watches nothing else, so continue watching holds only what this test plays
test.use({ storageState: authFile('otto') });

test('plays a movie, reports progress and resumes from continue watching', async ({ page }) => {
	// real playback, twice
	test.slow();
	// a retry starts over from the beginning, where the title offers Play rather than Resume
	await page.request.put('/api/v1/progress', {
		data: { titleId: movie.id, positionSeconds: 0, durationSeconds: 60 }
	});
	await page.goto(`/title/${movie.slug}`);
	// a click (not page.goto) gives the player the user gesture autoplay needs
	await page.getByRole('button', { name: t('common_play'), exact: true }).click();
	await expect(page).toHaveURL(`/watch/movie/${movie.id}`);
	await expect.poll(() => videoTime(page), { timeout: MEDIA_TIMEOUT }).toBeGreaterThan(0.5);

	// the player reports from 5 s in, every 10 s of play
	const report = page.waitForRequest(
		(r) => r.url().includes('/api/v1/progress') && r.method() === 'PUT'
	);
	await page.keyboard.press('ArrowRight');
	const body = (await report).postDataJSON();
	expect(body).toMatchObject({ titleId: movie.id, durationSeconds: 60 });
	expect(body.positionSeconds).toBeGreaterThanOrEqual(10);

	// paused, the player keeps its controls up instead of hiding them after 3 s
	await page.keyboard.press('Space');
	await page.getByRole('button', { name: t('common_back'), exact: true }).click();
	await expect(page).toHaveURL(`/title/${movie.slug}`);

	await page.getByRole('link', { name: t('nav_home'), exact: true }).click();
	const res = await page.request.get('/api/v1/me/continue-watching');
	const [item] = await res.json();
	expect(item).toMatchObject({ playbackKind: 'movie', playbackId: movie.id });
	expect(item.positionSeconds).toBeGreaterThanOrEqual(10);

	await row(page, t('home_row_continue_watching')).getByRole('link', { name: movie.name }).click();
	await expect(page).toHaveURL(`/watch/movie/${movie.id}`);
	await expect
		.poll(() => videoTime(page), { timeout: MEDIA_TIMEOUT })
		.toBeGreaterThanOrEqual(item.positionSeconds);
});
