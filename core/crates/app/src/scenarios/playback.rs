use serde_json::json;

use super::*;
use crate::messages::{AudioRendition, PlayerReport, PlayerSource};
use crate::modules::catalog::{PlayKind, PlayTarget};
use crate::modules::playback::{PlayerView, QualityChoice, QualityKind, TrackChoice};

const API: &str = "https://media.example.com/api/v1";
const GRANT: &str = "gr4nt";

fn signed_in() -> Shell {
    let mut shell = launched(returning(Platform::Ios, &[(1, "admin", Some("tok-1"))], 1));
    shell.answer_session(HTTPS, user(1, "admin"), Some("en"));
    shell
}

fn movie() -> PlayTarget {
    PlayTarget { kind: PlayKind::Movie, id: "m1".into() }
}

fn episode(id: &str) -> PlayTarget {
    PlayTarget { kind: PlayKind::Episode, id: id.into() }
}

fn info(mode: &str) -> Value {
    json!({
        "mode": mode, "mediaFileId": "f1", "grant": GRANT,
        "streamUrl": format!("{API}/media/{GRANT}/stream"),
        "hlsUrl": format!("{API}/media/{GRANT}/hls/master.m3u8"),
        "variants": [{ "name": "1080p", "height": 1080 }, { "name": "720p", "height": 720 }],
        "frameUrl": format!("{API}/media/{GRANT}/frame"),
        "durationSeconds": 2400.0, "resumePosition": 600, "allowRandomPlayback": false,
        "display": {
            "title": "Glass Harbor", "subtitle": "", "titleId": "t1", "titleSlug": "glass-harbor",
            "backdropId": "b1", "backdropVer": 5, "backdropAccent": "#3a6ea5",
        },
        "subtitles": [
            { "id": "s-en", "lang": "en", "label": "English", "forced": false, "url": format!("{API}/media/{GRANT}/subtitles/s-en.vtt") },
            { "id": "s-cs", "lang": "cs", "label": "Čeština", "forced": false, "url": format!("{API}/media/{GRANT}/subtitles/s-cs.vtt") },
        ],
    })
}

fn series_info(current: &str, next: Option<&str>) -> Value {
    let mut payload = info("direct");
    payload["currentEpisodeId"] = json!(current);
    payload["allowRandomPlayback"] = json!(true);
    payload["episodes"] = json!([
        { "episodeId": "e1", "episodeNumber": 1, "seasonNumber": 1, "name": "One", "thumbId": "th1", "thumbVer": 2 },
        { "episodeId": "e2", "episodeNumber": 2, "seasonNumber": 1, "name": "Two", "thumbId": null, "thumbVer": null },
        { "episodeId": "e3", "episodeNumber": 1, "seasonNumber": 2, "name": "Three", "thumbId": null, "thumbVer": null },
    ]);
    if let Some(next) = next {
        payload["nextEpisode"] = json!({
            "episodeId": next, "episodeNumber": 2, "seasonNumber": 1, "name": "Two",
            "titleId": "t1", "titleName": "Glass Harbor", "titleSlug": "glass-harbor",
        });
    }
    payload
}

fn play(shell: &mut Shell, target: PlayTarget, payload: Value) {
    let path = match target.kind {
        PlayKind::Movie => format!("{API}/playback/movie/{}?lang=en", target.id),
        PlayKind::Episode => format!("{API}/playback/episode/{}?lang=en", target.id),
    };
    shell.send(Event::PlayRequested(target));
    assert_eq!(shell.view::<PlayerView>(&Surface::Player).status, LoadStatus::Loading);
    shell.respond("GET", &path, 200, payload);
}

fn report(shell: &mut Shell, position: f64, playing: bool) {
    shell.send(Event::PlayerReported(PlayerReport {
        position_seconds: position,
        duration_seconds: 2400.0,
        playing,
        buffering: false,
        ended: false,
        failed: None,
    }));
}

fn last_load(shell: &Shell) -> crate::messages::PlayerLoad {
    shell
        .player
        .iter()
        .rev()
        .find_map(|c| match c {
            PlayerCommand::Load(load) => Some(load.clone()),
            _ => None,
        })
        .expect("a load")
}

