package io.stepes.couchverse.ranks

import io.stepes.couchverse.design.R

// The core names achievements, tiers and XP sources by stable codes; these are their words.
// Listed out rather than looked up by name, so release builds keep every one of them.

internal val AchievementNames: Map<String, Int> = mapOf(
    "achievements_10" to R.string.achievement_achievements_10_name,
    "achievements_20" to R.string.achievement_achievements_20_name,
    "active_days_200" to R.string.achievement_active_days_200_name,
    "active_days_50" to R.string.achievement_active_days_50_name,
    "avatar_set" to R.string.achievement_avatar_set_name,
    "couch_host_1" to R.string.achievement_couch_host_1_name,
    "couch_host_25" to R.string.achievement_couch_host_25_name,
    "couch_join_10" to R.string.achievement_couch_join_10_name,
    "couch_party_6" to R.string.achievement_couch_party_6_name,
    "decades_4" to R.string.achievement_decades_4_name,
    "early_bird_10" to R.string.achievement_early_bird_10_name,
    "emoji_100" to R.string.achievement_emoji_100_name,
    "episodes_100" to R.string.achievement_episodes_100_name,
    "first_play" to R.string.achievement_first_play_name,
    "genres_10" to R.string.achievement_genres_10_name,
    "marathon_6h" to R.string.achievement_marathon_6h_name,
    "movies_25" to R.string.achievement_movies_25_name,
    "night_owl_10" to R.string.achievement_night_owl_10_name,
    "series_done_1" to R.string.achievement_series_done_1_name,
    "series_done_10" to R.string.achievement_series_done_10_name,
    "streak_3" to R.string.achievement_streak_3_name,
    "streak_30" to R.string.achievement_streak_30_name,
    "streak_7" to R.string.achievement_streak_7_name,
    "titles_50" to R.string.achievement_titles_50_name,
    "veteran_365" to R.string.achievement_veteran_365_name,
    "watch_10h" to R.string.achievement_watch_10h_name,
    "watch_200h" to R.string.achievement_watch_200h_name,
    "watch_500h" to R.string.achievement_watch_500h_name,
    "watch_50h" to R.string.achievement_watch_50h_name,
    "watchlist_10" to R.string.achievement_watchlist_10_name,
)

internal val AchievementDescriptions: Map<String, Int> = mapOf(
    "achievements_10" to R.string.achievement_achievements_10_desc,
    "achievements_20" to R.string.achievement_achievements_20_desc,
    "active_days_200" to R.string.achievement_active_days_200_desc,
    "active_days_50" to R.string.achievement_active_days_50_desc,
    "avatar_set" to R.string.achievement_avatar_set_desc,
    "couch_host_1" to R.string.achievement_couch_host_1_desc,
    "couch_host_25" to R.string.achievement_couch_host_25_desc,
    "couch_join_10" to R.string.achievement_couch_join_10_desc,
    "couch_party_6" to R.string.achievement_couch_party_6_desc,
    "decades_4" to R.string.achievement_decades_4_desc,
    "early_bird_10" to R.string.achievement_early_bird_10_desc,
    "emoji_100" to R.string.achievement_emoji_100_desc,
    "episodes_100" to R.string.achievement_episodes_100_desc,
    "first_play" to R.string.achievement_first_play_desc,
    "genres_10" to R.string.achievement_genres_10_desc,
    "marathon_6h" to R.string.achievement_marathon_6h_desc,
    "movies_25" to R.string.achievement_movies_25_desc,
    "night_owl_10" to R.string.achievement_night_owl_10_desc,
    "series_done_1" to R.string.achievement_series_done_1_desc,
    "series_done_10" to R.string.achievement_series_done_10_desc,
    "streak_3" to R.string.achievement_streak_3_desc,
    "streak_30" to R.string.achievement_streak_30_desc,
    "streak_7" to R.string.achievement_streak_7_desc,
    "titles_50" to R.string.achievement_titles_50_desc,
    "veteran_365" to R.string.achievement_veteran_365_desc,
    "watch_10h" to R.string.achievement_watch_10h_desc,
    "watch_200h" to R.string.achievement_watch_200h_desc,
    "watch_500h" to R.string.achievement_watch_500h_desc,
    "watch_50h" to R.string.achievement_watch_50h_desc,
    "watchlist_10" to R.string.achievement_watchlist_10_desc,
)

internal val TierNames: Map<String, Int> = mapOf(
    "binger" to R.string.rank_tier_binger,
    "cinephile" to R.string.rank_tier_cinephile,
    "legend" to R.string.rank_tier_legend,
    "marathoner" to R.string.rank_tier_marathoner,
    "master" to R.string.rank_tier_master,
    "popcorn" to R.string.rank_tier_popcorn,
    "remote" to R.string.rank_tier_remote,
    "rookie" to R.string.rank_tier_rookie,
    "sage" to R.string.rank_tier_sage,
    "snack" to R.string.rank_tier_snack,
)

internal val XpSources: Map<String, Int> = mapOf(
    "achievements" to R.string.rank_source_achievements,
    "couch_hosted" to R.string.rank_source_couch_hosted,
    "couch_joined" to R.string.rank_source_couch_joined,
    "episodes" to R.string.rank_source_episodes,
    "movies" to R.string.rank_source_movies,
    "video" to R.string.rank_source_video,
)
