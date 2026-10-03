use serde_json::json;

use super::*;
use crate::modules::accounts::{
    ApprovalOutcome, DeviceRef, DevicesView, Link, PairingApproval, PairingApprovalView,
    PairingState, SignInView, UserCode,
};
use crate::modules::servers::ServerRef;

const POLL: &str = "https://media.example.com/api/v1/auth/pairings/poll";

/// A TV with the server added and no accounts, showing a pairing code.
fn pairing_tv() -> Shell {
    let mut shell = launched(returning(Platform::Tvos, &[], 0));
    assert_eq!(shell.phase(), AppPhase::SignIn);
    shell.send(Event::PairingStarted(ServerRef { server_id: SERVER_ID.into() }));
    let (_, start) = shell.request("POST", &format!("{HTTPS}/api/v1/auth/pairings"));
    assert_eq!(body(&start), json!({ "deviceName": "Living Room", "platform": "tvos" }));
    shell.respond(
        "POST",
        &format!("{HTTPS}/api/v1/auth/pairings"),
        201,
        json!({
            "deviceCode": "dc-secret", "userCode": "WDJB-MJHT",
            "verifyPath": "/pair?code=WDJB-MJHT", "expiresIn": 600, "interval": 5,
        }),
    );
    shell
}

fn poll_timer(shell: &Shell) -> U53 {
    let timers = shell.timers();
    timers.iter().find(|(_, ms, repeat)| *ms == 5_000 && *repeat).expect("poll timer").0
}

fn expiry_timer(shell: &Shell) -> U53 {
    let timers = shell.timers();
    timers.iter().find(|(_, ms, repeat)| *ms == 600_000 && !*repeat).expect("expiry timer").0
}

fn pairing(shell: &Shell) -> crate::modules::accounts::PairingView {
    shell.view::<SignInView>(&Surface::SignIn).pairing.expect("pairing shown")
}

#[test]
fn a_pairing_code_is_shown_with_its_qr_url_and_deadline() {
    let shell = pairing_tv();
    let view = pairing(&shell);
    assert_eq!(view.user_code, "WDJB-MJHT");
    assert_eq!(view.verify_url, format!("{HTTPS}/pair?code=WDJB-MJHT"));
    assert_eq!(view.expires_at_ms, shell.now + 600_000);
    assert_eq!(view.state, PairingState::Waiting);
    assert_eq!(shell.timers().len(), 2);
}

#[test]
fn pairing_polls_until_the_phone_approves() {
    let mut shell = pairing_tv();
    let tick = poll_timer(&shell);
    let expiry = expiry_timer(&shell);

    shell.fire(tick, 5_000);
    let (_, poll) = shell.request("POST", POLL);
    assert_eq!(body(&poll), json!({ "deviceCode": "dc-secret" }));
    shell.respond("POST", POLL, 200, json!({ "status": "pending" }));
    assert_eq!(pairing(&shell).state, PairingState::Waiting);

    // slow_down and network trouble just wait for the next tick
    shell.fire(tick, 5_000);
    shell.respond(
        "POST",
        POLL,
        429,
        json!({ "error": { "code": "slow_down", "message": "poll less often" } }),
    );
    shell.fire(tick, 5_000);
    shell.fail("POST", POLL, HttpFailureKind::Offline);
    assert_eq!(pairing(&shell).state, PairingState::Waiting);

    shell.fire(tick, 5_000);
    shell.respond(
        "POST",
        POLL,
        200,
        json!({ "status": "approved", "device": device_token("tok-tv", 2, "nora") }),
    );
    assert!(shell.was_cancelled(tick));
    assert!(shell.was_cancelled(expiry));
    assert_eq!(shell.secure[&format!("token.{}", account_id(2))], "tok-tv");
    assert_eq!(shell.phase(), AppPhase::Ready);
    assert_eq!(shell.view::<SignInView>(&Surface::SignIn).pairing, None);

    // a tick already in flight when the timer was cancelled changes nothing
    assert_eq!(shell.resolve_late(tick, EffectOutput::TimerFired), empty::<EffectRequest>());
}

#[test]
fn a_pairing_code_expires() {
    let mut shell = pairing_tv();
    let tick = poll_timer(&shell);
    let expiry = expiry_timer(&shell);
    shell.fire(expiry, 600_000);

    assert_eq!(pairing(&shell).state, PairingState::Expired);
    assert!(shell.was_cancelled(tick));
    assert_eq!(shell.timers(), empty::<(U53, U53, bool)>());
    assert_eq!(shell.resolve_late(tick, EffectOutput::TimerFired), empty::<EffectRequest>());
    assert_eq!(shell.phase(), AppPhase::SignIn);
}

#[test]
fn a_denied_or_forgotten_pairing_stops_polling() {
    let mut shell = pairing_tv();
    let tick = poll_timer(&shell);
    shell.fire(tick, 5_000);
    shell.respond("POST", POLL, 200, json!({ "status": "denied" }));
    assert_eq!(pairing(&shell).state, PairingState::Denied);
    assert_eq!(shell.timers(), empty::<(U53, U53, bool)>());

    // a server restart forgets pairings: the poll 404s
    let mut shell = pairing_tv();
    let tick = poll_timer(&shell);
    shell.fire(tick, 5_000);
    shell.respond(
        "POST",
        POLL,
        404,
        json!({ "error": { "code": "not_found", "message": "unknown pairing" } }),
    );
    assert_eq!(pairing(&shell).state, PairingState::Expired);
}

