import { api } from '$lib/api/client';
import { core } from '$lib/core';
import { getArtworkPath, type GetArtworkSize } from '$lib/generated/api';
import { LoadStatus, type BrowseKey, type Surface } from '$lib/generated/core';
import type { Genre } from './types';

// The viewer's catalog comes from the core: these name its screens, and a page's load starts
// loading its screen without waiting for it (optimistic navigation).

export const homeScreen: Surface = { type: 'home' };
export const genresScreen: Surface = { type: 'genres' };
export const myListScreen: Surface = { type: 'myList' };
export const searchScreen: Surface = { type: 'search' };
export const titleScreen = (slug: string): Surface => ({ type: 'title', content: slug });
export const browseScreen = (key: BrowseKey): Surface => ({ type: 'browse', content: key });

/**
 * A load for a screen that shows what changes elsewhere (progress, My List): reloads it even
 * when the core holds it as fresh, as every visit has.
 */
export function revisit(screen: Surface): Surface {
	core.revalidate(screen);
	return screen;
}

/** A load for a screen that seldom changes: loads it unless the core holds it as fresh. */
export function preload(screen: Surface): Surface {
	core.prefetch(screen);
	return screen;
}

/** `preload` for a listing, whose page then lets the visitor change its sort and genre. */
export function preloadListing(key: BrowseKey): BrowseKey {
	core.prefetch(browseScreen(key));
	return key;
}

/** A view's content, once there is any to show (stale beats blank). */
export function shown<V extends { status: LoadStatus }>(view: V | undefined): V | undefined {
	return view?.status === LoadStatus.Loaded || view?.status === LoadStatus.Stale ? view : undefined;
}

/** Adds a title to My List or removes it, at once; the core rolls back and says so on failure. */
export const setListed = (titleId: string, listed: boolean) =>
	core.send({ type: 'watchlistChanged', content: { titleId, listed } });

export const browseMore = (key: BrowseKey) =>
	core.send({ type: 'browseMoreRequested', content: key });

/** Every keystroke; the core waits for a pause and drops answers the user typed past. */
export const searchFor = (query: string) =>
	core.send({ type: 'searchChanged', content: { query } });

/** The genres the admin home-row editor offers. */
export const listGenres = () => api<Genre[]>('/genres');

// A couch guest without an account has no session cookie; the couch hands out
// an artwork grant that every artwork URL then carries.
let artworkGrant: string | null = null;
export const useArtworkGrant = (grant: string) => {
	artworkGrant = grant;
};

/**
 * Build an artwork URL. Pass `v` (the artwork's version token) to opt into
 * immutable browser caching - it busts automatically when the art is replaced.
 * `size` requests a resized variant (e.g. 'w342'). Catalog views come with theirs ready.
 */
export const artworkUrl = (id: string, v?: number | string | null, size?: GetArtworkSize) =>
	getArtworkPath(id, { size, v: v ? String(v) : undefined, g: artworkGrant ?? undefined });

/** Version token (unix seconds) for a full artwork object's `createdAt`. */
export const artworkVer = (createdAt: string) => Math.floor(new Date(createdAt).getTime() / 1000);
