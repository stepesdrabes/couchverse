import { describe, expect, it } from 'vitest';
import type { AdminTitle, MediaFile } from '$lib/generated/api';
import { airedFiles, fitExcerpt } from './files';

function file(id: string, extra: Partial<MediaFile>): MediaFile {
	return {
		id,
		audioCodec: 'aac',
		audioLang: '',
		audioRole: 'primary',
		durationSeconds: 1200,
		episodeId: null,
		height: 720,
		videoCodec: 'h264',
		...extra
	} as MediaFile;
}

describe('the files a title airs', () => {
	it('takes the file playback picks for each episode, seasons in order, no specials', () => {
		const episode = (id: string, n: number) => ({ id, episodeNumber: n, name: `E${n}` });
		const title = {
			title: { id: 't', kind: 'series', name: 'Show' },
			seasons: [
				{ seasonNumber: 2, episodes: [episode('s2e1', 1)] },
				{ seasonNumber: 0, episodes: [episode('special', 1)] },
				{ seasonNumber: 1, episodes: [episode('s1e2', 2), episode('s1e1', 1)] }
			],
			mediaFiles: [
				file('small', { episodeId: 's1e1', height: 480 }),
				file('big', { episodeId: 's1e1', height: 1080 }),
				file('dub', { episodeId: 's1e1', audioRole: 'audio_alt', videoCodec: '' }),
				file('two', { episodeId: 's1e2' }),
				file('sp', { episodeId: 'special' }),
				file('next', { episodeId: 's2e1' })
			]
		} as unknown as AdminTitle;
		expect(airedFiles(title).map((f) => [f.fileId, f.label, f.ownerId])).toEqual([
			['big', 'S01E01 E1', 's1e1'],
			['two', 'S01E02 E2', 's1e2'],
			['next', 'S02E01 E1', 's2e1']
		]);
	});

	it('takes the file of a movie, not its other-language audio', () => {
		const title = {
			title: { id: 'm', kind: 'movie', name: 'Film' },
			seasons: [],
			mediaFiles: [
				file('cs', { audioRole: 'audio_alt', videoCodec: '' }),
				file('main', { durationSeconds: 90 })
			]
		} as unknown as AdminTitle;
		expect(airedFiles(title)).toEqual([
			{ fileId: 'main', label: 'Film', kind: 'movie', ownerId: 'm', durationMs: 90_000 }
		]);
	});
});

describe('the promo excerpt', () => {
	it('stays within the file and 1 to 40 seconds', () => {
		expect(fitExcerpt(60_000, 30_000, 25_000, 40_000)).toEqual({ startMs: 0, durationMs: 25_000 });
		expect(fitExcerpt(10_000, 90_000, 600_000, 40_000)).toEqual({
			startMs: 10_000,
			durationMs: 40_000
		});
		expect(fitExcerpt(590_000, 30_000, 600_000, 40_000)).toEqual({
			startMs: 570_000,
			durationMs: 30_000
		});
		expect(fitExcerpt(-5, 200, 600_000, 40_000)).toEqual({ startMs: 0, durationMs: 1000 });
	});
});
