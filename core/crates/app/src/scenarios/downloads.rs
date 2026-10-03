use serde_json::json;

use super::*;
use crate::messages::{
    DownloadFailure, DownloadFinished, DownloadName, DownloadProgress, DownloadStart, PlayerReport,
    PlayerSource,
};
use crate::modules::catalog::{PlayKind, PlayTarget};
use crate::modules::downloads::{
    DownloadAsk, DownloadQuality, DownloadRef, DownloadState, DownloadsView,
};
use crate::modules::notices::NoticesView;
use crate::modules::playback::DeviceProfile;
use crate::modules::session::SessionView;

const API: &str = "https://media.example.com/api/v1";
const FILE_URL: &str = "/api/v1/media/gr4nt/downloads/f1";

fn signed_in() -> Shell {
    let mut shell = launched(returning(Platform::Ios, &[(1, "admin", Some("tok-1"))], 1));
    shell.answer_session(HTTPS, user(1, "admin"), Some("en"));
    let profile: DeviceProfile = serde_json::from_str(include_str!(
        "../../../../../contract/fixtures/device-profiles/apple-tv-4k.json"
    ))
    .expect("the fixture");
    shell.send(Event::CapabilitiesReported(profile));
    shell
}

fn movie() -> PlayTarget {
    PlayTarget { kind: PlayKind::Movie, id: "m1".into() }
}

fn ask(quality: DownloadQuality) -> Event {
    Event::DownloadRequested(DownloadAsk { target: movie(), quality, audio: vec![] })
}

fn download(status: &str, progress: i64) -> Value {
    let mut d = json!({
        "id": "d1", "kind": "movie", "titleId": "m1", "titleSlug": "glass-harbor",
        "title": "Glass Harbor", "posterId": "p1", "posterVer": 4, "backdropId": null,
        "thumbId": null, "quality": "original", "status": status, "progress": progress,
        "durationSeconds": 2400.0, "createdAt": "2026-10-02T12:00:00Z", "expiresAt": null,
        "audio": [{ "lang": "en", "label": "English" }, { "lang": "cs", "label": "Čeština" }],
        "subtitles": [{ "lang": "cs", "label": "Čeština", "forced": false }],
    });
    if status == "ready" {
        d["url"] = json!(FILE_URL);
        d["sizeBytes"] = json!(1000);
        d["expiresAt"] = json!("2026-10-05T12:00:00Z");
    }
    d
}

fn downloads(shell: &Shell) -> DownloadsView {
    shell.view(&Surface::Downloads)
}

/// The download effects started, as `(id, start)`, oldest first.
fn started(shell: &Shell) -> Vec<(U53, DownloadStart)> {
    shell
        .downloads
        .iter()
        .filter_map(|(id, c)| match c {
            DownloadCommand::Start(start) => Some((*id, start.clone())),
            _ => None,
        })
        .collect()
}

fn removed(shell: &Shell) -> Vec<String> {
    shell
        .downloads
        .iter()
        .filter_map(|(_, c)| match c {
            DownloadCommand::Remove(DownloadName { name }) => Some(name.clone()),
            _ => None,
        })
        .collect()
}

fn poll(shell: &Shell) -> U53 {
    let timers = shell.timers();
    timers.iter().find(|(_, after, repeat)| *after == 5_000 && *repeat).expect("a poll").0
}

/// Asks for the movie and takes it through preparation to a finished fetch.
fn fetched() -> Shell {
    let mut shell = signed_in();
    shell.send(ask(DownloadQuality::Original));
    shell.respond("POST", &format!("{API}/me/downloads?lang=en"), 200, download("ready", 100));
    let (id, _) = started(&shell)[0].clone();
    shell.resolve(id, EffectOutput::DownloadFinished(DownloadFinished { bytes: 1000 }));
    shell
}

