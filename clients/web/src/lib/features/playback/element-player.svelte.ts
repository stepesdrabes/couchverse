import type Hls from 'hls.js';
import type { PlayerHost } from '$lib/core/runtime.svelte';
import type { PlayerCommand, PlayerLoad, PlayerReport, PlayerSubtitle } from '$lib/generated/core';
import { isTV } from '$lib/tv/tv';

// the core accounts watched time and keeps couch followers in step from these
const REPORT_EVERY_MS = 1000;

type NativeAudioTracks = { length: number; [i: number]: { language: string; enabled: boolean } };

interface Subtitle {
	track: PlayerSubtitle;
	/** The sidecar file's element; absent for a track inside the media. */
	element?: HTMLTrackElement;
}

/**
 * The core's player on the web: a video element, with hls.js where the browser has no HLS of
 * its own (or no audio menu for it). It does what the core's commands say and reports what the
 * element does; the page around it draws the controls.
 */
export class ElementPlayer implements PlayerHost {
	/** The cues on screen, as the browser's VTT parser renders them. */
	cueHtml = $state('');
	/** The title's length as the payload has it, before the element knows its own. */
	durationSeconds = $state(0);

	#video: HTMLVideoElement;
	#report: (report: PlayerReport) => void;
	#hls: Hls | null = null;
	#load: PlayerLoad | null = null;
	#subtitles: Subtitle[] = [];
	#selected: string | null = null;
	#cueTrack: TextTrack | null = null;
	#pendingSeek: number | null = null;
	#buffering = false;
	#failed = false;
	// A browser may start the media over by itself, back at zero (WebKit does when its GPU
	// process restarts). This load's metadata being in tells that from a load of ours; the last
	// position and play state are what it comes back to.
	#ready = false;
	#position = 0;
	#playing = false;
	#restore: number | null = null;
	// bumped by every load, so an hls.js import that lands late is for an old one
	#generation = 0;
	#timer: ReturnType<typeof setInterval>;
	#listeners: [string, () => void][];

