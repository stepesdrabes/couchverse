export type CouchRole = 'host' | 'follower';

/** What the host is watching. An empty kind means the host is on the browse
 * screen ("choosing what to watch"). */
export interface MediaRef {
	kind: 'movie' | 'episode' | '';
	titleId?: string;
	episodeId?: string;
}

export interface Participant {
	id: string;
	displayName: string;
	avatarId?: string | null;
	seed?: string;
	isHost: boolean;
	isAnonymous: boolean;
	paused?: boolean; // follower paused their own playback locally
}

/** Authoritative play-state relayed from the host (server-stamped). */
export interface HostState {
	media: MediaRef;
	playing: boolean;
	positionSeconds: number;
	serverTimestamp: number;
	seq: number;
	away: boolean;
}

export interface Snapshot {
	sessionId: string;
	shareToken: string;
	myParticipantId: string;
	role: CouchRole;
	isAnonymous: boolean;
	state: HostState;
	participants: Participant[];
}

export interface Reaction {
	id: number;
	participantId: string;
	emoji: string;
}
