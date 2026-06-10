import { api } from '$lib/api/client';

export interface AlbumCard {
	id: number;
	name: string;
	year: number | null;
	artistId: number;
	artistName: string;
	coverId: number | null;
	trackCount: number;
}

export interface TrackItem {
	id: number;
	albumId: number;
	discNumber: number;
	trackNumber: number;
	name: string;
	durationSeconds: number;
	trackArtist: string | null;
	mediaFileId: number | null;
	albumName: string;
	artistId: number;
	artistName: string;
	coverId: number | null;
}

export interface ArtistCard {
	id: number;
	name: string;
	albumCount: number;
}

export interface Playlist {
	id: number;
	userId: number;
	name: string;
	trackCount: number;
	coverId: number | null;
	createdAt: string;
	updatedAt: string;
}

export interface PlaylistEntry {
	entryId: number;
	position: number;
	track: TrackItem;
}

export interface MusicHome {
	recentAlbums: AlbumCard[];
	artists: ArtistCard[];
	playlists: Playlist[];
	recentlyPlayed: AlbumCard[];
}

export const musicHome = () => api<MusicHome>('/music');

export const getAlbum = (id: number) =>
	api<{ album: AlbumCard; tracks: TrackItem[] }>(`/music/albums/${id}`);

export const getArtist = (id: number) =>
	api<{ artist: ArtistCard; albums: AlbumCard[] }>(`/music/artists/${id}`);

export const scrobble = (trackId: number) =>
	api<void>('/plays', { method: 'POST', body: { trackId } });

// playlists
export const listPlaylists = () => api<Playlist[]>('/me/playlists');

export const createPlaylist = (name: string) =>
	api<Playlist>('/me/playlists', { method: 'POST', body: { name } });

export const getPlaylist = (id: number) =>
	api<{ playlist: Playlist; entries: PlaylistEntry[] }>(`/me/playlists/${id}`);

export const renamePlaylist = (id: number, name: string) =>
	api<void>(`/me/playlists/${id}`, { method: 'PATCH', body: { name } });

export const deletePlaylist = (id: number) =>
	api<void>(`/me/playlists/${id}`, { method: 'DELETE' });

export const addPlaylistTrack = (playlistId: number, trackId: number) =>
	api<void>(`/me/playlists/${playlistId}/tracks`, { method: 'POST', body: { trackId } });

export const removePlaylistEntry = (playlistId: number, entryId: number) =>
	api<void>(`/me/playlists/${playlistId}/tracks/${entryId}`, { method: 'DELETE' });

export const reorderPlaylist = (playlistId: number, entryIds: number[]) =>
	api<void>(`/me/playlists/${playlistId}/order`, { method: 'PUT', body: { entryIds } });