	constructor(video: HTMLVideoElement, report: (report: PlayerReport) => void) {
		this.#video = video;
		this.#report = report;
		this.#listeners = [
			['play', () => this.#emit()],
			['pause', () => this.#settle()],
			['playing', () => this.#settle()],
			['canplay', () => this.#settle()],
			['seeked', () => this.#settle()],
			['waiting', () => this.#stall()],
			['stalled', () => this.#stall()],
			['ended', () => this.#settle()],
			['loadedmetadata', () => this.#metadata()],
			['emptied', () => this.#emptied()],
			['error', () => this.#fail(video.error?.message || `media error ${video.error?.code}`)],
			['enterpictureinpicture', () => this.#showSubtitle()],
			['leavepictureinpicture', () => this.#showSubtitle()]
		];
		for (const [type, listener] of this.#listeners) video.addEventListener(type, listener);
		this.#timer = setInterval(() => {
			if (!video.paused) this.#emit();
		}, REPORT_EVERY_MS);
	}

	command(command: PlayerCommand) {
		const video = this.#video;
		switch (command.type) {
			case 'load':
				this.#start(command.content);
				break;
			case 'play':
				// a browser that wants a gesture first stays paused, and says so
				video.play().catch(() => this.#emit());
				break;
			case 'pause':
				video.pause();
				break;
			case 'seek':
				if (video.readyState >= HTMLMediaElement.HAVE_METADATA) {
					video.currentTime = command.content.seconds;
				} else {
					this.#pendingSeek = command.content.seconds;
				}
				break;
			case 'selectAudio':
				this.#selectAudio(command.content.lang, command.content.index);
				break;
			case 'selectSubtitles':
				this.#selected = command.content.id ?? null;
				this.#showSubtitle();
				break;
			case 'stop':
				this.#stop();
				break;
		}
	}

	destroy() {
		clearInterval(this.#timer);
		for (const [type, listener] of this.#listeners) {
			this.#video.removeEventListener(type, listener);
		}
		this.#stop();
	}

	#start(load: PlayerLoad) {
		this.#stop();
		const video = this.#video;
		const generation = ++this.#generation;
		this.#load = load;
		this.durationSeconds = load.nowPlaying.durationSeconds;
		this.#failed = false;
		this.#ready = false;
		this.#restore = null;
		this.#selected = load.subtitle ?? null;
		this.#addSubtitles(load.subtitles);
		video.autoplay = load.autoplay;
		const start = load.startSeconds > 0 ? load.startSeconds : null;
		if (load.source !== 'hls') {
			this.#pendingSeek = start;
			video.src = load.url;
			return;
		}
		// Safari plays HLS natively (adaptive only) and switches its audio renditions through
		// video.audioTracks. Chrome and TV browsers claim native HLS too but have no audioTracks,
		// so there hls.js keeps the quality and audio menus.
		const nativeHls = video.canPlayType('application/vnd.apple.mpegurl') !== '';
		if (nativeHls && !isTV && 'audioTracks' in video) {
			this.#pendingSeek = start;
			video.src = load.url;
			return;
		}
		void import('hls.js').then(({ default: HlsPlayer }) => {
			if (generation !== this.#generation) return;
			if (!HlsPlayer.isSupported()) {
				this.#pendingSeek = start;
				video.src = load.url;
				return;
			}
			// the player draws the sidecar WebVTT tracks itself (with the account's subtitle
			// style), so hls.js adds no text tracks for the playlist's renditions
			const hls = new HlsPlayer({ renderTextTracksNatively: false, startPosition: start ?? -1 });
			this.#hls = hls;
			hls.on(HlsPlayer.Events.MANIFEST_PARSED, () => {
				if (load.maxHeight) {
					// a pinned quality: the highest rendition that fits
					const fits = hls.levels
						.map((level, i) => ({ height: level.height, i }))
						.filter((l) => l.height <= (load.maxHeight ?? 0))
						.sort((a, b) => b.height - a.height);
					if (fits.length) hls.currentLevel = fits[0].i;
				}
			});
			hls.on(HlsPlayer.Events.AUDIO_TRACKS_UPDATED, () => {
				if (load.audioLang) this.#selectAudio(load.audioLang, load.audioIndex);
			});
			hls.on(HlsPlayer.Events.ERROR, (_, data) => {
				if (data.fatal) this.#fail(`hls ${data.details}`);
			});
			hls.loadSource(load.url);
			hls.attachMedia(video);
		});
	}

	#stop() {
		this.#generation++;
		this.#hls?.destroy();
		this.#hls = null;
		this.#load = null;
		this.#pendingSeek = null;
		this.#detachCues();
		for (const { element } of this.#subtitles) element?.remove();
		this.#subtitles = [];
		const video = this.#video;
		if (video.hasAttribute('src')) {
			video.removeAttribute('src');
			video.load();
		}
	}

	#metadata() {
		const video = this.#video;
		if (this.#pendingSeek !== null) {
			video.currentTime = this.#pendingSeek;
			this.#pendingSeek = null;
		} else if (this.#restore !== null && Math.abs(video.currentTime - this.#restore) > 1) {
			video.currentTime = this.#restore;
		}
		this.#restore = null;
		this.#ready = true;
		const load = this.#load;
		if (load?.audioLang && !this.#hls) this.#selectAudio(load.audioLang, load.audioIndex);
		this.#showSubtitle();
		this.#emit();
	}

	/** The element was reset after this load's metadata, so not by a load of ours: it goes back
	 * to where it was (WebKit restores that itself), and plays again only if it was playing (its
	 * autoplay would resume a viewer's pause). */
	#emptied() {
		if (!this.#ready) return;
		this.#ready = false;
		this.#restore = this.#position;
		this.#video.autoplay = this.#playing;
	}

	#addSubtitles(tracks: PlayerSubtitle[]) {
		this.#subtitles = tracks.map((track) => {
			if (!track.url) return { track };
			const element = document.createElement('track');
			element.kind = 'subtitles';
			element.src = track.url;
			element.srclang = track.lang;
			element.label = track.label;
			this.#video.append(element);
			return { track, element };
		});
	}

	/** Shows the selected subtitles: in the page's own overlay, or natively in PiP, where the
	 * browser paints only the video element. */
	#showSubtitle() {
		this.#detachCues();
		const video = this.#video;
		for (let i = 0; i < video.textTracks.length; i++) video.textTracks[i].mode = 'disabled';
		const chosen = this.#subtitles.find((s) => s.track.id === this.#selected);
		if (!chosen) return;
		const track = chosen.element?.track ?? this.#embedded(chosen.track.lang);
		if (!track) return;
		track.mode = document.pictureInPictureElement === video ? 'showing' : 'hidden';
		this.#cueTrack = track;
		track.addEventListener('cuechange', this.#cues);
		this.#cues();
	}

	/** A subtitle track inside the media, by language. */
	#embedded(lang: string): TextTrack | undefined {
		const sidecar = this.#subtitles.map((s) => s.element?.track);
		const tracks = this.#video.textTracks;
		for (let i = 0; i < tracks.length; i++) {
			if (!sidecar.includes(tracks[i]) && tracks[i].language === lang) return tracks[i];
		}
	}

	#cues = () => {
		const cues = this.#cueTrack?.activeCues;
		if (!cues?.length) {
			this.cueHtml = '';
			return;
		}
		const holder = document.createElement('div');
		for (let i = 0; i < cues.length; i++) holder.append((cues[i] as VTTCue).getCueAsHTML());
		this.cueHtml = holder.innerHTML;
	};

	#detachCues() {
		this.#cueTrack?.removeEventListener('cuechange', this.#cues);
		this.#cueTrack = null;
		this.cueHtml = '';
	}

	/** The playlist lists the renditions in the payload's order, so `index` (the core's)
	 * tells two in one language apart; the language is the fallback. */
	#selectAudio(lang: string, index?: number) {
		const pick = (count: number, language: (i: number) => string | undefined) => {
			if (index !== undefined && index < count) return index;
			for (let i = 0; i < count; i++) if (language(i) === lang) return i;
			return -1;
		};
		const hls = this.#hls;
		if (hls) {
			const at = pick(hls.audioTracks.length, (i) => hls.audioTracks[i].lang);
			if (at >= 0 && hls.audioTrack !== at) hls.audioTrack = at;
			return;
		}
		// Safari's native HLS lists the playlist's audio group here
		const tracks = (this.#video as HTMLVideoElement & { audioTracks?: NativeAudioTracks })
			.audioTracks;
		if (!tracks) return;
		const at = pick(tracks.length, (i) => tracks[i].language);
		if (at < 0) return;
		for (let i = 0; i < tracks.length; i++) tracks[i].enabled = i === at;
	}

	#stall() {
		this.#buffering = true;
		this.#emit();
	}

	#settle() {
		this.#buffering = false;
		this.#emit();
	}

	#fail(reason: string) {
		if (this.#failed || !this.#load) return;
		this.#failed = true;
		this.#emit(reason);
	}

	#emit(failed?: string) {
		const load = this.#load;
		if (!load) return;
		const video = this.#video;
		const known = Number.isFinite(video.duration) && video.duration > 0;
		// before the metadata the element is at zero, not yet where it is going
		const position = this.#pendingSeek ?? this.#restore ?? video.currentTime;
		const playing = !video.paused && !video.ended;
		if (this.#ready) [this.#position, this.#playing] = [position, playing];
		this.#report({
			positionSeconds: position,
			durationSeconds: known ? video.duration : load.nowPlaying.durationSeconds,
			playing,
			buffering: this.#buffering,
			ended: video.ended,
			failed
		});
	}
}
