import { error } from '@sveltejs/kit';
import { core } from '$lib/core';
import { Metric, Period } from '$lib/generated/core';
import { features } from '$lib/features/settings/features.svelte';
import { leaderboardScreen, METRICS, PERIODS } from '$lib/features/ranks/api';

export async function load({ url, parent }) {
	// the flags arrive with the session, in the root layout's load
	await parent();
	if (!features.rankingsEnabled) error(404, 'Not found');

	const requested = url.searchParams.get('period') as Period;
	const period = PERIODS.includes(requested) ? requested : Period.All;
	const wanted = url.searchParams.get('metric') as Metric;
	const metric = METRICS.includes(wanted) ? wanted : Metric.Xp;

	// non-blocking; the period is the board the core loads, and a metric of a board it holds
	// sorts in place without a request
	const screen = leaderboardScreen(period, metric);
	core.prefetch(screen);
	return { period, metric, screen };
}
