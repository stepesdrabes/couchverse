import { describe, expect, it } from 'vitest';
import type { AdminTitle } from '$lib/generated/api';
import {
	buildSpec,
	DEFAULT_OPTIONS,
	previewTitle,
	shiftTime,
	type PreviewOptions
} from './preview';

function admin(id: string, kind: 'movie' | 'series', name: string): AdminTitle {
	const art = (kind: 'backdrop' | 'poster', accent: string) => ({
		id: `${id}-${kind}`,
		kind,
		accent,
		createdAt: '2026-10-01T00:00:00Z',
		height: 0,
		width: 0,
		lang: null,
		ownerId: id,
		ownerKind: 'title' as const,
		path: '',
		source: 'tmdb' as const
	});
	return {
		artwork: [art('backdrop', '#0a2a44'), art('poster', '-')],
		audioStreamsByFile: {},
		mediaFiles: [],
		seasons:
			kind === 'series'
				? [
						{
							id: 's1',
							name: 'Season 1',
							overview: '',
							seasonNumber: 1,
							titleId: id,
							episodes: [2, 1].map((n) => ({
								airDate: null,
								episodeNumber: n,
								id: `e${n}`,
								name: `Episode ${n}`,
								overview: '',
								runtimeMinutes: 23,
								seasonId: 's1',
								thumbId: null
							}))
						}
					]
				: [],
		subtitlesByFile: {},
		title: {
			addedAt: '',
			allowRandomPlayback: true,
			contentRating: '',
			genreLabels: [],
			genres: ['Comedy'],
			id,
			kind,
			metadataLanguages: ['en'],
			name,
			overview: '',
			releaseDate: null,
			runtimeMinutes: kind === 'movie' ? 126 : null,
			slug: id,
			sortName: name,
			status: 'published',
			tmdbId: null,
			updatedAt: '',
			year: 2008
		},
		translations: {}
	};
}

const url = (a: { id: string }, size?: string) => `/art/${a.id}${size ? `?size=${size}` : ''}`;
const movie = previewTitle(admin('m', 'movie', 'Iron Man'), url);
const series = previewTitle(admin('s', 'series', 'The Simpsons'), url);
const opts = (patch: Partial<PreviewOptions>): PreviewOptions => ({ ...DEFAULT_OPTIONS, ...patch });

describe('previewTitle', () => {
	it('takes the artwork, its accents and the first episode', () => {
		expect(movie.title.backdrop).toBe('/art/m-backdrop');
		expect(movie.title.poster).toBe('/art/m-poster?size=w780');
		expect(movie.title.accents).toEqual({ backdrop: '#0a2a44', poster: undefined });
		expect(movie.episode).toBeUndefined();
		expect(series.episode).toEqual({ code: 'S01E01', name: 'Episode 1', runtimeMinutes: 23 });
		expect(series.episodeCount).toBe(2);
	});
});

describe('buildSpec', () => {
	it('needs no title for the brand pieces', () => {
		expect(buildSpec(opts({ template: 'identFull' }), null, null)).toEqual({
			template: 'ident',
			cut: 'full'
		});
		expect(buildSpec(opts({ template: 'sting', stingLength: 5 }), null, null)).toEqual({
			template: 'sting',
			duration: 5
		});
		expect(buildSpec(opts({ template: 'promo' }), null, null)).toBeNull();
	});

	it('shows recorded bulletins without a title, credits and outlets on them', () => {
		const weather = buildSpec(opts({ template: 'weather' }), null, null);
		expect(weather).toMatchObject({ template: 'weather', duration: 15, asOf: '17:00' });
		if (weather?.template !== 'weather') return;
		expect(weather.cities[0].name).toBe('Prague');
		expect(weather.credits.forecast).toBe('Weather data by Open-Meteo.com');
		const news = buildSpec(opts({ template: 'news' }), null, null);
		expect(news).toMatchObject({ template: 'news', duration: 20 });
		if (news?.template !== 'news') return;
		expect(news.stories.map((s) => s.outlet)).toContain('iROZHLAS');
		expect(news.credit).toContain('as of 17:02');
	});

	it('frames library posters in the ident', () => {
		const spec = buildSpec(opts({ template: 'identShort' }), movie, null, [movie, series]);
		expect(spec).toEqual({
			template: 'ident',
			cut: 'short',
			posters: [movie.title.poster, series.title.poster]
		});
	});

	it('phrases the airing in English', () => {
		const spec = buildSpec(
			opts({ template: 'promo', airs: 'tomorrow', time: '21:00' }),
			movie,
			null
		);
		expect(spec).toMatchObject({
			template: 'promo',
			airing: { eyebrow: 'Tomorrow', time: '21:00' }
		});
		const premiere = buildSpec(opts({ template: 'promo', premiere: true, day: 5 }), movie, null);
		expect(premiere).toMatchObject({ airing: { eyebrow: 'Premiere', day: 'Saturday' } });
	});

	it('counts down to the program in up next', () => {
		const spec = buildSpec(opts({ template: 'upNext', time: '18:00' }), series, null);
		expect(spec).toMatchObject({ time: '18:00', clock: '17:59', episode: { code: 'S01E01' } });
	});

	it('fills a lineup with the following titles', () => {
		const spec = buildSpec(opts({ template: 'lineup', time: '18:00' }), series, null, [
			series,
			movie
		]);
		expect(spec?.template).toBe('lineup');
		if (spec?.template !== 'lineup') return;
		expect(spec.items.map((i) => [i.time, i.title.name, i.state])).toEqual([
			['18:00', 'The Simpsons', 'live'],
			['18:25', 'Iron Man', 'next']
		]);
		expect(spec.items[1].sub).toBe('Movie · Comedy · 126 min');
		expect(buildSpec(opts({ template: 'lineup' }), series, null, [])).toBeNull();
	});

	it('squeezes into the next title', () => {
		const spec = buildSpec(opts({ template: 'squeeze', handover: 'cursor' }), movie, series);
		expect(spec).toMatchObject({
			ending: { name: 'Iron Man' },
			next: { title: { name: 'The Simpsons' } },
			handover: 'cursor',
			program: { kind: 'standIn' }
		});
	});

	it('plays a placeholder bed or none', () => {
		const spec = buildSpec(opts({ template: 'shortPromo', bed: 'none' }), movie, null);
		expect(spec && 'bed' in spec ? spec.bed : 'missing').toBeUndefined();
		const groove = buildSpec(opts({ template: 'shortPromo', bed: 'synth:groove' }), movie, null);
		expect(groove).toMatchObject({ bed: { url: 'synth:groove', in: 0 } });
	});
});

describe('shiftTime', () => {
	it('wraps around midnight', () => {
		expect(shiftTime('00:00', -1)).toBe('23:59');
		expect(shiftTime('23:50', 25)).toBe('00:15');
	});
});
