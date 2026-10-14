// The data contract: one JSON-serializable spec per template. The server composes these
// (contract/broadcast-spec.schema.json is generated from this file by `npm run schema`), the
// templates turn them into picture and sound. Copy is English (D13) and every time arrives
// preformatted, so templates never do clock or calendar maths.

/**
 * A colour as `#rrggbb`.
 * @pattern ^#[0-9a-fA-F]{6}$
 */
export type Hex = string;

/**
 * A 24-hour clock time, `20:15`.
 * @pattern ^([01][0-9]|2[0-3]):[0-5][0-9]$
 */
export type ClockTime = string;

/** A normalized crop of an image: `[x, y, width, height]`, each 0..1. */
export type Crop = [number, number, number, number];

/**
 * A day of the week, 0 = Monday.
 * @minimum 0
 * @maximum 6
 * @asType integer
 */
export type Weekday = number;

/** The audio render's integrated loudness, set by the server from the programs around it. */
export interface Loudness {
	/**
	 * Integrated loudness (ITU-R BS.1770) the audio render is normalized to, in LUFS; the
	 * true peak stays at or under -1 dBTP even when that leaves it quieter.
	 * @minimum -31
	 * @maximum -10
	 * @default -18
	 */
	loudness?: number;
}

/** An excerpt of a soundtrack the template plays under itself (a bed). */
export interface Bed {
	/**
	 * The audio file. Use WAV, FLAC or Ogg Opus (every Chromium decodes them; the open-source
	 * builds have no AAC). `synth:groove`, `synth:pulse` and `synth:drone` are built-in
	 * placeholder beds.
	 */
	url: string;
	/**
	 * Where the excerpt starts in the file, seconds.
	 * @minimum 0
	 */
	in: number;
	/**
	 * Where it ends, seconds; templates never play past it.
	 * @minimum 0
	 */
	out: number;
	/**
	 * Linear gain on top of the template's own level.
	 * @minimum 0
	 * @maximum 2
	 * @default 1
	 */
	gain?: number;
}

/** Framing for a title's backdrop per use: Ken Burns moves are `[from, to]`. */
export interface TitleCrops {
	upNext?: [Crop, Crop];
	strip?: [Crop, Crop];
	short?: [Crop, Crop];
	lineup?: Crop;
	squeeze?: Crop;
}

/** A title's logo artwork. */
export interface Logo {
	/** A PNG trimmed to its visible pixels, transparent around the letters. */
	url: string;
	/**
	 * Its width over its height.
	 * @exclusiveMinimum 0
	 */
	aspect: number;
}

/** A title as the templates show it. */
export interface Title {
	/** The name as shown (English where the title has it). */
	name: string;
	/**
	 * The title's logo in the language of `name` (else one not tied to a language), which a
	 * template draws in place of the name where it shows the title big; absent, the name is set
	 * in type.
	 */
	logo?: Logo;
	/**
	 * Scenes from the title, in order (its footage shots), for the screens that show it in
	 * motion; absent, they show its artwork.
	 */
	footage?: FootageShot[];
	/** @asType integer */
	year?: number;
	/** @asType integer */
	runtimeMinutes?: number;
	/** English genre labels, most significant first. */
	genres: string[];
	/** The server-extracted artwork accents; the more chromatic one colours the template. */
	accents: { backdrop?: Hex; poster?: Hex };
	/** Backdrop image URL (16:9, ideally 1920 wide). */
	backdrop: string;
	/** Poster image URL (2:3). */
	poster: string;
	/** Short English lines for promos, in order (the tagline, split). */
	taglines?: string[];
	crops?: TitleCrops;
}

export interface Episode {
	/** `S01E02`. */
	code: string;
	name: string;
	/** @asType integer */
	runtimeMinutes?: number;
}

/** Real footage: one shot's frames as 25 fps JPEG URLs, shown frame by frame. */
export interface FootageShot {
	kind: 'footage';
	/** @minItems 1 */
	frames: string[];
	/** The part of the frame to keep in view when it is cropped (default: all of it). */
	focus?: Crop;
}

/** A still: a crop of the title's artwork with a slow move from one crop to another. */
export interface StillShot {
	kind: 'still';
	image: 'backdrop' | 'poster';
	from: Crop;
	to?: Crop;
}

export type Shot = FootageShot | StillShot;

/** When something airs, formatted in English by the server. */
export interface Airing {
	/** The small-caps line: `Tonight`, `Tomorrow`, `Premiere`, `Every weekday`. */
	eyebrow: string;
	/** A day shown before the time on end cards (`Saturday`); empty or absent for none. */
	day?: string;
	time: ClockTime;
}

