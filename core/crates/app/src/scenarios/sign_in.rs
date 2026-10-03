use serde_json::json;

use super::*;
use crate::modules::accounts::{AccountsView, Link, PasswordSignIn, SignInView};
use crate::modules::servers::{Server, ServerAddress, ServersView};
use crate::modules::session::SessionView;

const HTTP: &str = "http://media.example.com";

fn submit(shell: &mut Shell, address: &str) {
    shell.send(Event::ServerAddressSubmitted(ServerAddress { address: address.into() }));
}

#[test]
fn adding_a_server_falls_back_to_http_and_marks_it_insecure() {
    let mut shell = launched(Shell::new(Platform::Ios));
    submit(&mut shell, "Media.Example.com/web/");
    let servers: ServersView = shell.view(&Surface::Servers);
    assert_eq!(servers.add.status, LoadStatus::Loading);

    shell.fail("GET", &format!("{HTTPS}/api/v1/server"), HttpFailureKind::Tls);
    shell.respond("GET", &format!("{HTTP}/api/v1/server"), 200, server_info("#e50914"));

    let servers: ServersView = shell.view(&Surface::Servers);
    assert_eq!(servers.add.status, LoadStatus::Loaded);
    assert_eq!(servers.add.added.as_deref(), Some(SERVER_ID));
    let server = &servers.servers[0];
    assert_eq!(server.url, HTTP);
    assert!(server.insecure);
    assert_eq!(server.name, "Home Media");
    assert_eq!(shell.phase(), AppPhase::SignIn);
    assert!(shell.store["servers"].contains(HTTP));
}

#[test]
fn a_web_page_is_not_a_server() {
    let mut shell = launched(Shell::new(Platform::Ios));
    submit(&mut shell, "https://example.com");
    shell.respond_raw("GET", "https://example.com/api/v1/server", 200, "<!doctype html>");

    let servers: ServersView = shell.view(&Surface::Servers);
    assert_eq!(servers.add.status, LoadStatus::Failed);
    assert_eq!(servers.add.problem.expect("problem").code, "not_a_server");
    assert_eq!(servers.servers, empty::<Server>());
    assert_eq!(shell.phase(), AppPhase::Welcome);
}

#[test]
fn an_unreachable_or_outdated_server_is_reported() {
    let mut shell = launched(Shell::new(Platform::Ios));
    submit(&mut shell, "media.example.com");
    shell.fail("GET", &format!("{HTTPS}/api/v1/server"), HttpFailureKind::Offline);
    shell.fail("GET", &format!("{HTTP}/api/v1/server"), HttpFailureKind::Offline);
    let servers: ServersView = shell.view(&Surface::Servers);
    assert_eq!(servers.add.problem.expect("problem").code, "offline");

    submit(&mut shell, "https://media.example.com");
    let mut old = server_info("#e50914");
    old["apiLevel"] = json!(0);
    shell.respond("GET", &format!("{HTTPS}/api/v1/server"), 200, old);
    let servers: ServersView = shell.view(&Surface::Servers);
    assert_eq!(servers.add.problem.expect("problem").code, "server_outdated");

    submit(&mut shell, "my server");
    let servers: ServersView = shell.view(&Surface::Servers);
    assert_eq!(servers.add.problem.expect("problem").code, "invalid_address");
    assert_eq!(shell.http_summary(), empty::<String>());
}

#[test]
fn password_sign_in_stores_the_token_and_activates_the_account() {
    let mut shell = launched(Shell::new(Platform::Ipados));
    submit(&mut shell, "https://media.example.com");
    shell.respond("GET", &format!("{HTTPS}/api/v1/server"), 200, server_info("#3a6ea5"));

    shell.send(Event::PasswordSignInSubmitted(PasswordSignIn {
        server_id: SERVER_ID.into(),
        username: "nora".into(),
        password: "hunter22".into(),
    }));
    let (_, request) = shell.request("POST", &format!("{HTTPS}/api/v1/auth/token"));
    assert_eq!(header(&request, "Authorization"), None);
    assert_eq!(
        body(&request),
        json!({
            "username": "nora", "password": "hunter22",
            "deviceName": "Living Room", "platform": "ipados",
        })
    );
    let view: SignInView = shell.view(&Surface::SignIn);
    assert_eq!(view.status, LoadStatus::Loading);

    shell.respond(
        "POST",
        &format!("{HTTPS}/api/v1/auth/token"),
        200,
        device_token("tok-n", 2, "nora"),
    );
    let id = account_id(2);
    assert_eq!(shell.secure[&format!("token.{id}")], "tok-n");
    assert!(!shell.store["accounts"].contains("tok-n"), "tokens never leave the secure store");
    let view: SignInView = shell.view(&Surface::SignIn);
    assert_eq!(view.signed_in.as_deref(), Some(id.as_str()));
    assert_eq!(shell.phase(), AppPhase::Ready);
    assert_eq!(shell.active_account(), Some(id.clone()));

    // the first sign-in anywhere saves this device's language as the account's
    shell.answer_session(HTTPS, user(2, "nora"), None);
    let (_, save) = shell.request("PUT", &format!("{HTTPS}/api/v1/me/preferences"));
    assert_eq!(body(&save), json!({ "language": "cs" }));
    let session: SessionView = shell.view(&Surface::Session);
    assert_eq!(session.language, "cs");

    let accounts: AccountsView = shell.view(&Surface::Accounts);
    assert_eq!(accounts.accounts[0].display_name, "NORA");
    assert_eq!(accounts.accounts[0].avatar_url, None, "no artwork grant yet");
    shell.respond(
        "GET",
        &format!("{HTTPS}/api/v1/me/artwork-grant"),
        200,
        json!({ "grant": "g-1", "expiresIn": 604_800 }),
    );
    let accounts: AccountsView = shell.view(&Surface::Accounts);
    assert_eq!(
        accounts.accounts[0].avatar_url.as_deref(),
        Some(format!("{HTTPS}/api/v1/artwork/av-1?size=w342&g=g-1").as_str())
    );
}

