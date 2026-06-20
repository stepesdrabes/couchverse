import { api } from '$lib/api/client';

export interface User {
	id: number;
	username: string;
	displayName: string;
	role: 'admin' | 'member';
	disabled: boolean;
	avatarId: string | null;
	createdAt: string;
}

export const updateProfile = (displayName: string) =>
	api<User>('/me/profile', { method: 'PATCH', body: { displayName } });

export async function uploadAvatar(file: File): Promise<User> {
	const form = new FormData();
	form.set('file', file);
	const res = await fetch('/api/v1/me/avatar', {
		method: 'POST',
		body: form,
		credentials: 'same-origin'
	});
	const data = await res.json().catch(() => null);
	if (!res.ok) throw new Error(data?.error?.message ?? 'avatar upload failed');
	return data as User;
}

export const deleteAvatar = () => api<User>('/me/avatar', { method: 'DELETE' });

export const changePassword = (currentPassword: string, newPassword: string) =>
	api<void>('/me/password', { method: 'PATCH', body: { currentPassword, newPassword } });

export const me = () => api<User>('/auth/me', { skipAuthRedirect: true });

export const login = (username: string, password: string) =>
	api<User>('/auth/login', {
		method: 'POST',
		body: { username, password },
		skipAuthRedirect: true
	});

export const logout = () => api<void>('/auth/logout', { method: 'POST' });
