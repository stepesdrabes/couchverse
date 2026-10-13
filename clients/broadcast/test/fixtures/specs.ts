// Fixed specs over the stand-in media in test/media: the golden frames, the audio determinism
// test, the schema test and the dev playground all use these.

import type { Bed, BreakSpec, ClipSpec, Shot, Title } from '../../src/spec';

const M = '/test/media';

const bed = (inPoint = 0, out = 24): Bed => ({ url: `${M}/bed.ogg`, in: inPoint, out });

export const TITLES = {
	night: {
		name: 'The Long Night',
		year: 2008,
		runtimeMinutes: 126,
		genres: ['Action', 'Adventure', 'Sci-Fi'],
		accents: { backdrop: '#0a2a44', poster: '#072646' },
		backdrop: `${M}/night-backdrop.jpg`,
		poster: `${M}/night-poster.jpg`,
		taglines: ['Heroes aren’t born.', 'They’re built.']
	},
	alley: {
		name: 'Neon Alley',
		year: 1994,
		runtimeMinutes: 154,
		genres: ['Crime', 'Thriller'],
		accents: { backdrop: '#643906', poster: '#d62316' },
		backdrop: `${M}/alley-backdrop.jpg`,
		poster: `${M}/alley-poster.jpg`,
		taglines: ['Uninterrupted.', 'As it should be.']
	},
	sunny: {
		name: 'Sunny Side',
		genres: ['Comedy', 'Animation'],
		accents: { backdrop: '#f5cc36', poster: '#fed51c' },
		backdrop: `${M}/sunny-backdrop.jpg`,
		poster: `${M}/sunny-poster.jpg`,
		taglines: ['Same street.', 'Different trouble.']
	},
	water: {
		name: 'Deep Water',
		genres: ['Crime', 'Drama'],
		accents: { backdrop: '#d69a77', poster: '#d81d24' },
		backdrop: `${M}/water-backdrop.jpg`,
		poster: `${M}/water-poster.jpg`
	}
} satisfies Record<string, Title>;

// the longest names, lines and genres the channel shows (some from its library), for fitting
const LONG = {
	hallows: {
		...TITLES.night,
		name: 'Harry Potter and the Deathly Hallows: Part 2',
		year: 2011,
		runtimeMinutes: 130,
		genres: ['Adventure', 'Family', 'Fantasy'],
		taglines: [
			'Seven strangers, one snowed-in motel and a secret',
			'none of them can afford to keep past midnight.'
		]
	},
	phoenix: {
		...TITLES.alley,
		name: 'Harry Potter and the Order of the Phoenix',
		year: 2007,
		genres: ['Science Fiction', 'Dokumentární'],
		taglines: [
			'Something evil has',
			'returned to Hogwarts, and it brought every last one of its friends.'
		]
	},
	strangelove: {
		...TITLES.sunny,
		name: 'Dr. Strangelove or: How I Learned to Stop Worrying and Love the Bomb',
		year: 1964,
		runtimeMinutes: 95,
		genres: ['Action & Adventure', 'Sci-Fi & Fantasy', 'War & Politics'],
		taglines: ['Something wicked this way comes, and it', 'brought every one of its friends along.']
	},
	zabky: { ...TITLES.water, name: 'ŽABKY NA TĚLÁK' },
	verte: { ...TITLES.night, name: 'Věřte nevěřte', genres: ['Komedie', 'Dokumentární'] }
} satisfies Record<string, Title>;

const longEpisode = {
	code: 'S03E14',
	name: 'The One Where Everybody Finds Out About the Wedding',
	runtimeMinutes: 23
};
const czechEpisode = {
	code: 'S01E02',
	name: 'Případ ztracené nevěsty a dvou podivných svědků',
	runtimeMinutes: 52
};

const frames = (dir: string) =>
	Array.from({ length: 25 }, (_, i) => `${M}/${dir}/${String(i + 1).padStart(3, '0')}.jpg`);

