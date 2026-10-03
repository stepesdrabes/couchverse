import * as catalog from '$lib/features/catalog/api';
import { BrowseSort, TitleKind } from '$lib/generated/core';

export function load() {
	return { key: catalog.preloadListing({ kind: TitleKind.Movie, sort: BrowseSort.Added }) };
}
