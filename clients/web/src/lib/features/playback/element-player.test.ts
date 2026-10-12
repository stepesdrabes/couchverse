import { afterEach, describe, expect, it, vi } from 'vitest';
import { PlayerSource, type PlayerLoad, type PlayerReport } from '$lib/generated/core';
import { ElementPlayer } from './element-player.svelte';

/** Just enough of a video element for a file source without subtitles. */
class FakeVideo extends EventTarget {
	currentTime = 0;
	duration = 60;
	paused = true;
	ended = false;
	autoplay = false;
	readyState = 0;
	error = null;
	textTracks = { length: 0 };
	#src: string | null = null;

	set src(url: string) {
		this.#src = url;
	}
	hasAttribute() {
		return this.#src !== null;
	}
	removeAttribute() {
		this.#src = null;
	}
	load() {}
	canPlayType() {
		return '';
	}
	fire(type: string) {
		this.dispatchEvent(new Event(type));
	}
}

const load: PlayerLoad = {
	url: '/api/v1/media/g/stream',
	source: PlayerSource.File,
	startSeconds: 20,
	autoplay: true,
	subtitles: [],
	linear: false,
	nowPlaying: { title: 'Glass Harbor', durationSeconds: 60 }
};

let player: ElementPlayer | null = null;
afterEach(() => player?.destroy());

function playing() {
	const video = new FakeVideo();
	const reports: PlayerReport[] = [];
	player = new ElementPlayer(video as unknown as HTMLVideoElement, (r) => reports.push(r));
	player.command({ type: 'load', content: load });
	video.fire('loadedmetadata');
	video.paused = false;
	video.fire('playing');
	video.currentTime = 21.5;
	return { video, reports };
}

describe('a media reset the browser makes on its own', () => {
	it('keeps a paused viewer paused, where they were', () => {
		const { video, reports } = playing();
		video.paused = true;
		video.fire('pause');

		video.currentTime = 0;
		video.fire('emptied');
		expect(video.autoplay).toBe(false);
		video.fire('waiting');
		expect(reports.at(-1)?.positionSeconds).toBe(21.5);

		// WebKit puts the position back itself
		video.currentTime = 21.5;
		video.fire('loadedmetadata');
		expect(reports.at(-1)).toMatchObject({ positionSeconds: 21.5, playing: false });
	});

	it('plays on from where it was when it was playing', () => {
		const { video, reports } = playing();
		video.fire('waiting');

		video.currentTime = 0;
		video.paused = true;
		video.fire('emptied');
		expect(video.autoplay).toBe(true);
		video.fire('loadedmetadata');
		expect(video.currentTime).toBe(21.5);
		expect(reports.at(-1)?.positionSeconds).toBe(21.5);
	});

	it('is not one of our own loads', () => {
		const video = new FakeVideo();
		player = new ElementPlayer(video as unknown as HTMLVideoElement, () => {});
		player.command({ type: 'load', content: load });
		video.fire('emptied');
		expect(video.autoplay).toBe(true);
		video.fire('loadedmetadata');
		expect(video.currentTime).toBe(20);
	});
});

/** A browser playing a live stream natively: the date its timeline starts at. */
class LiveVideo extends FakeVideo {
	duration = Infinity;
	playbackRate = 1;
	textTracks = Object.assign(new EventTarget(), { length: 0 });
	getStartDate() {
		return new Date(Date.UTC(2026, 9, 12, 18, 0, 0));
	}
}

describe('the live channel', () => {
	const live: PlayerLoad = {
		...load,
		url: '/api/v1/tv/g/live/master.m3u8?lang=cs',
		source: PlayerSource.Hls,
		startSeconds: 0,
		linear: true,
		live: { latencySeconds: 30, targetDurationSeconds: 10 },
		nowPlaying: { title: 'Glass Harbor', durationSeconds: 0 }
	};

	it('reports the program date on screen and plays at the rate the core holds it at', async () => {
		const video = new LiveVideo();
		const reports: PlayerReport[] = [];
		player = new ElementPlayer(video as unknown as HTMLVideoElement, (r) => reports.push(r));
		player.command({ type: 'rate', content: { rate: 0.95 } });
		player.command({ type: 'load', content: live });
		expect(video.playbackRate).toBe(1);
		// without MediaSource here, hls.js leaves the stream to the element
		await vi.waitFor(() => expect(video.hasAttribute()).toBe(true));
		video.fire('loadedmetadata');
		video.currentTime = 12.5;
		video.paused = false;
		video.fire('playing');
		expect(reports.at(-1)?.programDateMs).toBe(Date.UTC(2026, 9, 12, 18, 0, 12, 500));
		player.command({ type: 'rate', content: { rate: 1.1 } });
		expect(video.playbackRate).toBe(1.1);
	});

	it('reports no program date for a title', () => {
		const { reports } = playing();
		expect(reports.at(-1)?.programDateMs).toBeUndefined();
	});
});

/** Safari: HLS of its own, with the playlist's audio renditions as `audioTracks`. */
class SafariVideo extends FakeVideo {
	audioTracks = ['en', 'cs', 'en'].map((language, i) => ({ language, enabled: i === 0 }));
	canPlayType(type?: string) {
		return type === 'application/vnd.apple.mpegurl' ? 'maybe' : '';
	}
}

describe('a source loaded for an audio choice', () => {
	const hls = { ...load, source: PlayerSource.Hls, url: '/api/v1/media/g/hls/master.m3u8' };
	const enabled = (video: SafariVideo) => video.audioTracks.map((t) => t.enabled);

	it('starts on the rendition the core names, of two in one language', () => {
		const video = new SafariVideo();
		player = new ElementPlayer(video as unknown as HTMLVideoElement, () => {});
		player.command({ type: 'load', content: { ...hls, audioLang: 'en', audioIndex: 2 } });
		video.fire('loadedmetadata');
		expect(enabled(video)).toEqual([false, false, true]);
	});

	it('finds it by its language without a place', () => {
		const video = new SafariVideo();
		player = new ElementPlayer(video as unknown as HTMLVideoElement, () => {});
		player.command({ type: 'load', content: { ...hls, audioLang: 'cs' } });
		video.fire('loadedmetadata');
		expect(enabled(video)).toEqual([false, true, false]);
	});
});
