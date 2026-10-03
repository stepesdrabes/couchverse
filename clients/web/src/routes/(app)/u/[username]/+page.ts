import { error } from '@sveltejs/kit';
import { core } from '$lib/core';
import { features } from '$lib/features/settings/features.svelte';
import { profileScreen } from '$lib/features/ranks/api';

export async function load({ params, parent }) {
	// the flags arrive with the session, in the root layout's load
	await parent();
	if (!features.rankingsEnabled) error(404, 'Not found');

	// non-blocking: the page paints the core's copy at once (or a skeleton on a cold visit)
	// while this loads it, unless the core holds it as fresh
	const screen = profileScreen(params.username);
	core.prefetch(screen);
	return { screen };
}
