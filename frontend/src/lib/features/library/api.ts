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
	posterId: number | null;
	backdropId: number | null;
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

export const getTitle = (id: number) =>
	api<{ title: Title; seasons?: Season[]; mediaFiles: MediaFile[]; artwork: ArtworkRef[] }>(
		`/admin/titles/${id}`
	);

export const updateTitle = (id: number, patch: TitlePatch) =>
	api<Title>(`/admin/titles/${id}`, { method: 'PATCH', body: patch });

export const deleteTitle = (id: number) => api<void>(`/admin/titles/${id}`, { method: 'DELETE' });

export const bulkTitles = (
	ids: number[],
	action: 'publish' | 'hide' | 'draft' | 'delete' | 'rescan'
) => api<void>('/admin/titles/bulk', { method: 'POST', body: { ids, action } });

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

export const applyTmdb = (titleId: number, tmdbId: number) =>
	api<{ jobId: number }>(`/admin/titles/${titleId}/metadata/apply`, {
		method: 'POST',
		body: { tmdbId }
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
	ownerId: number,
	kind: 'poster' | 'backdrop' | 'album_cover',
	file: File
) {
	const form = new FormData();
	form.set('ownerKind', ownerKind);
	form.set('ownerId', String(ownerId));
	form.set('kind', kind);
	form.set('file', file);
	return multipart<ArtworkRef>('/admin/artwork', form);
}

export const deleteArtwork = (id: number) =>
	api<void>(`/admin/artwork/${id}`, { method: 'DELETE' });

// subtitles
export interface SubtitleInfo {
	id: number;
	mediaFileId: number;
	lang: string;
	label: string;
	source: 'embedded' | 'uploaded';
	forced: boolean;
	createdAt: string;
}

export const listSubtitles = (mediaFileId: number) =>
	api<SubtitleInfo[]>(`/admin/media-files/${mediaFileId}/subtitles`);

export function uploadSubtitle(mediaFileId: number, lang: string, file: File) {
	const form = new FormData();
	form.set('lang', lang);
	form.set('file', file);
	return multipart<SubtitleInfo>(`/admin/media-files/${mediaFileId}/subtitles`, form);
}

export const deleteSubtitle = (id: number) =>
	api<void>(`/admin/subtitles/${id}`, { method: 'DELETE' });

// admin music (albums & tracks)
export interface AdminAlbumRow {
	id: number;
	name: string;
	year: number | null;
	artistId: number;
	artistName: string;
	coverId: number | null;
	trackCount: number;
	status: ContentStatus;
	sizeBytes: number;
	addedAt: string;
}

export const listAdminMusic = (query: { q?: string; sort?: string; page?: number }) =>
	api<{ items: AdminAlbumRow[]; total: number }>(`/admin/music${qs({ ...query })}`);

export const getAdminAlbum = (id: number) =>
	api<{ album: AdminAlbumRow; tracks: import('$lib/features/music/api').TrackItem[] }>(
		`/admin/albums/${id}`
	);

export const updateAlbum = (
	id: number,
	patch: Partial<{ name: string; year: number | null; status: string; artistName: string }>
) => api<{ album: AdminAlbumRow }>(`/admin/albums/${id}`, { method: 'PATCH', body: patch });

export const deleteAlbum = (id: number) => api<void>(`/admin/albums/${id}`, { method: 'DELETE' });

export const renameTrack = (id: number, name: string) =>
	api<void>(`/admin/tracks/${id}`, { method: 'PATCH', body: { name } });

export const deleteTrack = (id: number) => api<void>(`/admin/tracks/${id}`, { method: 'DELETE' });

// transcoding
export interface TranscodeVariant {
	id: number;
	mediaFileId: number;
	name: string;
	width: number;
	height: number;
	mode: 'copy' | 'transcode';
	status: 'queued' | 'processing' | 'ready' | 'failed';
	createdAt: string;
	completedAt: string | null;
}

export interface TranscodeInfo {
	detectedEncoders: string[];
	renditions: string[];
	settings: {
		hwAccel: string;
		ladder: string[];
		preset: string;
		maxConcurrent: number;
		jitEnabled: boolean | null;
		autoPrepare: boolean | null;
	};
}

export const transcodeInfo = () => api<TranscodeInfo>('/admin/transcode/info');

export const enqueueTranscode = (mediaFileId: number, variants?: string[]) =>
	api<{ queued: string[] }>(`/admin/media-files/${mediaFileId}/transcode`, {
		method: 'POST',
		body: { variants: variants ?? [] }
	});

export const listVariants = (mediaFileId: number) =>
	api<TranscodeVariant[]>(`/admin/media-files/${mediaFileId}/variants`);

export const deleteVariant = (id: number) =>
	api<void>(`/admin/transcode-variants/${id}`, { method: 'DELETE' });
