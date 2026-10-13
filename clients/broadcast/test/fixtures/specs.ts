// Fixed specs over the stand-in media in test/media: the golden frames, the audio determinism
// test, the schema test and the dev playground all use these.

import type {
	Bed,
	BreakSpec,
	ClipSpec,
	FootageShot,
	Logo,
	Shot,
	Title,
	WeatherCity,
	WeatherDay,
	WeatherWarning
} from '../../src/spec';

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

// a title's scenes as the screens around the programs show them: two shots of two seconds (the
// stand-in second, played forwards and back)
const pingPong = (dir: string) => [...frames(dir), ...frames(dir).reverse()];
const SCENES: FootageShot[] = [
	{ kind: 'footage', frames: pingPong('shot-a') },
	{ kind: 'footage', frames: pingPong('shot-b') }
];
/** With its logo (a 3.7:1 stand-in) and its scenes. */
const NIGHT_ON_AIR: Title = {
	...TITLES.night,
	logo: { url: `${M}/night-logo.png`, aspect: 892 / 240 },
	footage: SCENES
};

// the bulletins as the server writes them from the feeds recorded on 13 Oct 2026 at 17:02
// (backend/internal/feature/couchtv/testdata/bulletins); ČHMÚ's warning is the recorded one
// of 8 Oct, shown as if in force
const day = (
	label: string,
	weekday: string,
	min: number,
	max: number,
	code: number,
	precipitation?: number
): WeatherDay => ({
	label,
	weekday,
	short: weekday.slice(0, 3),
	min,
	max,
	code,
	...(precipitation != null ? { precipitation } : {})
});
const week = (t: [number, number, number, number?][]) =>
	t.map(([min, max, code, rain], i) =>
		day(
			['Today', 'Tomorrow', 'Thursday', 'Friday', 'Saturday'][i],
			['Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'][i],
			min,
			max,
			code,
			rain
		)
	);
const city = (
	name: string,
	lat: number,
	lon: number,
	temperature: number,
	code: number,
	days: WeatherDay[]
): WeatherCity => ({ name, lat, lon, now: { temperature, code, day: true }, days });
const PRAGUE = city(
	'Prague',
	50.088,
	14.4208,
	15,
	1,
	week([
		[4, 15, 2, 23],
		[10, 17, 2, 70],
		[6, 15, 3, 0],
		[5, 18, 1, 0],
		[6, 14, 61, 17]
	])
);
const BRNO = city(
	'Brno',
	49.1952,
	16.608,
	15,
	1,
	week([
		[4, 15, 1, 0],
		[7, 18, 2, 58],
		[5, 15, 2, 0],
		[4, 16, 1, 0],
		[4, 17, 3, 6]
	])
);
const OSTRAVA = city(
	'Ostrava',
	49.8347,
	18.282,
	12,
	0,
	week([
		[4, 13, 1, 33],
		[4, 14, 80, 85],
		[4, 13, 3, 10],
		[1, 15, 0, 0],
		[0, 14, 1, 8]
	])
);
const WIND: WeatherWarning = {
	level: 'yellow',
	event: 'Moderate Wind Related Impact',
	active: true,
	from: { time: '16:00' },
	until: { day: 'Wednesday', time: '02:00' },
	regions: [
		{ code: 'CZ-31', name: 'South Bohemian Region', partial: true },
		{ code: 'CZ-41', name: 'Karlovy Vary Region', partial: true },
		{ code: 'CZ-42', name: 'Ústí nad Labem Region', partial: true },
		{ code: 'CZ-53', name: 'Pardubice Region', partial: true },
		{ code: 'CZ-63', name: 'Vysočina Region', partial: false },
		{ code: 'CZ-64', name: 'South Moravian Region', partial: true },
		{ code: 'CZ-71', name: 'Olomouc Region', partial: true }
	]
};
const CREDITS = {
	forecast: 'Weather data by Open-Meteo.com',
	warnings: 'Source: ČHMÚ',
	borders: 'Borders: ČÚZK / geoBoundaries (CC BY 4.0)'
};
// the rest of the default cities, the longest names among them, with made-up weather
const MORE_CITIES = (
	[
		['Plzeň', 49.7475, 13.3776, 13],
		['Liberec', 50.7671, 15.0562, 11],
		['Olomouc', 49.5955, 17.2518, 14],
		['České Budějovice', 48.9745, 14.4743, 14],
		['Hradec Králové', 50.2092, 15.8328, 13],
		['Ústí nad Labem', 50.6607, 14.0323, -2],
		['Zlín', 49.2265, 17.6707, 15],
		['Jihlava', 49.3961, 15.5912, 10],
		['Karlovy Vary', 50.2327, 12.8712, 9]
	] as const
).map(([name, lat, lon, t]) => city(name, lat, lon, t, 3, PRAGUE.days));
const HEADLINES = [
	['ČT24', 'Bulharsko hlásí další útok na lodě v Černém moři, jedna se potopila', 4, true],
	['ČT24', 'ČEZ se bude bránit nařízení k Diag Human u lucemburského soudu, uvedl Babiš', 4, false],
	[
		'iROZHLAS',
		'Letadlo do Norimberku doprovázely stíhačky a policie uzavřela letiště. Piloti omylem nahlásili nouzi',
		5,
		false
	],
	['ČT24', 'Spojenci nás nevarovali před posledním útokem na Kyjev, uvedl Zelenskyj', 3, false],
	['ČT24', 'Pomoc pro přeživší popravy Pikeovou se značně zpozdila, píše AP', 2, true]
] as const;
// the longest headlines of the recorded feeds
const LONG_HEADLINES = [
	[
		'iROZHLAS',
		'Žádná šikana ČEZ, stát měl dávno jednat, říká advokát Diag Humanu Kalvoda. Babiš potvrdil podání žalob',
		3
	],
	[
		'iROZHLAS',
		'Klempíř chce změnit spolupráci s festivaly. Nové podmínky čekají Karlovy Vary i Pražské jaro',
		2
	],
	[
		'Novinky.cz',
		'Letiště v Norimberku po poplachu přerušilo provoz. Přistávající letadlo doprovázely stíhačky',
		5
	],
	[
		'Novinky.cz',
		'45,2 km na Marsu. Vozítko Perseverance překonalo rekord ve vzdálenosti ujeté na jiném světě',
		1
	],
	[
		'Seznam Zprávy',
		'Učitelé v Paříži vytvořili lidský řetěz, aby chránili protestující studenty před policií',
		2
	]
] as const;

