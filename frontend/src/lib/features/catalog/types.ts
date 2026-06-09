// Catalog entities shared by the user-facing app and the admin panel.

export type TitleKind = 'movie' | 'series';
export type ContentStatus = 'draft' | 'processing' | 'published' | 'hidden';

export interface Title {
	id: number;
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
	id: number;
	titleId: number;
	seasonNumber: number;
	name: string;
	overview: string;
	episodes: Episode[];
}

export interface Episode {
	id: number;
	seasonId: number;
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

export interface MediaFile {
	id: number;
	libraryId: number;
	titleId: number | null;
	episodeId: number | null;
	trackId: number | null;
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