#[test]
fn a_download_is_prepared_then_fetched_to_the_device() {
    let mut shell = signed_in();
    assert_eq!(downloads(&shell).status, LoadStatus::Loaded);
    shell.send(ask(DownloadQuality::Hd720));
    let (_, request) = shell.request("POST", &format!("{API}/me/downloads?lang=en"));
    let sent = body(&request);
    assert_eq!((sent["kind"].as_str(), sent["id"].as_str()), (Some("movie"), Some("m1")));
    assert_eq!(sent["quality"], "720p");
    assert!(sent["profile"]["video"].is_array(), "the device's profile goes along");
    assert!(sent.get("audio").is_none());

    shell.respond("POST", &format!("{API}/me/downloads?lang=en"), 200, download("queued", 0));
    let view = downloads(&shell);
    assert_eq!(view.items.len(), 1);
    assert_eq!(view.items[0].state, DownloadState::Queued);
    assert_eq!(view.items[0].title, "Glass Harbor");
    assert_eq!(
        view.items[0].image.as_ref().map(|i| i.url.as_str()),
        Some("https://media.example.com/api/v1/artwork/p1?size=w342&v=4&g=g-art")
    );

    // the server prepares it; the core asks every few seconds
    let timer = poll(&shell);
    shell.fire(timer, 5_000);
    shell.respond(
        "GET",
        &format!("{API}/me/downloads?lang=en"),
        200,
        json!({ "downloads": [download("preparing", 40)] }),
    );
    let item = &downloads(&shell).items[0];
    assert_eq!((item.state, item.progress), (DownloadState::Preparing, 0.4));

    shell.fire(timer, 5_000);
    shell.respond(
        "GET",
        &format!("{API}/me/downloads?lang=en"),
        200,
        json!({ "downloads": [download("ready", 100)] }),
    );
    assert!(shell.was_cancelled(timer), "nothing waits on the server anymore");
    let (transfer, start) = started(&shell)[0].clone();
    assert_eq!(start, DownloadStart { url: format!("{HTTPS}{FILE_URL}"), name: "d1.mp4".into() });
    assert_eq!(downloads(&shell).items[0].state, DownloadState::Fetching);

    let progress = DownloadProgress { received_bytes: 250, total_bytes: Some(1000) };
    shell.resolve(transfer, EffectOutput::DownloadProgress(progress));
    assert_eq!(downloads(&shell).items[0].progress, 0.25);

    shell.resolve(transfer, EffectOutput::DownloadFinished(DownloadFinished { bytes: 1000 }));
    let view = downloads(&shell);
    assert_eq!((view.items[0].state, view.used_bytes), (DownloadState::Ready, 1000));
    // the artwork is kept too, for the list without a network
    let (art, start) = started(&shell)[1].clone();
    assert_eq!(start.name, "d1.jpg");
    assert!(start.url.contains("/artwork/p1?size=w780&v=4&g=g-art"));
    shell.resolve(art, EffectOutput::DownloadFinished(DownloadFinished { bytes: 20 }));
    assert_eq!(downloads(&shell).items[0].artwork.as_deref(), Some("d1.jpg"));

    // the device's list survives a relaunch
    let mut again = launched(shell.relaunch(Platform::Ios));
    again.answer_session(HTTPS, user(1, "admin"), Some("en"));
    let view = downloads(&again);
    assert_eq!(view.items[0].state, DownloadState::Ready);
    assert_eq!(view.items[0].artwork.as_deref(), Some("d1.jpg"));
    assert!(again.find_request("GET", &format!("{API}/me/downloads?lang=en")).is_none());
}

#[test]
fn asking_twice_for_the_same_download_asks_once() {
    let mut shell = fetched();
    shell.send(ask(DownloadQuality::Original));
    assert!(shell.find_request("POST", &format!("{API}/me/downloads?lang=en")).is_none());
}

