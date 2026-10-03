use serde_json::json;

use super::*;
use crate::modules::notices::NoticesView;
use crate::modules::profile::{
    ImageChoice, ImageSlot, PasswordForm, ProfileEdit, ProfileEditorView,
};
use crate::modules::ranks::{
    CheckRequest, LeaderboardKey, LeaderboardView, Metric, Period, ProfileView, PublicChoice,
    RankView,
};
use crate::modules::session::SessionView;

const API: &str = "https://media.example.com/api/v1";
const CHECK: &str = "https://media.example.com/api/v1/me/achievements/check";

/// A phone signed in as the admin on a server with rankings on.
fn ranked() -> Shell {
    let mut shell = launched(returning(Platform::Ios, &[(1, "admin", Some("tok-1"))], 1));
    shell.respond("GET", &format!("{API}/auth/me"), 200, user(1, "admin"));
    shell.respond(
        "GET",
        &format!("{API}/features"),
        200,
        json!({ "couchEnabled": true, "rankingsEnabled": true, "downloadsEnabled": true }),
    );
    shell.respond("GET", &format!("{API}/me/preferences"), 200, json!({ "language": "en" }));
    shell.respond("GET", &format!("{API}/server"), 200, server_info("#3a6ea5"));
    shell
}

fn tier(code: &str, level: i64, min_xp: i64) -> Value {
    json!({ "code": code, "level": level, "colour": "#cd7f32", "minXp": min_xp })
}

fn rank(level: i64, xp: i64) -> Value {
    json!({
        "tier": tier(if level == 1 { "rookie" } else { "remote" }, level, (level - 1) * 100),
        "next": tier("snack", level + 1, level * 100),
        "xp": xp, "percent": 40, "intoTier": 40, "tierSpan": 100,
    })
}

fn achievement(code: &str, unlocked: bool) -> Value {
    json!({
        "code": code, "category": "watching", "tier": "bronze", "unlocked": unlocked,
        "unlockedAt": if unlocked { json!("2026-10-02T20:00:00Z") } else { Value::Null },
        "value": 1, "target": 1, "percent": 100, "xp": 50,
    })
}

fn check_result(level: i64, unlocked: &[&str]) -> Value {
    let unlocked: Vec<Value> = unlocked.iter().map(|c| achievement(c, true)).collect();
    json!({ "throttled": false, "rank": rank(level, 140), "unlocked": unlocked })
}

#[test]
fn unlocks_queue_up_for_celebration_and_level_ups_count_once() {
    let mut shell = ranked();
    shell.respond("POST", CHECK, 200, check_result(1, &["first_play", "watch_10h"]));

    let view: RankView = shell.view(&Surface::Rank);
    let badge = view.rank.expect("rank");
    assert_eq!((badge.tier.code.as_str(), badge.tier.level, badge.next.level), ("rookie", 1, 2));
    assert_eq!(view.level_ups, 0, "the first rank is not a level-up");
    assert_eq!(view.celebration.expect("celebration").code, "first_play");
    assert_eq!(view.queued, 1);

    shell.send(Event::CelebrationDismissed);
    let view: RankView = shell.view(&Surface::Rank);
    assert_eq!(view.celebration.expect("celebration").code, "watch_10h");
    shell.send(Event::CelebrationDismissed);
    assert_eq!(shell.view::<RankView>(&Surface::Rank).celebration, None);

    // within five minutes only a forced check goes out
    shell.now += 60_000;
    shell.send(Event::AchievementsCheckRequested(CheckRequest { force: false }));
    assert!(shell.find_request("POST", CHECK).is_none());
    shell.send(Event::AchievementsCheckRequested(CheckRequest { force: true }));
    shell.respond("POST", CHECK, 200, check_result(2, &[]));
    assert_eq!(shell.view::<RankView>(&Surface::Rank).level_ups, 1);

    // a throttled answer carries no rank and keeps the one on screen
    shell.now += 5 * 60_000;
    shell.send(Event::AppBecameActive);
    shell.respond("POST", CHECK, 200, json!({ "throttled": true, "rank": null, "unlocked": [] }));
    let view: RankView = shell.view(&Surface::Rank);
    assert_eq!((view.rank.expect("rank").tier.level, view.level_ups), (2, 1));
}

