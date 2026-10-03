import { error } from '@sveltejs/kit';
import { PlayKind } from '$lib/generated/core';

// Only names what to play: the page asks the core once it is on screen, so even a preload of
// this route could never start a transcode.
export function load({ params }) {
	const kind = params.kind as PlayKind;
	if (kind !== PlayKind.Movie && kind !== PlayKind.Episode) {
		error(404, 'Not found');
	}
	return { target: { kind, id: params.id } };
}
