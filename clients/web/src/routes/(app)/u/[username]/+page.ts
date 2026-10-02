import { error } from '@sveltejs/kit';
import { features } from '$lib/features/settings/features.svelte';
import { getProfile } from '$lib/features/ranks/api';
import { profileCache } from '$lib/features/ranks/cache.svelte';
import { currentLang } from '$lib/i18n/locale.svelte';

export function load({ params }) {
	if (!features.rankingsEnabled) error(404, 'Not found');

	// keyed by language because title names and the genre label are localized
	// server-side, the same reasoning as the catalog caches
	const key = `${params.username}|${currentLang()}`;
	return {
		key,
		fresh: profileCache.revalidate(key, () => getProfile(params.username))
	};
}
