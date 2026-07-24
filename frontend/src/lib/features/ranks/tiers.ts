import type { AchievementTier, TierCode } from './types';

/**
 * Rank colours are the storage/status palette re-spent as a ladder, so ranks read
 * as part of the same system rather than a bolted-on theme. The backend also
 * ships a colour per tier; this map is the client-side source so a tier can be
 * coloured before any payload has arrived (the nav ring on a cold load).
 */
export const RANK_TIERS: Record<TierCode, { color: string; glow: string }> = {
	rookie: { color: '#7c8496', glow: 'rgba(124,132,150,0.35)' },
	remote: { color: '#60a5fa', glow: 'rgba(96,165,250,0.35)' },
	snack: { color: '#22d3ee', glow: 'rgba(34,211,238,0.35)' },
	binger: { color: '#34d399', glow: 'rgba(52,211,153,0.35)' },
	popcorn: { color: '#a3e635', glow: 'rgba(163,230,53,0.35)' },
	marathoner: { color: '#facc15', glow: 'rgba(250,204,21,0.38)' },
	sage: { color: '#fb923c', glow: 'rgba(251,146,60,0.38)' },
	cinephile: { color: '#f87171', glow: 'rgba(248,113,113,0.38)' },
	master: { color: '#c084fc', glow: 'rgba(192,132,252,0.40)' },
	legend: { color: '#e879f9', glow: 'rgba(232,121,249,0.42)' }
};

export const rankColor = (code: TierCode) => RANK_TIERS[code]?.color ?? RANK_TIERS.rookie.color;
export const rankGlow = (code: TierCode) => RANK_TIERS[code]?.glow ?? RANK_TIERS.rookie.glow;

/** Medal metals. `from`/`to` make the gradient, `ring` is the readable accent. */
export const ACHIEVEMENT_TIERS: Record<
	AchievementTier,
	{ from: string; to: string; ring: string; glow: string }
> = {
	bronze: { from: '#d08b4e', to: '#8a4f22', ring: '#e0a066', glow: 'rgba(208,139,78,0.40)' },
	silver: { from: '#d7dfee', to: '#8e97ab', ring: '#e4ebf7', glow: 'rgba(215,223,238,0.35)' },
	gold: { from: '#f7cf63', to: '#c1861a', ring: '#ffdd80', glow: 'rgba(247,207,99,0.45)' },
	platinum: { from: '#a8f0e4', to: '#7fb3ff', ring: '#c9f4ff', glow: 'rgba(168,240,228,0.45)' }
};

/** Podium places reuse the medals, so one palette serves both surfaces. */
export const PODIUM = [
	ACHIEVEMENT_TIERS.gold,
	ACHIEVEMENT_TIERS.silver,
	ACHIEVEMENT_TIERS.bronze
] as const;

/** XP sources borrow the storage categories so the same thing is the same colour. */
export const XP_SOURCE_COLORS: Record<string, string> = {
	video: 'var(--color-accent)',
	music: '#f5b14c',
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
