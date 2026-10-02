use serde_json::json;

use super::*;
use crate::modules::catalog::{
    BrowseKey, BrowseSort, BrowseView, GenresView, HomeRowKind, HomeView, MyListView, PlayKind,
    Quality, SearchText, SearchView, TitleKind, TitleView, WatchlistChange,
};
use crate::modules::notices::{NoticeRef, NoticesView};
use crate::modules::session::LanguageChoice;

const API: &str = "https://media.example.com/api/v1";

/// A phone signed in as the admin with its session loaded (display language English).
fn signed_in() -> Shell {
    let mut shell = launched(returning(Platform::Ios, &[(1, "admin", Some("tok-1"))], 1));
    shell.answer_session(HTTPS, user(1, "admin"), Some("en"));
    shell
}

fn open(shell: &mut Shell, surface: Surface) {
    shell.send(Event::ScreenOpened(surface));
}

fn card_item(id: &str, kind: &str) -> Value {
    json!({
        "titleId": id, "slug": format!("{id}-slug"), "name": format!("Name {id}"), "kind": kind,
        "year": 2024, "posterId": format!("p-{id}"), "posterVer": 11, "posterAccent": "#102030",
        "backdropId": null, "backdropVer": null, "backdropAccent": null,
    })
}

fn home_payload() -> Value {
    json!({
        "featured": [{
            "id": "t1", "slug": "glass-harbor", "name": "Glass Harbor", "kind": "series",
            "year": 2025, "overview": "Lighthouses.", "genres": ["Drama"], "genreLabels": ["Drama"],
            "contentRating": "", "runtimeMinutes": null, "backdropId": "b1", "backdropVer": 5,
            "backdropAccent": "#3a6ea5", "inList": false, "addedAt": "2026-09-01T00:00:00Z",
            "allowRandomPlayback": true, "metadataLanguages": ["en"], "releaseDate": null,
            "sortName": "Glass Harbor", "status": "published", "tmdbId": null,
            "updatedAt": "2026-09-01T00:00:00Z",
        }],
        "rows": [
            {
                "kind": "continue_watching", "label": "Continue watching", "items": [],
                "continueWatching": [{
                    "titleId": "t1", "slug": "glass-harbor", "name": "Glass Harbor",
                    "kind": "series", "episodeLabel": "S1 E2", "episodeId": "e2",
                    "playbackKind": "episode", "playbackId": "e2", "positionSeconds": 600,
                    "durationSeconds": 2400, "updatedAt": "2026-09-02T00:00:00Z", "year": 2025,
                    "posterId": null, "posterVer": null, "posterAccent": null,
                    "backdropId": "b1", "backdropVer": 5, "backdropAccent": "#3a6ea5",
                }],
            },
            { "kind": "recently_added", "label": "New", "items": [card_item("t2", "movie")], "continueWatching": [] },
            { "kind": "genre", "label": "Empty", "items": [], "continueWatching": [] },
            { "kind": "from_the_future", "label": "Later", "items": [card_item("t3", "movie")], "continueWatching": [] },
        ],
    })
}

fn episode(id: &str, number: i64, season: &str) -> Value {
    json!({
        "id": id, "episodeNumber": number, "name": format!("Episode {number}"), "overview": "",
        "seasonId": season, "airDate": null, "runtimeMinutes": 42, "thumbId": format!("th-{id}"),
        "thumbVer": 3,
    })
}

fn media_file(id: &str, episode: Option<&str>, height: i64, range: &str) -> Value {
    json!({
        "id": id, "episodeId": episode, "titleId": null, "height": height, "width": height * 16 / 9,
        "videoRange": range, "audioCodec": "aac", "audioLang": "en", "audioRole": "main",
        "bitrate": 1, "channels": 2, "container": "mp4", "createdAt": "2026-01-01T00:00:00Z",
        "directPlay": true, "durationSeconds": 2400.0, "fileMtime": null, "libraryId": 1,
        "path": "/x", "sampleRate": 48000, "scannedAt": null, "sizeBytes": 1,
        "sourceDeletedAt": null, "videoCodec": "h264",
    })
}

