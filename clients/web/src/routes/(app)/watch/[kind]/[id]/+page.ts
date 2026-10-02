import { error } from '@sveltejs/kit';
import {
	createJitSession,
	getPlayback,
	type PlaybackInfo,
	type PlaybackKind
} from '$lib/features/playback/api';

async function loadPlayback(
	kind: PlaybackKind,
	id: string
): Promise<{ info: PlaybackInfo; jitSessionId: string | null }> {
	const info = await getPlayback(kind, id);
	let jitSessionId: string | null = null;

	if (info.mode === 'jit') {
		// open an instant-play session and treat it as a regular HLS stream
		const session = await createJitSession(info.mediaFileId, info.resumePosition);
		info.mode = 'hls';
		info.streamUrl = session.playlistUrl;
		jitSessionId = session.sessionId;
	}

	return { info, jitSessionId };
}

// Non-blocking: the player page swaps in immediately and shows a loader while this
// resolves, so switching episodes never freezes on the old frame. `depends` lets
// the transcode poll re-run only this load (not the layout's session/features fetch).
// Not cached: getPlayback carries a live resume position and JIT starts a transcode.
export function load({ params, depends }) {
	const kind = params.kind as PlaybackKind;
	if (kind !== 'movie' && kind !== 'episode') {
		error(404, 'Not found');
	}
	depends('app:playback');
	return { kind, id: params.id, playback: loadPlayback(kind, params.id) };
}