#[test]
fn cancelling_or_restarting_a_pairing_stops_the_old_timers() {
    let mut shell = pairing_tv();
    let (tick, expiry) = (poll_timer(&shell), expiry_timer(&shell));
    shell.send(Event::PairingStarted(ServerRef { server_id: SERVER_ID.into() }));
    assert!(shell.was_cancelled(tick) && shell.was_cancelled(expiry));

    shell.send(Event::PairingCancelled);
    assert_eq!(shell.timers(), empty::<(U53, U53, bool)>());
    assert_eq!(shell.view::<SignInView>(&Surface::SignIn).pairing, None);
}

#[test]
fn returning_to_the_foreground_polls_at_once() {
    let mut shell = pairing_tv();
    shell.now += 60_000;
    shell.send(Event::AppBecameActive);
    shell.request("POST", POLL);
}

/// A phone signed in as the admin, approving the TV's code.
fn approving_phone() -> Shell {
    let mut shell = launched(returning(Platform::Ios, &[(1, "admin", Some("tok-1"))], 1));
    shell.answer_session(HTTPS, user(1, "admin"), Some("en"));
    shell
}

#[test]
fn a_phone_approves_a_pairing_from_a_scanned_link() {
    let mut shell = approving_phone();
    shell.send(Event::LinkOpened(Link { url: "couchverse://pair?code=WDJB-MJHT".into() }));
    let url = format!("{HTTPS}/api/v1/me/pairings/WDJB-MJHT");
    let (_, get) = shell.request("GET", &url);
    assert_eq!(header(&get, "Authorization"), Some("Bearer tok-1"));
    shell.respond(
        "GET",
        &url,
        200,
        json!({
            "userCode": "WDJB-MJHT", "deviceName": "Apple TV", "platform": "tvos",
            "expiresAt": "2026-10-02T12:10:00Z",
        }),
    );
    let view: PairingApprovalView = shell.view(&Surface::PairingApproval);
    assert_eq!(view.status, LoadStatus::Loaded);
    assert_eq!(view.device_name, "Apple TV");
    assert_eq!(view.platform, "tvos");

    shell.send(Event::PairingApproved(PairingApproval {
        code: "WDJB-MJHT".into(),
        device_name: "Living Room TV".into(),
    }));
    let approve = format!("{url}/approve");
    let (_, request) = shell.request("POST", &approve);
    assert_eq!(body(&request), json!({ "deviceName": "Living Room TV" }));
    shell.respond("POST", &approve, 204, serde_json::Value::Null);
    let view: PairingApprovalView = shell.view(&Surface::PairingApproval);
    assert_eq!(view.outcome, Some(ApprovalOutcome::Approved));
}

#[test]
fn a_phone_denies_a_pairing_and_unknown_codes_are_not_found() {
    let mut shell = approving_phone();
    shell.send(Event::PairingApprovalOpened(UserCode { code: "ZZZZ-ZZZZ".into() }));
    shell.respond(
        "GET",
        &format!("{HTTPS}/api/v1/me/pairings/ZZZZ-ZZZZ"),
        404,
        json!({ "error": { "code": "not_found", "message": "no such pairing" } }),
    );
    let view: PairingApprovalView = shell.view(&Surface::PairingApproval);
    assert_eq!(view.status, LoadStatus::NotFound);

    shell.send(Event::PairingDenied(UserCode { code: "WDJB-MJHT".into() }));
    let deny = format!("{HTTPS}/api/v1/me/pairings/WDJB-MJHT/deny");
    shell.respond("POST", &deny, 204, serde_json::Value::Null);
    let view: PairingApprovalView = shell.view(&Surface::PairingApproval);
    assert_eq!(view.outcome, Some(ApprovalOutcome::Denied));
}

#[test]
fn approving_needs_a_signed_in_account() {
    let mut shell = launched(returning(Platform::Ios, &[], 0));
    shell.send(Event::PairingApprovalOpened(UserCode { code: "WDJB-MJHT".into() }));
    let view: PairingApprovalView = shell.view(&Surface::PairingApproval);
    assert_eq!(view.status, LoadStatus::Failed);
    assert_eq!(view.problem.expect("problem").code, "unauthorized");
    assert_eq!(shell.http_summary(), empty::<String>());
}

#[test]
fn devices_are_listed_and_revoked() {
    let mut shell = approving_phone();
    shell.send(Event::DevicesOpened);
    let url = format!("{HTTPS}/api/v1/me/devices");
    assert_eq!(shell.view::<DevicesView>(&Surface::Devices).status, LoadStatus::Loading);
    let device = |id: &str, name: &str, platform: &str, current: bool| {
        json!({
            "id": id, "name": name, "platform": platform, "kind": "device", "current": current,
            "createdAt": "2026-09-01T10:00:00Z", "lastSeenAt": "2026-10-02T09:00:00Z",
        })
    };
    shell.respond(
        "GET",
        &url,
        200,
        json!([device("d1", "iPhone", "ios", true), device("d2", "Apple TV", "tvos", false)]),
    );
    let view: DevicesView = shell.view(&Surface::Devices);
    assert_eq!(view.status, LoadStatus::Loaded);
    assert_eq!(view.devices.len(), 2);
    assert!(view.devices[0].current);

    shell.send(Event::DeviceRevoked(DeviceRef { device_id: "d2".into() }));
    assert_eq!(shell.view::<DevicesView>(&Surface::Devices).devices.len(), 1, "optimistic");
    shell.respond("DELETE", &format!("{url}/d2"), 204, serde_json::Value::Null);
    // the list is re-read, keeping what is shown meanwhile
    assert_eq!(shell.view::<DevicesView>(&Surface::Devices).status, LoadStatus::Stale);
    shell.respond("GET", &url, 200, json!([device("d1", "iPhone", "ios", true)]));
    assert_eq!(shell.view::<DevicesView>(&Surface::Devices).status, LoadStatus::Loaded);
}
