import * as catalog from '$lib/features/catalog/api';

// Non-blocking: Home paints what the core holds (on a cold start the last home it kept)
// or a skeleton, while this reload brings continue watching up to date behind it.
export function load() {
	return { screen: catalog.revisit(catalog.homeScreen) };
}
