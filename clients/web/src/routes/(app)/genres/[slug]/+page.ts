import * as catalog from '$lib/features/catalog/api';
import { BrowseSort } from '$lib/generated/core';

// a genre's URL carries its English name, the identity the listing is keyed by
export function load({ params }) {
	return { key: catalog.preloadListing({ genre: params.slug, sort: BrowseSort.Added }) };
}