#[test]
fn a_direct_movie_loads_the_file_and_resumes() {
    let mut shell = signed_in();
    play(&mut shell, movie(), info("direct"));

    let load = last_load(&shell);
    assert_eq!(load.url, format!("{API}/media/{GRANT}/stream"));
    assert_eq!(load.source, PlayerSource::File);
    assert!((load.start_seconds - 600.0).abs() < f64::EPSILON);
    assert!(load.autoplay && !load.linear);
    assert_eq!(load.subtitles.len(), 2);
    assert_eq!(load.now_playing.title, "Glass Harbor");
    assert_eq!(load.now_playing.artwork, Some(format!("{API}/artwork/b1?v=5&g=g-art")));

    let view: PlayerView = shell.view(&Surface::Player);
    assert_eq!(view.status, LoadStatus::Loaded);
    let keys: Vec<(&str, QualityKind)> =
        view.qualities.iter().map(|q| (q.key.as_str(), q.kind)).collect();
    assert_eq!(
        keys,
        [
            ("original", QualityKind::Original),
            ("auto", QualityKind::Auto),
            ("1080p", QualityKind::Rendition),
            ("720p", QualityKind::Rendition)
        ]
    );
    assert_eq!(view.quality, "original");
    assert_eq!(view.frame_url, Some(format!("{API}/media/{GRANT}/frame")));
}

#[test]
fn progress_counts_what_was_watched_and_not_what_was_skipped() {
    let mut shell = signed_in();
    play(&mut shell, movie(), info("direct"));
    report(&mut shell, 600.0, true);
    for second in 1..=10 {
        shell.now += 1_000;
        report(&mut shell, 600.0 + f64::from(second), true);
    }
    let (_, save) = shell.request("PUT", &format!("{API}/progress"));
    assert_eq!(
        body(&save),
        json!({ "titleId": "m1", "positionSeconds": 610, "durationSeconds": 2400, "watchedSeconds": 10 })
    );
    shell.respond("PUT", &format!("{API}/progress"), 204, Value::Null);

    // a seek is saved as such, adding nothing watched; pausing saves at once
    shell.now += 1_000;
    report(&mut shell, 1500.0, true);
    let (_, save) = shell.request("PUT", &format!("{API}/progress"));
    assert_eq!(
        (body(&save)["positionSeconds"].clone(), body(&save)["watchedSeconds"].clone()),
        (json!(1500), json!(0))
    );
    shell.respond("PUT", &format!("{API}/progress"), 204, Value::Null);
    shell.now += 1_000;
    report(&mut shell, 1501.0, false);
    let (_, save) = shell.request("PUT", &format!("{API}/progress"));
    assert_eq!(
        (body(&save)["positionSeconds"].clone(), body(&save)["watchedSeconds"].clone()),
        (json!(1501), json!(1))
    );
}

#[test]
fn qualities_and_audio_files_reload_where_playback_was() {
    let mut shell = signed_in();
    let mut payload = info("direct");
    payload["audio"] = json!([
        { "id": "a-en", "lang": "en", "label": "English", "default": true, "source": "file", "streamUrl": format!("{API}/media/{GRANT}/stream") },
        { "id": "a-cs", "lang": "cs", "label": "Čeština", "default": false, "source": "file", "streamUrl": format!("{API}/media/g-cs/stream") },
    ]);
    play(&mut shell, movie(), payload);
    report(&mut shell, 700.0, true);

    shell.send(Event::QualityChosen(QualityChoice { key: "720p".into() }));
    let load = last_load(&shell);
    assert_eq!((load.source, load.max_height), (PlayerSource::Hls, Some(720)));
    assert_eq!(load.url, format!("{API}/media/{GRANT}/hls/master.m3u8"));
    assert!((load.start_seconds - 700.0).abs() < f64::EPSILON && load.autoplay);
    assert_eq!(shell.view::<PlayerView>(&Surface::Player).quality, "720p");

    shell.send(Event::QualityChosen(QualityChoice { key: "original".into() }));
    shell.send(Event::AudioChosen(TrackChoice { id: Some("a-cs".into()) }));
    let load = last_load(&shell);
    assert_eq!(load.url, format!("{API}/media/g-cs/stream"));
    assert_eq!(load.audio_lang.as_deref(), Some("cs"));
    assert!(shell.store["player.prefs"].contains("\"audioLang\":\"cs\""));

    // the next title starts in the language picked last
    play(&mut shell, movie(), {
        let mut again = info("direct");
        again["audio"] = json!([
            { "id": "a-en", "lang": "en", "label": "English", "default": true, "source": "embedded" },
            { "id": "a-cs", "lang": "cs", "label": "Čeština", "default": false, "source": "embedded" },
            { "id": "a-dts", "lang": "en", "label": "DTS", "default": false, "source": "embedded" },
        ]);
        again
    });
    assert_eq!(shell.view::<PlayerView>(&Surface::Player).audio_selected.as_deref(), Some("a-cs"));
    shell.send(Event::AudioChosen(TrackChoice { id: Some("a-en".into()) }));
    let rendition = |r: &AudioRendition| (r.lang.clone(), r.index);
    assert!(
        matches!(shell.player.last(), Some(PlayerCommand::SelectAudio(r)) if rendition(r) == ("en".into(), Some(0)))
    );
    // a second track in the same language is told apart by its place
    shell.send(Event::AudioChosen(TrackChoice { id: Some("a-dts".into()) }));
    assert!(
        matches!(shell.player.last(), Some(PlayerCommand::SelectAudio(r)) if rendition(r) == ("en".into(), Some(2)))
    );
}

