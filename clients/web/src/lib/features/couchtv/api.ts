// Couch TV admin data: the channel's config, guest link and schedule, render health and
// per-title tools, plus the library reads the broadcast preview and the pickers use. Viewers
// watch through the core (channel.ts).
export {
	adminAnalyzeCouchTvTitle,
	adminCouchTvRenderHealth,
	// a soundtrack candidate's preview, played from the server's copy
	adminCouchTvSoundtrackPreviewPath,
	adminCouchTvWaveform,
	adminGetCouchTv,
	adminGetCouchTvTitle,
	adminGetMediaMarkers,
	adminGetTitle,
	adminInsertAiring,
	adminListAirings,
	adminListLibrary,
	adminRegenerateDay,
	adminRotateCouchTvLink,
	adminSearchCouchTvSoundtracks,
	adminSetCouchTvLink,
	adminUpdateAiring,
	adminUpdateCouchTv,
	adminUpdateCouchTvTitle,
	adminUpdateMediaMarkers,
	getStreamFramePath,
	// a media grant for a title's file: the stills and the promo audio play through it
	resolvePlayback
} from '$lib/generated/api';
export type {
	AdminTitle,
	Artwork,
	CouchTvAiring,
	CouchTvAiringEdit,
	CouchTvConfig,
	CouchTvGuestLink,
	CouchTvOverview,
	CouchTvRenderHealth,
	CouchTvSchedule,
	CouchTvShot,
	CouchTvSlotFill,
	CouchTvSlotPick,
	CouchTvSoundtrack,
	CouchTvTitle,
	CouchTvTitleUpdate,
	CouchTvTrailerCut,
	CouchTvWaveform,
	LibraryRow,
	MediaMarker,
	MediaMarkerEdit,
	MediaMarkerKind,
	MediaMarkers,
	PlaybackInfo
} from '$lib/generated/api';