const SHOTS: Shot[] = [
	{ kind: 'still', image: 'backdrop', from: [0, 0, 1, 1], to: [0.4, 0.02, 0.46, 0.46] },
	{ kind: 'footage', frames: frames('shot-a') },
	{ kind: 'still', image: 'poster', from: [0, 0.03, 0.62, 0.26] },
	{ kind: 'footage', frames: frames('shot-b') },
	{ kind: 'still', image: 'backdrop', from: [0.52, 0.5, 0.4, 0.48] }
];

// the promos' titles carry their scenes (as the composer gives them) and some their logos: a wide
// gold wordmark and a squarer red and paper one (test/media/generate.sh)
const PROMO_SCENES: FootageShot[] = [
	{ kind: 'footage', frames: frames('shot-a') },
	{ kind: 'footage', frames: frames('shot-b'), focus: [0.5, 0.2, 0.5, 0.6] }
];
const PROMO = {
	night: {
		...TITLES.night,
		logo: { url: `${M}/logo-wide.png`, aspect: 4 },
		footage: PROMO_SCENES
	},
	alley: {
		...TITLES.alley,
		logo: { url: `${M}/logo-tall.png`, aspect: 2.3 },
		footage: PROMO_SCENES
	},
	sunny: { ...TITLES.sunny, footage: PROMO_SCENES },
	// no artwork, no scenes: the channel paints the picture itself
	zabky: {
		...TITLES.water,
		name: 'ŽABKY NA TĚLÁK',
		year: 2026,
		runtimeMinutes: 98,
		genres: ['Comedy'],
		backdrop: '',
		poster: '',
		taglines: ['Léto, které nikdo nečekal.', 'A tělák, který nikdo nezapomene.']
	}
} satisfies Record<string, Title>;

const episode = { code: 'S01E02', name: 'Bart the Genius', runtimeMinutes: 23 };

// a title as the screens that show it big get it: its logo (where it has one) and its scenes
const SCENES: FootageShot[] = [
	{ kind: 'footage', frames: frames('shot-a') },
	{ kind: 'footage', frames: frames('shot-b') }
];
const LOGOS = {
	// yellow outlined in near-black
	sunny: { url: `${M}/logo-sunny.png`, aspect: 3 },
	// a flat dark red, too dark to read on night as it is
	water: { url: `${M}/logo-water.png`, aspect: 1000 / 420 }
} satisfies Record<string, Logo>;
const shown = (t: Title, logo?: Logo): Title => ({
	...t,
	...(logo ? { logo } : {}),
	footage: SCENES
});

