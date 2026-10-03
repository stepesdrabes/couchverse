import * as catalog from '$lib/features/catalog/api';

export function load() {
	return { screen: catalog.revisit(catalog.myListScreen) };
}
