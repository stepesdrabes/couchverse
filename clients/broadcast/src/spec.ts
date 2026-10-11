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

/** A title as the templates show it. */
export interface Title {
	/** The name as shown (English where the title has it). */
	name: string;
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
	/** The full cut (11.5 s, before the prime-time movie) or the short one (5 s). */
	cut: 'full' | 'short';
}

/** A gap filler in the ident's language: the mark and the mnemonic. */
export interface StingSpec extends Loudness {
	template: 'sting';
	/**
	 * Seconds, a whole number of frames; the library is 2, 3 and 5 s and the hold absorbs
	 * anything in between.
	 * @minimum 2
	 * @maximum 6
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
	/** `expand`: the next picture grows to full frame; `line`: all collapses into the ident's horizon. */
	handover: 'expand' | 'line';
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
	| StandbySpec;

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