#[test]
fn the_top_of_the_ladder_has_no_next_tier() {
    let mut shell = ranked();
    let top = json!({
        "tier": tier("legend", 10, 50_000), "next": null,
        "xp": 61_000, "percent": 100, "intoTier": 11_000, "tierSpan": 0,
    });
    shell.respond("POST", CHECK, 200, json!({ "throttled": false, "rank": top, "unlocked": [] }));
    let badge = shell.view::<RankView>(&Surface::Rank).rank.expect("rank");
    assert_eq!((badge.tier.code.as_str(), badge.next.code.as_str()), ("legend", "legend"));
    assert_eq!((badge.next.level, badge.percent), (10, 100));

    let me = Surface::Profile("admin".into());
    shell.send(Event::ScreenOpened(me.clone()));
    let mut payload = profile_payload("admin", true);
    payload["rank"] = top;
    shell.respond("GET", &format!("{API}/me/stats?lang=en"), 200, payload);
    let view: ProfileView = shell.view(&me);
    assert_eq!(view.status, LoadStatus::Loaded);
    assert_eq!(view.profile.expect("profile").rank.next.code, "legend");
}

#[test]
fn nothing_is_checked_while_rankings_are_off() {
    let mut shell = launched(returning(Platform::Ios, &[(1, "admin", Some("tok-1"))], 1));
    shell.answer_session(HTTPS, user(1, "admin"), Some("en"));
    shell.send(Event::AchievementsCheckRequested(CheckRequest { force: true }));
    assert!(shell.find_request("POST", CHECK).is_none());
    assert_eq!(shell.view::<RankView>(&Surface::Rank).rank, None);
}

fn profile_payload(username: &str, is_self: bool) -> Value {
    json!({
        "user": {
            "username": username, "displayName": username.to_uppercase(),
            "bio": "I watch **sci-fi**\\n<script>x</script>", "avatarId": "av", "bannerId": "bn",
            "bannerAccent": "#204060", "memberSince": "2025-01-01T00:00:00Z",
        },
        "isSelf": is_self, "public": true, "rank": rank(2, 140),
        "xp": { "total": 140, "sources": [{ "key": "watch_minutes", "units": 90, "rate": 1, "xp": 90 }] },
        "achievements": [achievement("first_play", true), achievement("watch_10h", false)],
        "achievementsWon": 1, "recentUnlocks": [achievement("first_play", true)],
        "totals": {
            "videoSeconds": 5400, "moviesCompleted": 1, "episodesCompleted": 2,
            "seriesCompleted": 0, "distinctTitles": 2, "distinctGenres": 3, "activeDays": 2,
            "currentStreak": 1, "longestStreak": 2, "bestDayMinutes": 60, "couchHosted": 0,
            "couchJoined": 1, "biggestCouch": 2, "emojiSent": 4,
        },
        "topTitles": [{ "slug": "glass-harbor", "name": "Glass Harbor", "kind": "series", "seconds": 3600, "posterId": "p1" }],
        "favouriteGenre": "Drama",
        "hours": [{ "hour": 21, "videoSeconds": 3600 }, { "hour": 7, "videoSeconds": 1800 }],
        "activity": { "from": "2026-09-30", "days": [0, 1800, 3600] },
    })
}