fn title_payload() -> Value {
    json!({
        "title": {
            "id": "t1", "slug": "glass-harbor", "name": "Glass Harbor", "kind": "series",
            "year": 2025, "overview": "Lighthouses.", "genres": ["Drama"], "genreLabels": ["Drama"],
            "contentRating": "15", "runtimeMinutes": null, "allowRandomPlayback": true,
            "addedAt": "2026-09-01T00:00:00Z", "metadataLanguages": ["en"], "releaseDate": null,
            "sortName": "Glass Harbor", "status": "published", "tmdbId": null,
            "updatedAt": "2026-09-01T00:00:00Z",
        },
        "artwork": [
            {
                "id": "b1", "kind": "backdrop", "accent": "#3a6ea5",
                "createdAt": "2026-10-02T12:00:00Z", "height": 1080, "width": 1920,
                "ownerId": "t1", "ownerKind": "title", "path": "x", "source": "tmdb",
            },
            {
                "id": "p1", "kind": "poster", "accent": null, "createdAt": "1970-01-01T00:00:10Z",
                "height": 750, "width": 500, "ownerId": "t1", "ownerKind": "title", "path": "x",
                "source": "tmdb",
            },
        ],
        "seasons": [
            {
                "id": "s1", "seasonNumber": 1, "name": "Season 1", "overview": "", "titleId": "t1",
                "episodes": [episode("e1", 1, "s1"), episode("e2", 2, "s1"), episode("e3", 3, "s1")],
            },
            { "id": "s2", "seasonNumber": 2, "name": "Season 2", "overview": "", "titleId": "t1", "episodes": [episode("e4", 1, "s2")] },
        ],
        "mediaFiles": [
            media_file("m1", Some("e1"), 2160, "hdr10"),
            media_file("m2", Some("e2"), 1080, "sdr"),
        ],
        "episodeProgress": {
            "e1": { "completed": true, "positionSeconds": 2390, "durationSeconds": 2400 },
            "e2": { "completed": false, "positionSeconds": 600, "durationSeconds": 2400 },
        },
        "progress": null,
        "inWatchlist": false,
    })
}

#[test]
fn home_shows_rows_with_ready_image_urls() {
    let mut shell = signed_in();
    open(&mut shell, Surface::Home);
    assert_eq!(shell.view::<HomeView>(&Surface::Home).status, LoadStatus::Loading);
    let (_, request) = shell.request("GET", &format!("{API}/home?lang=en"));
    assert_eq!(header(&request, "Authorization"), Some("Bearer tok-1"));
    shell.respond("GET", &format!("{API}/home?lang=en"), 200, home_payload());

    let home: HomeView = shell.view(&Surface::Home);
    assert_eq!(home.status, LoadStatus::Loaded);
    let hero = &home.featured[0];
    assert_eq!(hero.kind, TitleKind::Series);
    assert_eq!(hero.content_rating, None);
    let backdrop = hero.backdrop.as_ref().expect("backdrop");
    // a hero gets the original, carrying the account's artwork grant
    assert_eq!(backdrop.url, format!("{API}/artwork/b1?v=5&g=g-art"));
    assert_eq!(backdrop.accent.as_deref(), Some("#3a6ea5"));

    // the empty row and the row kind this client does not know are left out
    let kinds: Vec<HomeRowKind> = home.rows.iter().map(|r| r.kind).collect();
    assert_eq!(kinds, [HomeRowKind::ContinueWatching, HomeRowKind::RecentlyAdded]);
    let resume = &home.rows[0].continue_watching[0];
    assert_eq!(resume.play.kind, PlayKind::Episode);
    assert_eq!(resume.play.id, "e2");
    assert!((resume.progress - 0.25).abs() < f64::EPSILON);
    assert_eq!(resume.episode_label.as_deref(), Some("S1 E2"));
    let card = &home.rows[1].cards[0];
    assert_eq!(
        card.poster.as_ref().expect("poster").url,
        format!("{API}/artwork/p-t2?size=w342&v=11&g=g-art")
    );
    assert_eq!(card.backdrop, None);
}

