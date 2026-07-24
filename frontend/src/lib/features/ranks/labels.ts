import {
	Award,
	CalendarCheck,
	CalendarDays,
	CircleCheckBig,
	Clapperboard,
	Compass,
	Crown,
	Disc3,
	Film,
	Flame,
	Headphones,
	Heart,
	Hourglass,
	Image,
	ListMusic,
	ListVideo,
	Medal,
	Moon,
	Popcorn,
	Rocket,
	Smile,
	Sofa,
	Sparkles,
	Sunrise,
	Timer,
	Trophy,
	Tv,
	Users,
	type Icon as LucideIcon
} from 'lucide-svelte';
import * as m from '$lib/paraglide/messages';
import type { AchievementTier, Metric, TierCode } from './types';

// Achievement and rank codes arrive from the backend as strings, and Paraglide
// messages are static functions, so the lookup has to be an explicit map (same
// shape as job-label.ts). Unknown codes degrade to something readable rather
// than blanking the badge, so the backend can ship a new achievement before the
// frontend has strings for it.

const NAMES: Record<string, () => string> = {
	first_play: m.achievement_first_play_name,
	watch_10h: m.achievement_watch_10h_name,
	watch_50h: m.achievement_watch_50h_name,
	watch_200h: m.achievement_watch_200h_name,
	watch_500h: m.achievement_watch_500h_name,
	movies_25: m.achievement_movies_25_name,
	episodes_100: m.achievement_episodes_100_name,
	series_done_1: m.achievement_series_done_1_name,
	series_done_10: m.achievement_series_done_10_name,
	marathon_6h: m.achievement_marathon_6h_name,
	streak_3: m.achievement_streak_3_name,
	streak_7: m.achievement_streak_7_name,
	streak_30: m.achievement_streak_30_name,
	night_owl_10: m.achievement_night_owl_10_name,
	early_bird_10: m.achievement_early_bird_10_name,
	active_days_50: m.achievement_active_days_50_name,
	active_days_200: m.achievement_active_days_200_name,
	genres_10: m.achievement_genres_10_name,
	titles_50: m.achievement_titles_50_name,
	watchlist_10: m.achievement_watchlist_10_name,
	decades_4: m.achievement_decades_4_name,
	tracks_100: m.achievement_tracks_100_name,
	listen_50h: m.achievement_listen_50h_name,
	artists_25: m.achievement_artists_25_name,
	playlist_50: m.achievement_playlist_50_name,
	couch_host_1: m.achievement_couch_host_1_name,
	couch_host_25: m.achievement_couch_host_25_name,
	couch_join_10: m.achievement_couch_join_10_name,
	couch_party_6: m.achievement_couch_party_6_name,
	emoji_100: m.achievement_emoji_100_name,
	avatar_set: m.achievement_avatar_set_name,
	veteran_365: m.achievement_veteran_365_name,
	achievements_10: m.achievement_achievements_10_name,
	achievements_20: m.achievement_achievements_20_name
};

const DESCRIPTIONS: Record<string, () => string> = {
	first_play: m.achievement_first_play_desc,
	watch_10h: m.achievement_watch_10h_desc,
	watch_50h: m.achievement_watch_50h_desc,
	watch_200h: m.achievement_watch_200h_desc,
	watch_500h: m.achievement_watch_500h_desc,
	movies_25: m.achievement_movies_25_desc,
	episodes_100: m.achievement_episodes_100_desc,
	series_done_1: m.achievement_series_done_1_desc,
	series_done_10: m.achievement_series_done_10_desc,
	marathon_6h: m.achievement_marathon_6h_desc,
	streak_3: m.achievement_streak_3_desc,
	streak_7: m.achievement_streak_7_desc,
	streak_30: m.achievement_streak_30_desc,
	night_owl_10: m.achievement_night_owl_10_desc,
	early_bird_10: m.achievement_early_bird_10_desc,
	active_days_50: m.achievement_active_days_50_desc,
	active_days_200: m.achievement_active_days_200_desc,
	genres_10: m.achievement_genres_10_desc,
	titles_50: m.achievement_titles_50_desc,
	watchlist_10: m.achievement_watchlist_10_desc,
	decades_4: m.achievement_decades_4_desc,
	tracks_100: m.achievement_tracks_100_desc,
	listen_50h: m.achievement_listen_50h_desc,
	artists_25: m.achievement_artists_25_desc,
	playlist_50: m.achievement_playlist_50_desc,
	couch_host_1: m.achievement_couch_host_1_desc,
	couch_host_25: m.achievement_couch_host_25_desc,
	couch_join_10: m.achievement_couch_join_10_desc,
	couch_party_6: m.achievement_couch_party_6_desc,
	emoji_100: m.achievement_emoji_100_desc,
	avatar_set: m.achievement_avatar_set_desc,
	veteran_365: m.achievement_veteran_365_desc,
	achievements_10: m.achievement_achievements_10_desc,
	achievements_20: m.achievement_achievements_20_desc
};