#[test]
fn profiles_read_your_own_stats_or_someone_elses_public_page() {
    let mut shell = ranked();
    let me = Surface::Profile("admin".into());
    shell.send(Event::ScreenOpened(me.clone()));
    shell.respond("GET", &format!("{API}/me/stats?lang=en"), 200, profile_payload("admin", true));

    let view: ProfileView = shell.view(&me);
    let profile = view.profile.expect("profile");
    assert!(profile.is_self);
    // the bio arrives as the safe markdown tree, raw HTML as text
    let bio = serde_json::to_string(&profile.bio).unwrap();
    assert!(bio.contains("\"strong\"") && bio.contains("<script>x</script>"));
    assert_eq!(profile.avatar.expect("avatar").url, format!("{API}/artwork/av?size=w342&g=g-art"));
    assert_eq!(profile.banner.expect("banner").accent.as_deref(), Some("#204060"));
    assert_eq!(profile.hours.len(), 24);
    assert_eq!((profile.hours[21], profile.hours[7], profile.hours[0]), (3600, 1800, 0));
    let levels: Vec<u8> = profile.heatmap.days.iter().map(|d| d.level).collect();
    assert_eq!(levels, [0, 2, 4]);
    assert_eq!(profile.top_titles[0].kind, crate::modules::catalog::TitleKind::Series);
    // your own profile also refreshes the rank badge
    assert_eq!(shell.view::<RankView>(&Surface::Rank).rank.expect("rank").tier.level, 2);

    let nora = Surface::Profile("nora".into());
    shell.send(Event::ScreenOpened(nora.clone()));
    shell.respond(
        "GET",
        &format!("{API}/users/nora/profile?lang=en"),
        404,
        json!({ "error": { "code": "not_found", "message": "no such member" } }),
    );
    assert_eq!(shell.view::<ProfileView>(&nora).status, LoadStatus::NotFound);
}

#[test]
fn leaderboards_sort_by_the_chosen_metric_without_refetching() {
    let mut shell = ranked();
    let row = |name: &str, xp: i64, watch: i64, achievements: i64, me: bool| {
        json!({
            "username": name, "displayName": name, "avatarId": null, "level": 1,
            "tierCode": "rookie", "xp": xp, "watchSeconds": watch, "achievements": achievements,
            "isSelf": me,
        })
    };
    let board = json!({
        "period": "week", "hidden": false, "total": 3, "me": null,
        "rows": [row("a", 300, 10, 0, false), row("b", 200, 30, 0, true), row("c", 100, 20, 0, false)],
    });
    let xp = Surface::Leaderboard(LeaderboardKey { period: Period::Week, metric: Metric::Xp });
    shell.send(Event::ScreenOpened(xp.clone()));
    shell.respond("GET", &format!("{API}/leaderboard?period=week"), 200, board);

    let view: LeaderboardView = shell.view(&xp);
    let names: Vec<&str> = view.rows.iter().map(|r| r.username.as_str()).collect();
    assert_eq!((names, view.my_position, view.podium), (vec!["a", "b", "c"], Some(2), true));

    let watch =
        Surface::Leaderboard(LeaderboardKey { period: Period::Week, metric: Metric::Watch });
    shell.send(Event::ScreenOpened(watch.clone()));
    assert!(
        shell.http_summary().iter().all(|r| !r.contains("leaderboard")),
        "one payload, every metric"
    );
    let view: LeaderboardView = shell.view(&watch);
    assert_eq!(view.rows[0].username, "b");
    assert_eq!((view.rows[0].position, view.rows[0].value, view.my_position), (1, 30, Some(1)));

    let achievements =
        Surface::Leaderboard(LeaderboardKey { period: Period::Week, metric: Metric::Achievements });
    let view: LeaderboardView = shell.view(&achievements);
    assert!(view.all_zero && !view.podium);
}

