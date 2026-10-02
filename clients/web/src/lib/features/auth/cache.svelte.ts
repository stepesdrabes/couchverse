import { createSwrCache } from '$lib/api/cache.svelte';
import type { Device } from './api';

/** Keyed by username; the list reads the same in every display language. */
export const devicesCache = createSwrCache<Device[]>(1);
