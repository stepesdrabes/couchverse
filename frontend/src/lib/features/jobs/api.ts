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
	subject?: JobSubject | null;
}

// The content a job works on, resolved server-side from the payload.
export interface JobSubject {
	mediaFileId?: string;
	titleId?: string;
	titleName?: string;
	titleKind?: 'movie' | 'series';
	seasonNumber?: number;
	episodeNumber?: number;
	episodeName?: string;
	trackName?: string;
	variant?: string;
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

export const listJobs = (filter: { status?: string; limit?: number; mediaFileId?: string } = {}) =>
	api<Job[]>(
		`/admin/jobs${qs({
			status: filter.status ?? '',
			limit: filter.limit ?? 50,
			mediaFileId: filter.mediaFileId ?? ''
		})}`
	);

export const retryJob = (id: number) => api<void>(`/admin/jobs/${id}/retry`, { method: 'POST' });
export const cancelJob = (id: number) => api<void>(`/admin/jobs/${id}/cancel`, { method: 'POST' });

export interface ActiveTranscode {
	jobId: number;
	mediaFileId: string;
	titleId: string | null;
	episodeId: string | null;
	variant: string;
	status: 'pending' | 'running';
	progress: number;
}

export const listActiveTranscodes = () => api<ActiveTranscode[]>('/admin/transcode/active');

// storage & overview
export type StorageCategoryKind = 'movies' | 'series' | 'music' | 'transcodes' | 'cache';

export interface StorageCategory {
	kind: StorageCategoryKind;
	bytes: number;
}

export interface StorageInfo {
	diskTotal: number;
	free: number;
	used: number; // Couchverse's total footprint
	budget: number; // used + free = space available to Couchverse
	categories: StorageCategory[];
}

export const getStorage = () => api<StorageInfo>('/admin/storage');

export interface OverviewInfo {
	counts: {
		movies: number;
		series: number;
		episodes: number;
		albums: number;
		tracks: number;
		users: number;
	};
	pendingJobs: number;
	recentJobs: Job[];
}

export const getOverview = () => api<OverviewInfo>('/admin/overview');

export interface SystemStats {
	cpuPercent: number; // -1 when unavailable (non-Linux host)
	cpuCores: number;
	memUsed: number;
	memTotal: number;
	load1: number; // -1 when unavailable
	goHeapBytes: number;
	goroutines: number;
	uptimeSeconds: number;
	app: { cpuPercent: number; memBytes: number };
	ffmpeg: { cpuPercent: number; memBytes: number; processes: number };
}

export const getSystem = () => api<SystemStats>('/admin/system');

// analytics (watch/listen time rollup)
export interface AnalyticsDay {
	day: string; // YYYY-MM-DD
	videoSeconds: number;
	musicSeconds: number;
	activeUsers: number;
}

export interface AnalyticsInfo {
	days: number;
	daily: AnalyticsDay[];
	totals: { videoSeconds: number; musicSeconds: number; activeUsers: number };
	topTitles: { titleId: string; slug: string; name: string; kind: string; seconds: number }[];
	topUsers: { userId: number; displayName: string; seconds: number }[];
}

export const getAnalytics = (days = 30) =>
	api<AnalyticsInfo>(`/admin/analytics/overview${qs({ days })}`);

// home rows
export interface HomeRowConfig {
	id: number;
	position: number;
	kind: 'continue_watching' | 'recently_added' | 'genre' | 'recently_played_music';
	genreId: number | null;
	label: string;
	enabled: boolean;
}

export const getHomeRows = () => api<HomeRowConfig[]>('/admin/home-rows');

export const putHomeRows = (rows: HomeRowConfig[]) =>
	api<HomeRowConfig[]>('/admin/home-rows', { method: 'PUT', body: rows });