#[test]
fn hiding_your_profile_shows_at_once_and_rolls_back_on_failure() {
    let mut shell = ranked();
    let me = Surface::Profile("admin".into());
    shell.send(Event::ScreenOpened(me.clone()));
    shell.respond("GET", &format!("{API}/me/stats?lang=en"), 200, profile_payload("admin", true));

    shell.send(Event::ProfileVisibilityChanged(PublicChoice { public: false }));
    assert!(!shell.view::<ProfileView>(&me).profile.expect("profile").public);
    let (_, save) = shell.request("PUT", &format!("{API}/me/preferences"));
    assert_eq!(body(&save), json!({ "publicProfile": false }));
    shell.fail("PUT", &format!("{API}/me/preferences"), HttpFailureKind::Offline);
    assert!(shell.view::<ProfileView>(&me).profile.expect("profile").public);
    assert_eq!(shell.view::<NoticesView>(&Surface::Notices).notices[0].code, "visibility_failed");
}

#[test]
fn editing_the_profile_updates_every_place_it_shows() {
    let mut shell = ranked();
    shell.respond("POST", CHECK, 200, check_result(1, &[]));
    shell.send(Event::ProfileEditSubmitted(ProfileEdit {
        display_name: "  The Admin ".into(),
        bio: "Hi".into(),
    }));
    assert_eq!(
        shell.view::<ProfileEditorView>(&Surface::ProfileEditor).details.status,
        LoadStatus::Loading
    );
    let (_, patch) = shell.request("PATCH", &format!("{API}/me/profile"));
    assert_eq!(body(&patch), json!({ "displayName": "The Admin", "bio": "Hi" }));
    let mut updated = user(1, "admin");
    updated["displayName"] = json!("The Admin");
    shell.respond("PATCH", &format!("{API}/me/profile"), 200, updated);

    assert_eq!(
        shell.view::<ProfileEditorView>(&Surface::ProfileEditor).details.status,
        LoadStatus::Loaded
    );
    let session: SessionView = shell.view(&Surface::Session);
    assert_eq!(session.user.expect("user").display_name, "The Admin");
    assert!(shell.store["accounts"].contains("The Admin"));
    // the own profile reloads and a forced check looks for the profile achievements
    shell.request("GET", &format!("{API}/me/stats?lang=en"));
    shell.request("POST", CHECK);
}

#[test]
fn a_wrong_current_password_is_reported_without_signing_out() {
    let mut shell = ranked();
    shell.send(Event::PasswordChangeSubmitted(PasswordForm {
        current: "nope".into(),
        new: "longenough".into(),
    }));
    shell.respond(
        "PATCH",
        &format!("{API}/me/password"),
        400,
        json!({ "error": { "code": "invalid_password", "message": "current password is incorrect" } }),
    );
    let view: ProfileEditorView = shell.view(&Surface::ProfileEditor);
    assert_eq!(view.password.status, LoadStatus::Failed);
    assert_eq!(view.password.problem.expect("problem").code, "invalid_password");
    assert_eq!(shell.phase(), AppPhase::Ready);
}

#[test]
fn a_picked_image_is_uploaded_by_the_shell() {
    let mut shell = ranked();
    shell.send(Event::ImageChosen(ImageChoice { slot: ImageSlot::Avatar, file: "pick-1".into() }));
    let upload = shell
        .effects_waiting()
        .into_iter()
        .find_map(|e| match e.effect {
            Effect::Upload(u) => Some(u),
            _ => None,
        })
        .expect("an upload");
    assert_eq!((upload.file.as_str(), upload.field.as_str()), ("pick-1", "file"));
    assert_eq!(upload.request.url, format!("{API}/me/avatar"));
    assert_eq!(header(&upload.request, "Authorization"), Some("Bearer tok-1"));
    assert_eq!(upload.request.body, None);

    let mut updated = user(1, "admin");
    updated["avatarId"] = json!("av-new");
    shell.respond("POST", &format!("{API}/me/avatar"), 200, updated);
    let view: ProfileEditorView = shell.view(&Surface::ProfileEditor);
    assert_eq!(view.avatar.status, LoadStatus::Loaded);
    assert_eq!(
        shell.view::<SessionView>(&Surface::Session).user.expect("user").avatar_id.as_deref(),
        Some("av-new")
    );
}
