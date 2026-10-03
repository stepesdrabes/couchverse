import { untrack } from 'svelte';
import { goto } from '$app/navigation';
import { navigating, page } from '$app/state';
import { onUnauthorized } from '$lib/api/client';
import { resetAllCaches } from '$lib/api/cache.svelte';
import { core } from '$lib/core';
import { preferences } from '$lib/features/preferences/preferences.svelte';
import { rank } from '$lib/features/ranks/rank.svelte';
import type { Event as CoreEvent } from '$lib/generated/core';
import * as authApi from './api';

/** The signed-in session, as the core holds it; the web keeps no copy of its own. */
class Session {
	#rechecking: Promise<void> | null = null;
	// a session the visitor ended is not one that expired
	#leaving = false;

	get user() {
		return core.session.user ?? null;
	}

	get isAdmin() {
		return this.user?.admin ?? false;
	}

	/** the web's login form posts the credentials itself, then the core loads the session */
	async login(username: string, password: string) {
		await authApi.login(username, password);
		this.#leaving = false;
		await Promise.all([this.#send({ type: 'sessionStarted' }), preferences.init()]);
	}

	async logout() {
		const accountId = core.session.accountId ?? '';
		this.#leaving = true;
		await core.send({ type: 'signOutRequested', content: { accountId } });
		clear();
		goto('/login');
	}

	/** the web changed what the session shows through its own API call (profile, settings) */
	refresh() {
		return this.#send({ type: 'sessionChanged' });
	}

	/** an app tab coming back into view reads the session again */
	resume() {
		if (core.started && this.user) void this.#send({ type: 'appBecameActive' });
	}

	/**
	 * A web API call came back 401. The core confirms with its own read before anything is
	 * dropped, once however many calls failed together.
	 */
	recheck() {
		this.#rechecking ??= this.refresh().finally(() => (this.#rechecking = null));
		return this.#rechecking;
	}

	/**
	 * The core found the session gone by itself: a 401 on one of its own requests (the
	 * catalog's) or the check a web API call's 401 asked for. Session-only pages go to sign in.
	 */
	async ended() {
		if (this.#leaving) return;
		clear();
		// a navigation under way lands first: cutting it short throws in SvelteKit's link
		// handler, and the visitor comes back to where they were going
		await navigating.complete?.catch(() => {});
		if (requiresSession(page.route.id)) {
			goto(`/login?next=${encodeURIComponent(location.pathname + location.search)}`);
		}
	}

	async #send(event: CoreEvent) {
		// a 401 can come back before the root layout has started the core
		await core.start();
		await core.send(event);
	}
}

/** the next account starts clean */
function clear() {
	resetAllCaches();
	rank.reset();
}

// the anonymous pages (login, a couch link) stay put when a session ends
function requiresSession(routeId: string | null) {
	return routeId?.startsWith('/(app)') || routeId?.startsWith('/admin');
}

export const session = new Session();

// whichever request found the session gone, the web follows the user going away
$effect.root(() => {
	let signedIn = false;
	$effect(() => {
		const now = core.started && core.session.user !== undefined;
		if (signedIn && !now) untrack(() => void session.ended());
		signedIn = now;
	});
});

onUnauthorized(() => void session.recheck());

document.addEventListener('visibilitychange', () => {
	if (document.visibilityState === 'visible') session.resume();
});
