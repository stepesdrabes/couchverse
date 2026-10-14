// The admin Broadcast preview: library titles turned into broadcast specs, the way the server's
// composer will build them later (minus footage and real soundtracks, which come from the
// title analysis). Pure, so the page stays a view over it.

import {
	airing,
	type AiringKind,
	type ClipSpec,
	type Episode,
	type IdentDesign,
	type NewsSpec,
	type Shot,
	type Title,
	type WeatherDay,
	type WeatherSpec
} from '@couchverse/broadcast';
import type { AdminTitle, Artwork } from '$lib/generated/api';

export type PreviewTemplate =
	| 'identFull'
	| 'identShort'
	| 'sting'
	| 'upNext'
	| 'promo'
	| 'evergreenEnd'
	| 'stripPromo'
	| 'shortPromo'
	| 'lineup'
	| 'squeeze'
	| 'standby'
	| 'weather'
	| 'news';

export type PreviewBed = 'none' | 'synth:groove' | 'synth:pulse' | 'synth:drone';

export interface PreviewOptions {
	template: PreviewTemplate;
	identDesign: IdentDesign;
	promoMood: 'cinematic' | 'grit' | 'bright';
	stripMood: 'bright' | 'dark';
	premiere: boolean;
	part: 'full' | 'body' | 'end';
	handover: 'expand' | 'ident';
	stingLength: number;
	airs: AiringKind;
	/** `HH:MM`. */
	time: string;
	/** 0 = Monday. */
	day: number;
	bed: PreviewBed;
	taglines: [string, string];
}

export const DEFAULT_OPTIONS: PreviewOptions = {
	template: 'promo',
	identDesign: 'journey',
	promoMood: 'cinematic',
	stripMood: 'bright',
	premiere: false,
	part: 'full',
	handover: 'expand',
	stingLength: 6,
	airs: 'tonight',
	time: '20:15',
	day: 5,
	bed: 'synth:groove',
	taglines: ['Every story', 'has a couch.']
};

/** Which templates show a title, and which also need the next one. */
export const NEEDS_TITLE: Record<PreviewTemplate, boolean> = {
	identFull: false,
	identShort: false,
	sting: false,
	upNext: true,
	promo: true,
	evergreenEnd: true,
	stripPromo: true,
	shortPromo: true,
	lineup: true,
	squeeze: true,
	standby: false,
	weather: false,
	news: false
};

/** A library title as the templates show it. */
export interface PreviewTitle {
	id: string;
	kind: 'movie' | 'series';
	title: Title;
	/** The first episode of the first season, for series. */
	episode?: Episode;
	episodeCount: number;
}

const accentOf = (a?: Artwork) =>
	a?.accent && /^#[0-9a-f]{6}$/i.test(a.accent) ? a.accent : undefined;

/**
 * A library title for the templates. `url` builds an artwork URL; `taglines` stand in for the
 * title's tagline until the metadata keeps it.
 */
export function previewTitle(
	t: AdminTitle,
	url: (a: Artwork, size?: 'w780') => string,
	taglines: string[] = []
): PreviewTitle {
	const backdrop = t.artwork.find((a) => a.kind === 'backdrop');
	const poster = t.artwork.find((a) => a.kind === 'poster');
	const seasons = [...t.seasons].sort((a, b) => a.seasonNumber - b.seasonNumber);
	const season = seasons.find((s) => s.seasonNumber > 0 && s.episodes.length) ?? seasons[0];
	const first = season?.episodes.slice().sort((a, b) => a.episodeNumber - b.episodeNumber)[0];
	const pad = (n: number) => String(n).padStart(2, '0');
	return {
		id: t.title.id,
		kind: t.title.kind,
		title: {
			name: t.title.name,
			year: t.title.year ?? undefined,
			runtimeMinutes: t.title.runtimeMinutes ?? undefined,
			genres: t.title.genres,
			accents: { backdrop: accentOf(backdrop), poster: accentOf(poster) },
			backdrop: backdrop ? url(backdrop) : '',
			poster: poster ? url(poster, 'w780') : '',
			taglines: taglines.map((l) => l.trim()).filter(Boolean)
		},
		episode:
			season && first
				? {
						code: `S${pad(season.seasonNumber)}E${pad(first.episodeNumber)}`,
						name: first.name,
						runtimeMinutes: first.runtimeMinutes ?? undefined
					}
				: undefined,
		episodeCount: t.seasons.reduce((n, s) => n + s.episodes.length, 0)
	};
}

