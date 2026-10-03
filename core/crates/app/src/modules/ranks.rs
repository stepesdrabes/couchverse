//! Progression: the viewer's rank (the nav badge, with a pulse on a genuine level-up), the
//! queue of unlocked achievements waiting to be celebrated, profiles with their activity
//! heatmap, and the leaderboards. Achievement checks are pull-based and throttled on the server;
//! the core throttles too, so a burst of triggers (a few episodes in a row) costs one request.

use std::collections::{HashMap, VecDeque};

use couchverse_api::ops::{GetLeaderboardQuery, GetMyStatsQuery, GetProfileQuery};
use couchverse_api::types::{
    AchievementCheck, AchievementProgress, GetLeaderboardPeriod, Leaderboard, LeaderboardRow,
    NextRankTier, RankTier, UserProfile,
};
use couchverse_api::{Call, ops};
use serde::{Deserialize, Serialize};
use typeshare::typeshare;

use crate::api::{Endpoint, Failure, decode};
use crate::core::Pending;
use crate::effects::Ctx;
use crate::messages::{EffectOutput, LoadStatus, Problem, Surface, U53};
use crate::modules::catalog::TitleKind;
use crate::modules::images::{Image, Images, Size};
use crate::modules::markdown::{self, MarkdownDoc};

/// At most one check per this window, mirroring the server's throttle.
const CHECK_INTERVAL_MS: U53 = 5 * 60 * 1000;
/// A profile or leaderboard reopened within this window is not refetched.
const FRESH_MS: U53 = 60_000;

#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum Period {
    #[default]
    All,
    Month,
    Week,
}

/// What a leaderboard ranks by; one payload carries every metric, so switching costs nothing.
#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum Metric {
    #[default]
    Xp,
    Watch,
    Achievements,
}

#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct LeaderboardKey {
    pub period: Period,
    pub metric: Metric,
}

#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CheckRequest {
    /// Check even inside the throttle window (the end of a title).
    #[serde(default)]
    pub force: bool,
}

#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PublicChoice {
    pub public: bool,
}

/// A rank tier; `code` is localized by the shell (`rank_tier_<code>`).
#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Tier {
    pub code: String,
    pub level: u32,
    /// `#rrggbb`.
    pub colour: String,
    pub min_xp: U53,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RankBadge {
    pub tier: Tier,
    /// The tier after this one; the same as `tier` at the top.
    pub next: Tier,
    pub xp: U53,
    /// Progress through the current tier, from 0 to 100.
    pub percent: u32,
}

