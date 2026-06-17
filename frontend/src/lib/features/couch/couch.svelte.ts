import { goto } from '$app/navigation';
import { toast } from 'svelte-sonner';
import type { PlaybackInfo, PlaybackKind } from '$lib/features/playback/api';
import * as m from '$lib/paraglide/messages';
import * as couchApi from './api';
import type { CouchRole, HostState, MediaRef, Participant, Reaction, Snapshot } from './types';

const DRIFT_THRESHOLD = 3; // seconds before a follower hard-seeks back into sync
const HEARTBEAT_MS = 2000; // host re-broadcasts its play-state at this cadence
const MAX_BACKOFF_MS = 8000;
const REACTION_TTL_MS = 2200; // matches the rise-and-grow animation before cleanup
const RESYNC_NOTE_MS = 2500;
const RECENT_EMOJI_KEY = 'cv.couchRecentEmojis';
const RECENT_EMOJI_MAX = 3;

function loadRecentEmojis(): string[] {
	try {
		const raw = localStorage.getItem(RECENT_EMOJI_KEY);
		return raw ? (JSON.parse(raw) as string[]).slice(0, RECENT_EMOJI_MAX) : [];
	} catch {
		return [];
	}
}

/**
 * Single source of truth for a live couch session, mirroring the music player's
 * singleton-store pattern. It owns the WebSocket, the follower sync loop and the
 * host broadcast, and is consumed directly by VideoPlayer at its seams.
 */
class Couch {
	token = $state<string | null>(null);
	role = $state<CouchRole | null>(null);
	myParticipantId = $state<string | null>(null);
	isAnonymous = $state(false);
	participants = $state<Participant[]>([]);
	hostState = $state<HostState | null>(null);
	hostAway = $state(false);
	status = $state<'idle' | 'connecting' | 'open' | 'reconnecting' | 'closed'>('idle');
	reactions = $state<Reaction[]>([]);
	resyncVisible = $state(false);
	playerInfo = $state<PlaybackInfo | null>(null); // follower's current media payload
	jitSessionId = $state<string | null>(null); // follower's own instant-play session, if any
	mediaKey = $state(0); // bumped on media switch to re-key the follower's player
	playerControlsVisible = $state(false); // player chrome state, so the couch bar can dodge it
	playerMounts = $state(0); // >0 while a VideoPlayer is on screen (it hosts the couch bar)
	recentEmojis = $state<string[]>(loadRecentEmojis()); // last few sent, for quick re-send

	get playerMounted() {
		return this.playerMounts > 0;
	}

	private locallyPaused = false;
	private hostMedia: MediaRef = { kind: '' };
	private video: HTMLVideoElement | undefined;
	private receivedAtMs = 0;
	private lastSeq = -1;
	private ws: WebSocket | null = null;
	private reconnectTimer: ReturnType<typeof setTimeout> | undefined;
	private heartbeatTimer: ReturnType<typeof setInterval> | undefined;
	private resyncTimer: ReturnType<typeof setTimeout> | undefined;
	private reactionSeq = 0;
	private backoff = 500;

	// --- reactive getters the player and chrome read ---
	get active() {
		return this.token !== null;
	}
	get isHost() {
		return this.role === 'host';
	}
	get isFollower() {
		return this.active && this.role === 'follower';
	}
	get followerLocked() {
		return this.isFollower;
	}
	get waiting() {
		return this.isFollower && (this.hostAway || (this.hostState?.media.kind ?? '') === '');
	}
	get hostPaused() {
		return (
			this.isFollower &&
			!!this.hostState &&
			!this.hostState.playing &&
			!this.hostAway &&
			!this.locallyPaused
		);
	}
	get host() {
		return this.participants.find((p) => p.isHost) ?? null;
	}
	get shareLink() {
		return this.token ? `${location.origin}/couch/${this.token}` : '';
	}
	get shareCode() {
		return this.token ?? '';
	}
	get count() {
		return this.participants.length;
	}
	get expectedPosition(): number {
		const s = this.hostState;
		if (!s) return 0;
		if (!s.playing) return s.positionSeconds;
		return s.positionSeconds + (performance.now() - this.receivedAtMs) / 1000;
	}

	// --- lifecycle ---
	async startSession(kind: PlaybackKind, id: string) {
		const snap = await couchApi.createCouch(kind, id);
		this.hostMedia = mediaRefFor(kind, id);
		this.hydrate(snap);
		this.connect();
	}

	/** A follower joins on an explicit user gesture (the "Start watching" button),
	 * which is what lets the video autoplay. Joins, resolves the payload and
	 * connects in one go. */
	async joinByToken(token: string) {
		const snap = await couchApi.joinCouch(token);
		const { player, jitSessionId } = await couchApi.resolveCouchPlayer(token);
		this.hydrate(snap);
		this.playerInfo = player;
		this.jitSessionId = jitSessionId;
		this.mediaKey++;
		this.connect();
	}

