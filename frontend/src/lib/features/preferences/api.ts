import { api } from '$lib/api/client';

export interface SubtitleSettings {
	fontSizePct: number; // 50–200, default 100
	color: string; // hex
	fontFamily: 'sans' | 'serif' | 'mono' | 'rounded';
	backgroundOpacity: number; // 0–100
}

export interface Preferences {
	subtitles?: Partial<SubtitleSettings>;
	[key: string]: unknown;
}

export const getPreferences = () => api<Preferences>('/me/preferences');

export const putPreferences = (patch: Preferences) =>
	api<Preferences>('/me/preferences', { method: 'PUT', body: patch });
