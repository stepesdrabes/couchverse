import type { medals, tiers } from '$lib/generated/tokens';

// Progression arrives as the core's view models (`RankView`, `ProfileDetail`,
// `LeaderboardView`); these name the codes the design tokens colour.

export type TierCode = keyof typeof tiers;
export type AchievementTier = keyof typeof medals;

/** An achievement as the core shows it, named apart from the `AchievementCard` component. */
export type { AchievementCard as Achievement } from '$lib/generated/core';
