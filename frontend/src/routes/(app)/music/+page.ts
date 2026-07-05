import * as musicApi from '$lib/features/music/api';
import { musicHomeCache } from '$lib/features/music/cache.svelte';
import { currentLang } from '$lib/i18n/locale.svelte';

// Non-blocking: paints from cache at once, revalidates behind it.
export function load() {
	const lang = currentLang();
	return { lang, fresh: musicHomeCache.revalidate(lang, () => musicApi.musicHome()) };
}
