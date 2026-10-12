use serde_json::json;

use super::*;
use crate::messages::{PlayerLoad, PlayerRate, PlayerReport, PlayerSeek, SocketClosed};
use crate::modules::catalog::{PlayKind, PlayTarget};
use crate::modules::couch::CouchReaction;
use crate::modules::couchtv::{
    CouchTvConnection, CouchTvCues, CouchTvGuideView, CouchTvItemKind, CouchTvJoin,
    CouchTvOnNowView, CouchTvStatus, CouchTvView,
};
use crate::modules::playback::TrackChoice;

const API: &str = "https://media.example.com/api/v1";
const SOCKET: &str = "wss://media.example.com/api/v1/tv/socket";
const GRANT: &str = "chan-grant";
/// The server's clock when the shell's monotonic one reads zero.
const SERVER_EPOCH: i64 = 1_791_043_200_000;
const MINUTE: i64 = 60_000;
/// Three target durations of 10 s: how far the picture is held behind the server's clock.
const LATENCY: i64 = 30_000;

fn signed_in() -> Shell {
    let mut shell = launched(returning(Platform::Ios, &[(2, "nora", Some("tok-2"))], 2));
    shell.answer_session(HTTPS, user(2, "nora"), Some("en"));
    shell
}

/// The server's clock now.
fn server_now(shell: &Shell) -> i64 {
    SERVER_EPOCH + i64::try_from(shell.now).unwrap()
}

fn iso(ms: i64) -> String {
    crate::time::rfc3339(u64::try_from(ms).unwrap())
}

/// A program of a series airing from `start` to `end` (server milliseconds).
fn program(id: &str, start: i64, end: i64) -> Value {
    json!({
        "id": id, "kind": "program", "startsAt": iso(start), "endsAt": iso(end), "fill": "strip",
        "titleId": format!("t-{id}"), "titleName": format!("Show {id}"), "titleSlug": format!("show-{id}"),
        "titleKind": "series", "episodeId": format!("e-{id}"), "seasonNumber": 2, "episodeNumber": 5,
        "episodeName": "Pilot", "overview": "Things happen.", "posterId": format!("p-{id}"), "posterVer": 7,
    })
}

/// The channel as tuning in finds it: program `a` on since ten minutes ago until 12 minutes from
/// now, a break, then `b` and `c`.
fn tune_answer(shell: &Shell) -> Value {
    let now = server_now(shell);
    let (a_end, b_start) = (now + 12 * MINUTE, now + 13 * MINUTE);
    json!({
        "grant": GRANT, "streamUrl": format!("/api/v1/tv/{GRANT}/live/master.m3u8?lang=en"),
        "serverTime": now, "targetDuration": 10, "languages": ["cs", "en"],
        "now": program("a", now - 10 * MINUTE, a_end),
        "next": program("b", b_start, b_start + 22 * MINUTE),
        "later": [program("c", b_start + 23 * MINUTE, b_start + 45 * MINUTE)],
    })
}

fn view(shell: &Shell) -> CouchTvView {
    shell.view(&Surface::CouchTv)
}

fn report_at(shell: &mut Shell, position: f64, program_date: i64) {
    shell.send(Event::PlayerReported(PlayerReport {
        position_seconds: position,
        duration_seconds: 0.0,
        playing: true,
        buffering: false,
        ended: false,
        failed: None,
        program_date_ms: Some(u64::try_from(program_date).unwrap()),
    }));
}

/// A report of the picture `drift` milliseconds ahead of where the hold keeps it.
fn report_drift(shell: &mut Shell, position: f64, drift: i64) {
    let program_date = server_now(shell) - LATENCY + drift;
    report_at(shell, position, program_date);
}