	private hydrate(snap: Snapshot) {
		this.token = snap.shareToken;
		this.role = snap.role;
		this.myParticipantId = snap.myParticipantId;
		this.isAnonymous = snap.isAnonymous;
		this.participants = snap.participants;
		this.hostState = snap.state;
		this.receivedAtMs = performance.now();
		this.lastSeq = snap.state.seq;
		this.hostAway = snap.state.away;
	}

	/** The user explicitly leaves: disconnect and route them out of the locked
	 * player - anonymous viewers to login, logged-in followers back to the app. */
	leave() {
		const wasAnon = this.isAnonymous;
		if (!this.disconnect()) return;
		goto(wasAnon ? '/login' : '/');
	}

	/** Disconnect without routing (used on unmount, where navigation is already
	 * happening). Returns false if there was no live session. */
	disconnect(): boolean {
		const token = this.token;
		if (!token) return false;
		this.teardown();
		couchApi.leaveCouch(token).catch(() => {});
		return true;
	}

	async end() {
		const token = this.token;
		this.teardown();
		if (token) {
			try {
				await couchApi.endCouch(token);
			} catch {
				// best effort
			}
		}
	}

	// --- player seams (called by VideoPlayer) ---
	bindVideo(el: HTMLVideoElement | undefined) {
		this.video = el;
		if (!el || !this.isFollower) return;
		// jump to the live position as soon as the element can seek
		if (el.readyState >= 1) this.resync('hard');
		else el.addEventListener('loadedmetadata', () => this.resync('hard'), { once: true });
	}
	onPlayStateChange(playing: boolean, positionSeconds: number) {
		if (this.isHost) this.sendHostState(playing, positionSeconds);
	}
	onSeek(positionSeconds: number) {
		if (this.isHost) this.sendHostState(!this.video?.paused, positionSeconds);
	}
	markLocalPause() {
		if (this.isFollower) this.setLocalPaused(true);
	}
	onLocalUnpause() {
		if (this.isFollower) {
			this.setLocalPaused(false);
			this.resync('hard');
		}
	}

	private setLocalPaused(paused: boolean) {
		if (this.locallyPaused === paused) return;
		this.locallyPaused = paused;
		this.sendRaw({ type: 'paused', data: { paused } }); // shows on everyone's couch
	}

	/** The root layout calls this after every navigation so a host switching
	 * episode/title (or leaving the player) propagates to followers. */
	setHostMedia(kind: string, id: string) {
		if (!this.isHost) return;
		this.hostMedia = mediaRefFor(kind, id);
		this.sendHostState(this.video ? !this.video.paused : false, this.video?.currentTime ?? 0);
	}

	sendEmoji(emoji: string) {
		this.sendRaw({ type: 'emoji', data: { emoji } });
		this.recentEmojis = [emoji, ...this.recentEmojis.filter((e) => e !== emoji)].slice(
			0,
			RECENT_EMOJI_MAX
		);
		try {
			localStorage.setItem(RECENT_EMOJI_KEY, JSON.stringify(this.recentEmojis));
		} catch {
			// storage unavailable
		}
	}

	// --- WebSocket ---
	private connect() {
		this.backoff = 500;
		this.openSocket();
	}

	private openSocket() {
		if (!this.token) return;
		this.status = this.status === 'reconnecting' ? 'reconnecting' : 'connecting';
		const proto = location.protocol === 'https:' ? 'wss' : 'ws';
		const ws = new WebSocket(`${proto}://${location.host}/api/v1/couch/${this.token}/ws`);
		this.ws = ws;
		ws.onopen = () => {
			this.status = 'open';
			this.backoff = 500;
			if (this.isHost) this.startHeartbeat();
			if (this.isFollower) this.resync('hard');
		};
		ws.onmessage = (e) => this.onMessage(e);
		ws.onclose = () => this.onClose();
		ws.onerror = () => ws.close();
	}

	private onClose() {
		if (!this.token) return; // intentional teardown
		this.status = 'reconnecting';
		clearTimeout(this.reconnectTimer);
		this.reconnectTimer = setTimeout(() => this.openSocket(), this.backoff);
		this.backoff = Math.min(this.backoff * 2, MAX_BACKOFF_MS);
	}

	private onMessage(e: MessageEvent) {
		if (!this.token) return;
		let env: { type: string; data?: Record<string, unknown> };
		try {
			env = JSON.parse(e.data);
		} catch {
			return;
		}
		switch (env.type) {
			case 'hello':
				this.role = (env.data?.role as CouchRole) ?? this.role;
				this.myParticipantId = (env.data?.myParticipantId as string) ?? this.myParticipantId;
				this.participants = (env.data?.participants as Participant[]) ?? this.participants;
				this.applyState(env.data?.state as HostState, true);
				if (this.isHost) this.startHeartbeat();
				break;
			case 'host_state':
				this.applyState(env.data as unknown as HostState, false);
				break;
			case 'participants':
				this.participants = (env.data?.participants as Participant[]) ?? [];
				break;
			case 'media_changed':
				this.refreshPlayer();
				break;
			case 'host_away':
				this.hostAway = true;
				this.video?.pause();
				break;
			case 'host_returned':
				this.hostAway = false;
				if (this.isFollower) this.resync('hard');
				break;
			case 'emoji':
				this.pushReaction(env.data as { fromParticipantId: string; emoji: string });
				break;
			case 'session_ended':
				this.onSessionEnded();
				break;
		}
	}

