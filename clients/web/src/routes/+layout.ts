import { core } from '$lib/core';
import { session } from '$lib/features/auth/session.svelte';
import { preferences } from '$lib/features/preferences/preferences.svelte';
import { followAccountLang } from '$lib/i18n/locale.svelte';

// Static SPA: everything renders client-side; the Go server provides the
// index.html fallback for deep links.
export const ssr = false;
export const prerender = false;

export async function load() {
	// the core loads the session (user, flags, language, accent); the web's own preferences
	// follow for a signed-in visitor only, so a signed-out page asks the server once
	await core.start();
	if (session.user) await preferences.init();
	if (session.user && followAccountLang()) {
		// reloading into the account's language: rendering now would flash the old one
		await new Promise(() => {});
	}
}
