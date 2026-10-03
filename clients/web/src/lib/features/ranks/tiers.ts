import { medals, tiers } from '$lib/generated/tokens';
import type { AchievementTier, TierCode } from './types';

/**
 * Rank colours are the storage/status palette re-spent as a ladder, so ranks read
 * as part of the same system rather than a bolted-on theme. The palette comes from
 * the shared design tokens, so a tier is coloured before any payload has arrived
 * (the nav ring on a cold load) and matches the native apps.
 */
export const RANK_TIERS: Record<TierCode, { color: string; glow: string }> = tiers;

// codes arrive from the server as strings; a tier this build does not know is a rookie
const rankTier = (code: string) => RANK_TIERS[code as TierCode] ?? RANK_TIERS.rookie;
export const rankColor = (code: string) => rankTier(code).color;
export const rankGlow = (code: string) => rankTier(code).glow;

/** Medal metals. `from`/`to` make the gradient, `ring` is the readable accent. */
export const ACHIEVEMENT_TIERS: Record<
	AchievementTier,
	{ from: string; to: string; ring: string; glow: string }
> = medals;

export const medal = (tier: string) =>
	ACHIEVEMENT_TIERS[tier as AchievementTier] ?? ACHIEVEMENT_TIERS.bronze;

/** Podium places reuse the medals, so one palette serves both surfaces. */
export const PODIUM = [
	ACHIEVEMENT_TIERS.gold,
	ACHIEVEMENT_TIERS.silver,
	ACHIEVEMENT_TIERS.bronze
] as const;

/** XP sources borrow the storage categories so the same thing is the same colour. */
export const XP_SOURCE_COLORS: Record<string, string> = {
	video: 'var(--color-accent)',
	movies: '#38bdf8',
	episodes: '#8b7cf0',
	couchHosted: '#c084fc',
	couchJoined: '#a78bfa',
	achievements: '#34d399'
};

export const xpSourceColor = (key: string) => XP_SOURCE_COLORS[key] ?? 'var(--color-faint)';

/** One fill per heatmap level (0 to 4) the core gives a day. */
export const HEAT_FILLS = [
	'var(--color-surface-2)',
	'color-mix(in srgb, var(--color-accent) 22%, var(--color-surface-2))',
	'color-mix(in srgb, var(--color-accent) 45%, var(--color-surface-2))',
	'color-mix(in srgb, var(--color-accent) 70%, var(--color-surface-2))',
	'var(--color-accent)'
];
