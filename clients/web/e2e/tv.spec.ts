import { authFile, expect, row, t, test } from './fixtures';

test.use({ storageState: authFile('nora') });

test('TV mode starts on the hero and arrows move between cards', async ({ page }) => {
	await page.goto('/?tv=1');
	await expect(page.locator('html')).toHaveAttribute('data-tv', '');

	const hero = page.getByRole('region', { name: t('catalog_featured_titles') });
	await expect(hero.getByRole('button', { name: t('common_play') })).toBeFocused();

	// the catalog's row, not one built from progress another spec may be moving
	const cards = row(page, t('home_row_recently_added')).getByRole('link');
	await expect(cards).toHaveCount(2);
	const focusedCard = () =>
		cards.evaluateAll((links) => links.findIndex((a) => a === document.activeElement));

	// down from the hero, past its carousel controls and the rows above, into this one
	await expect(async () => {
		await page.keyboard.press('ArrowDown');
		expect(await focusedCard()).toBeGreaterThanOrEqual(0);
	}).toPass();

	// sideways stays in the row
	const from = await focusedCard();
	await page.keyboard.press(from === 0 ? 'ArrowRight' : 'ArrowLeft');
	await expect(cards.nth(1 - from)).toBeFocused();
	await page.keyboard.press(from === 0 ? 'ArrowLeft' : 'ArrowRight');
	await expect(cards.nth(from)).toBeFocused();
});