export interface IdentSpec extends Loudness {
	template: 'ident';
	/** The full cut (10.52 s, before the prime-time movie) or the short one (5.2 s). */
	cut: 'full' | 'short';
	/**
	 * The full cut's design (the channel rotates them, D35): `journey` (a dream panorama at
	 * dusk), `cinematic` (a short film from a hilltop into a room), `morph` (four posters joined
	 * by shape morphs) or `assemble` (the couch built from its geometry). Every design ends on
	 * the same lockup. The short cut ignores it.
	 * @default journey
	 */
	design?: 'journey' | 'cinematic' | 'morph' | 'assemble';
}

/** A gap filler in the brand's language: the ident's popcorn ending on the sonic logo. */
export interface StingSpec extends Loudness {
	template: 'sting';
	/**
	 * Seconds, a whole number of frames. One design for every length: the popcorn drops onto the
	 * couch, the sonic logo is stated whole and its chord rings out before the last frame; the
	 * length stretches the build before the landing and the hold after it.
	 * @minimum 5
	 * @maximum 10
	 */
	duration: number;
}

/** The next program, counting down into it. */
export interface UpNextSpec extends Loudness {
	template: 'upNext';
	title: Title;
	episode?: Episode;
	/** When the program starts. */
	time: ClockTime;
	/** The corner bug's clock; no bug without it. */
	clock?: ClockTime;
	/**
	 * Seconds.
	 * @minimum 6
	 * @maximum 12
	 * @default 8
	 */
	duration?: number;
	/** `continue` picks up a picture that is already on screen (after a squeeze). */
	enter?: 'cut' | 'continue';
	bed?: Bed;
}

/** What every promo body is made of. */
export interface PromoContent extends Loudness {
	mood: 'cinematic' | 'grit' | 'bright';
	/** The premiere variant (D24): a premiere card in the body, `Premiere` on the end card. */
	premiere?: boolean;
	title: Title;
	/**
	 * The shots, used in order and cycled; footage cuts on the bed's beats.
	 * @minItems 1
	 */
	shots: Shot[];
	/** The shot under the title wordmark (default: the next one). */
	hero?: Shot;
	/** Kinetic words for the body; timeless only (a genre, the year). Defaults from the title. */
	words?: string[];
	bed?: Bed;
}

/**
 * A promo: a timeless body (rendered once per title) and a 4 s end card with the airing. A
 * clip is one `part`; the break's audio renders `full`, so the bed runs across the cut.
 */
export interface PromoSpec extends PromoContent {
	template: 'promo';
	part: 'body' | 'end' | 'full';
	/** The end card's airing (the body ignores it); absent, the end card is the evergreen one. */
	airing?: Airing;
}

/** The evergreen end card ("Coming up on Couch TV") after a promo body, for fallback breaks. */
export interface EvergreenEndSpec extends PromoContent {
	template: 'evergreenEnd';
}

export type StripFoot =
	| {
			kind: 'count';
			/** @asType integer */
			count: number;
	  }
	| {
			kind: 'from';
			/** @asType integer */
			season: number;
			day: Weekday;
	  };

/** A strip ("Every weekday at 18:00"): the days light up on the beat. */
export interface StripPromoSpec extends Loudness {
	template: 'stripPromo';
	mood: 'bright' | 'dark';
	title: Title;
	airing: Airing & {
		/**
		 * The days it airs.
		 * @minItems 1
		 * @uniqueItems true
		 */
		days: Weekday[];
	};
	/** The line under the days: the episode count or the start of a season. */
	foot?: StripFoot;
	/**
	 * Seconds.
	 * @minimum 12
	 * @maximum 20
	 * @default 16
	 */
	duration?: number;
	bed?: Bed;
}

/** A short promo on the bed's first hit ("Tomorrow 21:00 INVINCIBLE"). */
export interface ShortPromoSpec extends Loudness {
	template: 'shortPromo';
	title: Title;
	airing: Airing;
	/**
	 * Seconds.
	 * @minimum 7
	 * @maximum 12
	 * @default 9
	 */
	duration?: number;
	bed?: Bed;
}

export interface LineupItem {
	time: ClockTime;
	title: Title;
	/** `S01E02 · Bart the Genius`, `Movie · Action · 126 min`. */
	sub: string;
	/** `live` (on now) or `next`; one row at most has either. */
	state?: 'live' | 'next';
	/** The `next` row's pill, preformatted: `Next · in 12 min`. */
	badge?: string;
	/**
	 * How far the live program is.
	 * @minimum 0
	 * @maximum 1
	 */
	progress?: number;
}

