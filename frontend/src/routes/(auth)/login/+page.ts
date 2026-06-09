import { redirect } from '@sveltejs/kit';
import { session } from '$lib/state/session.svelte';

export function load({ url }) {
	if (session.user) {
		redirect(307, url.searchParams.get('next') ?? '/');
	}
}
