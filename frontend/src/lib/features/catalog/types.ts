// Catalog entities shared by the user-facing app and the admin panel.

export type TitleKind = 'movie' | 'series';
export type ContentStatus = 'draft' | 'processing' | 'published' | 'hidden';

export interface Title {
	id: string;
	slug: string;
	kind: TitleKind;
	name: string;
	sortName: string;
	overview: string;
	year: number | null;
	releaseDate: string | null;
	contentRating: string;
	runtimeMinutes: number | null;
	status: ContentStatus;
	tmdbId: number | null;
	addedAt: string;
	updatedAt: string;
	genres: string[];
}

export interface Season {
	id: string;
	titleId: string;
	seasonNumber: number;
	name: string;
	overview: string;
	episodes: Episode[];
}

export interface Episode {
	id: string;
	seasonId: string;
	episodeNumber: number;
	name: string;
	overview: string;
	airDate: string | null;
	runtimeMinutes: number | null;
}

export interface Genre {
	id: number;
	name: string;
}

export interface CardItem {
	titleId: string;
	slug: string;
	kind: TitleKind;
	name: string;
	year: number | null;
	posterId: string | null;
	backdropId: string | null;
}

export interface ContinueItem extends CardItem {
	episodeId: string | null;
	episodeLabel: string;
	playbackKind: 'movie' | 'episode';
	playbackId: string;
	positionSeconds: number;
	durationSeconds: number;
	updatedAt: string;
}

export interface HomeRow {
	kind: 'continue_watching' | 'recently_added' | 'genre' | 'recently_played_music';
	label: string;
	items: CardItem[] | ContinueItem[] | import('$lib/features/music/api').AlbumCard[];
}

export interface HomeData {
	featured: Title | null;
	featuredBackdropId: string | null;
	featuredInList: boolean;
	rows: HomeRow[];
}

export interface ArtworkRef {
	id: string;
	ownerKind: string;
	ownerId: string;
	kind: 'poster' | 'backdrop' | 'thumb' | 'album_cover' | 'artist_photo';
	path: string;
	width: number;
	height: number;
	source: string;
	createdAt: string;
}

export interface EpisodeProgress {
	positionSeconds: number;
	durationSeconds: number;
	completed: boolean;
}

export interface TitleDetail {
	title: Title;
	inWatchlist: boolean;
	mediaFiles: MediaFile[];
	artwork: ArtworkRef[];
	seasons?: Season[];
	episodeProgress?: Record<string, EpisodeProgress>;
	progress?: EpisodeProgress;
}

export interface SearchHit {
	id: string;
	name: string;
	subtitle: string;
}

export interface SearchResults {
	titles: CardItem[];
	artists: SearchHit[];
	albums: SearchHit[];
	tracks: SearchHit[];
}

export interface MediaFile {
	id: string;
	libraryId: number;
	titleId: string | null;
	episodeId: string | null;
	trackId: string | null;
	path: string;
	sizeBytes: number;
	container: string;
	videoCodec: string;
	audioCodec: string;
	width: number;
	height: number;
	durationSeconds: number;
	bitrate: number;
	channels: number;
	sampleRate: number;
	videoRange: 'sdr' | 'hdr10' | 'hlg' | 'dv';
	directPlay: boolean;
	fileMtime: string | null;
	scannedAt: string | null;
	createdAt: string;
}