#[test]
fn reopening_a_fresh_surface_does_not_refetch_but_a_stale_one_does() {
    let mut shell = signed_in();
    open(&mut shell, Surface::Home);
    shell.respond("GET", &format!("{API}/home?lang=en"), 200, home_payload());
    shell.send(Event::ScreenClosed(Surface::Home));

    shell.now += 30_000;
    open(&mut shell, Surface::Home);
    assert!(shell.find_request("GET", &format!("{API}/home?lang=en")).is_none());

    shell.send(Event::ScreenClosed(Surface::Home));
    shell.now += 60_000;
    open(&mut shell, Surface::Home);
    // stale beats blank: the old home stays while the new one loads
    assert_eq!(shell.view::<HomeView>(&Surface::Home).status, LoadStatus::Stale);
    shell.respond("GET", &format!("{API}/home?lang=en"), 200, home_payload());
    assert_eq!(shell.view::<HomeView>(&Surface::Home).status, LoadStatus::Loaded);

    // pull to refresh always reloads
    shell.send(Event::RefreshRequested(Surface::Home));
    shell.request("GET", &format!("{API}/home?lang=en"));
}

#[test]
fn a_cold_start_paints_the_last_home_before_the_network_answers() {
    let mut first = signed_in();
    open(&mut first, Surface::Home);
    first.respond("GET", &format!("{API}/home?lang=en"), 200, home_payload());
    assert!(first.store.contains_key(&format!("warm.{}.home", account_id(1))));

    let mut shell = first.relaunch(Platform::Ios);
    shell.send(Event::AppStarted);
    open(&mut shell, Surface::Home);
    shell.answer_reads();
    let home: HomeView = shell.view(&Surface::Home);
    assert_eq!(home.status, LoadStatus::Stale);
    assert_eq!(home.featured[0].name, "Glass Harbor");
    shell.request("GET", &format!("{API}/home?lang=cs"));
}

#[test]
fn title_detail_shows_only_playable_episodes_and_what_play_does() {
    let mut shell = signed_in();
    let surface = Surface::Title("glass-harbor".into());
    open(&mut shell, surface.clone());
    shell.respond("GET", &format!("{API}/titles/glass-harbor?lang=en"), 200, title_payload());

    let view: TitleView = shell.view(&surface);
    assert_eq!(view.status, LoadStatus::Loaded);
    let detail = view.detail.expect("detail");
    assert_eq!(detail.kind, TitleKind::Series);
    assert_eq!(detail.quality, Some(Quality::Uhd));
    assert!(detail.hdr);
    assert!(detail.shuffle);
    assert_eq!(detail.content_rating.as_deref(), Some("15"));
    // the backdrop is versioned by its creation time and colours the page
    assert_eq!(
        detail.backdrop.as_ref().expect("backdrop").url,
        format!("{API}/artwork/b1?v=1790942400&g=g-art")
    );
    assert_eq!(detail.accent.expect("accent").accent, "#3a6ea5");
    assert_eq!(
        detail.poster.expect("poster").url,
        format!("{API}/artwork/p1?size=w780&v=10&g=g-art")
    );

    // episode 3 has no file and season 2 only unplayable ones
    assert_eq!(detail.seasons.len(), 1);
    let episodes = &detail.seasons[0].episodes;
    assert_eq!(episodes.iter().map(|e| e.id.as_str()).collect::<Vec<_>>(), ["e1", "e2"]);
    assert!(episodes[0].completed);
    assert!((episodes[0].progress - 1.0).abs() < f64::EPSILON);
    assert_eq!(
        episodes[1].still.as_ref().expect("still").url,
        format!("{API}/artwork/th-e2?size=w780&v=3&g=g-art")
    );

    // play resumes the first episode not watched to the end
    let play = detail.play.expect("play");
    assert_eq!(play.target.id, "e2");
    assert_eq!(play.resume_seconds, Some(600));
    assert_eq!(play.episode.map(|e| (e.season, e.episode)), Some((1, 2)));
}

#[test]
fn an_unknown_title_is_not_found() {
    let mut shell = signed_in();
    let surface = Surface::Title("nope".into());
    open(&mut shell, surface.clone());
    shell.respond(
        "GET",
        &format!("{API}/titles/nope?lang=en"),
        404,
        json!({ "error": { "code": "not_found", "message": "no such title" } }),
    );
    let view: TitleView = shell.view(&surface);
    assert_eq!(view.status, LoadStatus::NotFound);
    assert_eq!(view.detail, None);
}

