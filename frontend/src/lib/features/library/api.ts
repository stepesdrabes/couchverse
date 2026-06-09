// Admin library curation: the library table, titles, seasons and episodes.
import { api, qs } from '$lib/api/client';
import type {
	ContentStatus,
	Episode,
	MediaFile,
	Season,
	Title,
	TitleKind
} from '$lib/features/catalog/types';

export interface LibraryRow {
	id: number;
	kind: TitleKind;
	name: string;
	year: number | null;
	status: ContentStatus;
	seasonCount: number;
	episodeCount: number;
	sizeBytes: number;
	maxHeight: number;
	hdr: boolean;
	addedAt: string;
}

export interface LibraryQuery {
	type?: string;
	status?: string;
	q?: string;
	sort?: string;
	page?: number;
}

export const listLibrary = (query: LibraryQuery) =>
	api<{ items: LibraryRow[]; total: number; page: number }>(`/admin/library${qs({ ...query })}`);

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
	api<{ title: Title; seasons?: Season[]; mediaFiles: MediaFile[] }>(`/admin/titles/${id}`);

export const updateTitle = (id: number, patch: TitlePatch) =>
	api<Title>(`/admin/titles/${id}`, { method: 'PATCH', body: patch });

export const deleteTitle = (id: number) => api<void>(`/admin/titles/${id}`, { method: 'DELETE' });

export const bulkTitles = (ids: number[], action: 'publish' | 'hide' | 'draft' | 'delete') =>
	api<void>('/admin/titles/bulk', { method: 'POST', body: { ids, action } });

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
