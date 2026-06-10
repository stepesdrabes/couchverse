import type { StorageCategoryKind } from '$lib/features/jobs/api';

// Per-category colours for the storage bar segments and the breakdown table.
export const categoryStyle: Record<StorageCategoryKind, { label: string; color: string }> = {
	movies: { label: 'Movies', color: '#8b7cf0' }, // accent violet
	series: { label: 'Series', color: '#38bdf8' }, // sky
	music: { label: 'Music', color: '#f5b14c' }, // amber
	cache: { label: 'Cache', color: '#5b6072' } // faint grey
};
