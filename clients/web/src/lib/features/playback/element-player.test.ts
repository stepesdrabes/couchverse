import { afterEach, describe, expect, it } from 'vitest';
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
