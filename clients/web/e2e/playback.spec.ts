import type { Page } from '@playwright/test';
import {
	authFile,
	expect,
	MEDIA_TIMEOUT,
	movie,
	row,
	series,
	t,
	test,
	videoTime
} from './fixtures';

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

/** opens one of the player's menus and picks an option in it */
async function choose(page: Page, menu: string, option: string) {
	// a badge the first minutes of watching earned celebrates over the controls for a while
	await expect(page.locator('.achievement-celebrate')).toHaveCount(0, { timeout: MEDIA_TIMEOUT });
	// the controls hide while playing; moving the pointer brings them back
	await page.mouse.move(640, 400);
	await page.mouse.move(650, 410);
	await page.getByRole('button', { name: menu, exact: true }).click();
	await page
		.locator('[data-popover-content]')
		.getByRole('button', { name: option, exact: true })
		.click();
	await page.keyboard.press('Escape');
}

test.describe(() => {
	// the admin's progress on the movie matters to no other spec
	test.use({ storageState: authFile('admin') });

	test('subtitles and a second audio language switch through the core', async ({ page }) => {
		test.slow();
		await page.request.put('/api/v1/progress', {
			data: { titleId: movie.id, positionSeconds: 0, durationSeconds: 60 }
		});
		await page.goto(`/title/${movie.slug}`);
		await page.getByRole('button', { name: t('common_play'), exact: true }).click();
		await expect.poll(() => videoTime(page), { timeout: MEDIA_TIMEOUT }).toBeGreaterThan(0.5);

		// the player draws the cues itself, in the account's subtitle style
		await choose(page, t('player_subtitles'), 'English');
		const cues = page.locator('.subtitle-overlay');
		await expect(cues).toHaveText('The sea keeps a door.');

		// the Czech audio is a file of its own: it replaces the source where playback was
		const source = () => page.locator('video').evaluate((v: HTMLVideoElement) => v.currentSrc);
		const before = await source();
		const at = await videoTime(page);
		await choose(page, t('player_audio'), 'Čeština');
		await expect.poll(source).not.toBe(before);
		await expect.poll(() => videoTime(page), { timeout: MEDIA_TIMEOUT }).toBeGreaterThan(at);
		await expect(cues).toHaveText('The sea keeps a door.');
	});
});

test.describe(() => {
	// vera's series progress matters to no other spec
	test.use({ storageState: authFile('vera') });

	test('the next episode counts down and follows when one ends', async ({ page }) => {
		test.slow();
		const [pilot, next] = series.episodes;
		for (const episodeId of series.episodes) {
			await page.request.put('/api/v1/progress', {
				data: { episodeId, positionSeconds: 0, durationSeconds: 25 }
			});
		}
		await page.goto(`/title/${series.slug}`);
		await page.getByRole('button', { name: t('catalog_play_episode', { label: 'S1 E1' }) }).click();
		await expect(page).toHaveURL(`/watch/episode/${pilot}`);
		await expect.poll(() => videoTime(page), { timeout: MEDIA_TIMEOUT }).toBeGreaterThan(0.5);

		// near the end the core names what comes next, and plays it when this one ends
		await page.keyboard.press('ArrowRight');
		await expect(page.getByTestId('up-next')).toContainText('S1 E2 · Interference');
		await expect(page).toHaveURL(`/watch/episode/${next}`, { timeout: MEDIA_TIMEOUT });
		await expect.poll(() => videoTime(page), { timeout: MEDIA_TIMEOUT }).toBeGreaterThan(0.5);
	});
});
