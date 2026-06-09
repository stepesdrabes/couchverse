import { session } from '$lib/state/session.svelte';

// Static SPA: everything renders client-side; the Go server provides the
// index.html fallback for deep links.
export const ssr = false;
export const prerender = false;

export async function load() {
	await session.init();
}
