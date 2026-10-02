// Admin library curation: the library table, titles, seasons and episodes plus their
// metadata, artwork, subtitles and transcodes.
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
	adminUpdateTitle,
	adminUploadArtwork,
	adminUploadSubtitle
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
