import * as musicApi from '$lib/features/music/api';
import { artistCache } from '$lib/features/music/cache.svelte';

// Non-blocking: paints from cache at once; a cold 404 shows notFound in the page.
export function load({ params }) {
	return {
		id: params.id,
		fresh: artistCache.revalidate(params.id, () => musicApi.getArtist(params.id))
	};
}
