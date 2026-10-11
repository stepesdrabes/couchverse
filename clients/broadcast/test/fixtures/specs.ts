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
	'ident-full': { template: 'ident', cut: 'full' },
	'ident-short': { template: 'ident', cut: 'short' },
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
		handover: 'line'
	},
	'squeeze-frames': {
		template: 'squeeze',
		ending: TITLES.alley,
		program: { kind: 'frames', frames: frames('shot-a') },
		next: { title: TITLES.night, time: '20:15' },
		clock: '20:13',
		handover: 'expand'
	},
	standby: { template: 'standby', duration: 8 }
} satisfies Record<string, ClipSpec>;

export type FixtureName = keyof typeof FIXTURES;

/** A short break: squeeze, short ident, a promo (body and end), up next. */
export const BREAK: BreakSpec = {
	template: 'break',
	loudness: -20,
	items: [
		{ at: 0, spec: FIXTURES['squeeze-expand'] },
		{ at: 11.52, spec: FIXTURES['ident-short'] },
		{ at: 16.52, spec: { ...FIXTURES['promo-bright'], loudness: -20 } }
	]
};
