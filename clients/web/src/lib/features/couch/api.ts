import { api } from '$lib/api/client';
import { useArtworkGrant } from '$lib/features/catalog/api';

// The couch itself (joining, the socket, following the host) runs in the core; the web only
// previews a session before the viewer joins it.

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
