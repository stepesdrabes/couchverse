//! Accounts on servers: signing in (password, pairing a keyboardless device, a scanned connect
//! link), the account list behind "Who's watching?", device tokens in the secure store, the
//! devices list, and approving another device's pairing. Several accounts per server and
//! several servers per install (D10).

use std::collections::HashMap;

use couchverse_api::types::{
    ConnectRedemption, Device, DeviceSignIn, DeviceToken, Pairing, PairingPoll, PairingRequest,
    PairingStatus, PairingStatusStatus, User,
};
use couchverse_api::{Call, NoContent, ops, types};
use serde::{Deserialize, Serialize};
use typeshare::typeshare;

use crate::api::{Endpoint, Failure, decode};
use crate::core::Pending;
use crate::effects::Ctx;
use crate::messages::{CoreConfig, EffectOutput, LoadStatus, Problem, Surface, U53};
use crate::modules::servers::{Server, Servers};

const STORE_KEY: &str = "accounts";

/// Signed user ids (typeshare maps the name to a 53-bit-safe integer).
pub type I54 = i64;

fn token_key(account_id: &str) -> String {
    format!("token.{account_id}")
}

/// An account as persisted. The token lives in the secure store, never here.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct Account {
    id: String,
    server_id: String,
    user_id: I54,
    username: String,
    display_name: String,
    avatar_id: Option<String>,
    /// Lets the shells load avatars without the session (system image loaders send no
    /// headers); renewed whenever the account becomes active.
    artwork_grant: Option<String>,
}

