use serde_json::json;

use super::*;
use crate::modules::accounts::{AccountRef, AccountsView};
use crate::modules::servers::ServersView;
use crate::modules::session::SessionView;

#[test]
fn a_first_launch_reads_the_stores_and_welcomes() {
    let mut shell = Shell::new(Platform::Ios);
    shell.send(Event::AppStarted);
    assert_eq!(shell.phase(), AppPhase::Starting);
    assert!(shell.http_summary().is_empty());

    shell.answer_reads();
    assert_eq!(shell.phase(), AppPhase::Welcome);
    assert!(shell.take_renders().contains(&Surface::App));
    assert_eq!(shell.core.pending_effects(), 0);
}

#[test]
fn a_phone_resumes_its_last_account() {
    let users = [(1, "admin", Some("tok-1")), (2, "nora", Some("tok-2"))];
    let mut shell = launched(returning(Platform::Ios, &users, 2));

    assert_eq!(shell.phase(), AppPhase::Ready);
    assert_eq!(shell.active_account(), Some(account_id(2)));
    let (_, me) = shell.request("GET", &format!("{HTTPS}/api/v1/auth/me"));
    assert_eq!(header(&me, "Authorization"), Some("Bearer tok-2"));
    shell.request("GET", &format!("{HTTPS}/api/v1/me/artwork-grant"));

    shell.answer_session(HTTPS, user(2, "nora"), Some("en"));
    let session: SessionView = shell.view(&Surface::Session);
    assert_eq!(session.status, LoadStatus::Loaded);
    assert_eq!(session.user.expect("user").username, "nora");
    assert_eq!(session.language, "en");
    assert!(session.features.couch);
    assert!(!session.features.rankings);
    assert_eq!(session.accent.accent, "#3a6ea5");
}

#[test]
fn a_tv_always_asks_who_is_watching() {
    let users = [(1, "admin", Some("tok-1")), (2, "nora", Some("tok-2"))];
    let mut shell = launched(returning(Platform::Tvos, &users, 2));

    assert_eq!(shell.phase(), AppPhase::ChooseAccount);
    assert!(shell.http_summary().is_empty(), "nothing loads before someone is picked");
    let accounts: AccountsView = shell.view(&Surface::Accounts);
    assert_eq!(accounts.accounts.len(), 2);
    assert!(accounts.accounts.iter().all(|a| a.signed_in && a.server_name == "Home Media"));

    shell.send(Event::AccountSelected(AccountRef { account_id: account_id(1) }));
    assert_eq!(shell.phase(), AppPhase::Ready);
    assert_eq!(shell.active_account(), Some(account_id(1)));
    let (_, me) = shell.request("GET", &format!("{HTTPS}/api/v1/auth/me"));
    assert_eq!(header(&me, "Authorization"), Some("Bearer tok-1"));
    let stored: serde_json::Value = serde_json::from_str(&shell.store["accounts"]).unwrap();
    assert_eq!(stored["active"], json!(account_id(1)));
}

#[test]
fn an_account_without_its_token_stays_listed_but_signed_out() {
    let users = [(1, "admin", None), (2, "nora", Some("tok-2"))];
    let shell = launched(returning(Platform::Ios, &users, 1));

    // the last account cannot resume without a token, so the picker shows instead
    assert_eq!(shell.phase(), AppPhase::ChooseAccount);
    let accounts: AccountsView = shell.view(&Surface::Accounts);
    let signed_in: Vec<bool> = accounts.accounts.iter().map(|a| a.signed_in).collect();
    assert_eq!(signed_in, [false, true]);
}

#[test]
fn unreadable_stores_start_fresh() {
    let mut shell = Shell::new(Platform::Ios);
    shell.store.insert("servers".into(), "{not json".into());
    shell.store.insert("accounts".into(), "[]".into());
    let shell = launched(shell);
    assert_eq!(shell.phase(), AppPhase::Welcome);
    let servers: ServersView = shell.view(&Surface::Servers);
    assert!(servers.servers.is_empty());
}

#[test]
fn signing_out_drops_the_account_and_its_token() {
    let users = [(1, "admin", Some("tok-1")), (2, "nora", Some("tok-2"))];
    let mut shell = launched(returning(Platform::Ios, &users, 2));
    shell.answer_session(HTTPS, user(2, "nora"), Some("cs"));

    shell.send(Event::SignOutRequested(AccountRef { account_id: account_id(2) }));
    let (_, logout) = shell.request("POST", &format!("{HTTPS}/api/v1/auth/logout"));
    assert_eq!(header(&logout, "Authorization"), Some("Bearer tok-2"));
    assert!(!shell.secure.contains_key(&format!("token.{}", account_id(2))));
    assert_eq!(shell.phase(), AppPhase::ChooseAccount);
    assert_eq!(shell.active_account(), None);
    let accounts: AccountsView = shell.view(&Surface::Accounts);
    assert_eq!(accounts.accounts.len(), 1);
    assert_eq!(accounts.active, None);

    // relaunching shows the same state
    let shell = launched(shell.relaunch(Platform::Ios));
    assert_eq!(shell.phase(), AppPhase::ChooseAccount);
}

#[test]
fn removing_the_active_server_ends_the_session() {
    let users = [(1, "admin", Some("tok-1"))];
    let mut shell = launched(returning(Platform::Ios, &users, 1));
    shell.answer_session(HTTPS, user(1, "admin"), Some("en"));

    shell.send(Event::ServerRemoved(crate::modules::servers::ServerRef {
        server_id: SERVER_ID.into(),
    }));
    assert_eq!(shell.phase(), AppPhase::Welcome);
    assert!(shell.secure.is_empty());
    let session: SessionView = shell.view(&Surface::Session);
    assert_eq!(session.account_id, None);
    assert_eq!(session.status, LoadStatus::Idle);
}
