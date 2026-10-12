import type { Page } from '@playwright/test';
import {
	authFile,
	expect,
	MEDIA_TIMEOUT,
	series,
	t,
	test,
	tPattern,
	videoPaused
} from './fixtures';

// e2e/setup.sql turns Couch TV on and airs the series' first episode every 32 s (24 s of it,
// then the built-in slate), so a program comes on screen within a cycle, the corner bug with it
// and its Now caption with the controls. The picture runs three target durations behind the
// server's clock.
const CYCLE = 45_000;
const pilot = series.episodes[0];

/** the controls hide while nothing moves: a pointer move brings them back */
async function poke(page: Page) {
	await page.mouse.move(200 + Math.random() * 400, 200 + Math.random() * 200);
}

test.describe('a member', () => {
	test.use({ storageState: authFile('otto') });

	test('tunes in, reacts, opens the guide and watches from the start', async ({ page }) => {
		// live media, a whole cycle of the channel at worst, and the guide
		test.slow();
		await page.goto('/');
		await page
			.getByRole('navigation')
			.getByRole('link', { name: t('nav_tv'), exact: true })
			.click();
		await expect(page).toHaveURL('/tv');
		await expect.poll(() => videoPaused(page), { timeout: MEDIA_TIMEOUT }).toBe(false);

		// a program comes on: the corner bug, LIVE with the clock, stays up; its Now caption
		// comes with the controls and leaves with them
		const live = page.getByText(t('couchtv_live'), { exact: true });
		await expect(live).toBeVisible({ timeout: CYCLE });
		await expect(page.getByRole('img', { name: 'Couch TV' }).first()).toBeVisible();
		const now = page.getByText(t('couchtv_now'), { exact: true });
		await expect(async () => {
			await poke(page);
			await expect(now).toBeVisible({ timeout: 1000 });
		}).toPass({ timeout: CYCLE });
		await expect(page.getByText(series.name, { exact: true }).first()).toBeVisible();
		await expect(now).toBeHidden({ timeout: 15_000 });
		await expect(live).toBeVisible();

		// over a program, the controls offer it from the start in the on-demand player
		const fromStart = page.getByRole('link', { name: t('couchtv_watch_from_start') });
		await expect(async () => {
			await poke(page);
			await expect(fromStart).toHaveAttribute('href', `/watch/episode/${pilot}`, { timeout: 1000 });
		}).toPass({ timeout: CYCLE });

		// a reaction floats up for everyone watching, its sender included
		await poke(page);
		await page
			.getByRole('button', { name: t('couch_react') })
			.last()
			.click();
		const emoji = page.locator('emoji-picker [role="menuitem"]').first();
		const sent = (await emoji.textContent())?.trim() ?? '';
		const floated = page
			.locator('.couchtv-reaction')
			.filter({ hasText: sent })
			.waitFor({ state: 'attached' });
		await emoji.click();
		await floated;
		await page.keyboard.press('Escape');

		// the guide: the week by day, a program's details and the way back to the channel
		await expect(async () => {
			await poke(page);
			await page.getByRole('link', { name: t('couchtv_guide') }).click({ timeout: 1000 });
		}).toPass({ timeout: CYCLE });
		await expect(page).toHaveURL('/tv/guide');
		await expect(page.getByRole('heading', { level: 1, name: t('couchtv_guide') })).toBeVisible();
		const today = page
			.getByRole('region', { name: t('couchtv_guide_days') })
			.getByRole('group', { name: t('couchtv_today') });
		// the schedule's programs are seconds long, a sliver each on the timeline: the click goes
		// to the button itself rather than to a point on screen
		await today
			.getByRole('button', { name: new RegExp(`${series.name}.*Pilot`) })
			.first()
			.dispatchEvent('click');
		const sheet = page.getByRole('dialog');
		await expect(sheet.getByRole('heading', { name: series.name })).toBeVisible();
		await expect(sheet.getByRole('link', { name: t('couchtv_watch_from_start') })).toHaveAttribute(
			'href',
			`/watch/episode/${pilot}`
		);
		await page.keyboard.press('Escape');
		await expect(sheet).toBeHidden();

		await page.getByRole('link', { name: t('couchtv_tune_in') }).click();
		await expect(page).toHaveURL('/tv');
		await expect.poll(() => videoPaused(page), { timeout: MEDIA_TIMEOUT }).toBe(false);
	});
});

