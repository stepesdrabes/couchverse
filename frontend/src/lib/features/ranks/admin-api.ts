import { api } from '$lib/api/client';
import type { AchievementCategory, AchievementTier, TierCode } from './types';

export interface RankRates {
	videoMinute: number;
	musicMinute: number;
	movie: number;
	episode: number;
	couchHost: number;
	couchJoin: number;
	bronze: number;
	silver: number;
	gold: number;
	platinum: number;
}

/** tier codes and colours are fixed; only the XP each one starts at is tunable */
export interface RankConfig {
	rates: RankRates;
	tiers: number[];
}

export interface AdminMember {
	userId: number;
	username: string;
	displayName: string;
	avatarId: string | null;
	level: number;
	tierCode: TierCode;
	xp: number;
	achievements: number;
	watchSeconds: number;
	musicSeconds: number;
	couchHosted: number;
	public: boolean;
}

export interface AchievementStat {
	code: string;
	category: AchievementCategory;
	tier: AchievementTier;
	unlocked: number;
	/** gated off by a feature flag right now */
	hidden: boolean;
}

export interface TierBucket {
	code: TierCode;
	level: number;
	minXp: number;
	colour: string;
	members: number;
}

export interface AdminRanksOverview {
	members: number;
	totalXp: number;
	totalUnlocks: number;
	averageLevel: number;
	tiers: TierBucket[];
	/** rarest first */
	achievements: AchievementStat[];
	rows: AdminMember[];
	config: RankConfig;
}

export const getAdminRanks = () => api<AdminRanksOverview>('/admin/ranks');

export const putRankConfig = (config: RankConfig) =>
	api<RankConfig>('/admin/ranks/config', { method: 'PUT', body: config });