export const FIXTURES = {
	'ident-journey': { template: 'ident', cut: 'full', design: 'journey' },
	'ident-cinematic': { template: 'ident', cut: 'full', design: 'cinematic' },
	'ident-morph': { template: 'ident', cut: 'full', design: 'morph' },
	'ident-assemble': { template: 'ident', cut: 'full', design: 'assemble' },
	'ident-short': { template: 'ident', cut: 'short' },
	'sting-2': { template: 'sting', duration: 2 },
	'sting-3': { template: 'sting', duration: 3 },
	'sting-5': { template: 'sting', duration: 5 },
	upnext: {
		template: 'upNext',
		title: { ...TITLES.sunny, footage: SCENES },
		episode,
		time: '18:00',
		clock: '17:59',
		bed: bed(2, 24)
	},
	// after an expanding squeeze: its first frame is the squeeze's last
	'upnext-continue': {
		template: 'upNext',
		title: { ...TITLES.sunny, footage: SCENES },
		episode,
		time: '18:00',
		clock: '17:57',
		enter: 'continue',
		bed: bed(2, 24)
	},
	// a movie with its logo and scenes, scored by the house
	'upnext-logo': {
		template: 'upNext',
		title: NIGHT_ON_AIR,
		time: '20:15',
		clock: '20:14'
	},
	'promo-cinematic': {
		template: 'promo',
		part: 'full',
		mood: 'cinematic',
		title: PROMO.night,
		shots: SHOTS,
		airing: { eyebrow: 'Tonight', time: '20:15' },
		bed: bed()
	},
	'promo-grit-premiere': {
		template: 'promo',
		part: 'full',
		mood: 'grit',
		premiere: true,
		title: PROMO.alley,
		shots: SHOTS,
		airing: { eyebrow: 'Premiere', day: 'Saturday', time: '20:15' },
		bed: bed()
	},
	'promo-bright': {
		template: 'promo',
		part: 'full',
		mood: 'bright',
		title: PROMO.sunny,
		shots: SHOTS,
		airing: { eyebrow: 'Tomorrow', time: '18:00' },
		bed: bed()
	},
	// the cinematic premiere, scored by the house (no bed), its end card on the hero still
	'promo-premiere': {
		template: 'promo',
		part: 'full',
		mood: 'cinematic',
		premiere: true,
		title: TITLES.water,
		shots: SHOTS,
		airing: { eyebrow: 'Premiere', day: 'Saturday', time: '20:15' }
	},
	evergreen: {
		template: 'evergreenEnd',
		mood: 'cinematic',
		title: PROMO.night,
		shots: SHOTS,
		bed: bed()
	},
	'strip-bright': {
		template: 'stripPromo',
		mood: 'bright',
		title: shown(TITLES.sunny, LOGOS.sunny),
		airing: { eyebrow: 'Every weekday', time: '18:00', days: [0, 1, 2, 3, 4] },
		foot: { kind: 'count', count: 801 },
		bed: bed()
	},
	'strip-dark': {
		template: 'stripPromo',
		mood: 'dark',
		title: shown(TITLES.water, LOGOS.water),
		airing: { eyebrow: 'Every night', time: '22:30', days: [0, 1, 2, 3, 4, 5, 6] },
		foot: { kind: 'from', season: 1, day: 0 },
		bed: bed(4, 24)
	},
	short: {
		template: 'shortPromo',
		title: shown(TITLES.alley),
		airing: { eyebrow: 'Tomorrow', time: '21:00' },
		bed: bed()
	},
	// a logo and the house's own music
	'short-logo': {
		template: 'shortPromo',
		title: shown(TITLES.sunny, LOGOS.sunny),
		airing: { eyebrow: 'Tonight', time: '20:15' }
	},
	lineup: {
		template: 'lineup',
		items: [
			{
				time: '18:00',
				title: { ...TITLES.sunny, footage: SCENES },
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
			{ time: '20:15', title: NIGHT_ON_AIR, sub: 'Movie · Action · 126 min' },
			{ time: '22:30', title: TITLES.alley, sub: 'Movie · Crime · 154 min' }
		]
	},
	'squeeze-expand': {
		template: 'squeeze',
		ending: TITLES.night,
		program: { kind: 'standIn' },
		next: { title: { ...TITLES.sunny, footage: SCENES }, episode, time: '18:00' },
		clock: '17:57',
		handover: 'expand',
		bed: bed(2, 24)
	},
	'squeeze-key': {
		template: 'squeeze',
		ending: TITLES.alley,
		program: { kind: 'key' },
		next: { title: TITLES.water, time: '22:30' },
		handover: 'ident'
	},
	// the program's own frames; the next title's logo and scenes on the card
	'squeeze-frames': {
		template: 'squeeze',
		ending: TITLES.alley,
		program: { kind: 'frames', frames: frames('shot-a') },
		next: { title: NIGHT_ON_AIR, time: '20:15' },
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
	// no artwork and no scenes: every picture is the painted dusk; the house scores grit
	'promo-zabky': {
		template: 'promo',
		part: 'full',
		mood: 'grit',
		title: PROMO.zabky,
		shots: [{ kind: 'still', image: 'backdrop', from: [0, 0, 1, 1] }],
		airing: { eyebrow: 'Tomorrow', time: '23:45' }
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
	// the longest name on the paper panel, with the house's own music
	'strip-long-bright': {
		template: 'stripPromo',
		mood: 'bright',
		title: shown(LONG.strangelove),
		airing: { eyebrow: 'Every night', time: '19:30', days: [0, 1, 2, 3, 4, 5, 6] },
		foot: { kind: 'count', count: 26 },
		duration: 12
	},
	'short-long': {
		template: 'shortPromo',
		title: shown(LONG.hallows),
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
			// no artwork: the channel's own plate
			{ time: '23:45', title: { ...LONG.zabky, backdrop: '' }, sub: 'Movie · Crime · 98 min' }
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
	},
	weather: {
		template: 'weather',
		duration: 15,
		asOf: '17:00',
		cities: [PRAGUE, BRNO, OSTRAVA],
		warnings: [WIND],
		credits: CREDITS
	},
	// every default city and a warning of each level
	'weather-long': {
		template: 'weather',
		duration: 15,
		asOf: '06:30',
		cities: [PRAGUE, BRNO, OSTRAVA, ...MORE_CITIES],
		warnings: [
			{
				level: 'red',
				event: 'Extreme Thunderstorm Related Impact',
				active: false,
				from: { time: '14:00' },
				until: { time: '22:00' },
				regions: [{ code: 'CZ-10', name: 'Prague', partial: false }]
			},
			{
				level: 'orange',
				event: 'Major Heat Related Impact',
				active: true,
				from: { day: 'Monday', time: '12:00' },
				regions: [
					{ code: 'CZ-20', name: 'Central Bohemian Region', partial: true },
					{ code: 'CZ-52', name: 'Hradec Králové Region', partial: false },
					{ code: 'CZ-80', name: 'Moravian-Silesian Region', partial: true }
				]
			},
			WIND
		],
		credits: CREDITS
	},
	// a quiet day, and a day ČHMÚ could not be read
	'weather-quiet': {
		template: 'weather',
		duration: 15,
		asOf: '12:30',
		cities: [PRAGUE],
		warnings: [],
		credits: CREDITS
	},
	'weather-unread': {
		template: 'weather',
		duration: 15,
		asOf: '12:30',
		cities: [BRNO, OSTRAVA],
		credits: CREDITS
	},
	news: {
		template: 'news',
		duration: 20,
		asOf: '17:02',
		stories: HEADLINES.map(([outlet, headline, outlets, trending]) => ({
			outlet,
			headline,
			outlets,
			...(trending ? { trending } : {})
		})),
		credit: 'Headlines from the outlets’ RSS feeds, as of 17:02'
	},
	'news-long': {
		template: 'news',
		duration: 20,
		asOf: '23:50',
		stories: LONG_HEADLINES.map(([outlet, headline, outlets]) => ({ outlet, headline, outlets })),
		credit: 'Headlines from the outlets’ RSS feeds, as of 23:50'
	},
	'news-three': {
		template: 'news',
		duration: 20,
		asOf: '06:10',
		stories: HEADLINES.slice(2).map(([outlet, headline, outlets]) => ({
			outlet,
			headline,
			outlets
		})),
		credit: 'Headlines from the outlets’ RSS feeds, as of 06:10'
	}
} satisfies Record<string, ClipSpec>;

export type FixtureName = keyof typeof FIXTURES;

/** A short break: the squeeze handing over to the short ident, a promo (body and end). */
export const BREAK: BreakSpec = {
	template: 'break',
	loudness: -20,
	items: [
		{ at: 0, spec: { ...FIXTURES['squeeze-expand'], handover: 'ident' } },
		{ at: 10.6, spec: FIXTURES['ident-short'] },
		{ at: 15.12, spec: { ...FIXTURES['promo-bright'], loudness: -20 } }
	]
};