test.describe('a guest', () => {
	// the admin makes the link; the guest watches in a browser of their own
	test.use({ storageState: authFile('admin') });

	test('joins through the channel link and sees the live channel only', async ({
		page,
		browser,
		errors
	}) => {
		test.slow();
		await page.goto('/admin/couch-tv/template');
		const panel = page.getByRole('region', { name: t('admin_couchtv_guest_link') });
		const make = panel.getByRole('button', { name: t('admin_couchtv_guest_link_make') });
		const renew = panel.getByRole('button', { name: t('admin_couchtv_guest_link_new') });
		await expect(make.or(renew)).toBeVisible();
		// a link from an earlier run (the other browser's, a retry) is replaced, after a warning
		if (await renew.isVisible()) {
			await renew.click();
			await page
				.getByRole('alertdialog')
				.getByRole('button', { name: t('admin_couchtv_guest_link_new') })
				.click();
		} else {
			await make.click();
		}
		const shown = panel.getByRole('textbox', { name: t('admin_couchtv_guest_link_url') });
		await expect(shown).toHaveValue(/\/tv\/join\/[\w-]+$/);
		const link = new URL(await shown.inputValue());
		const guests = panel.getByRole('switch', { name: t('admin_couchtv_guest_link_switch') });
		await expect(guests).toBeChecked();

		const guestContext = await browser.newContext({ storageState: { cookies: [], origins: [] } });
		errors.watch(guestContext);
		const guest = await guestContext.newPage();
		await guest.goto(link.pathname);
		await guest.getByLabel(t('couchtv_join_name')).fill('Petra');
		await guest.getByRole('button', { name: t('couchtv_join_submit') }).click();
		await expect(guest).toHaveURL('/tv');
		await expect.poll(() => videoPaused(guest), { timeout: MEDIA_TIMEOUT }).toBe(false);
		await expect(async () => {
			await poke(guest);
			await expect(guest.getByText(t('couchtv_now'), { exact: true })).toBeVisible({
				timeout: 1000
			});
		}).toPass({ timeout: CYCLE });

		// live only: no guide, nothing to watch from the start, no library to go back to
		await poke(guest);
		await expect(guest.getByRole('button', { name: t('couch_react') }).last()).toBeVisible();
		await expect(guest.getByRole('link', { name: t('couchtv_guide') })).toHaveCount(0);
		await expect(guest.getByRole('link', { name: t('couchtv_watch_from_start') })).toHaveCount(0);
		await expect(guest.getByRole('link', { name: t('common_back') })).toHaveCount(0);

		// in the room under the name they typed
		await expect(async () => {
			await poke(guest);
			await guest
				.getByRole('button', { name: tPattern('couchtv_watching') })
				.click({ timeout: 1000 });
		}).toPass({ timeout: 10_000 });
		const viewers = guest.getByRole('list', { name: t('couchtv_viewers') });
		await expect(viewers.getByText('Petra', { exact: true })).toBeVisible();
		await expect(viewers.getByText(t('couch_you_badge'), { exact: true })).toBeVisible();
		await guest.keyboard.press('Escape');

		// turning the link off ends the guest's channel: their stream and tune-in are refused
		errors.allow(/status of 40[13] /);
		await guests.click();
		await expect(guests).not.toBeChecked();
		await expect(panel.getByText(t('admin_couchtv_guest_link_off'))).toBeVisible();
		await expect(guest.getByRole('link', { name: t('login_submit') })).toBeVisible({
			timeout: MEDIA_TIMEOUT
		});
		await guestContext.close();
	});
});