const SHOTS: Shot[] = [
	{ kind: 'still', image: 'backdrop', from: [0, 0, 1, 1], to: [0.4, 0.02, 0.46, 0.46] },
	{ kind: 'footage', frames: frames('shot-a') },
	{ kind: 'still', image: 'poster', from: [0, 0.03, 0.62, 0.26] },
	{ kind: 'footage', frames: frames('shot-b') },
	{ kind: 'still', image: 'backdrop', from: [0.52, 0.5, 0.4, 0.48] }
];

const episode = { code: 'S01E02', name: 'Bart the Genius', runtimeMinutes: 23 };

export const FIXTURES = {
	'ident-full': {
		template: 'ident',
		cut: 'full',
		posters: [TITLES.night.poster, TITLES.alley.poster, TITLES.sunny.poster, TITLES.water.poster]
	},
	'ident-short': { template: 'ident', cut: 'short', posters: [TITLES.night.poster] },
	'ident-bare': { template: 'ident', cut: 'full' },
	'sting-2': { template: 'sting', duration: 2 },
	'sting-3': { template: 'sting', duration: 3 },
	'sting-5': { template: 'sting', duration: 5 },
	upnext: {
		template: 'upNext',
		title: TITLES.sunny,
		episode,
		time: '18:00',
		clock: '17:59',
		bed: bed(2, 24)
	},
	'promo-cinematic': {
		template: 'promo',
		part: 'full',
		mood: 'cinematic',
		title: TITLES.night,
		shots: SHOTS,
		airing: { eyebrow: 'Tonight', time: '20:15' },
		bed: bed()
	},
	'promo-grit-premiere': {
		template: 'promo',
		part: 'full',
		mood: 'grit',
		premiere: true,
		title: TITLES.alley,
		shots: SHOTS,
		airing: { eyebrow: 'Premiere', day: 'Saturday', time: '20:15' },
		bed: bed()
	},
	'promo-bright': {
		template: 'promo',
		part: 'full',
		mood: 'bright',
		title: TITLES.sunny,
		shots: SHOTS,
		airing: { eyebrow: 'Tomorrow', time: '18:00' },
		bed: bed()
	},
	evergreen: {
		template: 'evergreenEnd',
		mood: 'cinematic',
		title: TITLES.night,
		shots: SHOTS,
		bed: bed()
	},
	'strip-bright': {
		template: 'stripPromo',
		mood: 'bright',
		title: TITLES.sunny,
		airing: { eyebrow: 'Every weekday', time: '18:00', days: [0, 1, 2, 3, 4] },
		foot: { kind: 'count', count: 801 },
		bed: bed()
	},
	'strip-dark': {
		template: 'stripPromo',
		mood: 'dark',
		title: TITLES.water,
		airing: { eyebrow: 'Every night', time: '22:30', days: [0, 1, 2, 3, 4, 5, 6] },
		foot: { kind: 'from', season: 1, day: 0 },
		bed: bed(4, 24)
	},
	short: {
		template: 'shortPromo',
		title: TITLES.alley,
		airing: { eyebrow: 'Tomorrow', time: '21:00' },
		bed: bed()
	},
	lineup: {
		template: 'lineup',
		items: [
			{
				time: '18:00',
				title: TITLES.sunny,
				sub: 'S01E02 · Bart the Genius',
				state: 'live',
				progress: 0.16
			},
			{
				time: '18:25',
				title: TITLES.water,
				sub: 'S01E02 · Crocodile',
				state: 'next',
				badge: 'Next · in 21 min'
			},
			{ time: '20:15', title: TITLES.night, sub: 'Movie · Action · 126 min' },
			{ time: '22:30', title: TITLES.alley, sub: 'Movie · Crime · 154 min' }
		]
	},
	'squeeze-expand': {
		template: 'squeeze',
		ending: TITLES.night,
		program: { kind: 'standIn' },
		next: { title: TITLES.sunny, episode, time: '18:00' },
		clock: '17:57',
		handover: 'expand',
		bed: bed(2, 24)
	},
	'squeeze-key': {
		template: 'squeeze',
		ending: TITLES.alley,
		program: { kind: 'key' },
		next: { title: TITLES.water, time: '22:30' },
		handover: 'cursor'
	},
	'squeeze-frames': {
		template: 'squeeze',
		ending: TITLES.alley,
		program: { kind: 'frames', frames: frames('shot-a') },
		next: { title: TITLES.night, time: '20:15' },
		clock: '20:13',
		handover: 'expand'
	},
	standby: { template: 'standby', duration: 8 },
	'promo-cinematic-long': {
		template: 'promo',
		part: 'full',
		mood: 'cinematic',
		title: LONG.hallows,
		shots: SHOTS,
		airing: { eyebrow: 'This week', day: 'Wednesday', time: '20:15' },
		bed: bed()
	},
	'promo-grit-long': {
		template: 'promo',
		part: 'full',
		mood: 'grit',
		title: LONG.phoenix,
		shots: SHOTS,
		airing: { eyebrow: 'Tomorrow', time: '21:00' },
		bed: bed()
	},
	'promo-bright-long': {
		template: 'promo',
		part: 'full',
		mood: 'bright',
		title: LONG.strangelove,
		shots: SHOTS,
		airing: { eyebrow: 'Today', time: '18:00' },
		bed: bed()
	},
	'evergreen-long': {
		template: 'evergreenEnd',
		mood: 'bright',
		title: LONG.strangelove,
		shots: SHOTS,
		bed: bed()
	},
	'upnext-long': {
		template: 'upNext',
		title: LONG.hallows,
		episode: longEpisode,
		time: '18:00',
		clock: '17:59',
		bed: bed(2, 24)
	},
	'strip-long': {
		template: 'stripPromo',
		mood: 'dark',
		title: LONG.hallows,
		airing: { eyebrow: 'Every weekday', time: '22:30', days: [0, 1, 2, 3, 4] },
		foot: { kind: 'from', season: 12, day: 2 },
		bed: bed(4, 24)
	},
	'short-long': {
		template: 'shortPromo',
		title: LONG.hallows,
		airing: { eyebrow: 'This week', day: 'Wednesday', time: '21:00' },
		bed: bed()
	},
	'lineup-long': {
		template: 'lineup',
		items: [
			{
				time: '18:00',
				title: LONG.hallows,
				sub: `${longEpisode.code} · ${longEpisode.name}`,
				state: 'live',
				progress: 0.3
			},
			{
				time: '18:25',
				title: LONG.phoenix,
				sub: 'Movie · Science Fiction · 138 min',
				state: 'next',
				badge: 'Next · in 125 min'
			},
			{ time: '20:15', title: LONG.strangelove, sub: 'Movie · Action & Adventure · 95 min' },
			{ time: '22:30', title: LONG.verte, sub: `${czechEpisode.code} · ${czechEpisode.name}` },
			{ time: '23:45', title: LONG.zabky, sub: 'Movie · Crime · 98 min' }
		]
	},
	'squeeze-long': {
		template: 'squeeze',
		ending: TITLES.night,
		program: { kind: 'standIn' },
		next: { title: LONG.strangelove, episode: czechEpisode, time: '18:00' },
		clock: '17:57',
		handover: 'expand',
		bed: bed(2, 24)
	}
} satisfies Record<string, ClipSpec>;

export type FixtureName = keyof typeof FIXTURES;

/** A short break: the squeeze handing over to the short ident, a promo (body and end). */
export const BREAK: BreakSpec = {
	template: 'break',
	loudness: -20,
	items: [
		{ at: 0, spec: { ...FIXTURES['squeeze-expand'], handover: 'cursor' } },
		{ at: 10.6, spec: FIXTURES['ident-short'] },
		{ at: 15.12, spec: { ...FIXTURES['promo-bright'], loudness: -20 } }
	]
};