	private applyState(state: HostState | undefined, force: boolean) {
		if (!state) return;
		if (!force && state.seq <= this.lastSeq) return; // stale / out of order
		this.lastSeq = state.seq;
		this.hostState = state;
		this.receivedAtMs = performance.now();
		this.hostAway = state.away;
		if (this.isFollower) this.resync(force ? 'hard' : 'soft');
	}

	private onSessionEnded() {
		const role = this.role;
		const wasAnon = this.isAnonymous;
		this.teardown();
		if (role === 'host') return; // the host stays put; playback simply unsynced now
		if (wasAnon) {
			goto('/login');
		} else {
			toast.info(m.couch_session_ended_title());
			goto('/');
		}
	}

	private async refreshPlayer() {
		if (!this.token || this.isHost) return;
		this.setLocalPaused(false); // a fresh title starts unpaused
		try {
			const { player, jitSessionId } = await couchApi.resolveCouchPlayer(this.token);
			this.playerInfo = player;
			this.jitSessionId = jitSessionId;
		} catch {
			this.playerInfo = null;
			this.jitSessionId = null;
		}
		this.mediaKey++;
	}

	private sendHostState(playing: boolean, positionSeconds: number) {
		this.sendRaw({ type: 'host_state', data: { media: this.hostMedia, playing, positionSeconds } });
	}

	private sendRaw(msg: unknown) {
		if (this.ws?.readyState === WebSocket.OPEN) this.ws.send(JSON.stringify(msg));
	}

	private startHeartbeat() {
		clearInterval(this.heartbeatTimer);
		this.heartbeatTimer = setInterval(() => {
			if (!this.isHost || !this.video) return;
			this.sendHostState(!this.video.paused, this.video.currentTime);
		}, HEARTBEAT_MS);
	}

	// --- follower drift correction (local-clock based; no server clock math) ---
	private resync(mode: 'soft' | 'hard') {
		const v = this.video;
		const s = this.hostState;
		if (!v || !s || !this.isFollower || this.locallyPaused) return;

		// match the host's play/pause first
		if (this.hostAway || !s.playing) {
			if (!v.paused) v.pause();
			return; // nothing to drift-correct while stopped
		}
		if (v.paused) v.play().catch(() => {}); // stays blocked until the user gesture (pre-join screen)

		if (v.readyState < 1) return; // can't seek before metadata
		// for soft (steady-state) correction, don't fight a seeking/buffering element -
		// repeatedly seeking a not-ready video is what caused the jumping. A hard
		// resync (join/reconnect/unpause) still snaps to the live position.
		if (mode === 'soft' && (v.seeking || v.readyState < 3)) return;

		const expected = this.expectedPosition;
		if (!Number.isFinite(expected) || expected < 0) return;
		const drift = Math.abs(v.currentTime - expected);
		if (mode === 'hard' || drift > DRIFT_THRESHOLD) {
			v.currentTime = expected;
			if (drift > DRIFT_THRESHOLD) this.flashResync();
		}
	}

	private flashResync() {
		this.resyncVisible = true;
		clearTimeout(this.resyncTimer);
		this.resyncTimer = setTimeout(() => (this.resyncVisible = false), RESYNC_NOTE_MS);
	}

	private pushReaction(d: { fromParticipantId: string; emoji: string }) {
		const id = ++this.reactionSeq;
		this.reactions = [
			...this.reactions,
			{ id, participantId: d.fromParticipantId, emoji: d.emoji }
		];
		setTimeout(() => (this.reactions = this.reactions.filter((r) => r.id !== id)), REACTION_TTL_MS);
	}

	private teardown() {
		const ws = this.ws;
		this.ws = null;
		if (ws) {
			ws.onclose = null;
			ws.onmessage = null;
			ws.onerror = null;
			ws.onopen = null;
			try {
				ws.close();
			} catch {
				// already closing
			}
		}
		clearTimeout(this.reconnectTimer);
		clearInterval(this.heartbeatTimer);
		clearTimeout(this.resyncTimer);
		this.heartbeatTimer = undefined;
		this.token = null;
		this.role = null;
		this.myParticipantId = null;
		this.isAnonymous = false;
		this.participants = [];
		this.hostState = null;
		this.hostAway = false;
		this.playerInfo = null;
		this.jitSessionId = null;
		this.reactions = [];
		this.resyncVisible = false;
		this.locallyPaused = false;
		this.hostMedia = { kind: '' };
		this.lastSeq = -1;
		this.status = 'idle';
	}
}

function mediaRefFor(kind: string, id: string): MediaRef {
	if (kind === 'movie') return { kind, titleId: id };
	if (kind === 'episode') return { kind, episodeId: id };
	return { kind: '' };
}

export const couch = new Couch();
