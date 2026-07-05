import { createSwrCache } from '$lib/api/cache.svelte';
import type { CardItem, HomeData, TitleDetail } from './types';

/** title detail keyed by slug; Home keyed by display language */
export const titleCache = createSwrCache<TitleDetail>();
export const homeCache = createSwrCache<HomeData>();
/** browse grid keyed by `kind|genre|sort` */
export const browseCache = createSwrCache<CardItem[]>();
