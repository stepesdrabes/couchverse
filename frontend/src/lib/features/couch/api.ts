import { api } from '$lib/api/client';
import {
	clientCaps,
	createJitSession,
	type PlaybackInfo,
	type PlaybackKind
} from '$lib/features/playback/api';
import type { MediaRef, Snapshot } from './types';

export interface CouchPlaybackResp {
	media: MediaRef;
	player: PlaybackInfo | null; // null while the host is choosing
}

/** Host creates (or reclaims) a session for the title/episode they're watching. */
export const createCouch = (kind: PlaybackKind, id: string) =>
	api<Snapshot>('/couch', { method: 'POST', body: { kind, id } });

/** Anyone (logged-in or anonymous) joins via the share token. skipAuthRedirect
 * keeps an anonymous viewer from being bounced to the login screen. */
export const joinCouch = (token: string) =>
	api<Snapshot>(`/couch/${token}/join`, { method: 'POST', skipAuthRedirect: true });

/** The follower player payload for the session's current media (couch-authorized). */
export const getCouchPlayback = (token: string) =>
	api<CouchPlaybackResp>(`/couch/${token}/playback?caps=${clientCaps().join(',')}`, {
		skipAuthRedirect: true
	});

/** Fetch the follower's current media payload and, when it needs on-demand
 * transcoding, open a per-viewer JIT session (anonymous followers are allowed to
 * because the couch cookie authorizes the host's current media). */
export async function resolveCouchPlayer(
	token: string
): Promise<{ player: PlaybackInfo | null; jitSessionId: string | null }> {
	const resp = await getCouchPlayback(token);
	let player = resp.player;
	let jitSessionId: string | null = null;
	if (player && player.mode === 'jit') {
		const session = await createJitSession(player.mediaFileId, 0);
		player = { ...player, mode: 'hls', streamUrl: session.playlistUrl };
		jitSessionId = session.sessionId;
	}
	return { player, jitSessionId };
}

export const leaveCouch = (token: string) =>
	api<void>(`/couch/${token}/leave`, { method: 'POST', skipAuthRedirect: true });

export const endCouch = (token: string) =>
	api<void>(`/couch/${token}/end`, { method: 'POST', skipAuthRedirect: true });