const ICONS: Record<string, typeof LucideIcon> = {
	first_play: Clapperboard,
	watch_10h: Timer,
	watch_50h: Hourglass,
	watch_200h: Film,
	watch_500h: Crown,
	movies_25: Popcorn,
	episodes_100: ListVideo,
	series_done_1: Tv,
	series_done_10: Award,
	marathon_6h: Rocket,
	streak_3: Flame,
	streak_7: Flame,
	streak_30: Flame,
	night_owl_10: Moon,
	early_bird_10: Sunrise,
	active_days_50: CalendarDays,
	active_days_200: CalendarCheck,
	genres_10: Compass,
	titles_50: CircleCheckBig,
	watchlist_10: Heart,
	decades_4: Sparkles,
	tracks_100: Headphones,
	listen_50h: Disc3,
	artists_25: Medal,
	playlist_50: ListMusic,
	couch_host_1: Sofa,
	couch_host_25: Sofa,
	couch_join_10: Users,
	couch_party_6: Users,
	emoji_100: Smile,
	avatar_set: Image,
	veteran_365: Trophy,
	achievements_10: Trophy,
	achievements_20: Crown
};

export const achievementName = (code: string) => NAMES[code]?.() ?? code.replaceAll('_', ' ');
export const achievementDesc = (code: string) => DESCRIPTIONS[code]?.() ?? '';
export const achievementIcon = (code: string) => ICONS[code] ?? Trophy;

const TIER_NAMES: Record<TierCode, () => string> = {
	rookie: m.rank_tier_rookie,
	remote: m.rank_tier_remote,
	snack: m.rank_tier_snack,
	binger: m.rank_tier_binger,
	popcorn: m.rank_tier_popcorn,
	marathoner: m.rank_tier_marathoner,
	sage: m.rank_tier_sage,
	cinephile: m.rank_tier_cinephile,
	master: m.rank_tier_master,
	legend: m.rank_tier_legend
};

export const tierName = (code: TierCode) => TIER_NAMES[code]?.() ?? code;

const ACHIEVEMENT_TIER_NAMES: Record<AchievementTier, () => string> = {
	bronze: m.achievement_tier_bronze,
	silver: m.achievement_tier_silver,
	gold: m.achievement_tier_gold,
	platinum: m.achievement_tier_platinum
};

export const achievementTierName = (tier: AchievementTier) =>
	ACHIEVEMENT_TIER_NAMES[tier]?.() ?? tier;

const XP_SOURCE_LABELS: Record<string, () => string> = {
	video: m.rank_source_video,
	music: m.rank_source_music,
	movies: m.rank_source_movies,
	episodes: m.rank_source_episodes,
	couchHosted: m.rank_source_couch_hosted,
	couchJoined: m.rank_source_couch_joined,
	achievements: m.rank_source_achievements
};

export const xpSourceLabel = (key: string) => XP_SOURCE_LABELS[key]?.() ?? key;

const METRIC_LABELS: Record<Metric, () => string> = {
	xp: m.leaderboard_metric_xp,
	watch: m.leaderboard_metric_watch,
	music: m.leaderboard_metric_listen,
	achievements: m.leaderboard_metric_achievements
};

export const metricLabel = (metric: Metric) => METRIC_LABELS[metric]?.() ?? metric;
