import { api } from '$lib/api/client';

export type Settings = Record<string, unknown>;

export const getSettings = () => api<Settings>('/admin/settings');

export const putSettings = (patch: Settings) =>
	api<Settings>('/admin/settings', { method: 'PUT', body: patch });

export interface FeatureFlags {
	musicEnabled: boolean;
}

export const getFeatures = () => api<FeatureFlags>('/features');