#[test]
fn subtitles_remember_their_language() {
    let mut shell = signed_in();
    play(&mut shell, movie(), info("direct"));
    shell.send(Event::SubtitlesChosen(TrackChoice { id: Some("s-cs".into()) }));
    assert!(
        matches!(shell.player.last(), Some(PlayerCommand::SelectSubtitles(s)) if s.id.as_deref() == Some("s-cs"))
    );
    play(&mut shell, movie(), info("direct"));
    assert_eq!(last_load(&shell).subtitle.as_deref(), Some("s-cs"));
    shell.send(Event::SubtitlesChosen(TrackChoice { id: None }));
    play(&mut shell, movie(), info("direct"));
    assert_eq!(last_load(&shell).subtitle, None);
}

#[test]
fn an_instant_play_session_is_kept_alive_and_stopped() {
    let mut shell = signed_in();
    play(&mut shell, movie(), info("jit"));
    let jit = format!("{API}/media/{GRANT}/jit");
    let (_, start) = shell.request("POST", &jit);
    assert_eq!(body(&start), json!({ "startAt": 600.0 }));
    shell.respond(
        "POST",
        &jit,
        201,
        json!({ "sessionId": "sid1", "playlistUrl": format!("{API}/media/{GRANT}/jit/sid1/index.m3u8") }),
    );
    let load = last_load(&shell);
    assert_eq!(
        (load.source, load.url.ends_with("/jit/sid1/index.m3u8")),
        (PlayerSource::Hls, true)
    );

    let (keepalive, every, repeat) = shell.timers()[0];
    assert_eq!((every, repeat), (15_000, true));
    shell.fire(keepalive, 15_000);
    shell.respond("POST", &format!("{jit}/sid1/keepalive"), 204, Value::Null);

    report(&mut shell, 640.0, true);
    shell.send(Event::PlayerClosed);
    assert!(shell.was_cancelled(keepalive));
    assert!(matches!(shell.player.last(), Some(PlayerCommand::Stop)));
    shell.request("DELETE", &format!("{jit}/sid1"));
    let (_, beacon) = shell.request("POST", &format!("{API}/progress"));
    assert_eq!(body(&beacon)["positionSeconds"], json!(640));
    assert_eq!(shell.view::<PlayerView>(&Surface::Player).status, LoadStatus::Idle);
}

#[test]
fn a_title_still_being_prepared_is_polled_until_it_plays() {
    let mut shell = signed_in();
    let mut preparing = info("preparing");
    preparing["jobProgress"] = json!(40);
    play(&mut shell, movie(), preparing);
    let view: PlayerView = shell.view(&Surface::Player);
    assert_eq!((view.status, view.preparing), (LoadStatus::Loading, Some(40)));
    assert_eq!(shell.player, empty::<PlayerCommand>());

    let (timer, after, _) = shell.timers()[0];
    assert_eq!(after, 3_000);
    shell.fire(timer, 3_000);
    shell.respond("GET", &format!("{API}/playback/movie/m1?lang=en"), 200, info("hls"));
    assert_eq!(last_load(&shell).source, PlayerSource::Hls);
    assert_eq!(shell.view::<PlayerView>(&Surface::Player).quality, "auto");
}

