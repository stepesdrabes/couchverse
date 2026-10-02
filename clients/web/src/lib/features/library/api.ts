// Admin library curation: the library table, titles, seasons and episodes plus their
// metadata, artwork, subtitles and transcodes.
import type { Artwork, Subtitle } from '$lib/generated/api';

export {
	adminApplyMetadata,
	adminBulkTitles,
	adminCreateEpisode,
	adminCreateSeason,
	adminCreateTitle,
	adminDeleteArtwork,
	adminDeleteEpisode,
	// hard-deletes a media file: its source, caches and subtitles on disk plus the row
	adminDeleteMediaFile,
	adminDeleteSeason,
	adminDeleteSubtitle,
	adminDeleteTitle,
	// drops the language's translations across the title, seasons and episodes, promoting
	// the next language to base when the base one goes; 400 on the last language
	adminDeleteTitleLanguage,
	adminDeleteVariant,
	adminEnqueueTranscode,
	adminGetEpisodeTranslations,
	adminGetTitle,
	adminGetTitleStorage,
	adminGetTranscodeInfo,
	adminImportEpisodes,
	adminListLibrary,
	adminListMetadataSeasons,
	adminListVariants,
	adminSearchMetadata,
	adminSetEpisodeTranslation,
	adminSetTitleTranslation,
	adminUpdateEpisode,
	adminUpdateMediaFile,
	adminUpdateTitle
} from '$lib/generated/api';

export type {
	AdminListLibrarySort,
	AdminListLibraryStatus,
	AdminListLibraryType,
	AdminSearchMetadataKind,
	AdminTitle,
	Artwork,
	Episode,
	LibraryRow,
	MediaFile,
	MediaFileAudioAudioRole,
	Season,
	Subtitle,
	Title,
	TitleInputKind,
	TitleStorageBreakdown,
	TitleUpdate,
	TitleUpdateStatus,
	TmdbSearchResult,
	TmdbSeason,
	TranscodeInfo,
	TranscodeVariant,
	Translation
} from '$lib/generated/api';

// The generated client only speaks JSON, so the multipart uploads stay hand-written.
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
	kind: 'poster' | 'backdrop',
	file: File
) {
	const form = new FormData();
	form.set('ownerKind', ownerKind);
	form.set('ownerId', ownerId);
	form.set('kind', kind);
	form.set('file', file);
	return multipart<Artwork>('/admin/artwork', form);
}

export function uploadSubtitle(mediaFileId: string, lang: string, file: File) {
	const form = new FormData();
	form.set('lang', lang);
	form.set('file', file);
	return multipart<Subtitle>(`/admin/media-files/${mediaFileId}/subtitles`, form);
}
