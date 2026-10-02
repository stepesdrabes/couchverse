import { api } from '$lib/api/client';
import type { ContinueItem } from '$lib/features/catalog/types';

export type PlaybackKind = 'movie' | 'episode';

export interface EpisodeRef {
	episodeId: string;
	seasonNumber: number;
	episodeNumber: number;
	name: string;
	titleId: string;
	titleName: string;
}

export interface SubtitleTrack {
	id: string;
	lang: string;
	label: string;
	forced: boolean;
	url: string;
}

export interface AudioTrack {
	id: string;
	lang: string;
	label: string;
	default: boolean;
	source: 'file' | 'embedded';
	streamUrl?: string;
	hlsUrl?: string;
}

export interface PlaybackInfo {
	mode: 'direct' | 'hls' | 'jit' | 'preparing' | 'unsupported';
	mediaFileId: string;
	streamUrl?: string;
	durationSeconds: number;
	resumePosition: number;
	display: {
		title: string;
		subtitle: string;
		titleId: string;
		titleSlug: string;
		backdropId: string | null;
		backdropVer?: number;
		backdropAccent?: string;
	};
	nextEpisode: EpisodeRef | null;
	subtitles: SubtitleTrack[];
	audio?: AudioTrack[];
	episodes?: SeriesEpisode[];
	currentEpisodeId?: string;
	hlsUrl?: string;
	variants?: QualityVariant[];
	jobProgress?: number;
	allowRandomPlayback?: boolean;
}

export interface QualityVariant {
	name: string;
	height: number;
}

export interface SeriesEpisode {
	episodeId: string;
	seasonNumber: number;
	episodeNumber: number;
	name: string;
	thumbId?: string | null;
	thumbVer?: number;
}

export const getPlayback = (kind: PlaybackKind, id: string) =>
	api<PlaybackInfo>(`/playback/${kind}/${id}?caps=${clientCaps().join(',')}`);

/** seek-preview still from the source video at `seconds` */
export const frameUrl = (mediaFileId: string, seconds: number) =>
	`/api/v1/stream/${mediaFileId}/frame?t=${Math.max(0, Math.floor(seconds))}`;

/** codecs this browser can direct-play beyond the h264 baseline */
export function clientCaps(): string[] {
	const caps: string[] = [];
	if (typeof MediaSource !== 'undefined') {
		if (MediaSource.isTypeSupported('video/mp4; codecs="hvc1.1.6.L93.B0"')) caps.push('hevc');
		if (MediaSource.isTypeSupported('video/mp4; codecs="av01.0.04M.08"')) caps.push('av1');
	}
	return caps;
}

export interface ProgressReport {
	titleId?: string;
	episodeId?: string;
	positionSeconds: number;
	durationSeconds: number;
	/** actually-played seconds since the last report (feeds analytics) */
	watchedSeconds?: number;
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

// JIT ("instant play") sessions
export const createJitSession = (mediaFileId: string, startAt: number) =>
	api<{ sessionId: string; playlistUrl: string }>(`/stream/${mediaFileId}/sessions`, {
		method: 'POST',
		body: { startAt }
	});

export const jitKeepalive = (sessionId: string) =>
	api<void>(`/stream/sessions/${sessionId}/keepalive`, { method: 'POST' });
