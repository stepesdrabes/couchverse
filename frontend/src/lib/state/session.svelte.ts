import { goto } from '$app/navigation';
import * as authApi from '$lib/api/auth';
import { onUnauthorized } from '$lib/api/client';
import type { User } from '$lib/api/types';

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
