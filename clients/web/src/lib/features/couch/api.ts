import { api } from '$lib/api/client';
import { useArtworkGrant } from '$lib/features/catalog/api';
import {
	clientCaps,
	createJitSession,
	type PlaybackInfo,
	type PlaybackKind
} from '$lib/features/playback/api';
import type { MediaRef, Snapshot } from './types';

export { couchSocketPath } from '$lib/generated/api';

export interface CouchPlaybackResp {
	media: MediaRef;
	player?: PlaybackInfo; // absent while the host is choosing
}

export interface CouchInfo {
	shareCode: string;
	hostName: string;
	hostAvatarId: string | null;
	hostSeed: string;
	playing: boolean;
	participants: number;
	/** absent while the host is choosing */
	display?: {
		title: string;
		subtitle: string;
		backdropId: string | null;
		backdropAccent: string;
	};
	artworkGrant: string;
}

/** Preview a session for the pre-join screen, without joining. Public. */
export async function getCouchInfo(token: string) {
	const info = await api<CouchInfo>(`/couch/${token}/info`, { skipAuthRedirect: true });
	useArtworkGrant(info.artworkGrant);
	return info;
}

/** Host creates (or reclaims) a session for the title/episode they're watching. */
export const createCouch = (kind: PlaybackKind, id: string) =>
	api<Snapshot>('/couch', { method: 'POST', body: { kind, id } });

/** Anyone (logged-in or anonymous) joins via the share token. skipAuthRedirect
 * keeps an anonymous viewer from being bounced to the login screen. */
export async function joinCouch(token: string) {
	const snap = await api<Snapshot>(`/couch/${token}/join`, {
		method: 'POST',
		skipAuthRedirect: true
	});
	useArtworkGrant(snap.artworkGrant);
	return snap;
}

/** The follower player payload for the session's current media (couch-authorized). */
export const getCouchPlayback = (token: string) =>
	api<CouchPlaybackResp>(`/couch/${token}/playback?caps=${clientCaps().join(',')}`, {
		skipAuthRedirect: true
	});

/** Fetch the follower's current media payload and, when it needs on-demand
 * transcoding, open a per-viewer JIT session (the payload's media grant, bound to
 * this follower, authorizes it even without an account). */
export async function resolveCouchPlayer(
	token: string
): Promise<{ player: PlaybackInfo | null; jitSessionId: string | null }> {
	const resp = await getCouchPlayback(token);
	let player = resp.player ?? null;
	let jitSessionId: string | null = null;
	if (player && player.mode === 'jit') {
		const session = await createJitSession(player.grant, 0);
		player = { ...player, mode: 'hls', streamUrl: session.playlistUrl };
		jitSessionId = session.sessionId;
	}
	return { player, jitSessionId };
}

export const leaveCouch = (token: string) =>
	api<void>(`/couch/${token}/leave`, { method: 'POST', skipAuthRedirect: true });

export const endCouch = (token: string) =>
	api<void>(`/couch/${token}/end`, { method: 'POST', skipAuthRedirect: true });