#[test]
fn wrong_credentials_keep_the_form() {
    let mut shell = launched(returning(Platform::Ios, &[], 0));
    assert_eq!(shell.phase(), AppPhase::SignIn);
    shell.send(Event::PasswordSignInSubmitted(PasswordSignIn {
        server_id: SERVER_ID.into(),
        username: "nora".into(),
        password: "nope".into(),
    }));
    shell.respond(
        "POST",
        &format!("{HTTPS}/api/v1/auth/token"),
        401,
        json!({ "error": { "code": "invalid_credentials", "message": "wrong password" } }),
    );
    let view: SignInView = shell.view(&Surface::SignIn);
    assert_eq!(view.status, LoadStatus::Failed);
    assert_eq!(view.problem.expect("problem").code, "invalid_credentials");
    assert_eq!(shell.phase(), AppPhase::SignIn);
    assert!(shell.secure.is_empty());
}

#[test]
fn a_connect_link_adds_the_server_and_redeems_the_code() {
    let mut shell = launched(Shell::new(Platform::Ios));
    shell.send(Event::LinkOpened(Link {
        url: "couchverse://connect?server=https%3A%2F%2Fmedia.example.com&code=K7PQ-2MXD".into(),
    }));
    let view: SignInView = shell.view(&Surface::SignIn);
    assert_eq!(view.status, LoadStatus::Loading);

    shell.respond("GET", &format!("{HTTPS}/api/v1/server"), 200, server_info("#3a6ea5"));
    let (_, redeem) = shell.request("POST", &format!("{HTTPS}/api/v1/auth/connect"));
    assert_eq!(
        body(&redeem),
        json!({ "code": "K7PQ-2MXD", "deviceName": "Living Room", "platform": "ios" })
    );
    shell.respond(
        "POST",
        &format!("{HTTPS}/api/v1/auth/connect"),
        200,
        device_token("tok-c", 1, "admin"),
    );
    assert_eq!(shell.phase(), AppPhase::Ready);
    assert_eq!(shell.active_account(), Some(account_id(1)));
}

#[test]
fn a_connect_link_to_an_unreachable_server_fails_the_sign_in() {
    let mut shell = launched(Shell::new(Platform::Ios));
    shell.send(Event::LinkOpened(Link {
        url: "couchverse://connect?server=http%3A%2F%2F10.0.0.9%3A8080&code=K7PQ".into(),
    }));
    shell.fail("GET", "http://10.0.0.9:8080/api/v1/server", HttpFailureKind::Timeout);
    let view: SignInView = shell.view(&Surface::SignIn);
    assert_eq!(view.status, LoadStatus::Failed);
    assert_eq!(view.problem.expect("problem").code, "timeout");
    assert_eq!(shell.http_summary(), empty::<String>());
}

#[test]
fn an_expired_connect_code_is_reported() {
    let mut shell = launched(Shell::new(Platform::Ios));
    shell.send(Event::LinkOpened(Link {
        url: "couchverse://connect?server=https%3A%2F%2Fmedia.example.com&code=OLD".into(),
    }));
    shell.respond("GET", &format!("{HTTPS}/api/v1/server"), 200, server_info("#3a6ea5"));
    shell.respond(
        "POST",
        &format!("{HTTPS}/api/v1/auth/connect"),
        404,
        json!({ "error": { "code": "not_found", "message": "unknown or expired code" } }),
    );
    let view: SignInView = shell.view(&Surface::SignIn);
    assert_eq!(view.status, LoadStatus::Failed);
    assert_eq!(view.problem.expect("problem").code, "not_found");
    // the server itself was added and stays
    assert_eq!(shell.phase(), AppPhase::SignIn);
}

#[test]
fn other_links_are_ignored() {
    let mut shell = launched(Shell::new(Platform::Ios));
    shell.take_renders();
    shell.send(Event::LinkOpened(Link { url: "https://evil.example/connect?code=1".into() }));
    assert_eq!(shell.http_summary(), empty::<String>());
    assert_eq!(shell.take_renders(), empty::<Surface>());
}
