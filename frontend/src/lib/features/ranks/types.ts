export type TierCode =
	| 'rookie'
	| 'remote'
	| 'snack'
	| 'binger'
	| 'popcorn'
	| 'marathoner'
	| 'sage'
	| 'cinephile'
	| 'master'
	| 'legend';

export type AchievementTier = 'bronze' | 'silver' | 'gold' | 'platinum';
export type AchievementCategory = 'watching' | 'streaks' | 'explorer' | 'music' | 'couch' | 'meta';

export type Metric = 'xp' | 'watch' | 'music' | 'achievements';
export type Period = 'all' | 'month' | 'week';

export interface Tier {
	code: TierCode;
	level: number;
	minXp: number;
	colour: string;
}

export interface RankProgress {
	xp: number;
	tier: Tier;
	next: Tier | null;
	intoTier: number;
	tierSpan: number;
	percent: number;
}

/** one line of the "where your XP comes from" breakdown */
export interface XpSource {
	key: string;
	units: number;
	rate: number;
	xp: number;
}

export interface XpResult {
	total: number;
	sources: XpSource[];
}

export interface Achievement {
	code: string;
	category: AchievementCategory;
	tier: AchievementTier;
	xp: number;
	target: number;
	value: number;
	percent: number;
	unlocked: boolean;
	unlockedAt: string | null;
}

export interface ProfileTotals {
	videoSeconds: number;
	musicSeconds: number;
	moviesCompleted: number;
	episodesCompleted: number;
	seriesCompleted: number;
	distinctTitles: number;
	distinctGenres: number;
	activeDays: number;
	currentStreak: number;
	longestStreak: number;
	bestDayMinutes: number;
	tracksPlayed: number;
	distinctArtists: number;
	couchHosted: number;
	couchJoined: number;
	biggestCouch: number;
	emojiSent: number;
}

export interface ProfileTopTitle {
	slug: string;
	name: string;
	kind: string;
	posterId: string | null;
	seconds: number;
}

/** daily seconds for the calendar heatmap, oldest first */
export interface Activity {
	from: string;
	days: number[];
}

export interface HourBucket {
	hour: number;
	videoSeconds: number;
	musicSeconds: number;
}

export interface Profile {
	user: {
		username: string;
		displayName: string;
		avatarId: string | null;
		bannerId: string | null;
		/** hex extracted from the banner at upload; "" when there is none */
		bannerAccent: string;
		/** markdown, rendered with raw HTML disabled */
		bio: string;
		memberSince: string;
	};
	rank: RankProgress;
	xp: XpResult;
	totals: ProfileTotals;
	favouriteGenre: string;
	achievements: Achievement[];
	recentUnlocks: Achievement[];
	topTitles: ProfileTopTitle[];
	activity: Activity;
	hours: HourBucket[];
	achievementsWon: number;
	public: boolean;
	isSelf: boolean;
}

export interface LeaderRow {
	username: string;
	displayName: string;
	avatarId: string | null;
	level: number;
	tierCode: TierCode;
	xp: number;
	watchSeconds: number;
	musicSeconds: number;
	achievements: number;
	isSelf: boolean;
}

export interface Leaderboard {
	period: Period;
	total: number;
	rows: LeaderRow[];
	me: LeaderRow | null;
	hidden: boolean;
}

export interface CheckResult {
	unlocked: Achievement[];
	throttled: boolean;
	/** null on a throttled call: keep the rank already on screen */
	rank: RankProgress | null;
}
