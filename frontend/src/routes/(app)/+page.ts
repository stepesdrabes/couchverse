import * as catalog from '$lib/features/catalog/api';

export async function load() {
	return await catalog.home();
}
