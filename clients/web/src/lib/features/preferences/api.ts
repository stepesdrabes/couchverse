import { api } from '$lib/api/client';

export interface SubtitleSettings {
	fontSizePct: number; // 50–200, default 100
	color: string; // hex
	fontFamily: 'sans' | 'serif' | 'mono' | 'rounded';
	backgroundOpacity: number; // 0–100
}

export interface Preferences {
	subtitles?: Partial<SubtitleSettings>;
	language?: string;
	/** appear on public profiles and leaderboards; absent means yes */
	publicProfile?: boolean;
	[key: string]: unknown;
}

// read before the session is known, so a visitor's 401 is no cause to redirect
export const getPreferences = () => api<Preferences>('/me/preferences', { skipAuthRedirect: true });

export const putPreferences = (patch: Preferences) =>
	api<Preferences>('/me/preferences', { method: 'PUT', body: patch });