#[test]
fn the_next_episode_counts_down_and_plays_at_the_end() {
    let mut shell = signed_in();
    play(&mut shell, episode("e1"), series_info("e1", Some("e2")));
    let view: PlayerView = shell.view(&Surface::Player);
    assert_eq!(view.seasons.len(), 2);
    assert!(view.seasons[0].episodes[0].current);
    assert_eq!(
        view.seasons[0].episodes[0].still.as_ref().map(|s| s.url.clone()),
        Some(format!("{API}/artwork/th1?size=w780&v=2&g=g-art"))
    );

    report(&mut shell, 2384.6, true);
    let next = shell.view::<PlayerView>(&Surface::Player).next_up.expect("next up");
    assert_eq!((next.target.id.as_str(), next.countdown_seconds, next.shuffled), ("e2", 16, false));

    shell.send(Event::PlayerReported(PlayerReport {
        position_seconds: 2400.0,
        duration_seconds: 2400.0,
        playing: false,
        buffering: false,
        ended: true,
        failed: None,
    }));
    shell.request("PUT", &format!("{API}/progress"));
    shell.respond(
        "GET",
        &format!("{API}/playback/episode/e2?lang=en"),
        200,
        series_info("e2", None),
    );
    assert_eq!(shell.view::<PlayerView>(&Surface::Player).target, Some(episode("e2")));
}

#[test]
fn skipping_to_the_end_still_plays_the_next_episode() {
    let mut shell = signed_in();
    play(&mut shell, episode("e1"), series_info("e1", Some("e2")));
    report(&mut shell, 100.0, true);
    assert_eq!(shell.view::<PlayerView>(&Surface::Player).next_up, None);
    shell.send(Event::PlayerReported(PlayerReport {
        position_seconds: 2400.0,
        duration_seconds: 2400.0,
        playing: false,
        buffering: false,
        ended: true,
        failed: None,
    }));
    shell.request("GET", &format!("{API}/playback/episode/e2?lang=en"));
}

#[test]
fn a_dismissed_countdown_stops_at_the_end() {
    let mut shell = signed_in();
    play(&mut shell, episode("e1"), series_info("e1", Some("e2")));
    report(&mut shell, 2390.0, true);
    shell.send(Event::NextEpisodeCancelled);
    assert_eq!(shell.view::<PlayerView>(&Surface::Player).next_up, None);
    report(&mut shell, 2395.0, true);
    assert_eq!(shell.view::<PlayerView>(&Surface::Player).next_up, None);
    shell.send(Event::PlayerReported(PlayerReport {
        position_seconds: 2400.0,
        duration_seconds: 2400.0,
        playing: false,
        buffering: false,
        ended: true,
        failed: None,
    }));
    assert!(shell.find_request("GET", &format!("{API}/playback/episode/e2?lang=en")).is_none());
}

#[test]
fn shuffle_picks_another_episode_of_the_series() {
    let mut shell = signed_in();
    play(&mut shell, episode("e2"), series_info("e2", None));
    assert!(shell.view::<PlayerView>(&Surface::Player).shuffle_available);
    shell.send(Event::ShuffleToggled);
    assert!(shell.store["player.prefs"].contains("\"shuffle\":true"));
    report(&mut shell, 2390.0, true);
    let next = shell.view::<PlayerView>(&Surface::Player).next_up.expect("shuffled");
    assert!(next.shuffled && next.target.id != "e2");

    // the pick holds while the countdown runs
    report(&mut shell, 2391.0, true);
    let again = shell.view::<PlayerView>(&Surface::Player).next_up.expect("shuffled");
    assert_eq!((again.target, again.countdown_seconds), (next.target, 9));
}

#[test]
fn a_failed_player_reloads_once_with_a_fresh_grant() {
    let mut shell = signed_in();
    play(&mut shell, movie(), info("direct"));
    report(&mut shell, 900.0, true);
    let failed = |shell: &mut Shell| {
        shell.send(Event::PlayerReported(PlayerReport {
            position_seconds: 900.0,
            duration_seconds: 2400.0,
            playing: false,
            buffering: false,
            ended: false,
            failed: Some("forbidden".into()),
        }));
    };
    failed(&mut shell);
    assert_eq!(shell.view::<PlayerView>(&Surface::Player).status, LoadStatus::Stale);
    shell.respond("GET", &format!("{API}/playback/movie/m1?lang=en"), 200, info("direct"));
    assert!((last_load(&shell).start_seconds - 900.0).abs() < f64::EPSILON);

    failed(&mut shell);
    let view: PlayerView = shell.view(&Surface::Player);
    assert_eq!(view.status, LoadStatus::Failed);
    assert_eq!(view.problem.expect("problem").code, "playback_failed");
}

