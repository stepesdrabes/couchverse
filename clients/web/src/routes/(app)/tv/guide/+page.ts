import { error } from '@sveltejs/kit';
import { COUCH_TV_GUIDE } from '$lib/features/couchtv/channel';
import { features } from '$lib/features/settings/features.svelte';

// Non-blocking: the page opens the guide, which the core asks for on every visit (what is on
// moves on), and shows the last one it holds meanwhile.
export async function load({ parent }) {
	// the flags arrive with the session, in the root layout's load
	await parent();
	if (!features.couchTvEnabled) error(404, 'Not found');
	return { screen: COUCH_TV_GUIDE };
}
