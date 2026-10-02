// Admin user management. The User entity lives with the auth feature.
import { api } from '$lib/api/client';
import type { User } from '$lib/features/auth/api';

export const listUsers = () => api<User[]>('/admin/users');

export const createUser = (input: {
	username: string;
	displayName?: string;
	password: string;
	role: string;
}) => api<User>('/admin/users', { method: 'POST', body: input });

export const updateUser = (
	id: number,
	patch: Partial<{ displayName: string; role: string; disabled: boolean; password: string }>
) => api<User>(`/admin/users/${id}`, { method: 'PATCH', body: patch });

export const deleteUser = (id: number) => api<void>(`/admin/users/${id}`, { method: 'DELETE' });