#[test]
fn unplayable_titles_and_rejected_sessions() {
    let mut shell = signed_in();
    play(&mut shell, movie(), info("unsupported"));
    let view: PlayerView = shell.view(&Surface::Player);
    assert_eq!(
        (view.status, view.problem.expect("problem").code.as_str()),
        (LoadStatus::Failed, "unsupported")
    );

    shell.send(Event::PlayRequested(movie()));
    shell.respond(
        "GET",
        &format!("{API}/playback/movie/m1?lang=en"),
        401,
        json!({ "error": { "code": "unauthorized", "message": "sign in" } }),
    );
    assert_eq!(shell.phase(), AppPhase::ChooseAccount);
}

#[test]
fn a_remux_plays_the_copied_source_and_keeps_the_ladder_for_auto() {
    let mut shell = signed_in();
    let mut payload = info("hls");
    payload["tier"] = json!("remux");
    payload["streamUrl"] = json!(format!("{API}/media/{GRANT}/hls/source/master.m3u8"));
    payload["originalUrl"] = payload["streamUrl"].clone();
    play(&mut shell, movie(), payload);

    let load = last_load(&shell);
    assert_eq!(load.url, format!("{API}/media/{GRANT}/hls/source/master.m3u8"));
    assert_eq!(load.source, PlayerSource::Hls);
    let view: PlayerView = shell.view(&Surface::Player);
    assert_eq!(view.qualities[0].kind, QualityKind::Original);
    assert_eq!(view.quality, "original");

    shell.send(Event::QualityChosen(QualityChoice { key: "auto".into() }));
    let load = last_load(&shell);
    assert_eq!(load.url, format!("{API}/media/{GRANT}/hls/master.m3u8"));
    assert_eq!(load.max_height, None);
}

#[test]
fn a_transcode_starts_on_auto_without_an_original() {
    let mut shell = signed_in();
    let mut payload = info("hls");
    payload["tier"] = json!("transcode");
    payload["streamUrl"] = payload["hlsUrl"].clone();
    play(&mut shell, movie(), payload);

    let load = last_load(&shell);
    assert_eq!(load.url, format!("{API}/media/{GRANT}/hls/master.m3u8"));
    assert_eq!(load.source, PlayerSource::Hls);
    let view: PlayerView = shell.view(&Surface::Player);
    assert_eq!(view.quality, "auto");
    assert!(view.qualities.iter().all(|q| q.kind != QualityKind::Original));
}

/// The shells' profiles mirror the server's exactly: every contract fixture survives the trip
/// through the core's types into the request body unchanged.
#[test]
fn a_reported_device_profile_decides_how_to_play() {
    for fixture in [
        include_str!("../../../../../contract/fixtures/device-profiles/apple-tv-4k.json"),
        include_str!("../../../../../contract/fixtures/device-profiles/chrome-desktop.json"),
        include_str!("../../../../../contract/fixtures/device-profiles/android-tv.json"),
    ] {
        let mut shell = signed_in();
        let profile: crate::modules::playback::DeviceProfile =
            serde_json::from_str(fixture).expect("the core reads the fixture");
        shell.send(Event::CapabilitiesReported(profile));
        shell.send(Event::PlayRequested(movie()));
        let (_, request) = shell.request("POST", &format!("{API}/playback/movie/m1?lang=en"));
        let sent = body(&request);
        let expected: Value = serde_json::from_str(fixture).expect("JSON");
        assert_eq!(canonical(&sent), canonical(&expected));
    }
}

#[test]
fn without_a_profile_the_browser_baseline_decides() {
    let mut shell = signed_in();
    shell.send(Event::PlayRequested(movie()));
    shell.request("GET", &format!("{API}/playback/movie/m1?lang=en"));
}

/// Absent and empty or zero values mean the same to the server, so they compare equal.
fn canonical(value: &Value) -> Value {
    match value {
        Value::Object(map) => Value::Object(
            map.iter()
                .filter(|(_, v)| {
                    !matches!(v, Value::Null | Value::Bool(false))
                        && v.as_array().is_none_or(|a| !a.is_empty())
                        && v.as_f64().is_none_or(|n| n != 0.0)
                })
                .map(|(k, v)| (k.clone(), canonical(v)))
                .collect(),
        ),
        Value::Array(items) => Value::Array(items.iter().map(canonical).collect()),
        Value::Number(n) => serde_json::json!(n.as_f64()),
        other => other.clone(),
    }
}
