import { core } from '$lib/core';
import { Metric, Period, type Surface } from '$lib/generated/core';

// Ranks and profiles come from the core: these name its screens and send its events.

export const RANK: Surface = { type: 'rank' };

export const profileScreen = (username: string): Surface => ({
	type: 'profile',
	content: username
});

/** One payload carries every metric, so switching board costs no request. */
export const leaderboardScreen = (period: Period, metric: Metric): Surface => ({
	type: 'leaderboard',
	content: { period, metric }
});

export const PERIODS = [Period.All, Period.Month, Period.Week];
export const METRICS = [Metric.Xp, Metric.Watch, Metric.Achievements];

/** Asks the server whether anything new was earned; the core throttles it unless forced. */
export const checkAchievements = (force = false) =>
	core.send({ type: 'achievementsCheckRequested', content: { force } });

/** Shows the profile on leaderboards and public pages or hides it; the core rolls back on failure. */
export const setPublicProfile = (visible: boolean) =>
	core.send({ type: 'profileVisibilityChanged', content: { public: visible } });
