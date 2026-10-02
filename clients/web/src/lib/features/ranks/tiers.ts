import { medals, tiers } from '$lib/generated/tokens';
import type { AchievementTier, TierCode } from './types';

/**
 * Rank colours are the storage/status palette re-spent as a ladder, so ranks read
 * as part of the same system rather than a bolted-on theme. The palette comes from
 * the shared design tokens, so a tier is coloured before any payload has arrived
 * (the nav ring on a cold load) and matches the native apps.
 */
export const RANK_TIERS: Record<TierCode, { color: string; glow: string }> = tiers;

export const rankColor = (code: TierCode) => RANK_TIERS[code]?.color ?? RANK_TIERS.rookie.color;
export const rankGlow = (code: TierCode) => RANK_TIERS[code]?.glow ?? RANK_TIERS.rookie.glow;

/** Medal metals. `from`/`to` make the gradient, `ring` is the readable accent. */
export const ACHIEVEMENT_TIERS: Record<
	AchievementTier,
	{ from: string; to: string; ring: string; glow: string }
> = medals;

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

/**
 * Heatmap buckets come from the quartiles of this user's own active days: fixed
 * second-thresholds would leave a light viewer's whole year at level 1.
 */
export function heatThresholds(days: number[]): number[] {
	const active = days.filter((d) => d > 0).sort((a, b) => a - b);
	if (active.length === 0) return [0, 0, 0, 0];
	const at = (q: number) => active[Math.min(active.length - 1, Math.floor(active.length * q))];
	return [1, at(0.25), at(0.5), at(0.75)];
}

export function heatLevel(seconds: number, thresholds: number[]): number {
	if (seconds <= 0) return 0;
	let level = 1;
	for (let i = 1; i < thresholds.length; i++) {
		if (seconds >= thresholds[i]) level = i + 1;
	}
	return level;
}

export const HEAT_FILLS = [
	'var(--color-surface-2)',
	'color-mix(in srgb, var(--color-accent) 22%, var(--color-surface-2))',
	'color-mix(in srgb, var(--color-accent) 45%, var(--color-surface-2))',
	'color-mix(in srgb, var(--color-accent) 70%, var(--color-surface-2))',
	'var(--color-accent)'
];
