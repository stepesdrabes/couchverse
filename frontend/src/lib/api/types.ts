export interface User {
	id: number;
	username: string;
	displayName: string;
	role: 'admin' | 'member';
	disabled: boolean;
	createdAt: string;
}

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
