import { error } from '@sveltejs/kit';
import { ApiError } from '$lib/api/client';
import * as musicApi from '$lib/features/music/api';

export async function load({ params }) {
	try {
		return await musicApi.getAlbum(params.id);
	} catch (err) {
		if (err instanceof ApiError && err.status === 404) {
			error(404, 'Album not found');
		}
		throw err;
	}
}
