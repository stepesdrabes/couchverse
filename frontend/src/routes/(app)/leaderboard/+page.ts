import { error } from '@sveltejs/kit';
import { features } from '$lib/features/settings/features.svelte';
import { getLeaderboard } from '$lib/features/ranks/api';
import { leaderboardCache } from '$lib/features/ranks/cache.svelte';
import type { Metric, Period } from '$lib/features/ranks/types';

const PERIODS: Period[] = ['all', 'month', 'week'];
const METRICS: Metric[] = ['xp', 'watch', 'music', 'achievements'];

export function load({ url }) {
	if (!features.rankingsEnabled) error(404, 'Not found');

	const requested = url.searchParams.get('period') as Period;
	const period = PERIODS.includes(requested) ? requested : 'all';
	const wanted = url.searchParams.get('metric') as Metric;
	const metric = METRICS.includes(wanted) ? wanted : 'xp';

	// period is the only cache key: one payload carries every metric, so the
	// metric switcher sorts in place rather than refetching
	return {
		period,
		metric,
		fresh: leaderboardCache.revalidate(period, () => getLeaderboard(period))
	};
}
