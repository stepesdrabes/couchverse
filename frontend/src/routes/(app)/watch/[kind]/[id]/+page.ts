import { error } from '@sveltejs/kit';
import { ApiError } from '$lib/api/client';
import { getPlayback, type PlaybackKind } from '$lib/features/playback/api';

export async function load({ params }) {
	const kind = params.kind as PlaybackKind;
	if (kind !== 'movie' && kind !== 'episode') {
		error(404, 'Not found');
	}
	try {
		const info = await getPlayback(kind, Number(params.id));
		return { info, kind, id: Number(params.id) };
	} catch (err) {
		if (err instanceof ApiError && err.status === 404) {
			error(404, err.message);
		}
		throw err;
	}
}
