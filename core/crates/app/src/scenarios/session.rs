use serde_json::json;

use super::*;
use crate::modules::accounts::{AccountRef, AccountsView};
use crate::modules::session::{LanguageChoice, SessionView};

fn unauthorized() -> Value {
    json!({ "error": { "code": "unauthorized", "message": "sign in" } })
}

#[test]
fn a_rejected_token_signs_the_account_out_but_keeps_it_and_its_server() {
    let users = [(1, "admin", Some("tok-1")), (2, "nora", Some("tok-2"))];
    let mut shell = launched(returning(Platform::Ios, &users, 2));
    shell.respond("GET", &format!("{HTTPS}/api/v1/auth/me"), 401, unauthorized());

    assert_eq!(shell.phase(), AppPhase::ChooseAccount);
    assert_eq!(shell.active_account(), None);
    assert!(!shell.secure.contains_key(&format!("token.{}", account_id(2))));
    assert!(shell.secure.contains_key(&format!("token.{}", account_id(1))));
    let accounts: AccountsView = shell.view(&Surface::Accounts);
    let nora = accounts.accounts.iter().find(|a| a.username == "nora").expect("kept");
    assert!(!nora.signed_in);
    assert!(shell.store["servers"].contains(SERVER_ID));

    // the rest of the dead session's answers are dropped
    shell.respond("GET", &format!("{HTTPS}/api/v1/features"), 401, unauthorized());
    assert_eq!(shell.phase(), AppPhase::ChooseAccount);
}

#[test]
fn answers_for_a_previous_account_never_reach_the_next() {
    let users = [(1, "admin", Some("tok-1")), (2, "nora", Some("tok-2"))];
    let mut shell = launched(returning(Platform::Ios, &users, 2));
    let (nora_me, _) = shell.request("GET", &format!("{HTTPS}/api/v1/auth/me"));
    let (nora_prefs, _) = shell.request("GET", &format!("{HTTPS}/api/v1/me/preferences"));

    shell.send(Event::AccountSelected(AccountRef { account_id: account_id(1) }));
    let body = |v: Value| EffectOutput::Http(HttpResponse { status: 200, body: v.to_string() });
    shell.resolve(nora_me, body(user(2, "nora")));
    shell.resolve(nora_prefs, body(json!({ "language": "en" })));

    let session: SessionView = shell.view(&Surface::Session);
    assert_eq!(session.account_id, Some(account_id(1)));
    assert_eq!(session.user, None);
    assert_eq!(session.status, LoadStatus::Loading);
    // a switch starts from the device's language until the account's preference arrives
    assert_eq!(session.language, "cs");

    shell.answer_session(HTTPS, user(1, "admin"), Some("en"));
    let session: SessionView = shell.view(&Surface::Session);
    assert_eq!(session.user.expect("user").username, "admin");
}

#[test]
fn the_display_language_follows_the_account_and_is_saved() {
    let mut shell = launched(returning(Platform::Ios, &[(1, "admin", Some("tok-1"))], 1));
    shell.answer_session(HTTPS, user(1, "admin"), Some("en"));
    assert_eq!(shell.view::<SessionView>(&Surface::Session).language, "en");

    shell.send(Event::DisplayLanguageChanged(LanguageChoice { code: "cs".into() }));
    assert_eq!(shell.view::<SessionView>(&Surface::Session).language, "cs");
    let (_, save) = shell.request("PUT", &format!("{HTTPS}/api/v1/me/preferences"));
    assert_eq!(super::body(&save), json!({ "language": "cs" }));

    shell.send(Event::DisplayLanguageChanged(LanguageChoice { code: "de".into() }));
    assert_eq!(shell.view::<SessionView>(&Surface::Session).language, "cs");
}

#[test]
fn a_failed_load_is_stale_when_something_is_showing() {
    let mut shell = launched(returning(Platform::Ios, &[(1, "admin", Some("tok-1"))], 1));
    shell.fail("GET", &format!("{HTTPS}/api/v1/auth/me"), HttpFailureKind::Offline);
    let session: SessionView = shell.view(&Surface::Session);
    assert_eq!(session.status, LoadStatus::Failed);
    assert_eq!(session.problem.expect("problem").code, "offline");
    assert_eq!(shell.phase(), AppPhase::Ready, "offline is not signed out");

    // back in the foreground: the session reloads and recovers
    shell.send(Event::AppBecameActive);
    shell.respond("GET", &format!("{HTTPS}/api/v1/auth/me"), 200, user(1, "admin"));
    let session: SessionView = shell.view(&Surface::Session);
    assert_eq!(session.status, LoadStatus::Loaded);
    assert_eq!(session.problem, None);

    shell.send(Event::AppBecameActive);
    assert_eq!(shell.view::<SessionView>(&Surface::Session).status, LoadStatus::Stale);
    shell.fail("GET", &format!("{HTTPS}/api/v1/auth/me"), HttpFailureKind::Timeout);
    let session: SessionView = shell.view(&Surface::Session);
    assert_eq!(session.status, LoadStatus::Stale, "stale beats blank");
    assert!(session.user.is_some());
}

