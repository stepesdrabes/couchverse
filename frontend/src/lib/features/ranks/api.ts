import { api } from '$lib/api/client';
import { putPreferences } from '$lib/features/preferences/api';
import type { CheckResult, Leaderboard, Period, Profile } from './types';

export const getMyStats = () => api<Profile>('/me/stats');

export const getProfile = (username: string) =>
	api<Profile>(`/users/${encodeURIComponent(username)}/profile`);

/**
 * One payload carries every metric, so switching board costs no request and the
 * period is the only cache key.
 */
export const getLeaderboard = (period: Period) => api<Leaderboard>(`/leaderboard?period=${period}`);

export const checkAchievements = () =>
	api<CheckResult>('/me/achievements/check', { method: 'POST' });

/** The privacy switch rides the existing preferences blob, not a new endpoint. */
export const setPublicProfile = (on: boolean) => putPreferences({ publicProfile: on });
