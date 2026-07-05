import { createSwrCache } from '$lib/api/cache.svelte';
import type { getAlbum, getArtist, getPlaylist, MusicHome } from './api';

/** music home keyed by display language; detail pages keyed by their id */
export const musicHomeCache = createSwrCache<MusicHome>();
export const albumCache = createSwrCache<Awaited<ReturnType<typeof getAlbum>>>();
export const artistCache = createSwrCache<Awaited<ReturnType<typeof getArtist>>>();
export const playlistCache = createSwrCache<Awaited<ReturnType<typeof getPlaylist>>>();
