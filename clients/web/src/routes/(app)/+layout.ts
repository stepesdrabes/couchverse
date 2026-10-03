import { redirect } from '@sveltejs/kit';
import { core } from '$lib/core';
import { session } from '$lib/features/auth/session.svelte';
import { features } from '$lib/features/settings/features.svelte';
import { checkAchievements, profileScreen } from '$lib/features/ranks/api';

export async function load({ url, parent }) {
	// the root layout's load is where the core learns who is signed in
	await parent();
	if (!session.user) {
		redirect(307, `/login?next=${encodeURIComponent(url.pathname + url.search)}`);
	}

	// Your own profile, unless the core holds it as fresh, keeps the nav rank ring current
	// and warms the profile page; a check now and then catches what was earned away from the
	// player (My List, a streak). Neither blocks the page swap.
	if (features.rankingsEnabled) {
		core.prefetch(profileScreen(session.user.username));
		void checkAchievements();
	}
}
