import { api } from '$lib/api/client';
import type { ContinueItem } from '$lib/features/catalog/types';
import {
	createStreamSession,
	keepStreamSessionAlive,
	resolvePlayback,
	type AudioSupport,
	type DeviceProfile,
	type DeviceProfileContainersItem,
	type DeviceProfileHdrItem,
	type PlaybackAudioTrack,
	type PlaybackInfo,
	type PlaybackSubtitleTrack,
	type VideoSupport
} from '$lib/generated/api';

export type { EpisodeRef, PlaybackInfo, QualityVariant, SeriesEpisode } from '$lib/generated/api';
export type AudioTrack = PlaybackAudioTrack;
export type SubtitleTrack = PlaybackSubtitleTrack;

export type PlaybackKind = 'movie' | 'episode';

/** The server decides direct play, remux or transcode from what this browser reports it plays. */
export const getPlayback = (kind: PlaybackKind, id: string) =>
	resolvePlayback(kind, id, deviceProfile());

/** seek-preview still from the source video at `seconds` */
export const frameAt = (info: PlaybackInfo, seconds: number) =>
	`${info.frameUrl}?t=${Math.max(0, Math.floor(seconds))}`;

let measured: DeviceProfile | null = null;

/**
 * What this browser plays, measured once per page load: the containers the media
 * element loads, the codecs (profiles, bit depth) Media Source Extensions decode for
 * hls.js, the audio it outputs and whether the screen shows HDR. The server treats
 * it as authoritative, so it never offers a stream the browser cannot play.
 */
export function deviceProfile(): DeviceProfile {
	if (measured) return measured;
	const element = document.createElement('video');
	const native = (type: string) => element.canPlayType(type) !== '';
	const mse = (type: string) =>
		typeof MediaSource !== 'undefined' && MediaSource.isTypeSupported(type);
	const plays = (type: string) => mse(type) || native(type);
	const mp4 = (codecs: string) => plays(`video/mp4; codecs="${codecs}"`);

	const containers: DeviceProfileContainersItem[] = [];
	if (native('video/mp4')) containers.push('mp4');
	if (native('video/webm')) containers.push('webm');
	if (native('video/quicktime')) containers.push('mov');

	const video: VideoSupport[] = [];
	if (mp4('avc1.640028')) {
		video.push({
			codec: 'h264',
			profiles: ['baseline', 'main', 'high'],
			maxLevel: mp4('avc1.640034') ? 5.2 : mp4('avc1.640033') ? 5.1 : 4.2
		});
	}
	const hevcMain10 = mp4('hvc1.2.4.L153.B0');
	if (hevcMain10 || mp4('hvc1.1.6.L120.90')) {
		video.push({
			codec: 'hevc',
			profiles: hevcMain10 ? ['main', 'main10'] : ['main'],
			maxLevel: hevcMain10 || mp4('hvc1.1.6.L153.90') ? 5.1 : 4,
			maxBitDepth: hevcMain10 ? 10 : 8
		});
	}
	const av1Ten = mp4('av01.0.08M.10');
	if (av1Ten || mp4('av01.0.08M.08')) {
		video.push({ codec: 'av1', profiles: ['main'], maxBitDepth: av1Ten ? 10 : 8 });
	}
	const vp9Ten = plays('video/webm; codecs="vp09.02.10.10"');
	if (vp9Ten || plays('video/webm; codecs="vp09.00.10.08"')) {
		video.push({
			codec: 'vp9',
			profiles: vp9Ten ? ['profile0', 'profile2'] : ['profile0'],
			maxBitDepth: vp9Ten ? 10 : 8
		});
	}

	const audio: AudioSupport[] = [];
	const sound = (codec: AudioSupport['codec'], type: string, maxChannels: number) => {
		if (plays(type)) audio.push({ codec, maxChannels });
	};
	// browsers decode multichannel AAC and downmix it themselves
	sound('aac', 'audio/mp4; codecs="mp4a.40.2"', 6);
	sound('mp3', 'audio/mpeg', 2);
	sound('ac3', 'audio/mp4; codecs="ac-3"', 6);
	sound('eac3', 'audio/mp4; codecs="ec-3"', 6);
	sound('opus', 'audio/webm; codecs="opus"', 6);
	sound('vorbis', 'audio/webm; codecs="vorbis"', 2);
	sound('flac', 'audio/mp4; codecs="flac"', 6);

	// HDR needs a 10-bit decoder and a screen that shows it; otherwise the server
	// tone-maps to SDR rather than handing over washed-out PQ
	const hdr: DeviceProfileHdrItem[] = [];
	const tenBit = video.some((v) => (v.maxBitDepth ?? 8) >= 10);
	if (tenBit && matchMedia('(dynamic-range: high)').matches) {
		hdr.push('hdr10', 'hlg');
		if (mp4('dvh1.05.06')) hdr.push('dolbyVision5');
		if (mp4('dvh1.08.06')) hdr.push('dolbyVision8');
	}

	measured = {
		containers,
		video,
		audio,
		hdr,
		// hls.js plays both segment formats through MSE; Safari plays them natively
		hls:
			typeof MediaSource !== 'undefined' || native('application/vnd.apple.mpegurl')
				? ['fmp4', 'ts']
				: [],
		// the player renders WebVTT cues itself beside a direct-played file
		sidecarSubtitles: ['webvtt'],
		// a progressive file's audio tracks only switch in Safari; HLS covers the rest
		audioTrackSwitching: false
	};
	return measured;
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

/** Open the instant-play session the payload planned (mode jit), authorized by its media grant. */
export const createJitSession = (info: PlaybackInfo, startAt: number) =>
	createStreamSession(info.grant, { startAt, plan: info.jit }, { skipAuthRedirect: true });

export const jitKeepalive = (grant: string, sessionId: string) =>
	keepStreamSessionAlive(grant, sessionId, { skipAuthRedirect: true });

/** end the transcode as the player goes away; keepalive lets it outlive the page */
export function stopJitSession(grant: string, sessionId: string) {
	void fetch(`/api/v1/media/${grant}/jit/${sessionId}`, {
		method: 'DELETE',
		keepalive: true
	}).catch(() => {});
}
