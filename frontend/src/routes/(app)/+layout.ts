import { redirect } from '@sveltejs/kit';
import { session } from '$lib/features/auth/session.svelte';

export function load({ url }) {
	if (!session.user) {
		redirect(307, `/login?next=${encodeURIComponent(url.pathname + url.search)}`);
	}
}