/** "Later on Couch TV". */
export interface LineupSpec extends Loudness {
	template: 'lineup';
	/**
	 * @minItems 2
	 * @maxItems 5
	 */
	items: LineupItem[];
	/**
	 * Seconds.
	 * @minimum 10
	 * @maximum 16
	 * @default 12
	 */
	duration?: number;
}

/** What fills the squeezed program panel. */
export type ProgramSource =
	| {
			/** A solid black hole for an ffmpeg composite (`keyRect`, the `matte` layer). */
			kind: 'key';
	  }
	| {
			/** The program's last seconds (from the credits) as 25 fps JPEG frames. */
			kind: 'frames';
			/** @minItems 1 */
			frames: string[];
	  }
	| {
			/** The ending title's backdrop under a credits roll (the admin preview). */
			kind: 'standIn';
			credits?: [string, string][];
	  };

/** The credits squeeze: the program shrinks into a panel and "Up next" fills the rest. */
export interface SqueezeSpec extends Loudness {
	template: 'squeeze';
	ending: Title;
	program: ProgramSource;
	next: { title: Title; episode?: Episode; time: ClockTime };
	/** The corner bug's clock; no bug without it. */
	clock?: ClockTime;
	/**
	 * `expand` (11.52 s): the next picture grows to full frame; `ident` (10.6 s): the program
	 * shrinks onto the couch in its place in the lockup and dissolves into it, its last frame
	 * the short ident's first.
	 */
	handover: 'expand' | 'ident';
	/** The program's own sound from the credits, ducked under "Up next". */
	bed?: Bed;
}

/** "Technical difficulties": loops seamlessly, picture and sound, until the next airing. */
export interface StandbySpec extends Loudness {
	template: 'standby';
	/**
	 * One loop, seconds.
	 * @minimum 8
	 * @maximum 30
	 * @default 12
	 */
	duration?: number;
}

// Bulletins (plan 6.10): the weather and the news, rendered shortly before the break that airs
// them from data the server fetched. Each airs in a slot of its own in the break, so its sound
// starts and ends inside the clip: nothing may ring past its last frame. Chrome is English
// (D13); what the data brings stays as the source wrote it (city names, Czech headlines).

/**
 * A WMO weather code as Open-Meteo reports it: 0 clear, 1 mainly clear, 2 partly cloudy, 3
 * overcast, 45 and 48 fog, 51-57 drizzle, 61-67 rain, 71-77 snow, 80-82 rain showers, 85 and 86
 * snow showers, 95-99 thunderstorms.
 * @minimum 0
 * @maximum 99
 * @asType integer
 */
export type WmoCode = number;

/**
 * Whole degrees Celsius (the server rounds).
 * @asType integer
 */
export type Celsius = number;

/** A moment a bulletin names: a clock time, with its day when that is not the bulletin's own. */
export interface BulletinTime {
	/** The English weekday (`Wednesday`) when it is another day than the bulletin's; absent on that day. */
	day?: string;
	time: ClockTime;
}

/** The weather at the bulletin's time. */
export interface WeatherNow {
	temperature: Celsius;
	code: WmoCode;
	/** Daylight: a sun rather than a moon. */
	day: boolean;
}

/** One day's forecast. */
export interface WeatherDay {
	/** `Today`, `Tomorrow`, then the weekday (`Thursday`), from the bulletin's day. */
	label: string;
	/** The weekday, `Thursday`. */
	weekday: string;
	/** The weekday's first three letters, `Thu`. */
	short: string;
	min: Celsius;
	max: Celsius;
	/** The weather of the daytime, 09-18 (not the day's worst hour). */
	code: WmoCode;
	/**
	 * The day's highest chance of precipitation, %; absent when the forecast has none.
	 * @minimum 0
	 * @maximum 100
	 * @asType integer
	 */
	precipitation?: number;
}

/** One place of the weather map, in the order the channel's config lists them. */
export interface WeatherCity {
	/** As the config names it (`Prague`, `České Budějovice`). */
	name: string;
	/**
	 * Where it is, degrees north (WGS 84): `czechiaMap` (core/geo.ts) places it on the outline.
	 * @minimum 48.5
	 * @maximum 51.1
	 */
	lat: number;
	/**
	 * Degrees east.
	 * @minimum 12
	 * @maximum 18.9
	 */
	lon: number;
	now: WeatherNow;
	/**
	 * Today and the days after it, in order.
	 * @minItems 1
	 * @maxItems 5
	 */
	days: WeatherDay[];
}

/** A kraj (region) of Czechia, by its ISO 3166-2 code. */
export type KrajCode =
	| 'CZ-10'
	| 'CZ-20'
	| 'CZ-31'
	| 'CZ-32'
	| 'CZ-41'
	| 'CZ-42'
	| 'CZ-51'
	| 'CZ-52'
	| 'CZ-53'
	| 'CZ-63'
	| 'CZ-64'
	| 'CZ-71'
	| 'CZ-72'
	| 'CZ-80';

