import type { Page } from '@playwright/test';
import {
	authFile,
	expect,
	MEDIA_TIMEOUT,
	movie,
	t,
	test,
	videoPaused,
	videoTime
} from './fixtures';

test.use({ storageState: authFile('nora') });

const couchButton = (page: Page) => page.getByRole('button', { name: t('couch_open') }).first();

test('an anonymous viewer joins a couch session by its code and follows the host', async ({
	page: host,
	browser,
	errors
}) => {
	// two browsers and real playback
	test.slow();
	const resume = t('catalog_resume_from', { time: '' }).trim();

	// A retry must not reclaim the live session a failed attempt left behind (a host's
	// session outlives its tab for a grace period), so claim it here and end it.
	const stale = await host.request.post('/api/v1/couch', {
		data: { kind: 'movie', id: movie.id }
	});
	await host.request.post(`/api/v1/couch/${(await stale.json()).shareToken}/end`);
	// and wind the host back, so a retry never plays into the end of the one-minute clip
	await host.request.put('/api/v1/progress', {
		data: { titleId: movie.id, positionSeconds: 20, durationSeconds: 60 }
	});

	await host.goto(`/title/${movie.slug}`);
	await host.getByRole('button', { name: resume }).click();
	await expect(host).toHaveURL(`/watch/movie/${movie.id}`);
	// past the resume point, not just seeked to it: a Space before autoplay starts would play
	await expect.poll(() => videoTime(host), { timeout: MEDIA_TIMEOUT }).toBeGreaterThan(20.5);

	// paused, the host's controls (and the couch button among them) stay up
	await host.keyboard.press('Space');
	await expect.poll(() => videoPaused(host)).toBe(true);
	await couchButton(host).click();
	await host.getByRole('button', { name: t('couch_start_session') }).click();
	const shareLink = host.getByRole('textbox');
	await expect(shareLink).toHaveValue(/\/couch\/\d{6}$/);
	await expect(host.getByText(t('couch_on_couch_count', { count: 1 }))).toBeVisible();
	const code = (await shareLink.inputValue()).split('/').pop();
	const pausedAt = await videoTime(host);

	// contexts made in a test inherit its options, nora's cookies included
	const guestContext = await browser.newContext({ storageState: { cookies: [], origins: [] } });
	errors.watch(guestContext);
	const guest = await guestContext.newPage();
	await guest.goto(`/couch/${code}`);
	await expect(guest.getByText(t('couch_join_heading', { name: 'Nora' }))).toBeVisible();
	await expect(guest.getByRole('heading', { name: movie.name })).toBeVisible();
	await guest.getByRole('button', { name: t('couch_join_start') }).click();

	// follower mode: the host's title and paused state, and no timeline control
	await expect(guest.getByText(t('couch_host_paused'))).toBeVisible();
	await expect(guest.getByText(movie.name, { exact: true })).toBeVisible();
	await expect(guest.getByRole('button', { name: t('player_play_pause') })).toBeVisible();
	await expect(guest.getByRole('button', { name: t('player_forward_10_seconds') })).toHaveCount(0);
	await expect(host.getByText(t('couch_on_couch_count', { count: 2 }))).toBeVisible();

	await couchButton(guest).click();
	await expect(guest.getByText(t('couch_on_couch_count', { count: 2 }))).toBeVisible();
	await expect(guest.getByText('Nora', { exact: true })).toBeVisible();
	await expect(guest.getByText(t('couch_host_badge'), { exact: true })).toBeVisible();
	await expect(guest.getByText(t('couch_you_badge'), { exact: true })).toBeVisible();
	await expect(guest.getByRole('button', { name: t('couch_leave') })).toBeVisible();
	await guest.keyboard.press('Escape');

	// the host resumes and the follower plays along, in step with the host
	await host.keyboard.press('Escape');
	await host.getByRole('button', { name: t('player_play_pause') }).click();
	await expect(guest.getByText(t('couch_host_paused'))).toHaveCount(0);
	await expect
		.poll(() => videoTime(guest), { timeout: MEDIA_TIMEOUT })
		.toBeGreaterThan(pausedAt + 0.5);
	await expect
		.poll(async () => Math.abs((await videoTime(guest)) - (await videoTime(host))), {
			timeout: MEDIA_TIMEOUT
		})
		.toBeLessThan(3);

	// Ending the session sends the follower, who has nowhere left to be, to the login page.
	// The host pauses first so its progress stays clear of the end of the clip, which
	// would drop the movie from continue watching for the specs that read it. By key:
	// while playing, the controls hide (unmount) under the pointer every few seconds.
	await host.keyboard.press('Space');
	await expect.poll(() => videoPaused(host)).toBe(true);
	await couchButton(host).click();
	await host.getByRole('button', { name: t('couch_end_session') }).click();
	await expect(guest).toHaveURL(/\/login$/);
	await guestContext.close();
	const info = await host.request.get(`/api/v1/couch/${code}/info`);
	expect(info.status()).toBe(404);
});

test.describe(() => {
	// a member hosts one session at a time, so this one must not be nora's
	test.use({ storageState: authFile('otto') });

	test('a follower can leave a couch the host keeps going', async ({
		request,
		browser,
		errors
	}) => {
		const created = await request.post('/api/v1/couch', { data: { kind: 'movie', id: movie.id } });
		const { shareToken } = await created.json();

		const guestContext = await browser.newContext({ storageState: { cookies: [], origins: [] } });
		errors.watch(guestContext);
		const guest = await guestContext.newPage();
		await guest.goto(`/couch/${shareToken}`);
		await expect(guest.getByText(t('couch_join_heading', { name: 'Otto' }))).toBeVisible();
		await guest.getByRole('button', { name: t('couch_join_start') }).click();
		await couchButton(guest).click();
		await guest.getByRole('button', { name: t('couch_leave') }).click();
		await expect(guest).toHaveURL(/\/login$/);
		await guestContext.close();

		expect((await request.get(`/api/v1/couch/${shareToken}/info`)).ok()).toBe(true);
		await request.post(`/api/v1/couch/${shareToken}/end`);
	});
});