#[test]
fn the_session_updates_the_account_card() {
    let mut shell = launched(returning(Platform::Ios, &[(1, "admin", Some("tok-1"))], 1));
    let mut renamed = user(1, "admin");
    renamed["displayName"] = json!("The Admin");
    shell.respond("GET", &format!("{HTTPS}/api/v1/auth/me"), 200, renamed);
    let accounts: AccountsView = shell.view(&Surface::Accounts);
    assert_eq!(accounts.accounts[0].display_name, "The Admin");
    assert!(shell.store["accounts"].contains("The Admin"));
}

fn web() -> Shell {
    let mut shell = Shell::new(Platform::Web);
    shell.send(Event::AppStarted);
    shell
}

#[test]
fn the_web_runs_on_its_cookie() {
    let mut shell = web();
    assert_eq!(shell.phase(), AppPhase::Starting);
    let (_, me) = shell.request("GET", "/api/v1/auth/me");
    assert_eq!(header(&me, "Authorization"), None);

    shell.answer_session("", user(1, "admin"), Some("en"));
    assert_eq!(shell.phase(), AppPhase::Ready);
    let session: SessionView = shell.view(&Surface::Session);
    assert!(session.user.expect("user").admin);
    assert_eq!(session.accent.accent, "#3a6ea5");
    // the web keeps no accounts or servers of its own
    assert!(shell.store.is_empty() && shell.secure.is_empty());

    shell.send(Event::SignOutRequested(AccountRef { account_id: "web".into() }));
    shell.request("POST", "/api/v1/auth/logout");
    assert_eq!(shell.phase(), AppPhase::SignIn);
    assert_eq!(shell.view::<SessionView>(&Surface::Session).user, None);

    shell.send(Event::SessionStarted);
    shell.respond("GET", "/api/v1/auth/me", 200, user(2, "nora"));
    assert_eq!(shell.phase(), AppPhase::Ready);
}

#[test]
fn a_web_visitor_without_a_session_signs_in() {
    let mut shell = web();
    shell.respond("GET", "/api/v1/auth/me", 401, unauthorized());
    assert_eq!(shell.phase(), AppPhase::SignIn);
    // language can still change before signing in; nothing to save it to yet
    shell.send(Event::DisplayLanguageChanged(LanguageChoice { code: "en".into() }));
    assert_eq!(shell.view::<SessionView>(&Surface::Session).language, "en");
    assert!(shell.find_request("PUT", "/api/v1/me/preferences").is_none());
}

#[test]
fn the_web_sign_in_screen_keeps_the_servers_accent() {
    let mut shell = web();
    // the cookie is rejected before the server's identity arrives
    shell.respond("GET", "/api/v1/auth/me", 401, unauthorized());
    shell.respond("GET", "/api/v1/server", 200, server_info("#3a6ea5"));
    assert_eq!(shell.phase(), AppPhase::SignIn);
    assert_eq!(shell.view::<SessionView>(&Surface::Session).accent.accent, "#3a6ea5");

    // and signing in keeps it up while the session loads again
    shell.send(Event::SessionStarted);
    assert_eq!(shell.view::<SessionView>(&Surface::Session).accent.accent, "#3a6ea5");
}

#[test]
fn the_web_reads_its_session_again_after_changing_it() {
    let mut shell = web();
    shell.answer_session("", user(1, "admin"), Some("en"));
    let mut edited = user(1, "admin");
    edited["displayName"] = json!("The Admin");
    edited["bio"] = json!("# Hi");

    shell.send(Event::SessionChanged);
    assert_eq!(shell.view::<SessionView>(&Surface::Session).status, LoadStatus::Stale);
    shell.respond("GET", "/api/v1/auth/me", 200, edited);
    let user = shell.view::<SessionView>(&Surface::Session).user.expect("user");
    assert_eq!(user.display_name, "The Admin");
    assert_eq!(user.bio, "# Hi");
    assert_eq!(user.created_at, "2026-01-01T00:00:00Z");

    let flags = json!({"couchEnabled": false, "rankingsEnabled": true, "downloadsEnabled": true});
    shell.respond("GET", "/api/v1/features", 200, flags);
    assert!(!shell.view::<SessionView>(&Surface::Session).features.couch);
}

#[test]
fn the_web_leaves_achievement_checks_to_its_own_ranks_store() {
    let mut shell = web();
    shell.respond("GET", "/api/v1/auth/me", 200, user(1, "admin"));
    let flags = json!({"couchEnabled": true, "rankingsEnabled": true, "downloadsEnabled": true});
    shell.respond("GET", "/api/v1/features", 200, flags);
    shell.send(Event::AppBecameActive);
    assert!(shell.find_request("POST", "/api/v1/me/achievements/check").is_none());
}

#[test]
fn a_web_call_rejected_as_signed_out_ends_the_session_once_the_core_agrees() {
    let mut shell = web();
    shell.answer_session("", user(1, "admin"), Some("en"));

    shell.send(Event::SessionChanged);
    assert_eq!(shell.phase(), AppPhase::Ready, "the core's own read decides");
    shell.respond("GET", "/api/v1/auth/me", 401, unauthorized());
    assert_eq!(shell.phase(), AppPhase::SignIn);
    assert_eq!(shell.view::<SessionView>(&Surface::Session).user, None);

    // signed out, there is nothing left to read
    shell.send(Event::SessionChanged);
    assert!(shell.find_request("GET", "/api/v1/auth/me").is_none());
}
