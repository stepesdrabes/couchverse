// The media grants the title tools play a title's files through: the playback payload's
// grant (stills via the frame endpoint, the promo audio through its stream), never a URL
// of our own.
import type Hls from 'hls.js';
import { browserProbe, measureProfile } from '$lib/core/device-profile';
import type { PlaybackInfo } from '$lib/generated/api';
import { resolvePlayback } from './api';
import type { AiredFile } from './files';

/** Playback payloads per movie or episode, asked for once and shared by the tools. */
export class Grants {
	#payloads = new Map<string, Promise<PlaybackInfo>>();
	#profile: ReturnType<typeof measureProfile> | null = null;

	get(file: AiredFile): Promise<PlaybackInfo> {
		const key = `${file.kind}:${file.ownerId}`;
		let p = this.#payloads.get(key);
		if (!p) {
			this.#profile ??= measureProfile(browserProbe());
			p = resolvePlayback(file.kind, file.ownerId, this.#profile);
			// a failure is asked again next time
			p.catch(() => this.#payloads.delete(key));
			this.#payloads.set(key, p);
		}
		return p;
	}
}

/**
 * Plays a payload's stream in a media element: the file itself when it direct-plays, its
 * HLS through hls.js where the browser has none. Resolves with a detach; rejects when the
 * file cannot play in the browser until it is prepared.
 */
export async function attachStream(el: HTMLMediaElement, info: PlaybackInfo): Promise<() => void> {
	const url = info.streamUrl;
	if (!url || (info.mode !== 'direct' && info.mode !== 'hls')) throw new Error('not playable');
	if (info.mode === 'hls' && !el.canPlayType('application/vnd.apple.mpegurl')) {
		const { default: HlsJs } = await import('hls.js');
		const hls: Hls = new HlsJs();
		hls.loadSource(url);
		hls.attachMedia(el);
		return () => hls.destroy();
	}
	el.src = url;
	return () => {
		el.removeAttribute('src');
		el.load();
	};
}