#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct Persisted {
    accounts: Vec<Account>,
    active: Option<String>,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PasswordSignIn {
    pub server_id: String,
    pub username: String,
    pub password: String,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Link {
    /// A `couchverse://` URL from a QR code or a tapped link, or the pairing page a TV shows as
    /// a QR code (`<server>/pair?code=<code>`) scanned by the app.
    pub url: String,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AccountRef {
    pub account_id: String,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DeviceRef {
    pub device_id: String,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct UserCode {
    pub code: String,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PairingApproval {
    pub code: String,
    /// Renames the device; it keeps its own name when empty.
    #[serde(default)]
    pub device_name: String,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AccountsView {
    pub accounts: Vec<AccountCard>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub active: Option<String>,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AccountCard {
    pub id: String,
    pub server_id: String,
    pub server_name: String,
    /// The server is reached over plain http.
    pub insecure: bool,
    pub username: String,
    pub display_name: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub avatar_url: Option<String>,
    /// False once the server rejected the token (revoked or expired): sign in again.
    pub signed_in: bool,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SignInView {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub server_id: Option<String>,
    pub status: LoadStatus,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub problem: Option<Problem>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub pairing: Option<PairingView>,
    /// The account that just signed in; the shell moves on.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub signed_in: Option<String>,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PairingView {
    /// Shown large: `WDJB-MJHT`.
    pub user_code: String,
    /// Rendered as a QR code for a phone to open.
    pub verify_url: String,
    /// When the code stops working, on the shell's monotonic clock.
    pub expires_at_ms: U53,
    pub state: PairingState,
}

#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum PairingState {
    Waiting,
    Denied,
    Expired,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DevicesView {
    pub status: LoadStatus,
    pub devices: Vec<DeviceCard>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub problem: Option<Problem>,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DeviceCard {
    pub id: String,
    pub name: String,
    /// `ios`, `ipados`, `tvos`, `android`, `androidtv` or `web`.
    pub platform: String,
    /// RFC 3339.
    pub last_seen_at: String,
    pub current: bool,
    /// When it signed in, RFC 3339.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub signed_in_at: Option<String>,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PairingApprovalView {
    pub status: LoadStatus,
    pub code: String,
    pub device_name: String,
    pub platform: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub outcome: Option<ApprovalOutcome>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub problem: Option<Problem>,
}

#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum ApprovalOutcome {
    Approved,
    Denied,
}

#[derive(Debug, Clone, PartialEq)]
pub enum AccountsPending {
    Load,
    Token {
        account_id: String,
    },
    SignIn {
        server_id: String,
        call: Call<DeviceToken>,
    },
    PairingStarted {
        server_id: String,
        call: Call<Pairing>,
    },
    PairingTick,
    PairingPolled {
        call: Call<PairingStatus>,
    },
    PairingExpired,
    Connect {
        server_id: String,
        call: Call<DeviceToken>,
    },
    ArtworkGrant {
        account_id: String,
        call: Call<types::ArtworkGrant>,
    },
    Devices {
        generation: u64,
        call: Call<Vec<Device>>,
    },
    Revoked {
        generation: u64,
        call: Call<NoContent>,
    },
    ApprovalLoaded {
        call: Call<PairingRequest>,
    },
    ApprovalDecided {
        outcome: ApprovalOutcome,
        call: Call<NoContent>,
    },
    /// Fire-and-forget calls whose answer changes nothing (logout).
    Ignored,
}

struct PairingFlow {
    server_id: String,
    device_code: String,
    view: PairingView,
    poll_timer: U53,
    expiry_timer: U53,
}

/// What a resolved output means for the rest of the core.
pub enum AccountsChange {
    None,
    /// An account signed in (or was refreshed) and should become active.
    SignedIn(String),
    /// The server rejected an account's token.
    SignedOut(String),
    /// The server rejected the session of the web, which keeps no accounts.
    SessionRejected,
}

#[derive(Default)]
pub struct Accounts {
    persisted: Persisted,
    tokens: HashMap<String, String>,
    sign_in: SignInView,
    pairing: Option<PairingFlow>,
    devices: DevicesView,
    /// Where the devices list was last read from, to read it again after a revoke.
    devices_endpoint: Option<Endpoint>,
    /// Bumped when a session ends or another begins, so a list in flight for it is dropped.
    devices_generation: u64,
    approval: PairingApprovalView,
    /// A connect link waiting for its server to be identified: (server URL, code).
    connect: Option<(String, String)>,
}

impl Accounts {
    pub fn active(&self) -> Option<&str> {
        self.persisted.active.as_deref()
    }

    pub fn has_accounts(&self) -> bool {
        !self.persisted.accounts.is_empty()
    }

    pub fn count(&self) -> usize {
        self.persisted.accounts.len()
    }

    pub fn artwork_grant(&self, account_id: &str) -> Option<&str> {
        self.account(account_id).and_then(|a| a.artwork_grant.as_deref())
    }

    pub fn server_of(&self, account_id: &str) -> Option<&str> {
        self.account(account_id).map(|a| a.server_id.as_str())
    }

    fn account(&self, id: &str) -> Option<&Account> {
        self.persisted.accounts.iter().find(|a| a.id == id)
    }

    /// Where the account's API calls go, with its token; `None` without a server or token.
    pub fn endpoint(&self, servers: &Servers, account_id: &str) -> Option<Endpoint> {
        let account = self.account(account_id)?;
        let server = servers.get(&account.server_id)?;
        let token = self.tokens.get(account_id)?;
        Some(Endpoint { base: server.url.clone(), token: Some(token.clone()) })
    }

    pub fn artwork_url(
        &self,
        servers: &Servers,
        account_id: &str,
        artwork_id: &str,
    ) -> Option<String> {
        let account = self.account(account_id)?;
        let server = servers.get(&account.server_id)?;
        let grant = account.artwork_grant.as_ref()?;
        Some(format!("{}/api/v1/artwork/{artwork_id}?size=w342&g={grant}", server.url))
    }

    pub fn view(&self, servers: &Servers) -> AccountsView {
        let accounts = self
            .persisted
            .accounts
            .iter()
            .map(|a| {
                let server = servers.get(&a.server_id);
                AccountCard {
                    id: a.id.clone(),
                    server_id: a.server_id.clone(),
                    server_name: server.map(|s| s.name.clone()).unwrap_or_default(),
                    insecure: server.is_some_and(|s| s.insecure),
                    username: a.username.clone(),
                    display_name: a.display_name.clone(),
                    avatar_url: a
                        .avatar_id
                        .as_deref()
                        .and_then(|id| self.artwork_url(servers, &a.id, id)),
                    signed_in: self.tokens.contains_key(&a.id),
                }
            })
            .collect();
        AccountsView { accounts, active: self.persisted.active.clone() }
    }

    pub fn sign_in_view(&self) -> SignInView {
        let mut view = self.sign_in.clone();
        view.pairing = self.pairing.as_ref().map(|p| p.view.clone());
        view
    }

    pub fn devices_view(&self) -> DevicesView {
        self.devices.clone()
    }

    pub fn approval_view(&self) -> PairingApprovalView {
        self.approval.clone()
    }

    pub fn load(&mut self, ctx: &mut Ctx) {
        ctx.store_read(STORE_KEY, Pending::Accounts(AccountsPending::Load));
    }

    fn persist(&self, ctx: &mut Ctx) {
        let json = serde_json::to_string(&self.persisted).expect("accounts serialize");
        ctx.store_write(STORE_KEY, json);
    }

    pub fn password_sign_in(
        &mut self,
        ctx: &mut Ctx,
        servers: &Servers,
        config: &CoreConfig,
        req: &PasswordSignIn,
    ) {
        let Some(server) = servers.get(&req.server_id) else {
            return;
        };
        let body = DeviceSignIn {
            username: req.username.clone(),
            password: req.password.clone(),
            device_name: config.device_name.clone(),
            platform: api_enum(config),
        };
        let call = ops::sign_in_device(&body);
        let request = Endpoint::anonymous(&server.url).request(&call.request);
        self.cancel_pairing(ctx);
        self.sign_in = SignInView {
            server_id: Some(server.id.clone()),
            status: LoadStatus::Loading,
            ..SignInView::default()
        };
        ctx.http(
            request,
            Pending::Accounts(AccountsPending::SignIn { server_id: server.id.clone(), call }),
        );
        ctx.render(Surface::SignIn);
    }

    pub fn start_pairing(
        &mut self,
        ctx: &mut Ctx,
        servers: &Servers,
        config: &CoreConfig,
        server_id: &str,
    ) {
        let Some(server) = servers.get(server_id) else {
            return;
        };
        self.cancel_pairing(ctx);
        let body = types::DeviceInfo {
            device_name: config.device_name.clone(),
            platform: api_enum(config),
        };
        let call = ops::start_pairing(&body);
        let request = Endpoint::anonymous(&server.url).request(&call.request);
        self.sign_in = SignInView {
            server_id: Some(server.id.clone()),
            status: LoadStatus::Loading,
            ..SignInView::default()
        };
        ctx.http(
            request,
            Pending::Accounts(AccountsPending::PairingStarted {
                server_id: server.id.clone(),
                call,
            }),
        );
        ctx.render(Surface::SignIn);
    }

    pub fn cancel_pairing(&mut self, ctx: &mut Ctx) {
        if let Some(flow) = self.pairing.take() {
            ctx.cancel_timer(flow.poll_timer);
            ctx.cancel_timer(flow.expiry_timer);
            ctx.render(Surface::SignIn);
        }
    }

    /// A `couchverse://` link. Returns the server URL to identify for a connect link.
    pub fn open_link(&mut self, ctx: &mut Ctx, servers: &Servers, url: &str) -> Option<String> {
        match parse_link(url)? {
            ParsedLink::Connect { server, code } => {
                self.sign_in = SignInView { status: LoadStatus::Loading, ..SignInView::default() };
                ctx.render(Surface::SignIn);
                self.connect = Some((server.clone(), code));
                Some(server)
            }
            ParsedLink::Pair { code } => {
                self.open_approval(ctx, servers, &code);
                None
            }
        }
    }

    /// The server of a pending connect link was identified: redeem the code on it.
    pub fn server_identified(&mut self, ctx: &mut Ctx, config: &CoreConfig, server: &Server) {
        let Some((url, code)) = self.connect.take() else {
            return;
        };
        if !crate::modules::servers::candidate_urls(&url).contains(&server.url) {
            self.connect = Some((url, code));
            return;
        }
        let body = ConnectRedemption {
            code,
            device_name: config.device_name.clone(),
            platform: api_enum(config),
        };
        let call = ops::connect_device(&body);
        let request = Endpoint::anonymous(&server.url).request(&call.request);
        ctx.http(
            request,
            Pending::Accounts(AccountsPending::Connect { server_id: server.id.clone(), call }),
        );
    }

    /// Adding a server for a connect link failed: surface it on the sign-in view.
    pub fn connect_failed(&mut self, ctx: &mut Ctx, problem: Option<Problem>) {
        if self.connect.take().is_some() {
            self.sign_in.status = LoadStatus::Failed;
            self.sign_in.problem = problem;
            ctx.render(Surface::SignIn);
        }
    }

    pub fn select(&mut self, ctx: &mut Ctx, account_id: &str) -> bool {
        if self.account(account_id).is_none() {
            return false;
        }
        self.persisted.active = Some(account_id.to_string());
        self.persist(ctx);
        ctx.render(Surface::Accounts);
        true
    }

    pub fn sign_out(&mut self, ctx: &mut Ctx, servers: &Servers, account_id: &str) {
        if let Some(endpoint) = self.endpoint(servers, account_id) {
            let call = ops::logout();
            ctx.http(endpoint.request(&call.request), Pending::Accounts(AccountsPending::Ignored));
        }
        self.persisted.accounts.retain(|a| a.id != account_id);
        if self.persisted.active.as_deref() == Some(account_id) {
            self.persisted.active = None;
        }
        self.tokens.remove(account_id);
        ctx.secure_delete(&token_key(account_id));
        self.persist(ctx);
        ctx.render(Surface::Accounts);
    }

    /// Drops every account of a removed server.
    pub fn remove_server(&mut self, ctx: &mut Ctx, server_id: &str) {
        let ids: Vec<String> = self
            .persisted
            .accounts
            .iter()
            .filter(|a| a.server_id == server_id)
            .map(|a| a.id.clone())
            .collect();
        for id in &ids {
            self.tokens.remove(id);
            ctx.secure_delete(&token_key(id));
        }
        self.persisted.accounts.retain(|a| a.server_id != server_id);
        if self.persisted.active.as_ref().is_some_and(|a| ids.contains(a)) {
            self.persisted.active = None;
        }
        self.persist(ctx);
        ctx.render(Surface::Accounts);
    }

    /// The server rejected the account's token: keep the account and its server, but sign it
    /// out so the user signs in again.
    pub fn token_rejected(&mut self, ctx: &mut Ctx, account_id: &str) {
        if self.tokens.remove(account_id).is_some() {
            ctx.secure_delete(&token_key(account_id));
            ctx.render(Surface::Accounts);
        }
    }

    /// Renews the account's artwork grant when it becomes active.
    pub fn refresh(&mut self, ctx: &mut Ctx, servers: &Servers, account_id: &str) {
        let Some(endpoint) = self.endpoint(servers, account_id) else {
            return;
        };
        let grant = ops::get_artwork_grant();
        ctx.http(
            endpoint.request(&grant.request),
            Pending::Accounts(AccountsPending::ArtworkGrant {
                account_id: account_id.to_string(),
                call: grant,
            }),
        );
    }

    /// Forgets the devices list: the session ended or another began.
    pub fn forget_devices(&mut self, ctx: &mut Ctx) {
        self.devices = DevicesView::default();
        self.devices_endpoint = None;
        self.devices_generation += 1;
        ctx.render(Surface::Devices);
    }

    /// The signed-in session's devices, read through `endpoint` (the active account's server,
    /// or the web's cookie session).
    pub fn open_devices(&mut self, ctx: &mut Ctx, endpoint: &Endpoint) {
        let call = ops::list_devices();
        self.devices.status =
            if self.devices.devices.is_empty() { LoadStatus::Loading } else { LoadStatus::Stale };
        let generation = self.devices_generation;
        ctx.http(
            endpoint.request(&call.request),
            Pending::Accounts(AccountsPending::Devices { generation, call }),
        );
        self.devices_endpoint = Some(endpoint.clone());
        ctx.render(Surface::Devices);
    }

    pub fn revoke_device(&mut self, ctx: &mut Ctx, endpoint: &Endpoint, device_id: &str) {
        let call = ops::revoke_device(device_id);
        self.devices.devices.retain(|d| d.id != device_id);
        let generation = self.devices_generation;
        ctx.http(
            endpoint.request(&call.request),
            Pending::Accounts(AccountsPending::Revoked { generation, call }),
        );
        ctx.render(Surface::Devices);
    }

    pub fn open_approval(&mut self, ctx: &mut Ctx, servers: &Servers, code: &str) {
        self.approval = PairingApprovalView {
            code: code.to_string(),
            status: LoadStatus::Loading,
            ..Default::default()
        };
        let Some(endpoint) = self.active().and_then(|a| self.endpoint(servers, a)) else {
            self.approval.status = LoadStatus::Failed;
            self.approval.problem =
                Some(Problem::new("unauthorized", "sign in to approve a device"));
            ctx.render(Surface::PairingApproval);
            return;
        };
        let call = ops::get_pairing_request(code);
        ctx.http(
            endpoint.request(&call.request),
            Pending::Accounts(AccountsPending::ApprovalLoaded { call }),
        );
        ctx.render(Surface::PairingApproval);
    }

    pub fn decide_pairing(
        &mut self,
        ctx: &mut Ctx,
        servers: &Servers,
        code: &str,
        approve: Option<&str>,
    ) {
        let Some(endpoint) = self.active().and_then(|a| self.endpoint(servers, a)) else {
            return;
        };
        let (outcome, call) = match approve {
            Some(name) => {
                let body = types::PairingApproval {
                    device_name: Some(name.to_string()).filter(|n| !n.is_empty()),
                };
                (ApprovalOutcome::Approved, ops::approve_pairing(code, Some(&body)))
            }
            None => (ApprovalOutcome::Denied, ops::deny_pairing(code)),
        };
        self.approval.status = LoadStatus::Loading;
        ctx.http(
            endpoint.request(&call.request),
            Pending::Accounts(AccountsPending::ApprovalDecided { outcome, call }),
        );
        ctx.render(Surface::PairingApproval);
    }

    pub fn resolve(
        &mut self,
        ctx: &mut Ctx,
        servers: &Servers,
        pending: AccountsPending,
        output: EffectOutput,
    ) -> AccountsChange {
        match pending {
            AccountsPending::Load => self.loaded(ctx, output),
            AccountsPending::Token { account_id } => {
                if let EffectOutput::Stored(stored) = output
                    && let Some(token) = stored.value
                {
                    self.tokens.insert(account_id, token);
                }
                ctx.render(Surface::Accounts);
            }
            AccountsPending::SignIn { server_id, call }
            | AccountsPending::Connect { server_id, call } => {
                return match decode(&call, output) {
                    Ok(token) => self.signed_in(ctx, &server_id, token),
                    Err(failure) => {
                        self.sign_in_failed(ctx, &failure);
                        AccountsChange::None
                    }
                };
            }
            AccountsPending::PairingStarted { server_id, call } => match decode(&call, output) {
                Ok(pairing) => self.pairing_started(ctx, servers, server_id, pairing),
                Err(failure) => self.sign_in_failed(ctx, &failure),
            },
            AccountsPending::PairingTick => self.poll(ctx, servers),
            AccountsPending::PairingPolled { call } => {
                return self.polled(ctx, decode(&call, output));
            }
            AccountsPending::PairingExpired => self.pairing_ended(ctx, PairingState::Expired),
            AccountsPending::ArtworkGrant { account_id, call } => {
                if let Ok(grant) = decode(&call, output)
                    && let Some(account) = self.account_mut(&account_id)
                {
                    account.artwork_grant = Some(grant.grant);
                    self.persist(ctx);
                    ctx.render(Surface::Accounts);
                }
            }
            // an answer for a session that has since ended
            AccountsPending::Devices { generation, .. }
            | AccountsPending::Revoked { generation, .. }
                if generation != self.devices_generation => {}
            AccountsPending::Devices { call, .. } => {
                return self.devices_loaded(ctx, decode(&call, output));
            }
            AccountsPending::Revoked { call, .. } => {
                if let Err(failure) = decode(&call, output) {
                    self.devices.problem = Some(failure.problem());
                }
                if let Some(endpoint) = self.devices_endpoint.clone() {
                    self.open_devices(ctx, &endpoint);
                }
            }
            AccountsPending::ApprovalLoaded { call } => {
                self.approval_loaded(ctx, decode(&call, output));
            }
            AccountsPending::ApprovalDecided { outcome, call } => {
                match decode(&call, output) {
                    Ok(NoContent) => {
                        self.approval.status = LoadStatus::Loaded;
                        self.approval.outcome = Some(outcome);
                    }
                    Err(failure) => {
                        self.approval.status = LoadStatus::Failed;
                        self.approval.problem = Some(failure.problem());
                    }
                }
                ctx.render(Surface::PairingApproval);
            }
            AccountsPending::Ignored => {}
        }
        AccountsChange::None
    }

    /// The persisted accounts arrived; each one's token is read from secure storage next.
    fn loaded(&mut self, ctx: &mut Ctx, output: EffectOutput) {
        if let EffectOutput::Stored(stored) = output {
            self.persisted =
                stored.value.and_then(|j| serde_json::from_str(&j).ok()).unwrap_or_default();
        }
        for account in &self.persisted.accounts {
            ctx.secure_read(
                &token_key(&account.id),
                Pending::Accounts(AccountsPending::Token { account_id: account.id.clone() }),
            );
        }
        ctx.render(Surface::Accounts);
    }

    fn sign_in_failed(&mut self, ctx: &mut Ctx, failure: &Failure) {
        self.sign_in.status = LoadStatus::Failed;
        self.sign_in.problem = Some(failure.problem());
        ctx.render(Surface::SignIn);
    }

    fn pairing_started(
        &mut self,
        ctx: &mut Ctx,
        servers: &Servers,
        server_id: String,
        pairing: Pairing,
    ) {
        let Some(server) = servers.get(&server_id) else {
            return;
        };
        let ttl = u64::try_from(pairing.expires_in).unwrap_or(0) * 1000;
        let interval = u64::try_from(pairing.interval).unwrap_or(5).max(1) * 1000;
        self.pairing = Some(PairingFlow {
            server_id,
            device_code: pairing.device_code,
            view: PairingView {
                user_code: pairing.user_code,
                verify_url: format!("{}{}", server.url, pairing.verify_path),
                expires_at_ms: ctx.now + ttl,
                state: PairingState::Waiting,
            },
            poll_timer: ctx.every(interval, Pending::Accounts(AccountsPending::PairingTick)),
            expiry_timer: ctx.after(ttl, Pending::Accounts(AccountsPending::PairingExpired)),
        });
        self.sign_in.status = LoadStatus::Idle;
        ctx.render(Surface::SignIn);
    }

    /// Keeps the account's card in step with its profile, from the session's fresh copy.
    pub fn update_profile(&mut self, ctx: &mut Ctx, account_id: &str, user: &User) {
        let Some(account) = self.account_mut(account_id) else {
            return;
        };
        let changed = account.username != user.username
            || account.display_name != user.display_name
            || account.avatar_id != user.avatar_id;
        if changed {
            account.username.clone_from(&user.username);
            account.display_name.clone_from(&user.display_name);
            account.avatar_id.clone_from(&user.avatar_id);
            self.persist(ctx);
            ctx.render(Surface::Accounts);
        }
    }

    fn devices_loaded(
        &mut self,
        ctx: &mut Ctx,
        result: Result<Vec<Device>, Failure>,
    ) -> AccountsChange {
        match result {
            Ok(devices) => {
                self.devices = DevicesView {
                    status: LoadStatus::Loaded,
                    devices: devices.into_iter().map(device_card).collect(),
                    problem: None,
                };
            }
            Err(failure) => {
                if failure.unauthorized() {
                    return match self.persisted.active.clone() {
                        Some(active) => AccountsChange::SignedOut(active),
                        None => AccountsChange::SessionRejected,
                    };
                }
                self.devices.status = LoadStatus::Failed;
                self.devices.problem = Some(failure.problem());
            }
        }
        ctx.render(Surface::Devices);
        AccountsChange::None
    }

    fn approval_loaded(&mut self, ctx: &mut Ctx, result: Result<PairingRequest, Failure>) {
        match result {
            Ok(req) => {
                self.approval = PairingApprovalView {
                    status: LoadStatus::Loaded,
                    code: req.user_code,
                    device_name: req.device_name,
                    platform: req.platform.as_str().to_string(),
                    outcome: None,
                    problem: None,
                };
            }
            Err(failure) => {
                let gone = matches!(&failure, Failure::Api(e) if e.status == 404);
                self.approval.status = if gone { LoadStatus::NotFound } else { LoadStatus::Failed };
                self.approval.problem = Some(failure.problem());
            }
        }
        ctx.render(Surface::PairingApproval);
    }

    fn account_mut(&mut self, account_id: &str) -> Option<&mut Account> {
        self.persisted.accounts.iter_mut().find(|a| a.id == account_id)
    }

    /// Polls a waiting pairing right away, e.g. when the app returns from the background.
    pub fn poll_pairing_now(&mut self, ctx: &mut Ctx, servers: &Servers) {
        if self.pairing.as_ref().is_some_and(|p| p.view.state == PairingState::Waiting) {
            self.poll(ctx, servers);
        }
    }

    fn poll(&mut self, ctx: &mut Ctx, servers: &Servers) {
        let Some(flow) = &self.pairing else { return };
        let Some(server) = servers.get(&flow.server_id) else {
            return;
        };
        let call = ops::poll_pairing(&PairingPoll { device_code: flow.device_code.clone() });
        let request = Endpoint::anonymous(&server.url).request(&call.request);
        ctx.http(request, Pending::Accounts(AccountsPending::PairingPolled { call }));
    }

    fn polled(&mut self, ctx: &mut Ctx, result: Result<PairingStatus, Failure>) -> AccountsChange {
        let Some(server_id) = self.pairing.as_ref().map(|p| p.server_id.clone()) else {
            return AccountsChange::None;
        };
        match result {
            Ok(status) => match status.status {
                PairingStatusStatus::Approved => {
                    let Some(token) = status.device else {
                        return AccountsChange::None;
                    };
                    self.cancel_pairing(ctx);
                    return self.signed_in(ctx, &server_id, token);
                }
                PairingStatusStatus::Denied => self.pairing_ended(ctx, PairingState::Denied),
                PairingStatusStatus::Expired => self.pairing_ended(ctx, PairingState::Expired),
                PairingStatusStatus::Pending | PairingStatusStatus::Unknown => {}
            },
            // a consumed or forgotten pairing (server restart) cannot complete any more
            Err(Failure::Api(e)) if e.status == 404 => {
                self.pairing_ended(ctx, PairingState::Expired);
            }
            // slow_down and network trouble: the next tick tries again
            Err(_) => {}
        }
        AccountsChange::None
    }

    fn pairing_ended(&mut self, ctx: &mut Ctx, state: PairingState) {
        if let Some(flow) = &mut self.pairing {
            flow.view.state = state;
            let (poll, expiry) = (flow.poll_timer, flow.expiry_timer);
            ctx.cancel_timer(poll);
            ctx.cancel_timer(expiry);
            ctx.render(Surface::SignIn);
        }
    }

    fn signed_in(&mut self, ctx: &mut Ctx, server_id: &str, token: DeviceToken) -> AccountsChange {
        let user = token.user;
        let id = format!("{server_id}/{}", user.id);
        let account = Account {
            id: id.clone(),
            server_id: server_id.to_string(),
            user_id: user.id,
            username: user.username,
            display_name: user.display_name,
            avatar_id: user.avatar_id,
            artwork_grant: None,
        };
        match self.persisted.accounts.iter_mut().find(|a| a.id == id) {
            Some(existing) => *existing = account,
            None => self.persisted.accounts.push(account),
        }
        self.tokens.insert(id.clone(), token.token.clone());
        ctx.secure_write(&token_key(&id), token.token);
        self.persisted.active = Some(id.clone());
        self.persist(ctx);
        self.sign_in = SignInView {
            server_id: Some(server_id.to_string()),
            status: LoadStatus::Loaded,
            signed_in: Some(id.clone()),
            ..SignInView::default()
        };
        ctx.render(Surface::SignIn);
        ctx.render(Surface::Accounts);
        AccountsChange::SignedIn(id)
    }
}

fn device_card(d: Device) -> DeviceCard {
    DeviceCard {
        id: d.id,
        name: d.name,
        platform: d.platform.as_str().to_string(),
        last_seen_at: d.last_seen_at,
        current: d.current,
        signed_in_at: Some(d.created_at),
    }
}

/// The generated API types give every inline enum its own type; they all decode from the same
/// strings, so the platform travels as its API name.
fn api_enum<T: serde::de::DeserializeOwned>(config: &CoreConfig) -> T {
    serde_json::from_value(serde_json::Value::String(config.platform.api_name().to_string()))
        .expect("every native platform is an API platform")
}

#[derive(Debug, PartialEq, Eq)]
enum ParsedLink {
    Connect { server: String, code: String },
    Pair { code: String },
}

/// `couchverse://connect?server=<url>&code=<code>`, `couchverse://pair?code=<code>`, and a
/// server's pairing page (`<server>/pair?code=<code>`): the QR code a TV shows opens the web app
/// in a phone's camera, but the app's own scanner approves the code in place.
fn parse_link(url: &str) -> Option<ParsedLink> {
    if let Some(rest) = url.strip_prefix("couchverse://") {
        let (path, query) = rest.split_once('?').unwrap_or((rest, ""));
        return match path.trim_end_matches('/') {
            "connect" => Some(ParsedLink::Connect {
                server: query_param(query, "server")?,
                code: query_param(query, "code")?,
            }),
            "pair" => Some(ParsedLink::Pair { code: query_param(query, "code")? }),
            _ => None,
        };
    }
    let rest = url.strip_prefix("https://").or_else(|| url.strip_prefix("http://"))?;
    let (location, query) = rest.split_once('?').unwrap_or((rest, ""));
    let path = location.split_once('/').map_or("", |(_, path)| path);
    if path.trim_end_matches('/') != "pair" {
        return None;
    }
    Some(ParsedLink::Pair { code: query_param(query, "code")? })
}

fn query_param(query: &str, name: &str) -> Option<String> {
    query
        .split('&')
        .filter_map(|pair| pair.split_once('='))
        .find(|(k, _)| *k == name)
        .map(|(_, v)| percent_decode(v))
}

fn percent_decode(s: &str) -> String {
    let bytes = s.as_bytes();
    let mut out = Vec::with_capacity(bytes.len());
    let mut i = 0;
    while i < bytes.len() {
        let hex = (bytes[i] == b'%' && i + 2 < bytes.len())
            .then(|| std::str::from_utf8(&bytes[i + 1..i + 3]).ok())
            .flatten()
            .and_then(|h| u8::from_str_radix(h, 16).ok());
        match (hex, bytes[i]) {
            (Some(b), _) => {
                out.push(b);
                i += 3;
                continue;
            }
            (None, b'+') => out.push(b' '),
            (None, b) => out.push(b),
        }
        i += 1;
    }
    String::from_utf8_lossy(&out).into_owned()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn connect_links_carry_the_server_and_code() {
        assert_eq!(
            parse_link("couchverse://connect?server=https%3A%2F%2Fmedia.example.com&code=abc-123"),
            Some(ParsedLink::Connect {
                server: "https://media.example.com".into(),
                code: "abc-123".into()
            })
        );
        assert_eq!(
            parse_link("couchverse://pair?code=WDJB-MJHT"),
            Some(ParsedLink::Pair { code: "WDJB-MJHT".into() })
        );
    }

    #[test]
    fn a_servers_pairing_page_is_a_pair_link() {
        assert_eq!(
            parse_link("http://192.168.1.5:8080/pair?code=WDJB-MJHT"),
            Some(ParsedLink::Pair { code: "WDJB-MJHT".into() })
        );
        assert_eq!(
            parse_link("https://media.example.com/pair/?code=wdjb-mjht"),
            Some(ParsedLink::Pair { code: "wdjb-mjht".into() })
        );
        assert_eq!(parse_link("https://media.example.com/pair"), None);
        assert_eq!(parse_link("https://media.example.com/watch?code=WDJB-MJHT"), None);
        assert_eq!(parse_link("https://media.example.com?code=WDJB-MJHT"), None);
    }

    #[test]
    fn other_links_are_not_sign_in_links() {
        assert_eq!(parse_link("couchverse://connect?code=only"), None);
        assert_eq!(parse_link("https://evil.example/connect?server=x&code=y"), None);
        assert_eq!(parse_link("couchverse://title/glass-harbor"), None);
    }
}
