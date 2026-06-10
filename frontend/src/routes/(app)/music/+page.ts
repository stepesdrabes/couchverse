import * as musicApi from '$lib/features/music/api';

export async function load() {
	return await musicApi.musicHome();
}
