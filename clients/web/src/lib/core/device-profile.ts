import {
	AudioCodec,
	Container,
	HdrFormat,
	HlsFormat,
	SubtitleFormat,
	VideoCodec,
	VideoProfile,
	type AudioSupport,
	type DeviceProfile,
	type VideoSupport
} from '$lib/generated/core';

/** What the browser answers about media, so the measurement can be tested with set answers. */
export interface MediaProbe {
	/** The media element plays it (`canPlayType`). */
	native(type: string): boolean;
	/** Media Source Extensions decode it for hls.js; false without MSE. */
	mse(type: string): boolean;
	/** MSE exists at all. */
	hasMse: boolean;
	/** The screen shows HDR. */
	hdrScreen: boolean;
}

/**
 * What this browser plays: the containers the media element loads, the codecs (profiles, bit
 * depth) MSE decodes for hls.js, the audio it outputs and whether the screen shows HDR. The
 * server treats it as authoritative, so it never offers a stream the browser cannot play.
 */
export function measureProfile(probe: MediaProbe): DeviceProfile {
	const plays = (type: string) => probe.mse(type) || probe.native(type);
	const mp4 = (codecs: string) => plays(`video/mp4; codecs="${codecs}"`);

	const containers: Container[] = [];
	if (probe.native('video/mp4')) containers.push(Container.Mp4);
	if (probe.native('video/webm')) containers.push(Container.Webm);
	if (probe.native('video/quicktime')) containers.push(Container.Mov);

	const video: VideoSupport[] = [];
	if (mp4('avc1.640028')) {
		video.push({
			codec: VideoCodec.H264,
			profiles: [VideoProfile.Baseline, VideoProfile.Main, VideoProfile.High],
			maxLevel: mp4('avc1.640034') ? 5.2 : mp4('avc1.640033') ? 5.1 : 4.2
		});
	}
	const hevcMain10 = mp4('hvc1.2.4.L153.B0');
	if (hevcMain10 || mp4('hvc1.1.6.L120.90')) {
		video.push({
			codec: VideoCodec.Hevc,
			profiles: hevcMain10 ? [VideoProfile.Main, VideoProfile.Main10] : [VideoProfile.Main],
			maxLevel: hevcMain10 || mp4('hvc1.1.6.L153.90') ? 5.1 : 4,
			maxBitDepth: hevcMain10 ? 10 : 8
		});
	}
	const vp9Ten = plays('video/webm; codecs="vp09.02.10.10"');
	if (vp9Ten || plays('video/webm; codecs="vp09.00.10.08"')) {
		video.push({
			codec: VideoCodec.Vp9,
			profiles: vp9Ten ? [VideoProfile.Profile0, VideoProfile.Profile2] : [VideoProfile.Profile0],
			maxBitDepth: vp9Ten ? 10 : 8
		});
	}
	const av1Ten = mp4('av01.0.08M.10');
	if (av1Ten || mp4('av01.0.08M.08')) {
		video.push({
			codec: VideoCodec.Av1,
			profiles: [VideoProfile.Main],
			maxBitDepth: av1Ten ? 10 : 8
		});
	}

	const audio: AudioSupport[] = [];
	const sound = (codec: AudioCodec, type: string, maxChannels: number) => {
		if (plays(type)) audio.push({ codec, maxChannels });
	};
	// browsers decode multichannel AAC and downmix it themselves
	sound(AudioCodec.Aac, 'audio/mp4; codecs="mp4a.40.2"', 6);
	sound(AudioCodec.Mp3, 'audio/mpeg', 2);
	sound(AudioCodec.Ac3, 'audio/mp4; codecs="ac-3"', 6);
	sound(AudioCodec.Eac3, 'audio/mp4; codecs="ec-3"', 6);
	sound(AudioCodec.Opus, 'audio/webm; codecs="opus"', 6);
	sound(AudioCodec.Vorbis, 'audio/webm; codecs="vorbis"', 2);
	sound(AudioCodec.Flac, 'audio/mp4; codecs="flac"', 6);

	// HDR needs a 10-bit decoder and a screen that shows it; otherwise the server tone-maps to
	// SDR rather than handing over washed-out PQ
	const hdr: HdrFormat[] = [];
	if (video.some((v) => (v.maxBitDepth ?? 8) >= 10) && probe.hdrScreen) {
		hdr.push(HdrFormat.Hdr10, HdrFormat.Hlg);
		if (mp4('dvh1.05.06')) hdr.push(HdrFormat.DolbyVision5);
		if (mp4('dvh1.08.06')) hdr.push(HdrFormat.DolbyVision8);
	}

	return {
		containers,
		video,
		audio,
		hdr,
		// hls.js plays both segment formats through MSE; Safari plays them natively
		hls:
			probe.hasMse || probe.native('application/vnd.apple.mpegurl')
				? [HlsFormat.Fmp4, HlsFormat.Ts]
				: [],
		// the player renders WebVTT cues itself beside a direct-played file
		sidecarSubtitles: [SubtitleFormat.Webvtt],
		// a progressive file's audio tracks only switch in Safari; HLS covers the rest
		audioTrackSwitching: false
	};
}

/** The running browser's answers. */
export function browserProbe(): MediaProbe {
	const element = document.createElement('video');
	const hasMse = typeof MediaSource !== 'undefined';
	return {
		native: (type) => element.canPlayType(type) !== '',
		mse: (type) => hasMse && MediaSource.isTypeSupported(type),
		hasMse,
		hdrScreen: matchMedia('(dynamic-range: high)').matches
	};
}
