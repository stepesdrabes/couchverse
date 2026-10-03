import { goto } from '$app/navigation';
import { toast } from 'svelte-sonner';
import { untrack } from 'svelte';
import { core } from '$lib/core';
import { session } from '$lib/features/auth/session.svelte';
import {
	CouchRole,
	CouchStatus,
	type CouchMember,
	type CouchView,
	type Surface
} from '$lib/generated/core';
import * as m from '$lib/paraglide/messages';

const COUCH: Surface = { type: 'couch' };

// any page can show the couch bar, so the view is kept current for the app's lifetime
core.watch(COUCH);

/**
 * The live couch session as the core runs it: the socket, the follower's sync to the host and
 * the host's broadcasts all live there. This reads the core's view for the couch UI and sends
 * the viewer's actions.
 */
class Couch {
	/** the player's chrome is up, so the couch bar can dodge it */
	playerControlsVisible = $state(false);
	/** >0 while a VideoPlayer is on screen (it hosts the couch bar) */
	playerMounts = $state(0);

	#leaving = false;

	get view(): CouchView | undefined {
		return core.started ? core.view<CouchView>(COUCH) : undefined;
	}

	get playerMounted() {
		return this.playerMounts > 0;
	}

	get active() {
		const status = this.view?.status;
		return (
			status === CouchStatus.Connecting ||
			status === CouchStatus.Open ||
			status === CouchStatus.Reconnecting
		);
	}

	get isHost() {
		return this.view?.role === CouchRole.Host;
	}

	get isFollower() {
		return this.active && this.view?.role === CouchRole.Follower;
	}

	get participants(): CouchMember[] {
		return this.view?.members ?? [];
	}

	get count() {
		return this.participants.length;
	}

	get host() {
		return this.participants.find((p) => p.host) ?? null;
	}

	get reactions() {
		return this.view?.reactions ?? [];
	}

	get recentEmojis() {
		return this.view?.recentEmojis ?? [];
	}

	get shareCode() {
		return this.view?.code ?? '';
	}

	get shareLink() {
		return this.view?.shareUrl ?? '';
	}

	/** A follower waits while the host chooses what to watch or is away. */
	get waiting() {
		return this.isFollower && !!this.view?.waiting;
	}

	get hostPaused() {
		const view = this.view;
		return (
			this.isFollower &&
			!!view &&
			!view.playing &&
			!view.hostAway &&
			!view.localPaused &&
			!view.waiting
		);
	}

	get resyncVisible() {
		return this.isFollower && !!this.view?.resynced;
	}

	/** Hosts a session around what is playing. */
	async start() {
		this.#leaving = false;
		await core.send({ type: 'couchStartRequested' });
		if (!this.active && this.view?.problem) toast.error(m.couch_start_failed());
	}

	/** Joins on the viewer's click, which is what lets the follower's video start playing. */
	join(code: string) {
		this.#leaving = false;
		return core.send({ type: 'couchJoinRequested', content: { code } });
	}

	/** Leaves on the viewer's word, out of the follower's locked player: a guest without an
	 * account to the sign-in page, a member back to the app. */
	leave() {
		if (!this.disconnect()) return;
		goto(session.user ? '/' : '/login');
	}

	/** Leaves without going anywhere (the page is already going); false without a session. */
	disconnect(): boolean {
		if (!this.active) return false;
		this.#leaving = true;
		void core.send({ type: 'couchLeft' });
		return true;
	}

	end() {
		this.#leaving = true;
		return core.send({ type: 'couchEndRequested' });
	}

	sendEmoji(emoji: string) {
		void core.send({ type: 'couchEmojiSent', content: { emoji } });
	}

	/** A follower pauses for themselves; resuming catches up with the host. */
	pauseLocally(paused: boolean) {
		void core.send({ type: 'couchLocalPauseChanged', content: { paused } });
	}

	/** The host ended the session on a follower: nothing is left to watch. */
	ended() {
		if (this.#leaving) return;
		if (session.user) {
			toast.info(m.couch_session_ended_title());
			goto('/');
		} else {
			goto('/login');
		}
	}
}

export const couch = new Couch();

$effect.root(() => {
	let follower = false;
	$effect(() => {
		const view = couch.view;
		const ended = view?.status === CouchStatus.Ended;
		if (follower && ended) untrack(() => couch.ended());
		if (view?.role || ended) follower = view?.role === CouchRole.Follower;
	});
});