#[test]
fn failures_tell_the_user() {
    let mut shell = signed_in();
    shell.send(ask(DownloadQuality::Original));
    shell.respond(
        "POST",
        &format!("{API}/me/downloads?lang=en"),
        422,
        json!({ "error": { "code": "unsupported", "message": "no" } }),
    );
    shell.send(ask(DownloadQuality::Hd720));
    shell.fail("POST", &format!("{API}/me/downloads?lang=en"), HttpFailureKind::Offline);
    let notices: NoticesView = shell.view(&Surface::Notices);
    let codes: Vec<&str> = notices.notices.iter().map(|n| n.code.as_str()).collect();
    assert_eq!(codes, ["download_unsupported", "download_failed"]);
    assert_eq!(downloads(&shell).items, empty::<crate::modules::downloads::DownloadItem>());

    // without a measured profile there is nothing to plan the file for
    let mut bare = launched(returning(Platform::Ios, &[(1, "admin", Some("tok-1"))], 1));
    bare.answer_session(HTTPS, user(1, "admin"), Some("en"));
    bare.send(ask(DownloadQuality::Original));
    assert!(bare.find_request("POST", &format!("{API}/me/downloads?lang=en")).is_none());
}

#[test]
fn a_transfer_that_failed_or_expired_can_be_asked_for_again() {
    let mut shell = signed_in();
    shell.send(ask(DownloadQuality::Original));
    shell.respond("POST", &format!("{API}/me/downloads?lang=en"), 200, download("ready", 100));
    let (transfer, _) = started(&shell)[0].clone();
    let failure = DownloadFailure { message: "disk full".into(), no_space: true };
    shell.resolve(transfer, EffectOutput::DownloadFailed(failure));
    let item = &downloads(&shell).items[0];
    assert_eq!(item.state, DownloadState::Failed);
    assert_eq!(item.problem.as_ref().map(|p| p.code.as_str()), Some("no_space"));

    shell.send(Event::DownloadRetried(DownloadRef { id: "d1".into() }));
    shell.respond("POST", &format!("{API}/me/downloads?lang=en"), 200, download("ready", 100));
    assert_eq!(started(&shell).len(), 2, "fetched again");
    assert_eq!(downloads(&shell).items.len(), 1);

    // a download the server dropped before the device fetched it
    let mut gone = signed_in();
    gone.send(ask(DownloadQuality::Original));
    gone.respond("POST", &format!("{API}/me/downloads?lang=en"), 200, download("preparing", 10));
    let timer = poll(&gone);
    gone.fire(timer, 5_000);
    gone.respond("GET", &format!("{API}/me/downloads?lang=en"), 200, json!({ "downloads": [] }));
    let item = &downloads(&gone).items[0];
    assert_eq!(item.problem.as_ref().map(|p| p.code.as_str()), Some("expired"));
}

#[test]
fn a_relaunch_picks_up_the_transfer_or_starts_it_again() {
    let mut shell = signed_in();
    shell.send(ask(DownloadQuality::Original));
    shell.respond("POST", &format!("{API}/me/downloads?lang=en"), 200, download("ready", 100));
    let (transfer, _) = started(&shell)[0].clone();
    let progress = DownloadProgress { received_bytes: 600, total_bytes: Some(1000) };
    shell.resolve(transfer, EffectOutput::DownloadProgress(progress));

    // the transfer finished in the background while the app was gone
    let mut again = launched(shell.relaunch(Platform::Ios));
    let (attach, start) = started(&again)[0].clone();
    assert_eq!(start, DownloadStart { url: String::new(), name: "d1.mp4".into() });
    again.resolve(attach, EffectOutput::DownloadFinished(DownloadFinished { bytes: 1000 }));
    assert_eq!(downloads(&again).items[0].state, DownloadState::Ready);

    // or the shell lost it: it starts again with a fresh URL
    let mut lost = launched(shell.relaunch(Platform::Ios));
    let (attach, _) = started(&lost)[0].clone();
    let failure = DownloadFailure { message: "unknown".into(), no_space: false };
    lost.resolve(attach, EffectOutput::DownloadFailed(failure));
    assert_eq!(downloads(&lost).items[0].state, DownloadState::Fetching);
    let mut fresh = download("ready", 100);
    fresh["url"] = json!("/api/v1/media/fresh/downloads/f1");
    // asked at once, in the device's language until the account's arrives
    lost.respond(
        "GET",
        &format!("{API}/me/downloads?lang=cs"),
        200,
        json!({ "downloads": [fresh] }),
    );
    let (_, start) = started(&lost)[1].clone();
    assert_eq!(start.name, "d1.mp4");
    assert_eq!(start.url, format!("{HTTPS}/api/v1/media/fresh/downloads/f1"));
}

