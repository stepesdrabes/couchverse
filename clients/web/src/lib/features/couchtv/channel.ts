// Couch TV for viewers: the core's screens, the events the pages send and the maths behind
// the live overlays and the guide. Every time is Unix milliseconds, shown in the viewer's
// time zone on the channel's 24-hour clock.
import { core } from '$lib/core';
import {
	CouchTvItemKind,
	LoadStatus,
	type CouchTvEpisode,
	type CouchTvProgram,
	type CouchTvView,
	type PlayerView,
	type Surface
} from '$lib/generated/core';
import { langLabel } from '$lib/i18n/content-langs';
import * as m from '$lib/paraglide/messages';
import { formatDay } from './time';

/** The channel's name: a brand, the same in every language. */
export const CHANNEL_NAME = 'Couch TV';

export const COUCH_TV: Surface = { type: 'couchTv' };
export const COUCH_TV_GUIDE: Surface = { type: 'couchTvGuide' };
export const COUCH_TV_ON_NOW: Surface = { type: 'couchTvOnNow' };

/** Tunes in (signed in, or as the guest the channel cookie names) and plays the channel. */
export const tuneIn = () => core.send({ type: 'couchTvTuneRequested' });
/** Stops the channel's player and leaves the room. */
export const tuneOut = () => core.send({ type: 'couchTvClosed' });
export const react = (emoji: string) =>
	core.send({ type: 'couchTvReactionSent', content: { emoji } });
export const chooseAudio = (id: string) =>
	core.send({ type: 'couchTvAudioChosen', content: { id } });
export const chooseSubtitles = (id: string | null) =>
	core.send({ type: 'couchTvSubtitlesChosen', content: id ? { id } : {} });
/** Joins through the channel link under the name a guest typed. */
export const joinAsGuest = (code: string, name: string) =>
	core.send({ type: 'couchTvJoinRequested', content: { code, name } });

/** Reactions anyone can send in one press, before the full picker. */
export const QUICK_REACTIONS = ['😂', '🔥', '😮', '👏', '❤️'];

/** The picture's program date at `nowMs` on the core's clock: its last report, run on. */
export function pictureDate(view: CouchTvView, nowMs: number): number | undefined {
	if (view.programDateMs === undefined) return undefined;
	return view.programDateMs + Math.max(0, nowMs - view.programDateAtMs);
}

/** How far into an item `at` is, from 0 to 1. */
export function progressAt(item: { startsAtMs: number; endsAtMs: number }, at: number): number {
	const length = item.endsAtMs - item.startsAtMs;
	if (length <= 0) return 0;
	return Math.min(1, Math.max(0, (at - item.startsAtMs) / length));
}

const clocks = new Map<string, Intl.DateTimeFormat>();

/** A time of day on the channel's 24-hour clock, in the viewer's time zone. */
export function clock(ms: number, lang: string): string {
	let f = clocks.get(lang);
	if (!f) {
		f = new Intl.DateTimeFormat(lang, { hour: '2-digit', minute: '2-digit', hourCycle: 'h23' });
		clocks.set(lang, f);
	}
	return f.format(ms);
}

/** When an item airs: `20:15–22:05`. */
export const timeRange = (item: { startsAtMs: number; endsAtMs: number }, lang: string) =>
	m.couchtv_time_range({ start: clock(item.startsAtMs, lang), end: clock(item.endsAtMs, lang) });

/** An episode as a line under its series: `S1 E2 · Pilot`. */
export function episodeLabel(episode: CouchTvEpisode | undefined): string {
	if (!episode) return '';
	const numbers = { season: episode.season, episode: episode.number };
	return episode.name
		? m.couchtv_episode_named({ ...numbers, name: episode.name })
		: m.catalog_episode_short(numbers);
}

/** A badge for the rules worth pointing out on screen; empty for the rest. */
export function fillBadge(fill: string | undefined): string {
	switch (fill) {
		case 'premiere':
			return m.couchtv_fill_premiere();
		case 'saga':
			return m.couchtv_fill_saga();
		case 'marathon':
			return m.couchtv_fill_marathon();
		default:
			return '';
	}
}

export const isProgram = (item: CouchTvProgram | undefined): item is CouchTvProgram =>
	item?.kind === CouchTvItemKind.Program;

/**
 * The channel as the player's chrome shows it: what is on as its title, the channel's audio
 * and subtitle languages as its menus, linear (the channel decides what plays and when).
 */
export function livePlayerView(view: CouchTvView): PlayerView {
	const now = isProgram(view.now) ? view.now : undefined;
	const track = (t: { id: string; lang: string }) => ({ ...t, label: langLabel(t.lang) });
	return {
		status: LoadStatus.Loaded,
		title: now?.title ?? CHANNEL_NAME,
		subtitle: now ? episodeLabel(now.episode) : '',
		titleSlug: now?.titleSlug ?? '',
		backdrop: now?.backdrop,
		qualities: [],
		quality: '',
		audio: view.audio.map(track),
		audioSelected: view.audio.find((t) => t.selected)?.id,
		subtitles: view.subtitles.map(track),
		subtitleSelected: view.subtitles.find((t) => t.selected)?.id,
		seasons: [],
		shuffleAvailable: false,
		shuffle: false,
		linear: true
	};
}

/**
 * A broadcast day's name in the guide: its days come in order from today's, which runs until
 * the next morning's first program, so after midnight "Today" is still the evening before.
 */
export function dayName(date: string, index: number, lang: string): string {
	if (index === 0) return m.couchtv_today();
	if (index === 1) return m.couchtv_tomorrow();
	return formatDay(date, lang, 'long');
}

/** A guide row: one broadcast day, its programs placed on a timeline in rem from its start. */
export interface GuideRow {
	date: string;
	startsAtMs: number;
	endsAtMs: number;
	programs: { program: CouchTvProgram; left: number; width: number }[];
}

/** The guide's scale: how wide an hour is, in rem. */
export const HOUR_REM = 12;
const HOUR_MS = 3_600_000;

/** Places each day's programs on its row, clipped to the day. */
export function guideRows(
	days: { date: string; startsAtMs: number; endsAtMs: number; programs: CouchTvProgram[] }[]
): GuideRow[] {
	const rem = (ms: number) => (ms / HOUR_MS) * HOUR_REM;
	return days.map((day) => ({
		date: day.date,
		startsAtMs: day.startsAtMs,
		endsAtMs: day.endsAtMs,
		programs: day.programs.map((program) => {
			const start = Math.max(program.startsAtMs, day.startsAtMs);
			const end = Math.min(program.endsAtMs, day.endsAtMs);
			return { program, left: rem(start - day.startsAtMs), width: rem(Math.max(0, end - start)) };
		})
	}));
}

/** Where `at` falls on a row, in rem; undefined outside the day. */
export function guideOffset(row: { startsAtMs: number; endsAtMs: number }, at: number) {
	if (at < row.startsAtMs || at >= row.endsAtMs) return undefined;
	return ((at - row.startsAtMs) / HOUR_MS) * HOUR_REM;
}

/** The longest day's hours, so every row and the ruler span the same width. */
export function guideHours(rows: { startsAtMs: number; endsAtMs: number }[]): number {
	return Math.max(24, ...rows.map((r) => Math.ceil((r.endsAtMs - r.startsAtMs) / HOUR_MS)));
}
