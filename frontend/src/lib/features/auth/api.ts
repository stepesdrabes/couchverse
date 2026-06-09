import { api } from '$lib/api/client';

export interface User {
	id: number;
	username: string;
	displayName: string;
	role: 'admin' | 'member';
	disabled: boolean;
	createdAt: string;
}

export const me = () => api<User>('/auth/me', { skipAuthRedirect: true });

export const login = (username: string, password: string) =>
	api<User>('/auth/login', {
		method: 'POST',
		body: { username, password },
		skipAuthRedirect: true
	});

export const logout = () => api<void>('/auth/logout', { method: 'POST' });
