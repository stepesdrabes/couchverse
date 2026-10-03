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

export const login = (username: string, password: string) =>
	api<User>('/auth/login', {
		method: 'POST',
		body: { username, password },
		skipAuthRedirect: true
	});

export {
	approvePairing,
	createConnectCode,
	denyPairing,
	getPairingRequest,
	listDevices,
	revokeDevice
} from '$lib/generated/api';
export type { Device, DevicePlatform, PairingRequest } from '$lib/generated/api';
