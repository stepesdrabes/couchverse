import { error } from '@sveltejs/kit';
import { ApiError } from '$lib/api/client';
import { createJitSession, getPlayback, type PlaybackKind } from '$lib/features/playback/api';

export async function load({ params }) {
	const kind = params.kind as PlaybackKind;
	if (kind !== 'movie' && kind !== 'episode') {
		error(404, 'Not found');
	}
	try {
		const info = await getPlayback(kind, Number(params.id));
		let jitSessionId: string | null = null;

		if (info.mode === 'jit') {
			// open an instant-play session and treat it as a regular HLS stream
			const session = await createJitSession(info.mediaFileId, info.resumePosition);
			info.mode = 'hls';
			info.streamUrl = session.playlistUrl;
			jitSessionId = session.sessionId;
		}

		return { info, kind, id: Number(params.id), jitSessionId };
	} catch (err) {
		if (err instanceof ApiError && err.status === 404) {
			error(404, err.message);
		}
		throw err;
	}
}
