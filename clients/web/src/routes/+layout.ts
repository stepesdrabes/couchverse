import { getTheme } from '$lib/features/settings/api';
import { session } from '$lib/features/auth/session.svelte';
import { features } from '$lib/features/settings/features.svelte';
import { preferences } from '$lib/features/preferences/preferences.svelte';
import { applySavedLang } from '$lib/i18n/locale.svelte';
import { applyAccent } from '$lib/theme';

// Static SPA: everything renders client-side; the Go server provides the
// index.html fallback for deep links.
export const ssr = false;
export const prerender = false;

export async function load() {
	// accent is public so it themes the login screen too
	getTheme()
		.then((t) => applyAccent(t.accent))
		.catch(() => {});

	await session.init();
	if (session.user) {
		await Promise.all([features.init(), preferences.init()]);
		applySavedLang(preferences.language);
	}
}
