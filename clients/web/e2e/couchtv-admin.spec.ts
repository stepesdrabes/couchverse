import { authFile, expect, movie, series, t, test } from './fixtures';

test.use({ storageState: authFile('admin') });

// The schedule editor replans the week (the startup plan ran before the seed), locks a
// program, inserts an episode after it and puts everything back as it was. Replanning an
// unchanged library is idempotent, so the spec is retry-safe.
test('the Couch TV schedule shows the week and edits a program', async ({ page }) => {
	await page.goto('/admin');
	await page.getByRole('link', { name: t('admin_nav_couch_tv') }).click();
	await expect(
		page.getByRole('heading', { level: 1, name: t('admin_nav_couch_tv') })
	).toBeVisible();
	await expect(page.getByRole('link', { name: t('admin_couchtv_tab_schedule') })).toHaveAttribute(
		'aria-current',
		'page'
	);

	const grid = page.getByRole('region', { name: t('admin_couchtv_schedule_grid') });
	// today's column comes first
	await grid
		.getByRole('button', {
			name: new RegExp(`^${t('admin_couchtv_regenerate_day_named', { day: '' })}`)
		})
		.first()
		.click();
	await page
		.getByRole('alertdialog')
		.getByRole('button', { name: t('admin_couchtv_regenerate_day') })
		.click();
	// the last airing of the movie in the week is well in the future
	const program = grid.getByRole('button', { name: new RegExp(`^${movie.name},`) }).last();
	await expect(program).toBeAttached({ timeout: 15_000 });
	await program.click();

	const panel = page.getByRole('complementary', { name: t('admin_couchtv_program_details') });
	await expect(panel.getByRole('heading', { name: movie.name })).toBeVisible();

	// locked, it says so in the grid
	await panel.getByRole('button', { name: t('admin_couchtv_action_lock') }).click();
	await expect(panel.getByRole('button', { name: t('admin_couchtv_action_unlock') })).toBeVisible();
	await expect(grid.getByRole('button', { pressed: true })).toHaveAccessibleName(
		new RegExp(t('admin_couchtv_locked'))
	);

	// an episode inserted after it, through the program picker
	await panel.getByRole('button', { name: t('admin_couchtv_action_insert_after') }).click();
	await panel.getByRole('searchbox', { name: t('admin_couchtv_picker_search') }).fill('Static');
	await panel.getByRole('button', { name: series.name }).click();
	await panel.getByRole('button', { name: new RegExp(t('admin_couchtv_picker_episode')) }).click();
	await page.getByRole('option', { name: 'S01E01 Pilot' }).click();
	const inserted = page.waitForResponse(
		(r) =>
			new URL(r.url()).pathname === '/api/v1/admin/couchtv/airings' &&
			r.request().method() === 'POST'
	);
	await panel.getByRole('button', { name: t('admin_couchtv_action_insert_after') }).click();
	const response = await inserted;
	expect(response.status()).toBe(201);
	const created = await response.json();
	await expect(page.getByText(t('admin_couchtv_inserted'))).toBeVisible();
	expect(created).toMatchObject({ titleName: series.name, source: 'admin', fill: 'reruns' });

	// put it all back: the inserted episode goes, the movie unlocks
	const removed = await page.request.patch(`/api/v1/admin/couchtv/airings/${created.id}`, {
		data: { action: 'remove' }
	});
	expect(removed.ok()).toBe(true);
	await panel.getByRole('button', { name: t('admin_couchtv_action_unlock') }).click();
	await expect(panel.getByRole('button', { name: t('admin_couchtv_action_lock') })).toBeVisible();
	await panel.getByRole('button', { name: t('admin_couchtv_close') }).click();
	await expect(panel).toBeHidden();
});

test('the template, render health and preview tabs open', async ({ page }) => {
	await page.goto('/admin/couch-tv');
	const tabs = page.getByRole('navigation', { name: t('admin_couchtv_sections') });

	await tabs.getByRole('link', { name: t('admin_couchtv_tab_template') }).click();
	await expect(page.getByRole('heading', { name: t('admin_couchtv_week_template') })).toBeVisible();
	const save = page.getByRole('button', { name: t('common_save'), exact: true });
	await expect(save).toBeDisabled();
	// a change enables Save; discarding it disables it again
	const gap = page.getByLabel(t('admin_couchtv_rot_movie_gap'));
	await gap.fill(String(Number(await gap.inputValue()) + 1));
	await expect(save).toBeEnabled();
	await page.getByRole('button', { name: t('admin_couchtv_discard') }).click();
	await expect(save).toBeDisabled();

	await tabs.getByRole('link', { name: t('admin_couchtv_tab_render') }).click();
	await expect(page.getByText(t('admin_couchtv_health_queue'))).toBeVisible();

	await tabs.getByRole('link', { name: t('admin_couchtv_tab_preview') }).click();
	await expect(page.getByRole('heading', { name: t('admin_couchtv_preview_title') })).toBeVisible();
});
