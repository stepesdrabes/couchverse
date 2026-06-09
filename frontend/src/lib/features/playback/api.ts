import { api } from '$lib/api/client';
import type { ContinueItem } from '$lib/features/catalog/types';

export type PlaybackKind = 'movie' | 'episode';

export interface EpisodeRef {
	episodeId: number;
	seasonNumber: number;
	episodeNumber: number;
	name: string;
	titleId: number;
	titleName: string;
}

export interface PlaybackInfo {
	mode: 'direct' | 'hls' | 'jit' | 'preparing' | 'unsupported';
	mediaFileId: number;
	streamUrl?: string;
	durationSeconds: number;
	resumePosition: number;
	display: { title: string; subtitle: string; titleId: number };
	nextEpisode: EpisodeRef | null;
}

export const getPlayback = (kind: PlaybackKind, id: number) =>
	api<PlaybackInfo>(`/playback/${kind}/${id}`);

export interface ProgressReport {
	titleId?: number;
	episodeId?: number;
	positionSeconds: number;
	durationSeconds: number;
}

export const reportProgress = (report: ProgressReport) =>
	api<void>('/progress', { method: 'PUT', body: report });

/** fire-and-forget progress on page unload (sendBeacon can only POST) */
export function beaconProgress(report: ProgressReport) {
	navigator.sendBeacon(
		'/api/v1/progress',
		new Blob([JSON.stringify(report)], { type: 'application/json' })
	);
}

export const continueWatching = () => api<ContinueItem[]>('/me/continue-watching');