#[test]
fn removing_a_download_deletes_it_here_and_on_the_server() {
    let mut shell = fetched();
    shell.send(Event::DownloadRemoved(DownloadRef { id: "d1".into() }));
    assert_eq!(removed(&shell), ["d1.mp4", "d1.jpg"]);
    shell.respond("DELETE", &format!("{API}/me/downloads/d1"), 204, Value::Null);
    assert_eq!(downloads(&shell).items, empty::<crate::modules::downloads::DownloadItem>());
    let key = format!("downloads.{}", account_id(1));
    assert!(!shell.store[&key].contains("d1"));
}

#[test]
fn signing_out_takes_the_accounts_downloads_off_the_device() {
    let mut shell = fetched();
    shell.send(Event::SignOutRequested(crate::modules::accounts::AccountRef {
        account_id: account_id(1),
    }));
    assert!(removed(&shell).contains(&"d1.mp4".to_string()));
    assert!(!shell.store.contains_key(&format!("downloads.{}", account_id(1))));
}

#[test]
fn offline_a_download_plays_from_the_device_and_its_progress_waits() {
    let mut shell = fetched();
    // the server is out of reach
    shell.send(Event::AppBecameActive);
    shell.fail("GET", &format!("{API}/auth/me"), HttpFailureKind::Offline);
    assert!(shell.view::<SessionView>(&Surface::Session).offline);

    shell.send(Event::PlayRequested(movie()));
    assert!(shell.find_request("POST", &format!("{API}/playback/movie/m1?lang=en")).is_none());
    let PlayerCommand::Load(load) = shell.player.last().cloned().expect("a load") else {
        panic!("not a load")
    };
    assert_eq!((load.url.as_str(), load.source), ("d1.mp4", PlayerSource::Download));
    assert_eq!(load.subtitles.len(), 1);
    assert_eq!(load.subtitles[0].url, None, "the subtitles are inside the file");
    assert_eq!(load.now_playing.title, "Glass Harbor");

    for position in [0.0, 6.0, 12.0] {
        shell.now += 6_000;
        shell.send(Event::PlayerReported(PlayerReport {
            position_seconds: position,
            duration_seconds: 2400.0,
            playing: true,
            buffering: false,
            ended: false,
            failed: None,
        }));
    }
    let saved_at = shell.now;
    shell.fail("PUT", &format!("{API}/progress"), HttpFailureKind::Offline);
    let kept = &shell.store[&format!("downloads.{}", account_id(1))];
    assert!(kept.contains(&crate::time::rfc3339(WALL_EPOCH + saved_at)), "{kept}");

    // a relaunch, still offline, resumes where it stopped
    let mut again = launched(shell.relaunch(Platform::Ios));
    again.fail("GET", &format!("{API}/auth/me"), HttpFailureKind::Offline);
    again.send(Event::DownloadPlayRequested(DownloadRef { id: "d1".into() }));
    let PlayerCommand::Load(load) = again.player.last().cloned().expect("a load") else {
        panic!("not a load")
    };
    assert!((load.start_seconds - 12.0).abs() < f64::EPSILON);

    // back online: the progress goes up with the time it was watched
    again.send(Event::AppBecameActive);
    again.answer_session(HTTPS, user(1, "admin"), Some("en"));
    let (_, replay) = again.request("PUT", &format!("{API}/progress"));
    let sent = body(&replay);
    assert_eq!(sent["titleId"], "m1");
    assert_eq!(sent["watchedAt"], crate::time::rfc3339(WALL_EPOCH + saved_at));
    again.respond("PUT", &format!("{API}/progress"), 204, Value::Null);
    let kept = &again.store[&format!("downloads.{}", account_id(1))];
    assert!(!kept.contains("watchedAt"), "{kept}");
}
