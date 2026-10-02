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

export { uploadAvatar, uploadBanner } from '$lib/generated/api';

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

export {
	approvePairing,
	createConnectCode,
	denyPairing,
	getPairingRequest,
	listDevices,
	revokeDevice
} from '$lib/generated/api';
export type { Device, DevicePlatform, PairingRequest } from '$lib/generated/api';
