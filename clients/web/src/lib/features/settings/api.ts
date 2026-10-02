import { api } from '$lib/api/client';

export { adminGetSettings, adminUpdateSettings } from '$lib/generated/api';
export type { ServerSettings } from '$lib/generated/api';

export interface FeatureFlags {
	couchEnabled: boolean;
	rankingsEnabled: boolean;
}

export const getFeatures = () => api<FeatureFlags>('/features');

export interface ThemeInfo {
	accent: string;
}

export const getTheme = () => api<ThemeInfo>('/theme');
