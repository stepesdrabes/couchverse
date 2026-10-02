import { createSwrCache } from '$lib/api/cache.svelte';
import type { Leaderboard, Profile } from './types';

/**
 * Keyed by `username|lang` because title names and the favourite-genre label are
 * localized server-side, the same reasoning as the catalog caches. Your own
 * `/profile` progress tab reads this very entry, so the hop to your public page
 * and back is instant in both directions.
 */
export const profileCache = createSwrCache<Profile>();

/** Keyed by period alone: one payload carries every metric. */
export const leaderboardCache = createSwrCache<Leaderboard>(8);
