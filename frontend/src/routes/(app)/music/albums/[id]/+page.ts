import * as musicApi from '$lib/features/music/api';
import { albumCache } from '$lib/features/music/cache.svelte';

// Non-blocking: paints from cache at once; a cold 404 shows notFound in the page.
export function load({ params }) {
	return {
		id: params.id,
		fresh: albumCache.revalidate(params.id, () => musicApi.getAlbum(params.id))
	};
}
