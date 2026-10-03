import * as catalog from '$lib/features/catalog/api';

// Non-blocking: navigation swaps in immediately. The page paints the core's cached title at
// once (or a skeleton on a cold visit) and this reload fills in behind it.
export function load({ params }) {
	return { screen: catalog.revisit(catalog.titleScreen(params.slug)) };
}
