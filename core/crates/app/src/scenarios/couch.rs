use serde_json::json;

use super::*;
use crate::messages::{PlayerReport, PlayerSeek, SocketClosed};
use crate::modules::catalog::{PlayKind, PlayTarget};
use crate::modules::couch::{
    CouchCode, CouchPause, CouchReaction, CouchRole, CouchStatus, CouchView, RemoteAction,
    RemoteControl,
};
use crate::modules::notices::NoticesView;

const API: &str = "https://media.example.com/api/v1";
const SOCKET: &str = "wss://media.example.com/api/v1/couch/123456/ws";
const GRANT: &str = "gr4nt";

fn signed_in() -> Shell {
    let mut shell = launched(returning(Platform::Ios, &[(1, "admin", Some("tok-1"))], 1));
    shell.answer_session(HTTPS, user(1, "admin"), Some("en"));
    shell
}

fn player_info() -> Value {
    json!({
        "mode": "direct", "mediaFileId": "f1", "grant": GRANT,
        "streamUrl": format!("{API}/media/{GRANT}/stream"), "frameUrl": "",
        "durationSeconds": 2400.0, "resumePosition": 0, "allowRandomPlayback": false,
        "display": { "title": "Glass Harbor", "subtitle": "", "titleId": "t1", "titleSlug": "glass-harbor" },
        "subtitles": [],
    })
}

fn host_state(seq: i64, playing: bool, position: f64) -> Value {
    json!({
        "media": { "kind": "movie", "titleId": "m1" }, "playing": playing,
        "positionSeconds": position, "serverTimestamp": 1000, "seq": seq, "away": false,
    })
}

/// A `host_state` frame as the server sends it.
fn state_frame(seq: i64, playing: bool, position: f64) -> Value {
    json!({ "type": "host_state", "data": host_state(seq, playing, position) })
}