fn last_load(shell: &Shell) -> PlayerLoad {
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

fn loads(shell: &Shell) -> usize {
    shell.player.iter().filter(|c| matches!(c, PlayerCommand::Load(_))).count()
}

/// Nora tunes in, the socket opens and the server says hello.
fn tuned_in() -> (Shell, U53) {
    let mut shell = signed_in();
    shell.send(Event::CouchTvTuneRequested);
    let answer = tune_answer(&shell);
    shell.respond("GET", &format!("{API}/couchtv/tune?lang=en"), 200, answer);
    let socket = shell.socket(SOCKET);
    shell.resolve(socket, EffectOutput::SocketOpened);
    let hello = json!({ "type": "hello", "data": {
        "serverTime": server_now(&shell),
        "me": { "id": "user-2", "displayName": "Nora", "seed": "nora", "guest": false },
        "presence": { "count": 2, "viewers": [
            { "id": "user-2", "displayName": "Nora", "seed": "nora", "guest": false, "avatarId": "av-2" },
            { "id": "guest-g1", "displayName": "Babička", "seed": "g1", "guest": true },
        ] },
    } });
    shell.frame(socket, &hello);
    (shell, socket)
}

#[test]
fn tuning_in_mid_program_plays_the_channel_at_the_shared_moment() {
    let (mut shell, socket) = tuned_in();
    let load = last_load(&shell);
    assert_eq!(load.url, format!("{API}/tv/{GRANT}/live/master.m3u8?lang=en"));
    assert_eq!(load.source, crate::messages::PlayerSource::Hls);
    assert!(load.autoplay && load.linear);
    let live = load.live.expect("a live stream");
    assert!(
        (live.latency_seconds - 30.0).abs() < 1e-9
            && (live.target_duration_seconds - 10.0).abs() < 1e-9
    );
    // the display language's audio; subtitles off, both languages on offer
    assert_eq!(load.audio_lang.as_deref(), Some("en"));
    assert_eq!(load.subtitle, None);
    let subtitles: Vec<_> =
        load.subtitles.iter().map(|s| (s.id.as_str(), s.url.is_none())).collect();
    assert_eq!(subtitles, vec![("cs", true), ("en", true)]);
    assert_eq!(load.now_playing.title, "Show a");

    let tv = view(&shell);
    assert_eq!((tv.status, tv.connection), (CouchTvStatus::Tuning, CouchTvConnection::Open));
    assert_eq!(tv.viewers, 2);
    assert!(tv.members[0].me && tv.members[1].guest && !tv.members[1].me);
    assert_eq!(
        tv.members[0].avatar.as_ref().map(|a| a.url.clone()),
        Some(format!("{API}/artwork/av-2?size=w342&g=g-art"))
    );
    assert_eq!(tv.now.as_ref().map(|n| n.id.as_str()), Some("a"));

    // the player shows the picture exactly where everyone else is: nothing to correct
    shell.now += 2_000;
    let on_target = server_now(&shell) - LATENCY;
    report_at(&mut shell, 600.0, on_target);
    shell.now += 2_000;
    report_at(&mut shell, 602.0, on_target + 2_000);
    let corrections: Vec<_> = shell
        .player
        .iter()
        .filter(|c| matches!(c, PlayerCommand::Rate(_) | PlayerCommand::Seek(_)))
        .collect();
    assert!(corrections.is_empty(), "{corrections:?}");

    let tv = view(&shell);
    assert_eq!(tv.status, CouchTvStatus::OnAir);
    let now = tv.now.expect("what is on");
    assert_eq!(now.kind, CouchTvItemKind::Program);
    assert_eq!(now.title.as_deref(), Some("Show a"));
    assert_eq!(now.episode.as_ref().map(|e| (e.season, e.number)), Some((2, 5)));
    assert_eq!(now.play, Some(PlayTarget { kind: PlayKind::Episode, id: "e-a".into() }));
    assert_eq!(tv.next.map(|n| n.id), Some("b".into()));
    assert_eq!(tv.later.len(), 1);
    assert!(tv.cues.bug && !tv.cues.now_caption && !tv.cues.next_caption);
    assert_eq!(tv.program_date_ms, Some(u64::try_from(on_target + 2_000).unwrap()));
    assert_eq!(tv.program_date_at_ms, shell.now);

    // the room hears what the picture shows, every ten seconds
    let reports: Vec<_> =
        shell.sent_frames(socket).into_iter().filter(|f| f["type"] == "report").collect();
    assert_eq!(reports, vec![json!({ "type": "report", "data": { "programDate": on_target } })]);
    shell.now += 10_000;
    report_at(&mut shell, 612.0, on_target + 12_000);
    let reports = shell.sent_frames(socket).into_iter().filter(|f| f["type"] == "report").count();
    assert_eq!(reports, 2);
}

#[test]
fn the_overlays_follow_the_pictures_clock_across_a_program_boundary() {
    let (mut shell, _) = tuned_in();
    let a_end = server_now(&shell) + 12 * MINUTE;
    let b_start = a_end + MINUTE;
    // the server's clock is past the end of a, the picture is not: still a, Next up
    shell.now = u64::try_from(a_end - SERVER_EPOCH + LATENCY - 115_000).unwrap();
    report_drift(&mut shell, 700.0, 0);
    shell.take_renders();
    shell.now += 1_000;
    report_drift(&mut shell, 701.0, 0);
    assert!(!shell.take_renders().contains(&Surface::CouchTv), "nothing changed on screen");
    let tv = view(&shell);
    assert_eq!(tv.now.map(|n| n.id), Some("a".into()));
    assert!(tv.cues.next_caption && tv.cues.bug);

    // a has left the air by the server's clock, still on screen for another few seconds
    shell.now = u64::try_from(a_end - SERVER_EPOCH + LATENCY - 2_000).unwrap();
    report_drift(&mut shell, 800.0, 0);
    let tv = view(&shell);
    assert_eq!(tv.now.map(|n| n.id), Some("a".into()));
    assert!(!tv.cues.next_caption);

    // the picture reaches the break: no bug over the channel's own graphics
    shell.now += 3_000;
    shell.take_renders();
    report_drift(&mut shell, 803.0, 0);
    assert!(shell.take_renders().contains(&Surface::CouchTv));
    let tv = view(&shell);
    let now = tv.now.expect("the break");
    assert_eq!((now.kind, now.id.as_str()), (CouchTvItemKind::Break, "break-b"));
    assert_eq!(now.starts_at_ms, u64::try_from(a_end).unwrap());
    assert_eq!(tv.cues, CouchTvCues::default());
    assert_eq!(tv.next.map(|n| n.id), Some("b".into()));

    // b starts on the picture, with its Now caption for a few seconds
    shell.now = u64::try_from(b_start - SERVER_EPOCH + LATENCY + 1_000).unwrap();
    report_drift(&mut shell, 860.0, 0);
    let tv = view(&shell);
    assert_eq!(tv.now.map(|n| n.id), Some("b".into()));
    assert!(tv.cues.bug && tv.cues.now_caption);
    assert_eq!(tv.next.map(|n| n.id), Some("c".into()));
    shell.now += 8_000;
    report_drift(&mut shell, 868.0, 0);
    assert!(!view(&shell).cues.now_caption);
}

#[test]
fn the_hold_plays_a_small_drift_away_and_seeks_a_large_one() {
    let (mut shell, _) = tuned_in();
    // settled after the load
    shell.now += 4_000;
    let target = |shell: &Shell| server_now(shell) - LATENCY;
    let rates = |shell: &Shell| -> Vec<f64> {
        shell
            .player
            .iter()
            .filter_map(|c| match c {
                PlayerCommand::Rate(PlayerRate { rate }) => Some(*rate),
                _ => None,
            })
            .collect()
    };

    // a quarter of a second ahead: 5 % slower
    report_drift(&mut shell, 100.0, 250);
    assert_eq!(rates(&shell), vec![0.95]);
    // a second behind: as fast as it goes, 10 %
    shell.now += 1_000;
    report_drift(&mut shell, 101.0, -1_000);
    assert_eq!(rates(&shell), vec![0.95, 1.1]);
    // caught up: real time again, said once
    shell.now += 1_000;
    report_drift(&mut shell, 102.0, 40);
    shell.now += 1_000;
    report_drift(&mut shell, 103.0, -30);
    assert_eq!(rates(&shell), vec![0.95, 1.1, 1.0]);

    // a stall left it five seconds behind: a seek, then it settles before the next correction
    shell.now += 1_000;
    report_drift(&mut shell, 104.0, -5_000);
    let seek = shell.player.iter().rev().find_map(|c| match c {
        PlayerCommand::Seek(PlayerSeek { seconds }) => Some(*seconds),
        _ => None,
    });
    assert_eq!(seek, Some(109.0));
    let commands = shell.player.len();
    shell.now += 1_000;
    report_drift(&mut shell, 109.0, 600);
    assert_eq!(shell.player.len(), commands, "a seeking player is left to settle");
    shell.now += 3_000;
    report_drift(&mut shell, 112.0, 600);
    assert_eq!(rates(&shell).last(), Some(&0.9));

    // a buffering player is not pushed
    let commands = shell.player.len();
    shell.now += 1_000;
    shell.send(Event::PlayerReported(PlayerReport {
        position_seconds: 113.0,
        duration_seconds: 0.0,
        playing: true,
        buffering: true,
        ended: false,
        failed: None,
        program_date_ms: Some(u64::try_from(target(&shell) - 1_500).unwrap()),
    }));
    assert_eq!(shell.player.len(), commands);
}

#[test]
fn a_guest_joins_through_the_channel_link_and_watches_the_live_channel_only() {
    let mut shell = Shell::new(Platform::Web);
    shell.send(Event::AppStarted);
    let signed_out = json!({ "error": { "code": "unauthorized", "message": "sign in" } });
    shell.respond("GET", "/api/v1/auth/me", 401, signed_out);

    // a code that does not work says why
    shell.send(Event::CouchTvJoinRequested(CouchTvJoin {
        code: "old".into(),
        name: "Babička".into(),
    }));
    assert!(view(&shell).joining);
    let gone = json!({ "error": { "code": "no_channel_link", "message": "this Couch TV link does not work" } });
    shell.respond("POST", "/api/v1/tv/join/old", 404, gone);
    let tv = view(&shell);
    assert!(!tv.joining);
    assert_eq!(tv.problem.map(|p| p.code), Some("no_channel_link".into()));

    shell.send(Event::CouchTvJoinRequested(CouchTvJoin {
        code: " c0de ".into(),
        name: " Babička ".into(),
    }));
    let (_, join) = shell.request("POST", "/api/v1/tv/join/c0de");
    assert_eq!(body(&join), json!({ "name": "Babička" }));
    shell.respond("POST", "/api/v1/tv/join/c0de", 200, json!({ "id": "g1", "name": "Babička" }));
    let tv = view(&shell);
    assert_eq!((tv.guest.as_deref(), tv.problem), (Some("Babička"), None));

    // the channel cookie says who the guest is: no token anywhere
    shell.send(Event::CouchTvTuneRequested);
    let (_, tune) = shell.request("GET", "/api/v1/tv/tune?lang=cs");
    assert_eq!(header(&tune, "X-Couch-Tv-Token"), None);
    let mut answer = tune_answer(&shell);
    answer["streamUrl"] = json!(format!("/api/v1/tv/{GRANT}/live/master.m3u8?lang=cs"));
    answer["artworkGrant"] = json!("g-tv");
    answer["guest"] = json!({ "id": "g1", "name": "Babička" });
    shell.respond("GET", "/api/v1/tv/tune?lang=cs", 200, answer);
    let load = last_load(&shell);
    assert_eq!(load.url, format!("/api/v1/tv/{GRANT}/live/master.m3u8?lang=cs"));
    assert_eq!(load.audio_lang.as_deref(), Some("cs"));
    let socket = shell.socket("/api/v1/tv/socket");
    assert_eq!(shell.sockets.last().expect("the socket").1.headers, empty());

    let tv = view(&shell);
    let now = tv.now.expect("what is on");
    // no "Watch from the start" without an account; artwork through the channel's grant
    assert_eq!(now.play, None);
    assert_eq!(now.poster.map(|p| p.url), Some("/api/v1/artwork/p-a?size=w342&v=7&g=g-tv".into()));
    assert_eq!(tv.guest.as_deref(), Some("Babička"));

    // the link was rotated: the guest's stream fails and tuning in again is refused
    shell.resolve(socket, EffectOutput::SocketOpened);
    shell.send(Event::PlayerReported(PlayerReport {
        position_seconds: 3.0,
        duration_seconds: 0.0,
        playing: false,
        buffering: false,
        ended: false,
        failed: Some("hls fragLoadError".into()),
        program_date_ms: None,
    }));
    let ended = json!({ "error": { "code": "no_channel_guest", "message": "join through the channel link first" } });
    shell.respond("GET", "/api/v1/tv/tune?lang=cs", 401, ended);
    let tv = view(&shell);
    assert_eq!(tv.status, CouchTvStatus::Failed);
    assert_eq!(tv.problem.map(|p| p.code), Some("no_channel_guest".into()));
    assert_eq!(tv.guest, None);
    assert_eq!(shell.player.last(), Some(&PlayerCommand::Stop));
    assert!(shell.closed_sockets.contains(&socket));
    // a guest is not a signed-out account
    assert_eq!(shell.phase(), AppPhase::SignIn);
}

#[test]
fn reactions_float_up_for_everyone_and_go() {
    let (mut shell, socket) = tuned_in();
    shell.send(Event::CouchTvReactionSent(CouchReaction { emoji: "🍿".into() }));
    let sent = shell.sent_frames(socket);
    assert_eq!(sent.last(), Some(&json!({ "type": "reaction", "data": { "emoji": "🍿" } })));

    shell.frame(
        socket,
        &json!({ "type": "reaction", "data": { "viewerId": "guest-g1", "emoji": "😂" } }),
    );
    let tv = view(&shell);
    assert_eq!(tv.reactions.len(), 1);
    assert_eq!(
        (tv.reactions[0].viewer_id.as_str(), tv.reactions[0].emoji.as_str()),
        ("guest-g1", "😂")
    );
    let gone = shell.timers().iter().find(|(_, after, _)| *after == 2_200).expect("a fade").0;
    shell.fire(gone, 2_200);
    assert_eq!(view(&shell).reactions, empty());

    shell.frame(
        socket,
        &json!({ "type": "presence", "data": { "count": 1, "viewers": [
        { "id": "user-2", "displayName": "Nora", "seed": "nora", "guest": false },
    ] } }),
    );
    assert_eq!(view(&shell).viewers, 1);
}

#[test]
fn a_schedule_change_reads_what_is_on_again_without_reloading_the_picture() {
    let (mut shell, socket) = tuned_in();
    shell.send(Event::ScreenOpened(Surface::CouchTvGuide));
    let guide =
        shell.http_summary().into_iter().find(|r| r.contains("/couchtv/guide")).expect("the guide");
    let guide_url = guide.trim_start_matches("GET ").to_string();
    shell.respond("GET", &guide_url, 200, guide_answer(&shell));

    shell.frame(socket, &json!({ "type": "schedule_changed" }));
    let mut answer = tune_answer(&shell);
    let b_start = server_now(&shell) + 13 * MINUTE;
    answer["next"] = program("d", b_start, b_start + 30 * MINUTE);
    answer["later"] = json!([]);
    shell.respond("GET", &format!("{API}/couchtv/tune?lang=en"), 200, answer);
    assert_eq!(loads(&shell), 1, "the picture plays on");
    assert_eq!(view(&shell).next.map(|n| n.id), Some("d".into()));
    // the guide was read again too
    assert!(shell.http_summary().iter().any(|r| r.contains("/couchtv/guide")));
}

#[test]
fn a_dropped_socket_reconnects_and_a_failing_player_is_tuned_in_again() {
    let (mut shell, socket) = tuned_in();
    shell.resolve(
        socket,
        EffectOutput::SocketClosed(SocketClosed { code: 1006, reason: String::new() }),
    );
    assert_eq!(view(&shell).connection, CouchTvConnection::Reconnecting);
    let retry = shell.timers().iter().find(|(_, after, _)| *after == 500).expect("a retry").0;
    // the server is restarting: the tune-in fails, and the next retry waits longer
    shell.fire(retry, 500);
    shell.fail("GET", &format!("{API}/couchtv/tune?lang=en"), HttpFailureKind::Offline);
    let retry =
        shell.timers().iter().find(|(_, after, _)| *after == 1_000).expect("a longer retry").0;
    shell.fire(retry, 1_000);
    let answer = tune_answer(&shell);
    shell.respond("GET", &format!("{API}/couchtv/tune?lang=en"), 200, answer);
    assert_eq!(loads(&shell), 1, "reconnecting the room leaves the picture alone");
    let again = shell.socket(SOCKET);
    assert_ne!(again, socket);
    shell.resolve(again, EffectOutput::SocketOpened);
    assert_eq!(view(&shell).connection, CouchTvConnection::Open);

    // an expired grant fails the player: a fresh tune-in loads it again
    shell.send(Event::PlayerReported(PlayerReport {
        position_seconds: 3.0,
        duration_seconds: 0.0,
        playing: false,
        buffering: false,
        ended: false,
        failed: Some("hls manifestLoadError".into()),
        program_date_ms: None,
    }));
    assert_eq!(view(&shell).status, CouchTvStatus::Tuning);
    let answer = tune_answer(&shell);
    shell.respond("GET", &format!("{API}/couchtv/tune?lang=en"), 200, answer);
    assert_eq!(loads(&shell), 2);

    // closing the screen stops the player and leaves the room
    shell.send(Event::CouchTvClosed);
    assert_eq!(shell.player.last(), Some(&PlayerCommand::Stop));
    assert!(shell.closed_sockets.contains(&again));
    assert_eq!(view(&shell).status, CouchTvStatus::Idle);
}

#[test]
fn tuned_in_off_air_the_player_waits_for_the_next_airing() {
    let mut shell = signed_in();
    shell.send(Event::CouchTvTuneRequested);
    let mut answer = tune_answer(&shell);
    let starts = server_now(&shell) + 3 * MINUTE;
    answer["now"] = Value::Null;
    answer["next"] = program("b", starts, starts + 22 * MINUTE);
    shell.respond("GET", &format!("{API}/couchtv/tune?lang=en"), 200, answer);
    assert_eq!(loads(&shell), 0, "no playlists to load yet");
    assert_eq!(view(&shell).status, CouchTvStatus::OffAir);
    // the room is open meanwhile
    shell.socket(SOCKET);

    let due = shell
        .timers()
        .iter()
        .find(|(_, after, _)| *after == 3 * 60_000)
        .expect("the next airing")
        .0;
    shell.fire(due, 3 * 60_000);
    let answer = tune_answer(&shell);
    shell.respond("GET", &format!("{API}/couchtv/tune?lang=en"), 200, answer);
    assert_eq!(loads(&shell), 1);
    assert_eq!(view(&shell).status, CouchTvStatus::Tuning);
}

#[test]
fn the_channels_languages_are_chosen_on_the_player() {
    let (mut shell, _) = tuned_in();
    shell.send(Event::CouchTvSubtitlesChosen(TrackChoice { id: Some("cs".into()) }));
    shell.send(Event::CouchTvAudioChosen(TrackChoice { id: Some("cs".into()) }));
    shell.send(Event::CouchTvAudioChosen(TrackChoice { id: Some("de".into()) }));
    let commands: Vec<_> = shell
        .player
        .iter()
        .filter(|c| matches!(c, PlayerCommand::SelectAudio(_) | PlayerCommand::SelectSubtitles(_)))
        .cloned()
        .collect();
    assert_eq!(
        commands,
        vec![
            PlayerCommand::SelectSubtitles(crate::messages::SubtitleSelection {
                id: Some("cs".into())
            }),
            PlayerCommand::SelectAudio(crate::messages::AudioRendition {
                lang: "cs".into(),
                index: None
            }),
        ]
    );
    let tv = view(&shell);
    assert!(tv.audio.iter().any(|t| t.id == "cs" && t.selected));
    assert!(tv.subtitles.iter().any(|t| t.id == "cs" && t.selected));

    // the choices carry over to the next tune-in
    shell.send(Event::CouchTvClosed);
    shell.send(Event::CouchTvTuneRequested);
    let answer = tune_answer(&shell);
    shell.respond("GET", &format!("{API}/couchtv/tune?lang=en"), 200, answer);
    let load = last_load(&shell);
    assert_eq!((load.audio_lang.as_deref(), load.subtitle.as_deref()), (Some("cs"), Some("cs")));
}

#[test]
fn watching_from_the_start_takes_the_player_over_from_the_channel() {
    let (mut shell, socket) = tuned_in();
    shell.send(Event::PlayRequested(PlayTarget { kind: PlayKind::Episode, id: "e-a".into() }));
    assert!(shell.closed_sockets.contains(&socket));
    assert!(shell.find_request("GET", &format!("{API}/playback/episode/e-a?lang=en")).is_some());
    // the title's player reports to playback, not to the channel
    shell.send(Event::PlayerReported(PlayerReport {
        position_seconds: 1.0,
        duration_seconds: 1300.0,
        playing: true,
        buffering: false,
        ended: false,
        failed: None,
        program_date_ms: None,
    }));
    assert_eq!(view(&shell).status, CouchTvStatus::Idle);
}

fn guide_answer(shell: &Shell) -> Value {
    let now = server_now(shell);
    let day = |offset: i64| now - 2 * 3_600_000 + offset * 24 * 3_600_000;
    json!({
        "from": iso(now - 24 * 3_600_000), "to": iso(now + 7 * 24 * 3_600_000), "timeZone": "Europe/Prague",
        "days": [
            { "date": "2026-10-10", "startsAt": iso(day(-1)), "endsAt": iso(day(0)) },
            { "date": "2026-10-11", "startsAt": iso(day(0)), "endsAt": iso(day(1)) },
            { "date": "2026-10-12", "startsAt": iso(day(1)), "endsAt": iso(day(2)) },
        ],
        "programs": [
            program("y", day(-1) + MINUTE, day(-1) + 30 * MINUTE),
            program("a", day(0) + MINUTE, day(0) + 30 * MINUTE),
            program("b", day(1), day(1) + 30 * MINUTE),
        ],
    })
}

#[test]
fn the_guide_lists_today_and_the_days_after_it() {
    let mut shell = signed_in();
    shell.send(Event::ScreenOpened(Surface::CouchTvGuide));
    // without the server's clock yet, the shell's wall clock places the week
    let wall = i64::try_from(WALL_EPOCH + shell.now).unwrap();
    let url = format!(
        "{API}/couchtv/guide?lang=en&from={}&to={}",
        iso(wall - 24 * 3_600_000).replace(':', "%3A"),
        iso(wall + 7 * 24 * 3_600_000).replace(':', "%3A")
    );
    assert_eq!(shell.view::<CouchTvGuideView>(&Surface::CouchTvGuide).status, LoadStatus::Loading);
    let mut answer = guide_answer(&shell);
    // its days run around the wall clock in this test
    let day = |offset: i64| wall - 2 * 3_600_000 + offset * 24 * 3_600_000;
    answer["days"] = json!([
        { "date": "2026-10-01", "startsAt": iso(day(-1)), "endsAt": iso(day(0)) },
        { "date": "2026-10-02", "startsAt": iso(day(0)), "endsAt": iso(day(1)) },
    ]);
    answer["programs"] = json!([
        program("y", day(-1) + MINUTE, day(-1) + 30 * MINUTE),
        program("a", day(0) + MINUTE, day(0) + 30 * MINUTE),
    ]);
    shell.respond("GET", &url, 200, answer);
    let guide: CouchTvGuideView = shell.view(&Surface::CouchTvGuide);
    assert_eq!(guide.status, LoadStatus::Loaded);
    assert_eq!(guide.time_zone, "Europe/Prague");
    // yesterday's broadcast day is over: today first
    let days: Vec<_> = guide.days.iter().map(|d| (d.date.as_str(), d.programs.len())).collect();
    assert_eq!(days, vec![("2026-10-02", 1)]);
    let a = &guide.days[0].programs[0];
    assert_eq!(a.title.as_deref(), Some("Show a"));
    assert!(a.play.is_some(), "signed in: watch it from the start");
}

#[test]
fn the_home_card_shows_what_is_on_and_asks_again_when_it_ends() {
    let mut shell = signed_in();
    shell.send(Event::ScreenOpened(Surface::CouchTvOnNow));
    let answer = tune_answer(&shell);
    shell.respond("GET", &format!("{API}/couchtv/tune?lang=en"), 200, answer);
    let card: CouchTvOnNowView = shell.view(&Surface::CouchTvOnNow);
    assert_eq!(card.status, LoadStatus::Loaded);
    assert_eq!(card.now.map(|n| n.id), Some("a".into()));
    assert_eq!(card.next.map(|n| n.id), Some("b".into()));
    // a ends in twelve minutes: the card asks again then, and not while it is closed
    let due =
        shell.timers().iter().find(|(_, after, _)| *after == 12 * 60_000).expect("asks again").0;
    shell.fire(due, 12 * 60_000);
    assert!(shell.find_request("GET", &format!("{API}/couchtv/tune?lang=en")).is_some());
    shell.send(Event::ScreenClosed(Surface::CouchTvOnNow));
    let answer = tune_answer(&shell);
    shell.respond("GET", &format!("{API}/couchtv/tune?lang=en"), 200, answer);
    assert!(shell.timers().iter().all(|(_, after, _)| *after != 12 * 60_000));
}
