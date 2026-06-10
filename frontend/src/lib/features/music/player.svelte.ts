// Spotify-like music player: a module-scope singleton wrapping a DOM-detached
// Audio element, so SvelteKit navigation can never interrupt playback.
import { artworkUrl } from '$lib/features/catalog/api';
import { scrobble } from './api';
import type { TrackItem } from './api';

export type RepeatMode = 'off' | 'all' | 'one';

class MusicPlayer {
	queue = $state<TrackItem[]>([]);
	index = $state(0);
	playing = $state(false);
	currentTime = $state(0);
	duration = $state(0);
	volume = $state(1);
	muted = $state(false);
	shuffle = $state(false);
	repeat = $state<RepeatMode>('off');
	queueOpen = $state(false);

	current = $derived(this.queue[this.index] ?? null);

	private audio: HTMLAudioElement | null = null;
	private unshuffled: TrackItem[] | null = null;

	private ensureAudio(): HTMLAudioElement {
		if (this.audio) return this.audio;
		const audio = new Audio();
		audio.preload = 'auto';
		this.volume = Number(localStorage.getItem('cv.musicVolume') ?? 1);
		audio.volume = this.volume;

		audio.addEventListener('timeupdate', () => {
			this.currentTime = audio.currentTime;
			this.updatePositionState();
		});
		audio.addEventListener('durationchange', () => (this.duration = audio.duration || 0));
		audio.addEventListener('play', () => {
			this.playing = true;
			if ('mediaSession' in navigator) navigator.mediaSession.playbackState = 'playing';
		});
		audio.addEventListener('pause', () => {
			this.playing = false;
			if ('mediaSession' in navigator) navigator.mediaSession.playbackState = 'paused';
		});
		audio.addEventListener('ended', () => this.onEnded());

		this.setupMediaSession();
		this.audio = audio;
		return audio;
	}

	/** play a list of tracks starting at index (skips tracks without files) */
	playQueue(tracks: TrackItem[], startIndex = 0) {
		const playable = tracks.filter((t) => t.mediaFileId !== null);
		if (playable.length === 0) return;
		const startTrack = tracks[startIndex];
		this.queue = playable;
		this.index = Math.max(
			0,
			playable.findIndex((t) => t.id === startTrack?.id)
		);
		this.unshuffled = null;
		this.shuffle = false;
		this.load(true);
	}

	playNow(track: TrackItem) {
		this.playQueue([track]);
	}

	addToQueue(track: TrackItem) {
		if (track.mediaFileId === null) return;
		this.queue = [...this.queue, track];
		if (this.queue.length === 1) this.load(true);
	}

	jumpTo(index: number) {
		if (index < 0 || index >= this.queue.length) return;
		this.index = index;
		this.load(true);
	}

	removeFromQueue(index: number) {
		if (index === this.index) return; // keep the playing track
		this.queue = this.queue.filter((_, i) => i !== index);
		if (index < this.index) this.index -= 1;
	}

	private load(autoplay: boolean) {
		const track = this.current;
		if (!track?.mediaFileId) return;
		const audio = this.ensureAudio();
		audio.src = `/api/v1/stream/${track.mediaFileId}`;
		this.currentTime = 0;
		this.duration = track.durationSeconds;
		if (autoplay) audio.play().catch(() => {});
		scrobble(track.id).catch(() => {});
		this.updateMediaSessionMetadata();
	}

	toggle() {
		const audio = this.ensureAudio();
		if (audio.paused) audio.play().catch(() => {});
		else audio.pause();
	}

	pause() {
		this.audio?.pause();
	}

	next() {
		if (this.queue.length === 0) return;
		if (this.index + 1 < this.queue.length) {
			this.index += 1;
			this.load(true);
		} else if (this.repeat === 'all') {
			this.index = 0;
			this.load(true);
		} else {
			this.audio?.pause();
		}
	}

	prev() {
		const audio = this.ensureAudio();
		if (audio.currentTime > 3 || this.index === 0) {
			audio.currentTime = 0;
			return;
		}
		this.index -= 1;
		this.load(true);
	}

	private onEnded() {
		if (this.repeat === 'one') {
			const audio = this.ensureAudio();
			audio.currentTime = 0;
			audio.play().catch(() => {});
			return;
		}
		this.next();
	}

	seek(seconds: number) {
		const audio = this.ensureAudio();
		audio.currentTime = Math.min(Math.max(0, seconds), this.duration || 0);
	}

	setVolume(v: number) {
		this.volume = Math.min(1, Math.max(0, v));
		this.muted = this.volume === 0;
		this.ensureAudio().volume = this.volume;
		localStorage.setItem('cv.musicVolume', String(this.volume));
	}

	toggleShuffle() {
		if (this.queue.length < 2) return;
		const playingTrack = this.current;
		if (!this.shuffle) {
			this.unshuffled = this.queue;
			const rest = this.queue.filter((_, i) => i !== this.index);
			for (let i = rest.length - 1; i > 0; i--) {
				const j = Math.floor(Math.random() * (i + 1));
				[rest[i], rest[j]] = [rest[j], rest[i]];
			}
			this.queue = playingTrack ? [playingTrack, ...rest] : rest;
			this.index = 0;
			this.shuffle = true;
		} else {
			const restored = this.unshuffled ?? this.queue;
			this.index = Math.max(
				0,
				restored.findIndex((t) => t.id === playingTrack?.id)
			);
			this.queue = restored;
			this.unshuffled = null;
			this.shuffle = false;
		}
	}

	cycleRepeat() {
		this.repeat = this.repeat === 'off' ? 'all' : this.repeat === 'all' ? 'one' : 'off';
	}

	/** stop and clear everything (logout) */
	stop() {
		this.audio?.pause();
		if (this.audio) this.audio.src = '';
		this.queue = [];
		this.index = 0;
		this.playing = false;
	}

	private setupMediaSession() {
		if (!('mediaSession' in navigator)) return;
		navigator.mediaSession.setActionHandler('play', () => this.toggle());
		navigator.mediaSession.setActionHandler('pause', () => this.toggle());
		navigator.mediaSession.setActionHandler('previoustrack', () => this.prev());
		navigator.mediaSession.setActionHandler('nexttrack', () => this.next());
		navigator.mediaSession.setActionHandler('seekto', (details) => {
			if (details.seekTime !== undefined) this.seek(details.seekTime);
		});
	}

	private updateMediaSessionMetadata() {
		if (!('mediaSession' in navigator) || !this.current) return;
		navigator.mediaSession.metadata = new MediaMetadata({
			title: this.current.name,
			artist: this.current.trackArtist ?? this.current.artistName,
			album: this.current.albumName,
			artwork: this.current.coverId
				? [{ src: artworkUrl(this.current.coverId), sizes: '512x512' }]
				: []
		});
	}

	private updatePositionState() {
		if (!('mediaSession' in navigator) || !this.duration) return;
		try {
			navigator.mediaSession.setPositionState({
				duration: this.duration,
				position: Math.min(this.currentTime, this.duration)
			});
		} catch {
			// invalid transient state - ignore
		}
	}
}

export const musicPlayer = new MusicPlayer();
