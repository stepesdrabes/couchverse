// The channel's closed sets in the display language.
import type {
	CouchTvAiringFill,
	CouchTvAiringRender,
	CouchTvAiringState,
	CouchTvSlotPick,
	CouchTvSoundtrackProvider,
	CouchTvTitleMood,
	MediaMarkerKind
} from '$lib/generated/api';
import * as m from '$lib/paraglide/messages';
import type { Day } from './template';

const fills: Record<CouchTvAiringFill, () => string> = {
	strip: m.admin_couchtv_fill_strip,
	marathon: m.admin_couchtv_fill_marathon,
	reruns: m.admin_couchtv_fill_reruns,
	movie: m.admin_couchtv_fill_movie,
	matinee: m.admin_couchtv_fill_matinee,
	premiere: m.admin_couchtv_fill_premiere,
	saga: m.admin_couchtv_fill_saga,
	encore: m.admin_couchtv_fill_encore,
	filler: m.admin_couchtv_fill_filler
};
export const fillLabel = (fill: CouchTvAiringFill | undefined) => (fill ? fills[fill]() : '');

const picks: Record<CouchTvSlotPick, () => string> = {
	any: m.admin_couchtv_pick_any,
	comedy: m.admin_couchtv_pick_comedy,
	animation: m.admin_couchtv_pick_animation,
	drama: m.admin_couchtv_pick_drama,
	family: m.admin_couchtv_pick_family
};
export const pickLabel = (pick: CouchTvSlotPick) => picks[pick]();

const renders: Record<CouchTvAiringRender, () => string> = {
	uncomposed: m.admin_couchtv_render_uncomposed,
	pending: m.admin_couchtv_render_composed,
	composed: m.admin_couchtv_render_composed,
	rendering: m.admin_couchtv_render_rendering,
	ready: m.admin_couchtv_render_ready,
	failed: m.admin_couchtv_render_failed
};
export const renderLabel = (render: CouchTvAiringRender | undefined) =>
	renders[render ?? 'uncomposed']();

/** A break's colour in the grid by how far its render is. */
export const renderColor: Record<CouchTvAiringRender, string> = {
	uncomposed: 'var(--color-edge)',
	pending: 'var(--color-glow-violet)',
	composed: 'var(--color-glow-violet)',
	rendering: 'var(--color-glow-teal)',
	ready: 'var(--color-success)',
	failed: 'var(--color-danger)'
};

const states: Record<CouchTvAiringState, () => string> = {
	planned: m.admin_couchtv_state_planned,
	ready: m.admin_couchtv_state_ready,
	aired: m.admin_couchtv_state_aired,
	failed: m.admin_couchtv_state_failed,
	replaced: m.admin_couchtv_state_replaced
};
export const stateLabel = (state: CouchTvAiringState) => states[state]();

const moods: Record<CouchTvTitleMood, () => string> = {
	cinematic: m.admin_couchtv_mood_cinematic,
	grit: m.admin_couchtv_mood_grit,
	bright: m.admin_couchtv_mood_bright
};
export const moodLabel = (mood: CouchTvTitleMood) => moods[mood]();

const markers: Record<MediaMarkerKind, () => string> = {
	intro: m.admin_couchtv_marker_intro,
	credits: m.admin_couchtv_marker_credits,
	credits_music: m.admin_couchtv_marker_credits_music
};
export const markerLabel = (kind: MediaMarkerKind) => markers[kind]();

const catalogs: Record<CouchTvSoundtrackProvider, () => string> = {
	itunes: m.admin_couchtv_soundtrack_catalog_itunes,
	deezer: m.admin_couchtv_soundtrack_catalog_deezer
};
export const catalogLabel = (catalog: CouchTvSoundtrackProvider) => catalogs[catalog]();

const dayIndex: Record<Day, number> = { mon: 0, tue: 1, wed: 2, thu: 3, fri: 4, sat: 5, sun: 6 };

/** A template day's name in the display language (2024-01-01 was a Monday). */
export function dayLabel(day: Day, lang: string, style: 'short' | 'long' = 'short'): string {
	return new Intl.DateTimeFormat(lang, { weekday: style, timeZone: 'UTC' }).format(
		Date.UTC(2024, 0, 1 + dayIndex[day])
	);
}

/** An episode's place as S01E02. */
export function episodeCode(season?: number, episode?: number): string {
	if (!season && !episode) return '';
	return `S${String(season ?? 0).padStart(2, '0')}E${String(episode ?? 0).padStart(2, '0')}`;
}
