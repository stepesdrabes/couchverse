import { goto } from '$app/navigation';
import { onUnauthorized } from '$lib/api/client';
import { features } from '$lib/features/settings/features.svelte';
import { preferences } from '$lib/features/preferences/preferences.svelte';
import * as authApi from './api';
import type { User } from './api';

class Session {
	user = $state<User | null>(null);

	get isAdmin() {
		return this.user?.role === 'admin';
	}

	/** resolve the cookie session once at app start */
	async init() {
		try {
			this.user = await authApi.me();
		} catch {
			this.user = null;
		}
	}

	async login(username: string, password: string) {
		this.user = await authApi.login(username, password);
		await Promise.all([features.init(), preferences.init()]);
	}

	async logout() {
		try {
			await authApi.logout();
		} finally {
			this.user = null;
			goto('/login');
		}
	}
}

export const session = new Session();

onUnauthorized(() => {
	session.user = null;
	const here = location.pathname + location.search;
	goto(`/login?next=${encodeURIComponent(here)}`);
});
