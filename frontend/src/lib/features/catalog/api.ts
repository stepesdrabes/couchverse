import { api, qs } from '$lib/api/client';
import type { CardItem, Genre, HomeData, SearchResults, TitleDetail } from './types';

export const listGenres = () => api<Genre[]>('/genres');

export const home = () => api<HomeData>('/home');

export interface BrowseQuery {
	kind?: string;
	genre?: string;
	q?: string;
	sort?: string;
	page?: number;
}

export const browse = (query: BrowseQuery) =>
	api<{ items: CardItem[]; total: number }>(`/titles${qs({ ...query })}`);

export const getTitle = (slug: string) => api<TitleDetail>(`/titles/${slug}`);

export const search = (q: string, signal?: AbortSignal) =>
	api<SearchResults>(`/search${qs({ q })}`, { signal });

export const myList = () => api<CardItem[]>('/me/watchlist');

export const addToList = (titleId: string) =>
	api<void>(`/me/watchlist/${titleId}`, { method: 'PUT' });

export const removeFromList = (titleId: string) =>
	api<void>(`/me/watchlist/${titleId}`, { method: 'DELETE' });

/**
 * Build an artwork URL. Pass `v` (the artwork's version token) to opt into
 * immutable browser caching - it busts automatically when the art is replaced.
 * `size` requests a resized variant (e.g. 'w342').
 */
export const artworkUrl = (id: string, v?: number | string | null, size?: string) => {
	const params = new URLSearchParams();
	if (size) params.set('size', size);
	if (v) params.set('v', String(v));
	const query = params.toString();
	return `/api/v1/artwork/${id}${query ? `?${query}` : ''}`;
};

/** Version token (unix seconds) for a full artwork object's `createdAt`. */
export const artworkVer = (createdAt: string) => Math.floor(new Date(createdAt).getTime() / 1000);
