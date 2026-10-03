import { redirect } from '@sveltejs/kit';
import { session } from '$lib/features/auth/session.svelte';

export async function load({ url, parent }) {
	await parent();
	if (session.user) {
		redirect(307, url.searchParams.get('next') ?? '/');
	}
}
