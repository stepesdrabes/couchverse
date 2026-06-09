import { api } from './client';
import type { User } from './types';

export const me = () => api<User>('/auth/me', { skipAuthRedirect: true });

export const login = (username: string, password: string) =>
	api<User>('/auth/login', {
		method: 'POST',
		body: { username, password },
		skipAuthRedirect: true
	});

export const logout = () => api<void>('/auth/logout', { method: 'POST' });