/// An achievement; `code` is localized by the shell (`achievement_<code>_name` and `_desc`).
#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AchievementCard {
    pub code: String,
    /// `watching`, `streaks`, `explorer`, `couch` or `meta`.
    pub category: String,
    /// `bronze`, `silver`, `gold` or `platinum`.
    pub tier: String,
    pub unlocked: bool,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub unlocked_at: Option<String>,
    pub value: U53,
    pub target: U53,
    /// From 0 to 100.
    pub percent: u32,
    pub xp: U53,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RankView {
    /// Absent until the first check, or with rankings off.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub rank: Option<RankBadge>,
    /// Bumped on every genuine level-up, so a badge can pulse once per change.
    pub level_ups: U53,
    /// The unlock to celebrate now; dismissing it shows the next.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub celebration: Option<AchievementCard>,
    /// How many more are queued behind it.
    pub queued: u32,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct HeatDay {
    pub seconds: U53,
    /// 0 (nothing watched) to 4 (among the viewer's busiest days).
    pub level: u8,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Heatmap {
    /// The date of `days[0]` (`YYYY-MM-DD`); the run ends today.
    pub from: String,
    pub days: Vec<HeatDay>,
    pub total_seconds: U53,
    pub active_days: u32,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct TopTitle {
    pub slug: String,
    pub name: String,
    pub kind: TitleKind,
    pub seconds: U53,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub poster: Option<Image>,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct XpLine {
    /// What earned it; localized by the shell (`rank_xp_source_<key>`).
    pub key: String,
    pub units: U53,
    pub rate: U53,
    pub xp: U53,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ProfileTotals {
    pub watch_seconds: U53,
    pub movies_completed: U53,
    pub episodes_completed: U53,
    pub series_completed: U53,
    pub distinct_titles: U53,
    pub distinct_genres: U53,
    pub active_days: U53,
    pub current_streak: U53,
    pub longest_streak: U53,
    pub best_day_minutes: U53,
    pub couch_hosted: U53,
    pub couch_joined: U53,
    pub biggest_couch: U53,
    pub emoji_sent: U53,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ProfileDetail {
    pub username: String,
    pub display_name: String,
    pub bio: MarkdownDoc,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub avatar: Option<Image>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub banner: Option<Image>,
    pub member_since: String,
    pub is_self: bool,
    pub public: bool,
    pub rank: RankBadge,
    pub xp_total: U53,
    pub xp_sources: Vec<XpLine>,
    pub achievements: Vec<AchievementCard>,
    pub achievements_won: u32,
    pub recent_unlocks: Vec<AchievementCard>,
    pub totals: ProfileTotals,
    pub top_titles: Vec<TopTitle>,
    /// Empty when nothing has been watched.
    pub favourite_genre: String,
    /// Watch seconds per hour of the day, 24 entries.
    pub hours: Vec<U53>,
    pub heatmap: Heatmap,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ProfileView {
    pub username: String,
    pub status: LoadStatus,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub profile: Option<ProfileDetail>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub problem: Option<Problem>,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct LeaderRow {
    /// 1-based, by the board's metric.
    pub position: u32,
    pub username: String,
    pub display_name: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub avatar: Option<Image>,
    pub level: u32,
    pub tier_code: String,
    /// The board's metric for this member.
    pub value: U53,
    pub xp: U53,
    pub watch_seconds: U53,
    pub achievements: U53,
    pub is_self: bool,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct LeaderboardView {
    pub key: LeaderboardKey,
    pub status: LoadStatus,
    pub rows: Vec<LeaderRow>,
    /// The viewer's place on this board, when listed.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub my_position: Option<u32>,
    /// The viewer's own row, listed or not; its `position` is 0 while they are hidden.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub me: Option<LeaderRow>,
    /// How many members the board ranks.
    pub total: u32,
    /// The top three earned something, so a podium makes sense.
    pub podium: bool,
    /// Nobody has anything on this metric yet.
    pub all_zero: bool,
    /// The viewer opted out of leaderboards, so is missing from the rows.
    pub hidden: bool,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub problem: Option<Problem>,
}

/// Everything the module needs from the active session for a request.
pub struct Env<'a> {
    pub endpoint: &'a Endpoint,
    pub language: &'a str,
    /// The viewer's username, to read their own profile through `/me/stats`.
    pub username: Option<&'a str>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct RanksPending {
    generation: u64,
    request: Request,
}

#[derive(Debug, Clone, PartialEq)]
enum Request {
    Check(Call<AchievementCheck>),
    /// Tagged with the language generation: a profile carries localized labels.
    Profile {
        username: String,
        language: u64,
        call: Call<UserProfile>,
    },
    Leaderboard {
        period: Period,
        call: Call<Leaderboard>,
    },
    Visibility(Call<couchverse_api::types::Preferences>),
}

#[derive(Debug, PartialEq, Eq)]
pub enum RanksChange {
    None,
    Unauthorized,
    /// The visibility switch could not be saved and was rolled back.
    VisibilityFailed,
}

struct Slot<T> {
    value: Option<T>,
    fetched_at: Option<U53>,
    loading: bool,
    problem: Option<Problem>,
    not_found: bool,
}

impl<T> Default for Slot<T> {
    fn default() -> Self {
        Self { value: None, fetched_at: None, loading: false, problem: None, not_found: false }
    }
}

impl<T> Slot<T> {
    fn status(&self) -> LoadStatus {
        match (&self.value, self.loading) {
            (Some(_), true) => LoadStatus::Stale,
            (Some(_), false) if self.problem.is_some() => LoadStatus::Stale,
            (Some(_), false) => LoadStatus::Loaded,
            (None, true) => LoadStatus::Loading,
            (None, false) if self.not_found => LoadStatus::NotFound,
            (None, false) if self.problem.is_some() => LoadStatus::Failed,
            (None, false) => LoadStatus::Idle,
        }
    }

    fn wants(&self, now: U53, force: bool) -> bool {
        let fresh = self.value.is_some()
            && self.fetched_at.is_some_and(|at| now.saturating_sub(at) < FRESH_MS);
        (force || !fresh) && !self.loading
    }

    fn fill(&mut self, result: Result<T, Failure>, now: U53) -> Option<Failure> {
        self.loading = false;
        match result {
            Ok(value) => {
                self.value = Some(value);
                self.fetched_at = Some(now);
                self.problem = None;
                self.not_found = false;
                None
            }
            Err(failure) => {
                self.not_found = matches!(&failure, Failure::Api(e) if e.status == 404);
                if self.not_found {
                    self.value = None;
                }
                self.problem = Some(failure.problem());
                Some(failure)
            }
        }
    }
}

#[derive(Default)]
pub struct Ranks {
    generation: u64,
    /// Bumped by a language switch, which only outdates profiles.
    language: u64,
    rank: Option<RankBadge>,
    level_ups: U53,
    queue: VecDeque<AchievementProgress>,
    last_check: Option<U53>,
    checking: bool,
    profiles: HashMap<String, Slot<UserProfile>>,
    boards: HashMap<Period, Slot<Leaderboard>>,
    /// The visibility switch not yet confirmed by the server.
    public: Option<bool>,
}

/// Whether a surface belongs to ranks.
pub fn owns(surface: &Surface) -> bool {
    matches!(surface, Surface::Rank | Surface::Profile(_) | Surface::Leaderboard(_))
}

impl Ranks {
    /// Forgets everything (another account, rankings switched off).
    pub fn reset(&mut self, ctx: &mut Ctx) {
        *self = Ranks { generation: self.generation + 1, ..Ranks::default() };
        ctx.render(Surface::Rank);
    }

    /// Profiles carry localized labels (genres, titles), so another language reloads them.
    pub fn language_changed(&mut self) {
        self.language += 1;
        for slot in self.profiles.values_mut() {
            slot.fetched_at = None;
            slot.loading = false;
        }
    }

    /// Asks the server whether anything new was earned, at most once per window unless forced.
    pub fn check(&mut self, ctx: &mut Ctx, endpoint: &Endpoint, force: bool) {
        let recent =
            self.last_check.is_some_and(|at| ctx.now.saturating_sub(at) < CHECK_INTERVAL_MS);
        if self.checking || (recent && !force) {
            return;
        }
        self.last_check = Some(ctx.now);
        self.checking = true;
        let call = ops::check_achievements();
        ctx.http(endpoint.request(&call.request), self.pending(Request::Check(call)));
    }

    pub fn open(&mut self, ctx: &mut Ctx, env: &Env, surface: &Surface, force: bool) {
        let now = ctx.now;
        match surface {
            Surface::Profile(username) => {
                let slot = self.profiles.entry(username.clone()).or_default();
                if !slot.wants(now, force) {
                    return;
                }
                slot.loading = true;
                let lang = Some(env.language.to_string());
                let call = if env.username == Some(username.as_str()) {
                    ops::get_my_stats(&GetMyStatsQuery { lang })
                } else {
                    ops::get_profile(username, &GetProfileQuery { lang })
                };
                let request = env.endpoint.request(&call.request);
                let username = username.clone();
                let language = self.language;
                ctx.http(request, self.pending(Request::Profile { username, language, call }));
            }
            Surface::Leaderboard(key) => {
                let slot = self.boards.entry(key.period).or_default();
                if !slot.wants(now, force) {
                    return;
                }
                slot.loading = true;
                let period = match key.period {
                    Period::All => GetLeaderboardPeriod::All,
                    Period::Month => GetLeaderboardPeriod::Month,
                    Period::Week => GetLeaderboardPeriod::Week,
                };
                let call = ops::get_leaderboard(&GetLeaderboardQuery { period: Some(period) });
                let request = env.endpoint.request(&call.request);
                ctx.http(request, self.pending(Request::Leaderboard { period: key.period, call }));
            }
            _ => return,
        }
        ctx.render(surface.clone());
    }

    /// The celebration on screen was seen; the next one (if any) takes its place.
    pub fn celebrated(&mut self, ctx: &mut Ctx) {
        if self.queue.pop_front().is_some() {
            ctx.render(Surface::Rank);
        }
    }

    pub fn set_public(&mut self, ctx: &mut Ctx, endpoint: &Endpoint, public: bool) {
        self.public = Some(public);
        let prefs = couchverse_api::types::Preferences {
            public_profile: Some(public),
            language: None,
            subtitles: None,
        };
        let call = ops::update_preferences(&prefs);
        ctx.http(endpoint.request(&call.request), self.pending(Request::Visibility(call)));
        self.render_profiles(ctx);
    }

    fn pending(&self, request: Request) -> Pending {
        Pending::Ranks(RanksPending { generation: self.generation, request })
    }

    pub fn resolve(
        &mut self,
        ctx: &mut Ctx,
        pending: RanksPending,
        output: EffectOutput,
    ) -> RanksChange {
        if pending.generation != self.generation {
            return RanksChange::None;
        }
        let now = ctx.now;
        let failure = match pending.request {
            Request::Check(call) => {
                self.checking = false;
                match decode(&call, output) {
                    Ok(result) => {
                        // a throttled check skipped the snapshot and carries no rank
                        if let Some(r) = result.rank.as_ref().filter(|_| !result.throttled) {
                            self.adopt(badge(&r.tier, r.next.as_ref(), r.xp, r.percent));
                        }
                        self.queue.extend(result.unlocked);
                        ctx.render(Surface::Rank);
                        None
                    }
                    // progression must never interrupt watching: other failures stay quiet
                    Err(failure) => Some(failure),
                }
            }
            // an answer in the language the viewer just left
            Request::Profile { language, .. } if language != self.language => None,
            Request::Profile { username, call, .. } => {
                let result = decode(&call, output);
                if let Ok(profile) = &result
                    && profile.is_self
                {
                    let r = &profile.rank;
                    self.adopt(badge(&r.tier, r.next.as_ref(), r.xp, r.percent));
                    ctx.render(Surface::Rank);
                }
                ctx.render(Surface::Profile(username.clone()));
                self.profiles.entry(username).or_default().fill(result, now)
            }
            Request::Leaderboard { period, call } => {
                for metric in [Metric::Xp, Metric::Watch, Metric::Achievements] {
                    ctx.render(Surface::Leaderboard(LeaderboardKey { period, metric }));
                }
                self.boards.entry(period).or_default().fill(decode(&call, output), now)
            }
            Request::Visibility(call) => {
                let result = decode(&call, output);
                let public = self.public.take();
                if let (Ok(_), Some(public)) = (&result, public) {
                    for slot in self.profiles.values_mut() {
                        if let Some(profile) = slot.value.as_mut().filter(|p| p.is_self) {
                            profile.public = public;
                        }
                    }
                    // a board's rows change with the viewer's visibility
                    for slot in self.boards.values_mut() {
                        slot.fetched_at = None;
                    }
                }
                self.render_profiles(ctx);
                match result {
                    Ok(_) => None,
                    Err(failure) if failure.unauthorized() => Some(failure),
                    Err(_) => return RanksChange::VisibilityFailed,
                }
            }
        };
        match failure {
            Some(failure) if failure.unauthorized() => RanksChange::Unauthorized,
            _ => RanksChange::None,
        }
    }

    /// Takes a new rank; a higher level than the one showing is a level-up, the first is not.
    fn adopt(&mut self, next: RankBadge) {
        if self.rank.as_ref().is_some_and(|r| next.tier.level > r.tier.level) {
            self.level_ups += 1;
        }
        self.rank = Some(next);
    }

    fn render_profiles(&self, ctx: &mut Ctx) {
        for username in self.profiles.keys() {
            ctx.render(Surface::Profile(username.clone()));
        }
    }

    pub fn rank_view(&self) -> RankView {
        RankView {
            rank: self.rank.clone(),
            level_ups: self.level_ups,
            celebration: self.queue.front().map(card),
            queued: u32::try_from(self.queue.len().saturating_sub(1)).unwrap_or(u32::MAX),
        }
    }

    pub fn profile_view(&self, username: &str, images: &Images) -> ProfileView {
        let slot = self.profiles.get(username);
        ProfileView {
            username: username.to_string(),
            status: slot.map_or(LoadStatus::Idle, Slot::status),
            profile: slot.and_then(|s| s.value.as_ref()).map(|p| self.profile(p, images)),
            problem: slot.and_then(|s| s.problem.clone()),
        }
    }

    fn profile(&self, p: &UserProfile, images: &Images) -> ProfileDetail {
        let u = &p.user;
        let t = &p.totals;
        ProfileDetail {
            username: u.username.clone(),
            display_name: u.display_name.clone(),
            bio: markdown::parse(&u.bio),
            avatar: u.avatar_id.as_ref().map(|id| images.image(id, None, Size::Small, None)),
            // shown dimmed behind the header, where a resize is plenty; an uploaded original
            // can be a phone photo of several megabytes
            banner: u.banner_id.as_ref().map(|id| {
                let accent = Some(u.banner_accent.clone()).filter(|a| !a.is_empty());
                images.image(id, None, Size::Medium, accent)
            }),
            member_since: u.member_since.clone(),
            is_self: p.is_self,
            public: if p.is_self { self.public.unwrap_or(p.public) } else { p.public },
            rank: badge(&p.rank.tier, p.rank.next.as_ref(), p.rank.xp, p.rank.percent),
            xp_total: unsigned(p.xp.total),
            xp_sources: p
                .xp
                .sources
                .iter()
                .map(|s| XpLine {
                    key: s.key.as_str().to_string(),
                    units: unsigned(s.units),
                    rate: unsigned(s.rate),
                    xp: unsigned(s.xp),
                })
                .collect(),
            achievements: p.achievements.iter().map(card).collect(),
            achievements_won: u32::try_from(p.achievements_won).unwrap_or(0),
            recent_unlocks: p.recent_unlocks.iter().map(card).collect(),
            totals: ProfileTotals {
                watch_seconds: unsigned(t.video_seconds),
                movies_completed: unsigned(t.movies_completed),
                episodes_completed: unsigned(t.episodes_completed),
                series_completed: unsigned(t.series_completed),
                distinct_titles: unsigned(t.distinct_titles),
                distinct_genres: unsigned(t.distinct_genres),
                active_days: unsigned(t.active_days),
                current_streak: unsigned(t.current_streak),
                longest_streak: unsigned(t.longest_streak),
                best_day_minutes: unsigned(t.best_day_minutes),
                couch_hosted: unsigned(t.couch_hosted),
                couch_joined: unsigned(t.couch_joined),
                biggest_couch: unsigned(t.biggest_couch),
                emoji_sent: unsigned(t.emoji_sent),
            },
            top_titles: p
                .top_titles
                .iter()
                .map(|t| TopTitle {
                    slug: t.slug.clone(),
                    name: t.name.clone(),
                    kind: TitleKind::parse(t.kind.as_str()),
                    seconds: unsigned(t.seconds),
                    poster: t
                        .poster_id
                        .as_ref()
                        .map(|id| images.image(id, None, Size::Small, None)),
                })
                .collect(),
            favourite_genre: p.favourite_genre.clone(),
            hours: hours(p),
            heatmap: heatmap(&p.activity.from, &p.activity.days),
        }
    }

    pub fn leaderboard_view(&self, key: LeaderboardKey, images: &Images) -> LeaderboardView {
        let slot = self.boards.get(&key.period);
        let board = slot.and_then(|s| s.value.as_ref());
        let rows = board.map(|b| ranked(&b.rows, key.metric, images)).unwrap_or_default();
        let top = rows.first().map_or(0, |r| r.value);
        let my_position = rows.iter().find(|r| r.is_self).map(|r| r.position);
        LeaderboardView {
            key,
            status: slot.map_or(LoadStatus::Idle, Slot::status),
            my_position,
            me: board.and_then(|b| b.me.as_ref()).map(|me| LeaderRow {
                position: my_position.unwrap_or(0),
                ..leader_row(me, key.metric, images)
            }),
            total: board.map_or(0, |b| u32::try_from(b.total).unwrap_or(0)),
            podium: rows.len() >= 3 && top > 0,
            all_zero: !rows.is_empty() && top == 0,
            hidden: board.is_some_and(|b| b.hidden),
            rows,
            problem: slot.and_then(|s| s.problem.clone()),
        }
    }
}

/// `next` is null at the top of the ladder, where the badge names the top tier twice.
fn badge(tier: &RankTier, next: Option<&NextRankTier>, xp: i64, percent: i64) -> RankBadge {
    let tier = Tier {
        code: tier.code.as_str().to_string(),
        level: u32::try_from(tier.level).unwrap_or(0),
        colour: tier.colour.clone(),
        min_xp: unsigned(tier.min_xp),
    };
    let next = next.map_or_else(
        || tier.clone(),
        |next| Tier {
            code: next.code.as_str().to_string(),
            level: u32::try_from(next.level).unwrap_or(0),
            colour: next.colour.clone(),
            min_xp: unsigned(next.min_xp),
        },
    );
    RankBadge {
        tier,
        next,
        xp: unsigned(xp),
        percent: u32::try_from(percent.clamp(0, 100)).unwrap_or(0),
    }
}

fn card(a: &AchievementProgress) -> AchievementCard {
    AchievementCard {
        code: a.code.clone(),
        category: a.category.as_str().to_string(),
        tier: a.tier.as_str().to_string(),
        unlocked: a.unlocked,
        unlocked_at: a.unlocked_at.clone(),
        value: unsigned(a.value),
        target: unsigned(a.target),
        percent: u32::try_from(a.percent.clamp(0, 100)).unwrap_or(0),
        xp: unsigned(a.xp),
    }
}

fn metric_value(r: &LeaderboardRow, metric: Metric) -> U53 {
    match metric {
        Metric::Xp => unsigned(r.xp),
        Metric::Watch => unsigned(r.watch_seconds),
        Metric::Achievements => unsigned(r.achievements),
    }
}

/// Sorted by the metric, ties keeping the server's order.
fn ranked(rows: &[LeaderboardRow], metric: Metric, images: &Images) -> Vec<LeaderRow> {
    let mut sorted: Vec<&LeaderboardRow> = rows.iter().collect();
    sorted.sort_by_key(|r| std::cmp::Reverse(metric_value(r, metric)));
    sorted
        .into_iter()
        .zip(1..)
        .map(|(r, position)| LeaderRow { position, ..leader_row(r, metric, images) })
        .collect()
}

fn leader_row(r: &LeaderboardRow, metric: Metric, images: &Images) -> LeaderRow {
    LeaderRow {
        position: 0,
        username: r.username.clone(),
        display_name: r.display_name.clone(),
        avatar: r.avatar_id.as_ref().map(|id| images.image(id, None, Size::Small, None)),
        level: u32::try_from(r.level).unwrap_or(0),
        tier_code: r.tier_code.as_str().to_string(),
        value: metric_value(r, metric),
        xp: unsigned(r.xp),
        watch_seconds: unsigned(r.watch_seconds),
        achievements: unsigned(r.achievements),
        is_self: r.is_self,
    }
}

fn hours(p: &UserProfile) -> Vec<U53> {
    let mut hours = vec![0; 24];
    for bucket in &p.hours {
        if let Some(slot) = usize::try_from(bucket.hour).ok().and_then(|h| hours.get_mut(h)) {
            *slot = unsigned(bucket.video_seconds);
        }
    }
    hours
}

/// Levels relative to the viewer's own active days (quartiles), as the web drew it: a light
/// watcher's busiest days still light up.
fn heatmap(from: &str, days: &[i64]) -> Heatmap {
    let seconds: Vec<U53> = days.iter().map(|d| unsigned(*d)).collect();
    let mut active: Vec<U53> = seconds.iter().copied().filter(|s| *s > 0).collect();
    active.sort_unstable();
    let quartile =
        |q: usize| active.get((active.len() * q / 4).min(active.len().saturating_sub(1))).copied();
    let thresholds = [quartile(1), quartile(2), quartile(3)];
    let level = |s: U53| -> u8 {
        if s == 0 {
            return 0;
        }
        1 + u8::try_from(thresholds.iter().filter(|t| t.is_some_and(|t| s >= t)).count())
            .unwrap_or(0)
    };
    Heatmap {
        from: from.to_string(),
        days: seconds.iter().map(|s| HeatDay { seconds: *s, level: level(*s) }).collect(),
        total_seconds: seconds.iter().sum(),
        active_days: u32::try_from(active.len()).unwrap_or(u32::MAX),
    }
}

fn unsigned(value: i64) -> U53 {
    u64::try_from(value).unwrap_or(0)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn the_heatmap_lights_up_relative_to_the_viewer() {
        let map = heatmap("2026-01-01", &[0, 60, 120, 180, 240, 0, 3600]);
        let levels: Vec<u8> = map.days.iter().map(|d| d.level).collect();
        // active days sorted: 60 120 180 240 3600; quartiles at 120, 180, 240
        assert_eq!(levels, [0, 1, 2, 3, 4, 0, 4]);
        assert_eq!(map.total_seconds, 4200);
        assert_eq!(map.active_days, 5);
        assert!(heatmap("x", &[0, 0]).days.iter().all(|d| d.level == 0));
        assert_eq!(heatmap("x", &[30]).days[0].level, 4);
    }
}
