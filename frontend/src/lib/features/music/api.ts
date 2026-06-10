import { api } from '$lib/api/client';

export interface AlbumCard {
	id: string;
	name: string;
	year: number | null;
	artistId: string;
	artistName: string;
	coverId: string | null;
	trackCount: number;
}

export interface TrackItem {
	id: string;
	albumId: string;
	discNumber: number;
	trackNumber: number;
	name: string;
	durationSeconds: number;
	trackArtist: string | null;
	mediaFileId: string | null;
	albumName: string;
	artistId: string;
	artistName: string;
	coverId: string | null;
}

export interface ArtistCard {
	id: string;
	name: string;
	albumCount: number;
}

export interface Playlist {
	id: string;
	userId: number;
	name: string;
	trackCount: number;
	coverId: string | null;
	createdAt: string;
	updatedAt: string;
}

export interface PlaylistEntry {
	entryId: string;
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

export const getAlbum = (id: string) =>
	api<{ album: AlbumCard; tracks: TrackItem[] }>(`/music/albums/${id}`);

export const getArtist = (id: string) =>
	api<{ artist: ArtistCard; albums: AlbumCard[] }>(`/music/artists/${id}`);

export const scrobble = (trackId: string) =>
	api<void>('/plays', { method: 'POST', body: { trackId } });

// playlists
export const listPlaylists = () => api<Playlist[]>('/me/playlists');

export const createPlaylist = (name: string) =>
	api<Playlist>('/me/playlists', { method: 'POST', body: { name } });

export const getPlaylist = (id: string) =>
	api<{ playlist: Playlist; entries: PlaylistEntry[] }>(`/me/playlists/${id}`);

export const renamePlaylist = (id: string, name: string) =>
	api<void>(`/me/playlists/${id}`, { method: 'PATCH', body: { name } });

export const deletePlaylist = (id: string) =>
	api<void>(`/me/playlists/${id}`, { method: 'DELETE' });

export const addPlaylistTrack = (playlistId: string, trackId: string) =>
	api<void>(`/me/playlists/${playlistId}/tracks`, { method: 'POST', body: { trackId } });

export const removePlaylistEntry = (playlistId: string, entryId: string) =>
	api<void>(`/me/playlists/${playlistId}/tracks/${entryId}`, { method: 'DELETE' });

export const reorderPlaylist = (playlistId: string, entryIds: string[]) =>
	api<void>(`/me/playlists/${playlistId}/order`, { method: 'PUT', body: { entryIds } });
