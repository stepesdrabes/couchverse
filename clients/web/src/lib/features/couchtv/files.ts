// The files a title airs on Couch TV, as the title tools pick among them.
import type { AdminTitle, MediaFile } from '$lib/generated/api';
import { episodeCode } from './labels';

/** A file the channel airs: a movie's, or an episode's. */
export interface AiredFile {
	fileId: string;
	label: string;
	/** what the playback payload is asked for */
	kind: 'movie' | 'episode';
	ownerId: string;
	durationMs: number;
}

// the file playback (and the channel) picks among a movie's or an episode's
function primary(files: MediaFile[]): MediaFile | undefined {
	return files
		.filter((f) => f.videoCodec !== '' && f.durationSeconds > 0)
		.sort(
			(a, b) =>
				Number(b.audioRole === 'primary') - Number(a.audioRole === 'primary') ||
				b.height - a.height ||
				a.id.localeCompare(b.id)
		)[0];
}

/** The files a title airs, in broadcast order (specials are not on the channel). */
export function airedFiles(t: AdminTitle): AiredFile[] {
	if (t.title.kind === 'movie') {
		const f = primary(t.mediaFiles.filter((m) => m.episodeId === null));
		return f
			? [
					{
						fileId: f.id,
						label: t.title.name,
						kind: 'movie',
						ownerId: t.title.id,
						durationMs: f.durationSeconds * 1000
					}
				]
			: [];
	}
	const out: AiredFile[] = [];
	for (const season of [...t.seasons].sort((a, b) => a.seasonNumber - b.seasonNumber)) {
		if (season.seasonNumber < 1) continue;
		for (const e of [...season.episodes].sort((a, b) => a.episodeNumber - b.episodeNumber)) {
			const f = primary(t.mediaFiles.filter((m) => m.episodeId === e.id));
			if (f) {
				out.push({
					fileId: f.id,
					label: `${episodeCode(season.seasonNumber, e.episodeNumber)} ${e.name}`.trim(),
					kind: 'episode',
					ownerId: e.id,
					durationMs: f.durationSeconds * 1000
				});
			}
		}
	}
	return out;
}

/** An excerpt of length ms from start, at least a second and at most max, kept within a file of fileMs. */
export function fitExcerpt(start: number, length: number, fileMs: number, max: number) {
	const durationMs = Math.round(Math.min(Math.max(length, 1000), max, Math.max(fileMs, 1000)));
	const startMs = Math.round(Math.min(Math.max(start, 0), Math.max(fileMs - durationMs, 0)));
	return { startMs, durationMs };
}
