// Media libraries on disk, scans and the background job queue.
import { api, qs } from '$lib/api/client';

export interface Library {
	id: number;
	name: string;
	kind: 'movies' | 'series' | 'music';
	path: string;
	managed: boolean;
	lastScannedAt: string | null;
}

export interface Job {
	id: number;
	type: string;
	payload: Record<string, unknown>;
	status: 'pending' | 'running' | 'done' | 'failed' | 'cancelled';
	priority: number;
	runAt: string;
	attempts: number;
	maxAttempts: number;
	progress: number;
	lastError: string | null;
	claimedAt: string | null;
	createdAt: string;
	finishedAt: string | null;
}

export const listLibraries = () => api<Library[]>('/admin/libraries');

export const createLibrary = (input: { name: string; kind: string; path: string }) =>
	api<Library>('/admin/libraries', { method: 'POST', body: input });

export const deleteLibrary = (id: number) =>
	api<void>(`/admin/libraries/${id}`, { method: 'DELETE' });

export const scanLibrary = (id: number) =>
	api<{ jobId: number }>(`/admin/libraries/${id}/scan`, { method: 'POST' });

export const scanAllLibraries = () =>
	api<{ libraries: number }>('/admin/libraries/scan-all', { method: 'POST' });

export const listJobs = (status = '', limit = 50) =>
	api<Job[]>(`/admin/jobs${qs({ status, limit })}`);

export const retryJob = (id: number) => api<void>(`/admin/jobs/${id}/retry`, { method: 'POST' });
export const cancelJob = (id: number) => api<void>(`/admin/jobs/${id}/cancel`, { method: 'POST' });
