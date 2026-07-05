import { error } from '@sveltejs/kit';
import * as libraryApi from '$lib/features/library/api';
import { features } from '$lib/features/settings/features.svelte';

// Non-blocking: the editor swaps in immediately behind a skeleton; a save's
// invalidateAll revalidates in place (StreamedView keeps the last value, no flash).
export function load({ params }) {
	if (!features.musicEnabled) error(404, 'Not found');
	return { id: params.id, fresh: libraryApi.getAdminAlbum(params.id) };
}
