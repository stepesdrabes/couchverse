import { error } from '@sveltejs/kit';
import * as catalog from '$lib/features/catalog/api';
import { ApiError } from '$lib/api/client';

export async function load({ params }) {
	try {
		return await catalog.getTitle(params.slug);
	} catch (err) {
		if (err instanceof ApiError && err.status === 404) {
			error(404, 'Title not found');
		}
		throw err;
	}
}
