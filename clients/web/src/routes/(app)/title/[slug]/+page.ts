import * as catalog from '$lib/features/catalog/api';
import { titleCache } from '$lib/features/catalog/cache.svelte';

// Non-blocking: navigation swaps in immediately. The page paints cached data at
// once (or a skeleton on a cold visit); this revalidation fills in behind it and
// the 404 case is handled in the component from the rejected `fresh` promise.
export function load({ params }) {
	return {
		slug: params.slug,
		fresh: titleCache.revalidate(params.slug, () => catalog.getTitle(params.slug))
	};
}
