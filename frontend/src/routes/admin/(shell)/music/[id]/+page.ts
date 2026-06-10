import { error } from '@sveltejs/kit';
import { ApiError } from '$lib/api/client';
import * as libraryApi from '$lib/features/library/api';

export async function load({ params }) {
	try {
		return await libraryApi.getAdminAlbum(params.id);
	} catch (err) {
		if (err instanceof ApiError && err.status === 404) {
			error(404, 'Album not found');
		}
		throw err;
	}
}