/** A kraj a warning covers. */
export interface WarningRegion {
	code: KrajCode;
	/** In English, `Central Bohemian Region`. */
	name: string;
	/** Only some of its districts are warned. */
	partial: boolean;
}

/** One of ČHMÚ's warnings: an event at a level over the kraje it covers. */
export interface WeatherWarning {
	level: 'yellow' | 'orange' | 'red';
	/** ČHMÚ's English name of the event, `Moderate Wind Related Impact`. */
	event: string;
	/** In force when the bulletin was made; otherwise it begins later that day. */
	active: boolean;
	/** When it begins (the earliest of its areas; past when it is active). */
	from: BulletinTime;
	/** When it ends (the latest of its areas); absent: until further notice. */
	until?: BulletinTime;
	/** @minItems 1 */
	regions: WarningRegion[];
}

/** The credits the sources' terms ask for, each on screen whenever its data is. */
export interface WeatherCredits {
	/** `Weather data by Open-Meteo.com` (CC BY 4.0): always on screen. */
	forecast: string;
	/** `Source: ČHMÚ`: on screen whenever the warnings, or their absence, are. */
	warnings: string;
	/** `Borders: ČÚZK / geoBoundaries (CC BY 4.0)`: on screen whenever the map is drawn. */
	borders: string;
}

/** The weather: every configured city now and over the next days, and ČHMÚ's warnings. */
export interface WeatherSpec extends Loudness {
	template: 'weather';
	/**
	 * Seconds, a whole number of frames: the length of the break's weather slot.
	 * @minimum 10
	 * @maximum 30
	 * @default 15
	 */
	duration?: number;
	/** When the forecast was read, on the channel's clock. */
	asOf: ClockTime;
	/**
	 * @minItems 1
	 * @maxItems 20
	 */
	cities: WeatherCity[];
	/**
	 * Yellow, orange and red warnings in force now or later that day, the most severe first;
	 * empty on a quiet day, absent while ČHMÚ cannot be read.
	 */
	warnings?: WeatherWarning[];
	credits: WeatherCredits;
}

/** One story of the news. */
export interface NewsStory {
	/**
	 * The outlet's own headline in Czech, whole and unaltered: the outlets' terms forbid
	 * shortening, rewording or translating it. A no-break space follows one-letter words, so it
	 * wraps as Czech typesetting does; fit it, never cut it.
	 */
	headline: string;
	/**
	 * The outlet's name, shown with the headline as plain text, never as a logo: `ČT24`,
	 * `iROZHLAS`, `Seznam Zprávy`, `Novinky.cz`, `iDNES.cz`.
	 */
	outlet: string;
	/**
	 * How many outlets carry the story.
	 * @minimum 1
	 * @asType integer
	 */
	outlets: number;
	/** Among ČT24's most read of the day. */
	trending?: boolean;
}

/** The news: the top stories of the last hours, most important first. */
export interface NewsSpec extends Loudness {
	template: 'news';
	/**
	 * Seconds, a whole number of frames: the length of the break's news slot.
	 * @minimum 10
	 * @maximum 40
	 * @default 20
	 */
	duration?: number;
	/** When the headlines were read, on the channel's clock. */
	asOf: ClockTime;
	/**
	 * @minItems 1
	 * @maxItems 5
	 */
	stories: NewsStory[];
	/** The credit line, on screen throughout: `Headlines from the outlets’ RSS feeds, as of 17:40`. */
	credit: string;
}

export type ClipSpec =
	| IdentSpec
	| StingSpec
	| UpNextSpec
	| PromoSpec
	| EvergreenEndSpec
	| StripPromoSpec
	| ShortPromoSpec
	| LineupSpec
	| SqueezeSpec
	| StandbySpec
	| WeatherSpec
	| NewsSpec;

export interface BreakItem {
	/**
	 * Where the clip starts in the break, seconds (a whole number of frames).
	 * @minimum 0
	 */
	at: number;
	spec: ClipSpec;
}

/** A whole break's sound in one render: clips in sequence, each ringing out under the next. */
export interface BreakSpec extends Loudness {
	template: 'break';
	/** @minItems 1 */
	items: BreakItem[];
	/**
	 * Seconds the outgoing clip's sound fades under the incoming one.
	 * @minimum 0
	 * @maximum 2
	 * @default 0.3
	 */
	crossfade?: number;
}

export type Spec = ClipSpec | BreakSpec;

export type TemplateName = ClipSpec['template'];
