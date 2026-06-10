// Admin library curation: the library table, titles, seasons and episodes.
import { api, qs } from '$lib/api/client';
import type {
	ArtworkRef,
	ContentStatus,
	Episode,
	MediaFile,
	Season,
	Title,
	TitleKind
} from '$lib/features/catalog/types';

export interface LibraryRow {
	id: string;
	slug: string;
	kind: TitleKind;
	name: string;
	year: number | null;
	status: ContentStatus;
	seasonCount: number;
	episodeCount: number;
	sizeBytes: number;
	maxHeight: number;
	hdr: boolean;
	posterId: string | null;
	backdropId: string | null;
	needsPrepare: boolean;
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

export const getTitle = (id: string) =>
	api<{
		title: Title;
		seasons?: Season[];
		mediaFiles: MediaFile[];
		artwork: ArtworkRef[];
		subtitlesByFile: Record<string, SubtitleInfo[]>;
	}>(`/admin/titles/${id}`);

export const updateTitle = (id: string, patch: TitlePatch) =>
	api<Title>(`/admin/titles/${id}`, { method: 'PATCH', body: patch });

export const deleteTitle = (id: string) => api<void>(`/admin/titles/${id}`, { method: 'DELETE' });

export const bulkTitles = (
	ids: string[],
	action: 'publish' | 'hide' | 'draft' | 'delete' | 'rescan'
) => api<void>('/admin/titles/bulk', { method: 'POST', body: { ids, action } });

export const createSeason = (titleId: string, seasonNumber: number, name = '') =>
	api<Season>(`/admin/titles/${titleId}/seasons`, {
		method: 'POST',
		body: { seasonNumber, name }
	});

export const deleteSeason = (id: string) => api<void>(`/admin/seasons/${id}`, { method: 'DELETE' });

export interface EpisodeInput {
	episodeNumber: number;
	name: string;
	overview?: string;
	runtimeMinutes?: number | null;
}

export const createEpisode = (seasonId: string, input: EpisodeInput) =>
	api<Episode>(`/admin/seasons/${seasonId}/episodes`, { method: 'POST', body: input });

export const updateEpisode = (id: string, patch: Partial<EpisodeInput>) =>
	api<Episode>(`/admin/episodes/${id}`, { method: 'PATCH', body: patch });

export const deleteEpisode = (id: string) =>
	api<void>(`/admin/episodes/${id}`, { method: 'DELETE' });

// TMDB metadata
export interface TmdbResult {
	tmdbId: number;
	name: string;
	year: number;
	overview: string;
	posterUrl: string;
}

export const searchTmdb = (q: string, kind: TitleKind) =>
	api<TmdbResult[]>(`/admin/metadata/search${qs({ q, kind })}`);

export const applyTmdb = (titleId: string, tmdbId: number) =>
	api<{ jobId: number }>(`/admin/titles/${titleId}/metadata/apply`, {
		method: 'POST',
		body: { tmdbId }
	});

export interface TmdbSeasonPreview {
	seasonNumber: number;
	name: string;
	overview: string;
	episodeCount: number;
}

export const getTmdbSeasons = (titleId: string) =>
	api<TmdbSeasonPreview[]>(`/admin/titles/${titleId}/metadata/seasons`);

export const importEpisodes = (titleId: string, seasons?: number[]) =>
	api<{ jobId: number }>(`/admin/titles/${titleId}/metadata/import-episodes`, {
		method: 'POST',
		body: { seasons: seasons ?? [] }
	});

// artwork
async function multipart<T>(path: string, form: FormData): Promise<T> {
	const res = await fetch(`/api/v1${path}`, {
		method: 'POST',
		body: form,
		credentials: 'same-origin'
	});
	const data = await res.json().catch(() => null);
	if (!res.ok) throw new Error(data?.error?.message ?? 'upload failed');
	return data as T;
}

export function uploadArtwork(
	ownerKind: string,
	ownerId: string,
	kind: 'poster' | 'backdrop' | 'album_cover',
	file: File
) {
	const form = new FormData();
	form.set('ownerKind', ownerKind);
	form.set('ownerId', ownerId);
	form.set('kind', kind);
	form.set('file', file);
	return multipart<ArtworkRef>('/admin/artwork', form);
}

export const deleteArtwork = (id: string) =>
	api<void>(`/admin/artwork/${id}`, { method: 'DELETE' });

// subtitles
export interface SubtitleInfo {
	id: string;
	mediaFileId: string;
	lang: string;
	label: string;
	source: 'embedded' | 'uploaded';
	forced: boolean;
	createdAt: string;
}

export const listSubtitles = (mediaFileId: string) =>
	api<SubtitleInfo[]>(`/admin/media-files/${mediaFileId}/subtitles`);

export function uploadSubtitle(mediaFileId: string, lang: string, file: File) {
	const form = new FormData();
	form.set('lang', lang);
	form.set('file', file);
	return multipart<SubtitleInfo>(`/admin/media-files/${mediaFileId}/subtitles`, form);
}

export const deleteSubtitle = (id: string) =>
	api<void>(`/admin/subtitles/${id}`, { method: 'DELETE' });

// admin music (albums & tracks)
export interface AdminAlbumRow {
	id: string;
	name: string;
	year: number | null;
	artistId: string;
	artistName: string;
	coverId: string | null;
	trackCount: number;
	status: ContentStatus;
	sizeBytes: number;
	addedAt: string;
}

export const listAdminMusic = (query: { q?: string; sort?: string; page?: number }) =>
	api<{ items: AdminAlbumRow[]; total: number }>(`/admin/music${qs({ ...query })}`);

export const getAdminAlbum = (id: string) =>
	api<{ album: AdminAlbumRow; tracks: import('$lib/features/music/api').TrackItem[] }>(
		`/admin/albums/${id}`
	);

export const updateAlbum = (
	id: string,
	patch: Partial<{ name: string; year: number | null; status: string; artistName: string }>
) => api<{ album: AdminAlbumRow }>(`/admin/albums/${id}`, { method: 'PATCH', body: patch });

export const deleteAlbum = (id: string) => api<void>(`/admin/albums/${id}`, { method: 'DELETE' });

export const renameTrack = (id: string, name: string) =>
	api<void>(`/admin/tracks/${id}`, { method: 'PATCH', body: { name } });

export const deleteTrack = (id: string) => api<void>(`/admin/tracks/${id}`, { method: 'DELETE' });

// transcoding
export interface TranscodeVariant {
	id: string;
	mediaFileId: string;
	name: string;
	width: number;
	height: number;
	mode: 'copy' | 'transcode';
	status: 'queued' | 'processing' | 'ready' | 'failed';
	sizeBytes: number;
	createdAt: string;
	completedAt: string | null;
}

export interface TranscodeInfo {
	detectedEncoders: string[];
	detecting: boolean;
	renditions: string[];
	settings: {
		hwAccel: string;
		ladder: string[];
		preset: string;
		maxConcurrent: number;
		jitEnabled: boolean | null;
		autoPrepare: boolean | null;
		deleteSourceAfterTranscode: boolean | null;
	};
}

export const transcodeInfo = () => api<TranscodeInfo>('/admin/transcode/info');

export const enqueueTranscode = (mediaFileId: string, variants?: string[]) =>
	api<{ queued: string[] }>(`/admin/media-files/${mediaFileId}/transcode`, {
		method: 'POST',
		body: { variants: variants ?? [] }
	});

export const listVariants = (mediaFileId: string) =>
	api<TranscodeVariant[]>(`/admin/media-files/${mediaFileId}/variants`);

export const deleteVariant = (id: string) =>
	api<void>(`/admin/transcode-variants/${id}`, { method: 'DELETE' });
