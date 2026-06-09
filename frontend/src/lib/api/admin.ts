import { api } from './client';
import type { Episode, LibraryRow, Season, Title, User } from './types';

function qs(params: Record<string, string | number | undefined>) {
	const search = new URLSearchParams();
	for (const [key, value] of Object.entries(params)) {
		if (value !== undefined && value !== '') search.set(key, String(value));
	}
	const s = search.toString();
	return s ? `?${s}` : '';
}

// library
export interface LibraryQuery {
	type?: string;
	status?: string;
	q?: string;
	sort?: string;
	page?: number;
}

export const library = (query: LibraryQuery) =>
	api<{ items: LibraryRow[]; total: number; page: number }>(`/admin/library${qs({ ...query })}`);

// titles
export interface TitleInput {
	kind: string;
	name: string;
	overview?: string;
	year?: number | null;
	contentRating?: string;
	runtimeMinutes?: number | null;
	genres?: string[];
}

export type TitlePatch = Partial<{
	name: string;
	sortName: string;
	overview: string;
	year: number | null;
	contentRating: string;
	runtimeMinutes: number | null;
	status: string;
	tmdbId: number | null;
	genres: string[];
}>;

export const createTitle = (input: TitleInput) =>
	api<Title>('/admin/titles', { method: 'POST', body: input });

export const getTitle = (id: number) =>
	api<{ title: Title; seasons?: Season[] }>(`/admin/titles/${id}`);

export const updateTitle = (id: number, patch: TitlePatch) =>
	api<Title>(`/admin/titles/${id}`, { method: 'PATCH', body: patch });

export const deleteTitle = (id: number) => api<void>(`/admin/titles/${id}`, { method: 'DELETE' });

export const bulkTitles = (ids: number[], action: 'publish' | 'hide' | 'draft' | 'delete') =>
	api<void>('/admin/titles/bulk', { method: 'POST', body: { ids, action } });

// seasons & episodes
export const createSeason = (titleId: number, seasonNumber: number, name = '') =>
	api<Season>(`/admin/titles/${titleId}/seasons`, {
		method: 'POST',
		body: { seasonNumber, name }
	});

export const deleteSeason = (id: number) => api<void>(`/admin/seasons/${id}`, { method: 'DELETE' });

export interface EpisodeInput {
	episodeNumber: number;
	name: string;
	overview?: string;
	runtimeMinutes?: number | null;
}

export const createEpisode = (seasonId: number, input: EpisodeInput) =>
	api<Episode>(`/admin/seasons/${seasonId}/episodes`, { method: 'POST', body: input });

export const updateEpisode = (id: number, patch: Partial<EpisodeInput>) =>
	api<Episode>(`/admin/episodes/${id}`, { method: 'PATCH', body: patch });

export const deleteEpisode = (id: number) =>
	api<void>(`/admin/episodes/${id}`, { method: 'DELETE' });

// users
export const listUsers = () => api<User[]>('/admin/users');

export const createUser = (input: {
	username: string;
	displayName?: string;
	password: string;
	role: string;
}) => api<User>('/admin/users', { method: 'POST', body: input });

export const updateUser = (
	id: number,
	patch: Partial<{ displayName: string; role: string; disabled: boolean; password: string }>
) => api<User>(`/admin/users/${id}`, { method: 'PATCH', body: patch });

export const deleteUser = (id: number) => api<void>(`/admin/users/${id}`, { method: 'DELETE' });

// settings
export type Settings = Record<string, unknown>;
export const getSettings = () => api<Settings>('/admin/settings');
export const putSettings = (patch: Settings) =>
	api<Settings>('/admin/settings', { method: 'PUT', body: patch });
