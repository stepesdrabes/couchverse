import { error } from '@sveltejs/kit';
import { features } from '$lib/features/settings/features.svelte';

export function load() {
	if (!features.musicEnabled) error(404, 'Not found');
}