/** `HH:MM` moved by whole minutes, wrapping around midnight. */
export function shiftTime(hhmm: string, minutes: number): string {
	const [h, m] = hhmm.split(':').map(Number);
	const total = ((((h || 0) * 60 + (m || 0) + minutes) % 1440) + 1440) % 1440;
	const pad = (n: number) => String(n).padStart(2, '0');
	return `${pad(Math.floor(total / 60))}:${pad(total % 60)}`;
}

/** Stills from the artwork while the title has no analysed footage. */
const STILLS: Shot[] = [
	{ kind: 'still', image: 'backdrop', from: [0, 0, 1, 1], to: [0.3, 0.05, 0.5, 0.5] },
	{ kind: 'still', image: 'poster', from: [0, 0.05, 0.7, 0.3] },
	{ kind: 'still', image: 'backdrop', from: [0.5, 0.4, 0.45, 0.5] },
	{ kind: 'still', image: 'poster', from: [0.1, 0.45, 0.6, 0.25] },
	{ kind: 'still', image: 'backdrop', from: [0.1, 0.1, 0.5, 0.6] }
];

const sub = (t: PreviewTitle) =>
	t.kind === 'series' && t.episode
		? `${t.episode.code} · ${t.episode.name}`
		: ['Movie', t.title.genres[0], t.title.runtimeMinutes ? `${t.title.runtimeMinutes} min` : '']
				.filter(Boolean)
				.join(' · ');

const day = (
	label: string,
	weekday: string,
	temps: [number, number],
	code: number,
	rain: number
): WeatherDay => ({
	label,
	weekday,
	short: weekday.slice(0, 3),
	min: temps[0],
	max: temps[1],
	code,
	precipitation: rain
});

const DAYS = [
	day('Today', 'Tuesday', [4, 15], 2, 23),
	day('Tomorrow', 'Wednesday', [10, 17], 2, 70),
	day('Thursday', 'Thursday', [6, 15], 3, 0),
	day('Friday', 'Friday', [5, 18], 1, 0),
	day('Saturday', 'Saturday', [6, 14], 61, 17)
];

const city = (name: string, lat: number, lon: number, temperature: number, code: number) => ({
	name,
	lat,
	lon,
	now: { temperature, code, day: true },
	days: DAYS
});

/** A weather bulletin as the server writes one, from the forecast of 13 Oct 2026, 17:00. */
const WEATHER: WeatherSpec = {
	template: 'weather',
	duration: 15,
	asOf: '17:00',
	cities: [
		city('Prague', 50.088, 14.4208, 15, 1),
		city('Brno', 49.1952, 16.608, 15, 1),
		city('Ostrava', 49.8347, 18.282, 12, 0)
	],
	warnings: [
		{
			level: 'yellow',
			event: 'Moderate Wind Related Impact',
			active: true,
			from: { time: '16:00' },
			until: { day: 'Wednesday', time: '02:00' },
			regions: [
				{ code: 'CZ-63', name: 'Vysočina Region', partial: false },
				{ code: 'CZ-64', name: 'South Moravian Region', partial: true }
			]
		}
	],
	credits: {
		forecast: 'Weather data by Open-Meteo.com',
		warnings: 'Source: ČHMÚ',
		borders: 'Borders: ČÚZK / geoBoundaries (CC BY 4.0)'
	}
};

const story = (outlet: string, headline: string, outlets: number) => ({
	outlet,
	headline,
	outlets
});

