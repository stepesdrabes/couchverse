import { session } from '$lib/features/auth/session.svelte';
import { features } from '$lib/features/settings/features.svelte';

// Static SPA: everything renders client-side; the Go server provides the
// index.html fallback for deep links.
export const ssr = false;
export const prerender = false;

export async function load() {
	await session.init();
	if (session.user) {
		await features.init();
	}
}