#[test]
fn listings_page_until_everything_is_loaded() {
    let mut shell = signed_in();
    let key = BrowseKey { kind: Some(TitleKind::Movie), genre: None, sort: BrowseSort::Name };
    open(&mut shell, Surface::Browse(key.clone()));
    let first = format!("{API}/titles?lang=en&kind=movie&sort=name&page=1");
    shell.respond(
        "GET",
        &first,
        200,
        json!({ "items": [card_item("a", "movie"), card_item("b", "movie")], "total": 3 }),
    );
    let view: BrowseView = shell.view(&Surface::Browse(key.clone()));
    assert_eq!((view.cards.len(), view.total, view.more), (2, 3, true));

    shell.send(Event::BrowseMoreRequested(key.clone()));
    assert!(shell.view::<BrowseView>(&Surface::Browse(key.clone())).loading_more);
    // a second request while one is in flight is ignored
    shell.send(Event::BrowseMoreRequested(key.clone()));
    let second = format!("{API}/titles?lang=en&kind=movie&sort=name&page=2");
    shell.respond("GET", &second, 200, json!({ "items": [card_item("c", "movie")], "total": 3 }));
    assert!(shell.find_request("GET", &second).is_none());

    let view: BrowseView = shell.view(&Surface::Browse(key.clone()));
    assert_eq!(view.cards.iter().map(|c| c.title_id.as_str()).collect::<Vec<_>>(), ["a", "b", "c"]);
    assert!(!view.more && !view.loading_more);
    shell.send(Event::BrowseMoreRequested(key));
    assert!(shell.http_summary().iter().all(|r| !r.contains("page=3")));
}

#[test]
fn search_waits_for_a_pause_and_drops_superseded_answers() {
    let mut shell = signed_in();
    open(&mut shell, Surface::Search);
    let search = |shell: &mut Shell, q: &str| {
        shell.send(Event::SearchChanged(SearchText { query: q.into() }));
    };
    search(&mut shell, "gl");
    let first_timer = shell.timers()[0].0;
    search(&mut shell, "gla");
    assert!(shell.was_cancelled(first_timer));
    let searches =
        |shell: &Shell| shell.http_summary().iter().filter(|r| r.contains("/search")).count();
    assert_eq!(searches(&shell), 0, "nothing is sent while typing");

    let (timer, after, _) = shell.timers()[0];
    assert_eq!(after, 250);
    shell.fire(timer, 250);
    let (old, _) = shell.request("GET", &format!("{API}/search?lang=en&q=gla"));

    // the user keeps typing before the answer arrives
    search(&mut shell, "glass");
    let timer = shell.timers()[0].0;
    shell.fire(timer, 250);
    let ok = |titles: Value| {
        EffectOutput::Http(HttpResponse {
            status: 200,
            body: json!({ "titles": titles }).to_string(),
        })
    };
    shell.resolve(old, ok(json!([card_item("old", "movie")])));
    assert_eq!(shell.view::<SearchView>(&Surface::Search).cards, vec![]);
    let (new, _) = shell.request("GET", &format!("{API}/search?lang=en&q=glass"));
    shell.resolve(new, ok(json!([card_item("t1", "series")])));
    let view: SearchView = shell.view(&Surface::Search);
    assert_eq!(view.query, "glass");
    assert_eq!(view.status, LoadStatus::Loaded);
    assert_eq!(view.cards[0].title_id, "t1");

    search(&mut shell, "  ");
    let view: SearchView = shell.view(&Surface::Search);
    assert_eq!((view.status, view.cards.len()), (LoadStatus::Idle, 0));
}

#[test]
fn my_list_changes_show_at_once_and_roll_back_on_failure() {
    let mut shell = signed_in();
    let surface = Surface::Title("glass-harbor".into());
    open(&mut shell, surface.clone());
    shell.respond("GET", &format!("{API}/titles/glass-harbor?lang=en"), 200, title_payload());

    shell.send(Event::WatchlistChanged(WatchlistChange { title_id: "t1".into(), listed: true }));
    assert!(shell.view::<TitleView>(&surface).detail.expect("detail").in_list, "optimistic");
    shell.respond("PUT", &format!("{API}/me/watchlist/t1"), 204, Value::Null);
    assert!(shell.view::<TitleView>(&surface).detail.expect("detail").in_list);

    shell.send(Event::WatchlistChanged(WatchlistChange { title_id: "t1".into(), listed: false }));
    shell.fail("DELETE", &format!("{API}/me/watchlist/t1"), HttpFailureKind::Offline);
    assert!(shell.view::<TitleView>(&surface).detail.expect("detail").in_list, "rolled back");
    let notices: NoticesView = shell.view(&Surface::Notices);
    assert_eq!(notices.notices[0].code, "watchlist_failed");
    shell.send(Event::NoticeDismissed(NoticeRef { id: notices.notices[0].id }));
    assert!(shell.view::<NoticesView>(&Surface::Notices).notices.is_empty());
}

