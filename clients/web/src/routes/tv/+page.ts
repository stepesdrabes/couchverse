import { error } from '@sveltejs/kit';
import { session } from '$lib/features/auth/session.svelte';
import { features } from '$lib/features/settings/features.svelte';

// Public: a guest of the channel link watches signed out. Tuning in is the page's (a load runs
// for preloads too), and for a guest the server says whether the channel is on.
export async function load({ parent }) {
	// the flags arrive with the session, in the root layout's load
	await parent();
	if (session.user && !features.couchTvEnabled) error(404, 'Not found');
}