/** A news bulletin as the server writes one, from the feeds of 13 Oct 2026, 17:02. */
const NEWS: NewsSpec = {
	template: 'news',
	duration: 20,
	asOf: '17:02',
	stories: [
		story('ČT24', 'Bulharsko hlásí další útok na lodě v\u00a0Černém moři, jedna se potopila', 4),
		story(
			'ČT24',
			'ČEZ se bude bránit nařízení k\u00a0Diag Human u\u00a0lucemburského soudu, uvedl Babiš',
			4
		),
		story(
			'iROZHLAS',
			'Letadlo do Norimberku doprovázely stíhačky a\u00a0policie uzavřela letiště. Piloti omylem nahlásili nouzi',
			5
		),
		story('ČT24', 'Spojenci nás nevarovali před posledním útokem na Kyjev, uvedl Zelenskyj', 3),
		story('ČT24', 'Pomoc pro přeživší popravy Pikeovou se značně zpozdila, píše AP', 2)
	],
	credit: 'Headlines from the outlets’ RSS feeds, as of 17:02'
};

/**
 * The spec for the chosen template; null while a title it needs is missing. `following` are the
 * library titles after the chosen one (the lineup's rows).
 */
export function buildSpec(
	o: PreviewOptions,
	main: PreviewTitle | null,
	next: PreviewTitle | null,
	following: PreviewTitle[] = []
): ClipSpec | null {
	const bed = o.bed === 'none' ? undefined : { url: o.bed, in: 0, out: 28 };
	const when = airing(o.airs, o.time, o.day);
	switch (o.template) {
		case 'identFull':
			return { template: 'ident', cut: 'full', design: o.identDesign };
		case 'identShort':
			return { template: 'ident', cut: 'short' };
		case 'sting':
			return { template: 'sting', duration: o.stingLength };
		case 'standby':
			return { template: 'standby' };
		case 'weather':
			return WEATHER;
		case 'news':
			return NEWS;
	}
	if (!main) return null;
	const title = { ...main.title, taglines: o.taglines.map((l) => l.trim()).filter(Boolean) };
	switch (o.template) {
		case 'upNext':
			return {
				template: 'upNext',
				title,
				episode: main.kind === 'series' ? main.episode : undefined,
				time: o.time,
				clock: shiftTime(o.time, -1),
				bed
			};
		case 'promo':
			return {
				template: 'promo',
				part: o.part,
				mood: o.promoMood,
				premiere: o.premiere,
				title,
				shots: STILLS,
				airing: o.premiere ? airing('premiere', o.time, o.day) : when,
				bed
			};
		case 'evergreenEnd':
			return { template: 'evergreenEnd', mood: o.promoMood, title, shots: STILLS, bed };
		case 'stripPromo': {
			const nightly = o.airs === 'nightly';
			return {
				template: 'stripPromo',
				mood: o.stripMood,
				title,
				airing: {
					...airing(nightly ? 'nightly' : 'weekdays', o.time),
					days: nightly ? [0, 1, 2, 3, 4, 5, 6] : [0, 1, 2, 3, 4]
				},
				foot:
					main.kind === 'series' && main.episodeCount > 1
						? { kind: 'count', count: main.episodeCount }
						: { kind: 'from', season: 1, day: o.day },
				bed
			};
		}
		case 'shortPromo':
			return { template: 'shortPromo', title, airing: when, bed };
		case 'lineup': {
			const titles = [main, ...following.filter((t) => t.id !== main.id)].slice(0, 4);
			if (titles.length < 2) return null;
			const times = [o.time, shiftTime(o.time, 25), shiftTime(o.time, 135), shiftTime(o.time, 270)];
			return {
				template: 'lineup',
				items: titles.map((t, i) => ({
					time: times[i],
					title: t.title,
					sub: sub(t),
					...(i === 0 ? { state: 'live' as const, progress: 0.3 } : {}),
					...(i === 1 ? { state: 'next' as const, badge: 'Next · in 18 min' } : {})
				}))
			};
		}
		case 'squeeze': {
			const after = next ?? main;
			return {
				template: 'squeeze',
				ending: main.title,
				program: { kind: 'standIn' },
				next: {
					title: after.title,
					episode: after.kind === 'series' ? after.episode : undefined,
					time: o.time
				},
				clock: shiftTime(o.time, -3),
				handover: o.handover,
				bed
			};
		}
	}
	return null;
}