#[test]
fn removing_from_my_list_drops_the_card() {
    let mut shell = signed_in();
    open(&mut shell, Surface::MyList);
    shell.respond(
        "GET",
        &format!("{API}/me/watchlist?lang=en"),
        200,
        json!([card_item("t1", "series"), card_item("t2", "movie")]),
    );
    shell.send(Event::WatchlistChanged(WatchlistChange { title_id: "t1".into(), listed: false }));
    let cards = |shell: &Shell| shell.view::<MyListView>(&Surface::MyList).cards.len();
    assert_eq!(cards(&shell), 1);
    shell.respond("DELETE", &format!("{API}/me/watchlist/t1"), 204, Value::Null);
    assert_eq!(cards(&shell), 1);
}

#[test]
fn switching_the_language_reloads_what_is_open_and_keeps_it_showing() {
    let mut shell = signed_in();
    open(&mut shell, Surface::Genres);
    shell.respond(
        "GET",
        &format!("{API}/genres?lang=en"),
        200,
        json!([{ "id": 1, "name": "Drama", "label": "Drama" }]),
    );
    shell.send(Event::DisplayLanguageChanged(LanguageChoice { code: "cs".into() }));
    let view: GenresView = shell.view(&Surface::Genres);
    assert_eq!(view.status, LoadStatus::Stale);
    assert_eq!(view.genres[0].label, "Drama");
    shell.respond(
        "GET",
        &format!("{API}/genres?lang=cs"),
        200,
        json!([{ "id": 1, "name": "Drama", "label": "Drama (cs)" }]),
    );
    let view: GenresView = shell.view(&Surface::Genres);
    assert_eq!((view.status, view.genres[0].label.as_str()), (LoadStatus::Loaded, "Drama (cs)"));
}

#[test]
fn answers_for_a_previous_account_or_language_are_dropped() {
    let users = [(1, "admin", Some("tok-1")), (2, "nora", Some("tok-2"))];
    let mut shell = launched(returning(Platform::Ios, &users, 1));
    open(&mut shell, Surface::Home);
    let (admins_home, _) = shell.request("GET", &format!("{API}/home?lang=cs"));
    shell.send(Event::AccountSelected(crate::modules::accounts::AccountRef {
        account_id: account_id(2),
    }));

    // nora's home loads for the screen that is still open
    let (noras_home, request) = shell
        .requests("GET", &format!("{API}/home?lang=cs"))
        .into_iter()
        .find(|(id, _)| *id != admins_home)
        .expect("nora's home");
    assert_eq!(header(&request, "Authorization"), Some("Bearer tok-2"));
    let ok = |v: Value| EffectOutput::Http(HttpResponse { status: 200, body: v.to_string() });
    shell.resolve(admins_home, ok(home_payload()));
    assert_eq!(shell.view::<HomeView>(&Surface::Home).status, LoadStatus::Loading);
    shell.resolve(noras_home, ok(json!({ "featured": [], "rows": [] })));
    assert_eq!(shell.view::<HomeView>(&Surface::Home).status, LoadStatus::Loaded);
}

#[test]
fn a_rejected_token_while_browsing_signs_the_account_out() {
    let mut shell = signed_in();
    open(&mut shell, Surface::Genres);
    shell.respond(
        "GET",
        &format!("{API}/genres?lang=en"),
        401,
        json!({ "error": { "code": "unauthorized", "message": "sign in" } }),
    );
    assert_eq!(shell.phase(), AppPhase::ChooseAccount);
    assert_eq!(shell.view::<GenresView>(&Surface::Genres).status, LoadStatus::Idle);
}

#[test]
fn the_web_builds_origin_relative_urls_without_grants() {
    let mut shell = Shell::new(Platform::Web);
    shell.send(Event::AppStarted);
    shell.answer_session("", user(1, "admin"), Some("en"));
    open(&mut shell, Surface::Home);
    shell.respond("GET", "/api/v1/home?lang=en", 200, home_payload());
    let home: HomeView = shell.view(&Surface::Home);
    assert_eq!(home.featured[0].backdrop.as_ref().expect("backdrop").url, "/api/v1/artwork/b1?v=5");
}