fn session(role: &str, token: &str) -> Value {
    json!({
        "sessionId": "s1", "shareToken": "123456", "myParticipantId": format!("p-{role}"),
        "role": role, "isAnonymous": false, "participantToken": token, "artworkGrant": "g",
        "state": host_state(1, false, 0.0),
        "participants": [
            { "id": "p-host", "displayName": "Admin", "seed": "admin", "isHost": true, "isAnonymous": false, "paused": false, "avatarId": "av" },
        ],
    })
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

fn opened(shell: &mut Shell, socket: U53) {
    shell.resolve(socket, EffectOutput::SocketOpened);
}

fn couch(shell: &Shell) -> CouchView {
    shell.view(&Surface::Couch)
}

fn hosting() -> (Shell, U53) {
    let mut shell = signed_in();
    shell.send(Event::PlayRequested(PlayTarget { kind: PlayKind::Movie, id: "m1".into() }));
    shell.respond("GET", &format!("{API}/playback/movie/m1?lang=en"), 200, player_info());
    shell.send(Event::CouchStartRequested);
    let (_, create) = shell.request("POST", &format!("{API}/couch?delivery=body"));
    assert_eq!(body(&create), json!({ "kind": "movie", "id": "m1" }));
    shell.respond("POST", &format!("{API}/couch?delivery=body"), 201, session("host", "host-tok"));
    let socket = shell.socket(SOCKET);
    opened(&mut shell, socket);
    (shell, socket)
}

#[test]
fn a_host_shares_a_code_and_broadcasts_its_player() {
    let (mut shell, socket) = hosting();
    let open = &shell.sockets[0].1;
    assert_eq!(
        header(
            &HttpRequest {
                method: String::new(),
                url: String::new(),
                headers: open.headers.clone(),
                body: None
            },
            "X-Couch-Token"
        ),
        Some("host-tok")
    );
    let view = couch(&shell);
    assert_eq!((view.status, view.role), (CouchStatus::Open, Some(CouchRole::Host)));
    assert_eq!(view.share_url.as_deref(), Some("https://media.example.com/couch/123456"));
    assert_eq!(
        view.members[0].avatar.as_ref().map(|a| a.url.clone()),
        Some(format!("{API}/artwork/av?size=w342&g=g-art"))
    );

    // playing reaches the followers at once, not at the next heartbeat
    report(&mut shell, 10.0, true);
    let state = shell.sent_frames(socket).last().cloned().expect("a frame");
    assert_eq!(state["type"], json!("host_state"));
    assert_eq!(state["data"]["media"], json!({ "kind": "movie", "titleId": "m1" }));
    assert_eq!(state["data"]["playing"], json!(true));

    let heartbeat = shell
        .timers()
        .iter()
        .find(|(_, every, repeat)| *every == 2_000 && *repeat)
        .expect("heartbeat")
        .0;
    let before = shell.sent_frames(socket).len();
    shell.fire(heartbeat, 2_000);
    let frames = shell.sent_frames(socket);
    assert_eq!(frames.len(), before + 1);
    assert!(
        (frames.last().expect("frame")["data"]["positionSeconds"].as_f64().expect("position")
            - 12.0)
            .abs()
            < 1e-9
    );
}

#[test]
fn a_hosts_player_follows_a_remote() {
    let (mut shell, socket) = hosting();
    report(&mut shell, 10.0, true);
    shell.frame(socket, &json!({ "type": "remote_command", "data": { "action": "seek", "positionSeconds": 120.0 } }));
    assert_eq!(shell.player.last(), Some(&PlayerCommand::Seek(PlayerSeek { seconds: 120.0 })));
    shell.frame(socket, &json!({ "type": "remote_command", "data": { "action": "pause" } }));
    assert_eq!(shell.player.last(), Some(&PlayerCommand::Pause));
}

#[test]
fn reactions_float_for_a_moment_and_recent_emojis_are_kept() {
    let (mut shell, socket) = hosting();
    shell.frame(
        socket,
        &json!({ "type": "emoji", "data": { "fromParticipantId": "p-host", "emoji": "😂" } }),
    );
    assert_eq!(couch(&shell).reactions[0].emoji, "😂");
    let gone =
        shell.timers().iter().find(|(_, after, _)| *after == 2_200).expect("reaction timer").0;
    shell.fire(gone, 2_200);
    assert_eq!(couch(&shell).reactions, empty::<crate::modules::couch::Reaction>());

    for emoji in ["🍿", "😂", "🍿", "🔥", "👏"] {
        shell.send(Event::CouchEmojiSent(CouchReaction { emoji: emoji.into() }));
    }
    assert_eq!(couch(&shell).recent_emojis, ["👏", "🔥", "🍿"]);
    assert!(shell.store["couch.recentEmojis"].contains("👏"));
    assert_eq!(
        shell.sent_frames(socket).last().expect("frame"),
        &json!({ "type": "emoji", "data": { "emoji": "👏" } })
    );
}

#[test]
fn a_dropped_socket_reconnects_with_growing_backoff() {
    let (mut shell, socket) = hosting();
    shell.resolve(
        socket,
        EffectOutput::SocketClosed(SocketClosed { code: 1006, reason: String::new() }),
    );
    assert_eq!(couch(&shell).status, CouchStatus::Reconnecting);
    // nothing more is expected from the closed socket
    assert_eq!(shell.resolve_late(socket, EffectOutput::SocketOpened), empty::<EffectRequest>());

    let retry =
        shell.timers().iter().find(|(_, after, repeat)| *after == 500 && !repeat).expect("retry").0;
    shell.fire(retry, 500);
    let second = shell.socket(SOCKET);
    assert_ne!(second, socket);
    shell.resolve(
        second,
        EffectOutput::SocketClosed(SocketClosed { code: 1006, reason: String::new() }),
    );
    assert!(shell.timers().iter().any(|(_, after, _)| *after == 1_000));
    let retry = shell.timers().iter().find(|(_, after, _)| *after == 1_000).expect("retry").0;
    shell.fire(retry, 1_000);
    let third = shell.socket(SOCKET);
    opened(&mut shell, third);
    assert_eq!(couch(&shell).status, CouchStatus::Open);
}

#[test]
fn the_host_ends_the_session_for_everyone() {
    let (mut shell, socket) = hosting();
    shell.send(Event::CouchEndRequested);
    let (_, end) = shell.request("POST", &format!("{API}/couch/123456/end"));
    assert_eq!(header(&end, "X-Couch-Token"), Some("host-tok"));
    assert!(shell.closed_sockets.contains(&socket));
    let view = couch(&shell);
    assert_eq!((view.status, view.ended.as_deref()), (CouchStatus::Ended, Some("host_ended")));
}

fn following() -> (Shell, U53) {
    let mut shell = signed_in();
    shell.send(Event::CouchJoinRequested(CouchCode { code: " 123456 ".into() }));
    assert_eq!(couch(&shell).status, CouchStatus::Connecting);
    shell.respond(
        "POST",
        &format!("{API}/couch/123456/join?delivery=body"),
        200,
        session("follower", "guest-tok"),
    );
    let (_, playback) = shell.request("GET", &format!("{API}/couch/123456/playback?lang=en"));
    assert_eq!(header(&playback, "X-Couch-Token"), Some("guest-tok"));
    shell.respond(
        "GET",
        &format!("{API}/couch/123456/playback?lang=en"),
        200,
        json!({ "media": { "kind": "movie", "titleId": "m1" }, "player": player_info() }),
    );
    let socket = shell.socket(SOCKET);
    opened(&mut shell, socket);
    (shell, socket)
}

#[test]
fn a_follower_plays_the_hosts_media_locked_to_its_timeline() {
    let (mut shell, socket) = following();
    let PlayerCommand::Load(load) =
        shell.player.iter().find(|c| matches!(c, PlayerCommand::Load(_))).expect("a load").clone()
    else {
        unreachable!()
    };
    assert!(load.linear);
    assert!(load.start_seconds.abs() < f64::EPSILON);

    // joining snaps to where the host is
    shell.frame(
        socket,
        &json!({
            "type": "hello",
            "data": {
                "sessionId": "s1", "myParticipantId": "p-follower", "role": "follower",
                "state": host_state(5, true, 100.0), "participants": [], "serverTime": 1000,
            },
        }),
    );
    assert!(shell.player.contains(&PlayerCommand::Play));
    assert_eq!(shell.player.last(), Some(&PlayerCommand::Seek(PlayerSeek { seconds: 100.0 })));

    // close enough: left alone
    report(&mut shell, 100.0, true);
    shell.now += 2_000;
    report(&mut shell, 101.5, true);
    let before = shell.player.len();
    shell.frame(socket, &state_frame(6, true, 102.0));
    assert_eq!(shell.player.len(), before);
    // an old frame is dropped
    shell.frame(socket, &state_frame(4, false, 0.0));
    assert_eq!(shell.player.len(), before);

    // too far: snapped back, with a note that fades
    shell.frame(socket, &state_frame(7, true, 160.0));
    assert_eq!(shell.player.last(), Some(&PlayerCommand::Seek(PlayerSeek { seconds: 160.0 })));
    assert!(couch(&shell).resynced);
    let note = shell.timers().iter().find(|(_, after, _)| *after == 2_500).expect("note").0;
    shell.fire(note, 2_500);
    assert!(!couch(&shell).resynced);

    // followers never save progress of their own
    assert!(shell.find_request("PUT", &format!("{API}/progress")).is_none());
}

#[test]
fn a_follower_waits_out_the_host_and_pauses_on_its_own() {
    let (mut shell, socket) = following();
    shell.frame(socket, &state_frame(5, true, 100.0));
    report(&mut shell, 100.0, true);
    shell.frame(socket, &json!({ "type": "host_away", "data": { "graceSeconds": 60 } }));
    assert_eq!(shell.player.last(), Some(&PlayerCommand::Pause));
    assert!(couch(&shell).waiting);
    report(&mut shell, 100.0, false);
    shell.frame(socket, &json!({ "type": "host_returned" }));
    assert!(matches!(shell.player.last(), Some(PlayerCommand::Seek(_))));

    shell.send(Event::CouchLocalPauseChanged(CouchPause { paused: true }));
    assert_eq!(shell.player.last(), Some(&PlayerCommand::Pause));
    assert_eq!(
        shell.sent_frames(socket).last().expect("frame"),
        &json!({ "type": "paused", "data": { "paused": true } })
    );
    // the host's heartbeat does not undo a local pause
    let before = shell.player.len();
    shell.frame(socket, &state_frame(9, true, 130.0));
    assert_eq!(shell.player.len(), before);
    shell.send(Event::CouchLocalPauseChanged(CouchPause { paused: false }));
    assert!(matches!(shell.player.last(), Some(PlayerCommand::Seek(_))));
}

#[test]
fn a_follower_switches_with_the_host_and_stops_when_it_ends() {
    let (mut shell, socket) = following();
    shell.frame(socket, &json!({ "type": "media_changed", "data": { "media": { "kind": "episode", "episodeId": "e2" }, "seq": 9 } }));
    shell.respond(
        "GET",
        &format!("{API}/couch/123456/playback?lang=en"),
        200,
        json!({ "media": { "kind": "episode", "episodeId": "e2" }, "player": player_info() }),
    );
    let loads = shell.player.iter().filter(|c| matches!(c, PlayerCommand::Load(_))).count();
    assert_eq!(loads, 2);

    shell.frame(socket, &json!({ "type": "session_ended", "data": { "reason": "host_ended" } }));
    assert_eq!(shell.player.last(), Some(&PlayerCommand::Stop));
    let view = couch(&shell);
    assert_eq!((view.status, view.ended.as_deref()), (CouchStatus::Ended, Some("host_ended")));
    assert!(shell.closed_sockets.contains(&socket));
}

#[test]
fn the_hosts_phone_steers_as_a_remote() {
    let mut shell = signed_in();
    shell.send(Event::CouchRemoteRequested(CouchCode { code: "123456".into() }));
    shell.respond(
        "POST",
        &format!("{API}/couch/123456/join?delivery=body&remote=true"),
        200,
        session("remote", "phone-tok"),
    );
    let socket = shell.socket(SOCKET);
    opened(&mut shell, socket);
    assert_eq!(couch(&shell).role, Some(CouchRole::Remote));

    shell.frame(socket, &state_frame(3, true, 640.0));
    let view = couch(&shell);
    assert!(view.playing);
    assert!((view.position_seconds - 640.0).abs() < f64::EPSILON);

    shell.send(Event::CouchRemoteCommanded(RemoteControl {
        action: RemoteAction::Seek,
        position_seconds: Some(700.0),
    }));
    shell.send(Event::CouchRemoteCommanded(RemoteControl {
        action: RemoteAction::Next,
        position_seconds: Some(5.0),
    }));
    let frames = shell.sent_frames(socket);
    assert_eq!(
        frames[frames.len() - 2],
        json!({ "type": "remote_command", "data": { "action": "seek", "positionSeconds": 700.0 } })
    );
    // a position only travels with a seek
    assert_eq!(
        frames[frames.len() - 1],
        json!({ "type": "remote_command", "data": { "action": "next" } })
    );
    // a remote plays nothing
    assert_eq!(shell.player, empty::<PlayerCommand>());
}

#[test]
fn hosting_needs_something_playing() {
    let mut shell = signed_in();
    shell.send(Event::CouchStartRequested);
    assert_eq!(
        shell.view::<NoticesView>(&Surface::Notices).notices[0].code,
        "couch_nothing_playing"
    );
    assert!(shell.find_request("POST", &format!("{API}/couch?delivery=body")).is_none());
}
