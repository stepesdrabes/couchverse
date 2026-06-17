import { error } from '@sveltejs/kit';
import { ApiError } from '$lib/api/client';
import { joinCouch, resolveCouchPlayer } from '$lib/features/couch/api';

// Public route: works for anonymous viewers (join + the follower payload both use
// skipAuthRedirect, so the global 401 -> /login never fires here). Not under (app),
// so the auth-guard layout never runs.
export async function load({ params }) {
	try {
		const snapshot = await joinCouch(params.token);
		const { player, jitSessionId } = await resolveCouchPlayer(params.token);
		return { snapshot, player, jitSessionId };
	} catch (err) {
		if (err instanceof ApiError) {
			error(err.status >= 400 && err.status < 600 ? err.status : 500, err.message);
		}
		throw err;
	}
}
