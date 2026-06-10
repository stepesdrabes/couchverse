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

export const artworkUrl = (id: string) => `/api/v1/artwork/${id}`;
