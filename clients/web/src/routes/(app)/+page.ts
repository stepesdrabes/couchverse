import * as catalog from '$lib/features/catalog/api';
import { homeCache } from '$lib/features/catalog/cache.svelte';
import { currentLang } from '$lib/i18n/locale.svelte';

// Non-blocking: Home paints from cache at once (or a skeleton on a cold visit) and
// this revalidation refreshes continue-watching behind it. Keyed by display
// language, matching the ?lang= the fetch carries.
export function load() {
	const lang = currentLang();
	return { lang, fresh: homeCache.revalidate(lang, () => catalog.home()) };
}
