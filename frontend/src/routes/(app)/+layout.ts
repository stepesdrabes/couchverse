import { redirect } from '@sveltejs/kit';
import { session } from '$lib/features/auth/session.svelte';
import { features } from '$lib/features/settings/features.svelte';
import { rank } from '$lib/features/ranks/rank.svelte';
import { getMyStats } from '$lib/features/ranks/api';
import { profileCache } from '$lib/features/ranks/cache.svelte';
import { currentLang } from '$lib/i18n/locale.svelte';

export function load({ url }) {
	if (!session.user) {
		redirect(307, `/login?next=${encodeURIComponent(url.pathname + url.search)}`);
	}

	// One un-awaited fetch seeds the nav rank ring and warms the cache entry the
	// profile page reads, so neither costs a request of its own. Never blocks the
	// page swap.
	if (features.rankingsEnabled && session.user) {
		const key = `${session.user.username}|${currentLang()}`;
		profileCache
			.revalidate(key, getMyStats)
			.then((profile) => rank.seed(profile.rank))
			.catch(() => {});
		rank.check();
	}
}
