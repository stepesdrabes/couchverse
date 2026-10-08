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

	// another tab of the hosting browser cannot join: the browser's one couch cookie is the
	// playing tab's
	errors.allow(/status of 409 /);
	const tab = await host.context().newPage();
	await tab.goto(`/couch/${code}`);
	await tab.getByRole('button', { name: t('couch_join_start') }).click();
	await expect(tab.getByText(t('problem_already_hosting'))).toBeVisible();
	await tab.close();

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

	// a reaction floats up from the sender's seat on everyone's couch
	await guest.mouse.move(400, 300);
	await guest.getByRole('button', { name: t('couch_react') }).click();
	const emoji = guest.locator('emoji-picker [role="menuitem"]').first();
	const sent = (await emoji.textContent())?.trim() ?? '';
	// it floats for two seconds, so the host watches for it before it is sent: a sender's
	// click can take longer than that to return on a busy machine
	const floated = host
		.locator('.couch-reaction')
		.filter({ hasText: sent })
		.waitFor({ state: 'attached' });
	await emoji.click();
	await floated;
	await guest.keyboard.press('Escape');

	// the host resumes and the follower plays along, in step with the host
	await host.keyboard.press('Escape');
	await host.getByRole('button', { name: t('player_play_pause') }).click();
	await expect(guest.getByText(t('couch_host_paused'))).toHaveCount(0);
	await expect
		.poll(() => videoTime(guest), { timeout: MEDIA_TIMEOUT })
		.toBeGreaterThan(pausedAt + 0.5);
	const drift = async () => Math.abs((await videoTime(guest)) - (await videoTime(host)));
	await expect.poll(drift, { timeout: MEDIA_TIMEOUT }).toBeLessThan(3);

	// a follower that falls behind is put back on the host's timeline, and told so
	await guest
		.locator('video')
		.evaluate((v: HTMLVideoElement) => (v.currentTime = Math.max(0, v.currentTime - 15)));
	await expect(guest.getByText(t('couch_resynced'))).toBeVisible();
	await expect.poll(drift, { timeout: MEDIA_TIMEOUT }).toBeLessThan(3);

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
		// the apps join this server's couch by its code, without an account if need be
		const server = encodeURIComponent(new URL(guest.url()).origin);
		await expect(guest.getByRole('link', { name: t('couch_open_in_app') })).toHaveAttribute(
			'href',
			`couchverse://couch/${shareToken}?server=${server}`
		);
		await guest.getByRole('button', { name: t('couch_join_start') }).click();
		await couchButton(guest).click();
		await guest.getByRole('button', { name: t('couch_leave') }).click();
		await expect(guest).toHaveURL(/\/login$/);
		await guestContext.close();

		expect((await request.get(`/api/v1/couch/${shareToken}/info`)).ok()).toBe(true);
		await request.post(`/api/v1/couch/${shareToken}/end`);
	});
});

test.describe(() => {
	// the host's own account on another device: a member no other couch test hosts as
	test.use({ storageState: authFile('vera') });

	test("the host's account joining by code steers the host's player as a remote", async ({
		page,
		request
	}) => {
		// the request context hosts, the page is the account's other device
		const created = await request.post('/api/v1/couch', { data: { kind: 'movie', id: movie.id } });
		const { shareToken } = await created.json();

		await page.goto(`/couch/${shareToken}`);
		await page.getByRole('button', { name: t('couch_join_start') }).click();
		await expect(page.getByRole('heading', { name: t('couch_remote_title') })).toBeVisible();
		await expect(page.getByRole('button', { name: t('player_forward_10_seconds') })).toBeVisible();
		await expect(page.getByRole('button', { name: t('couch_remote_next') })).toBeVisible();
		await page.getByRole('button', { name: t('common_play'), exact: true }).click();

		// putting the remote down leaves the session to the host's player
		await page.getByRole('button', { name: t('couch_leave') }).click();
		await expect(page).toHaveURL(/\/$/);
		expect((await request.get(`/api/v1/couch/${shareToken}/info`)).ok()).toBe(true);
		await request.post(`/api/v1/couch/${shareToken}/end`);
	});
});
