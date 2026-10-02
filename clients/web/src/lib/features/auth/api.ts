import { api } from '$lib/api/client';

export interface User {
	id: number;
	username: string;
	displayName: string;
	role: 'admin' | 'member';
	disabled: boolean;
	avatarId: string | null;
	bannerId: string | null;
	/** markdown, rendered client-side with raw HTML disabled */
	bio: string;
	createdAt: string;
}

export const updateProfile = (displayName: string, bio?: string) =>
	api<User>('/me/profile', { method: 'PATCH', body: { displayName, bio } });

/** avatar and banner are the same multipart upload against different endpoints */
async function uploadImage(kind: 'avatar' | 'banner', file: File): Promise<User> {
	const form = new FormData();
	form.set('file', file);
	const res = await fetch(`/api/v1/me/${kind}`, {
		method: 'POST',
		body: form,
		credentials: 'same-origin'
	});
	const data = await res.json().catch(() => null);
	if (!res.ok) throw new Error(data?.error?.message ?? `${kind} upload failed`);
	return data as User;
}

export const uploadAvatar = (file: File) => uploadImage('avatar', file);
export const uploadBanner = (file: File) => uploadImage('banner', file);

export const deleteAvatar = () => api<User>('/me/avatar', { method: 'DELETE' });
export const deleteBanner = () => api<User>('/me/banner', { method: 'DELETE' });

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
