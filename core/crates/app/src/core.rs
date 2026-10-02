//! The core: one model, the modules that own its parts, and the routing of events, effect
//! outputs and view requests between them. Everything here is synchronous and deterministic:
//! time comes in with every message and all I/O goes out as effects.

use serde::{Deserialize, Serialize};
use typeshare::typeshare;

use crate::api::Endpoint;
use crate::effects::{Ctx, Registry};
use crate::messages::{
    AuthMode, CoreConfig, EffectOutput, EffectRequest, Event, LoadStatus, Message, Platform,
    Resolution, Surface,
};
use crate::modules::accounts::{Accounts, AccountsChange, AccountsPending};
use crate::modules::markdown;
use crate::modules::servers::{Servers, ServersPending};
use crate::modules::session::{Session, SessionChange, SessionPending};
use crate::modules::theme;

/// What a module is waiting for from an effect it issued.
#[derive(Debug, Clone, PartialEq)]
pub enum Pending {
    Servers(ServersPending),
    Accounts(AccountsPending),
    Session(SessionPending),
}

/// Where the app is, so a shell knows which root screen to show.
#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum AppPhase {
    /// Loading what was persisted.
    Starting,
    /// No server yet: welcome and add one.
    Welcome,
    /// A server but no account on it: sign in.
    SignIn,
    /// "Who's watching?": pick an account (always first on a TV, D10).
    ChooseAccount,
    /// An account is active.
    Ready,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AppView {
    pub phase: AppPhase,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub active_account: Option<String>,
}

/// The account id the web's single cookie session runs as.
const WEB_ACCOUNT: &str = "web";

pub struct Core {
    registry: Registry,
    model: Model,
}

/// Everything the core knows, apart from the effects in flight. Kept apart from the registry
/// so a module can hold a `Ctx` over the registry while the model routes between modules.
struct Model {
    config: CoreConfig,
    servers: Servers,
    accounts: Accounts,
    session: Session,
    phase: AppPhase,
    /// Persisted reads still outstanding at start-up; the phase is decided when they land.
    boot_reads: usize,
}

impl Core {
    pub fn new(config: CoreConfig) -> Self {
        let session = Session::new(&config.locale);
        let model = Model {
            config,
            servers: Servers::default(),
            accounts: Accounts::default(),
            session,
            phase: AppPhase::Starting,
            boot_reads: 0,
        };
        Self { registry: Registry::default(), model }
    }

    pub fn send(&mut self, message: Message) -> Vec<EffectRequest> {
        let mut ctx = Ctx::new(message.now_ms, &mut self.registry);
        self.model.send(&mut ctx, message.event);
        self.registry.drain()
    }

    pub fn resolve(&mut self, resolution: Resolution) -> Vec<EffectRequest> {
        // a cancelled timer's late tick, a fire-and-forget write: nothing is waiting
        if let Some(pending) = self.registry.take(resolution.id) {
            let mut ctx = Ctx::new(resolution.now_ms, &mut self.registry);
            self.model.resolve(&mut ctx, pending, resolution.output);
        }
        self.registry.drain()
    }

    pub fn view(&self, surface: &Surface) -> serde_json::Value {
        self.model.view(surface)
    }

    #[cfg(test)]
    pub fn pending_effects(&self) -> usize {
        self.registry.pending_count()
    }
}

impl Model {
    fn send(&mut self, ctx: &mut Ctx, event: Event) {
        match event {
            Event::AppStarted => {
                if self.config.auth_mode == AuthMode::Cookie {
                    self.start_web_session(ctx);
                } else {
                    self.servers.load(ctx);
                    self.accounts.load(ctx);
                    self.boot_reads = 2;
                }
            }
            Event::AppBecameActive => {
                if self.phase == AppPhase::Ready {
                    self.session.refresh(ctx);
                }
                // timers do not run while an app is suspended
                self.accounts.poll_pairing_now(ctx, &self.servers);
            }
            Event::SessionStarted => {
                if self.config.auth_mode == AuthMode::Cookie {
                    self.start_web_session(ctx);
                }
            }
            Event::ServerAddressSubmitted(address) => {
                self.servers.submit_address(ctx, &address.address);
            }
            Event::ServerRemoved(server) => {
                let active_server =
                    self.session.account_id().and_then(|a| self.accounts.server_of(a));
                if active_server == Some(server.server_id.as_str()) {
                    self.session.end(ctx);
                }
                self.servers.remove(ctx, &server.server_id);
                self.accounts.remove_server(ctx, &server.server_id);
                if self.session.account_id().is_none() {
                    self.settle(ctx);
                }
            }
            Event::PasswordSignInSubmitted(req) => {
                self.accounts.password_sign_in(ctx, &self.servers, &self.config, &req);
            }
            Event::PairingStarted(server) => {
                self.accounts.start_pairing(ctx, &self.servers, &self.config, &server.server_id);
            }
            Event::PairingCancelled => self.accounts.cancel_pairing(ctx),
            Event::LinkOpened(link) => {
                if let Some(server_url) = self.accounts.open_link(ctx, &self.servers, &link.url) {
                    self.servers.submit_address(ctx, &server_url);
                }
            }
            Event::AccountSelected(account) => {
                if self.accounts.select(ctx, &account.account_id) {
                    self.activate(ctx, &account.account_id);
                }
            }
            Event::SignOutRequested(_) if self.config.auth_mode == AuthMode::Cookie => {
                self.session.log_out(ctx);
                self.phase = AppPhase::SignIn;
                ctx.render(Surface::App);
            }
            Event::SignOutRequested(account) => {
                if self.session.account_id() == Some(account.account_id.as_str()) {
                    self.session.end(ctx);
                }
                self.accounts.sign_out(ctx, &self.servers, &account.account_id);
                if self.session.account_id().is_none() {
                    self.settle(ctx);
                }
            }
            Event::DevicesOpened => self.accounts.open_devices(ctx, &self.servers),
            Event::DeviceRevoked(device) => {
                self.accounts.revoke_device(ctx, &self.servers, &device.device_id);
            }
            Event::PairingApprovalOpened(code) => {
                self.accounts.open_approval(ctx, &self.servers, &code.code);
            }
            Event::PairingApproved(approval) => {
                let name = Some(approval.device_name.as_str());
                self.accounts.decide_pairing(ctx, &self.servers, &approval.code, name);
            }
            Event::PairingDenied(code) => {
                self.accounts.decide_pairing(ctx, &self.servers, &code.code, None);
            }
            Event::DisplayLanguageChanged(choice) => self.session.set_language(ctx, &choice.code),
        }
    }

    fn resolve(&mut self, ctx: &mut Ctx, pending: Pending, output: EffectOutput) {
        match pending {
            Pending::Servers(p) => {
                let boot = p == ServersPending::Load;
                if let Some(server) = self.servers.resolve(ctx, p, output) {
                    self.accounts.server_identified(ctx, &self.config, &server);
                    if matches!(self.phase, AppPhase::Welcome | AppPhase::SignIn) {
                        self.settle(ctx);
                    }
                } else {
                    let add = self.servers.view().add;
                    if add.status == LoadStatus::Failed {
                        self.accounts.connect_failed(ctx, add.problem);
                    }
                }
                if boot {
                    self.boot_read_done(ctx, 0);
                }
            }
            Pending::Accounts(p) => {
                let load = p == AccountsPending::Load;
                let boot = load || matches!(p, AccountsPending::Token { .. });
                match self.accounts.resolve(ctx, &self.servers, p, output) {
                    AccountsChange::SignedIn(id) => self.activate(ctx, &id),
                    AccountsChange::SignedOut(id) => self.signed_out(ctx, &id),
                    AccountsChange::None => {}
                }
                if boot {
                    // loading the accounts queued one token read per account
                    let queued = if load { self.accounts.count() } else { 0 };
                    self.boot_read_done(ctx, queued);
                }
            }
            Pending::Session(p) => match self.session.resolve(ctx, p, output) {
                SessionChange::Ready(user) => {
                    if let Some(id) = self.session.account_id() {
                        self.accounts.update_profile(ctx, id, &user);
                    }
                    if self.phase != AppPhase::Ready {
                        // the web's cookie session checked out
                        self.phase = AppPhase::Ready;
                        ctx.render(Surface::App);
                    }
                }
                SessionChange::Unauthorized(id) => self.signed_out(ctx, &id),
                SessionChange::None => {}
            },
        }
    }

    /// Counts down the start-up reads (`queued` more were just issued) and decides the phase
    /// once all have landed: a phone resumes its last account, a TV always asks who's watching.
    fn boot_read_done(&mut self, ctx: &mut Ctx, queued: usize) {
        if self.boot_reads == 0 {
            return;
        }
        self.boot_reads = self.boot_reads - 1 + queued;
        if self.boot_reads > 0 {
            return;
        }
        let tv = matches!(self.config.platform, Platform::Tvos | Platform::Androidtv);
        let resume = self
            .accounts
            .active()
            .map(str::to_string)
            .filter(|id| self.accounts.endpoint(&self.servers, id).is_some());
        match resume {
            Some(id) if !tv => self.activate(ctx, &id),
            _ => self.settle(ctx),
        }
    }

    /// Falls back to the phase without an active account.
    fn settle(&mut self, ctx: &mut Ctx) {
        self.phase = if self.servers.view().servers.is_empty() {
            AppPhase::Welcome
        } else if self.accounts.has_accounts() {
            AppPhase::ChooseAccount
        } else {
            AppPhase::SignIn
        };
        ctx.render(Surface::App);
    }

    fn activate(&mut self, ctx: &mut Ctx, account_id: &str) {
        let Some(endpoint) = self.accounts.endpoint(&self.servers, account_id) else {
            self.settle(ctx);
            return;
        };
        let accent = self
            .accounts
            .server_of(account_id)
            .and_then(|s| self.servers.get(s))
            .map_or_else(|| theme::default_accent().to_string(), |s| s.accent.clone());
        self.session.activate(ctx, account_id, endpoint, &accent);
        self.accounts.refresh(ctx, &self.servers, account_id);
        self.phase = AppPhase::Ready;
        ctx.render(Surface::App);
    }

    /// The web has one session: the browser's cookie for the server it was loaded from.
    fn start_web_session(&mut self, ctx: &mut Ctx) {
        let endpoint = Endpoint { base: String::new(), token: None };
        self.session.activate(ctx, WEB_ACCOUNT, endpoint, theme::default_accent());
        self.phase = AppPhase::Starting;
        ctx.render(Surface::App);
    }

    fn signed_out(&mut self, ctx: &mut Ctx, account_id: &str) {
        if self.config.auth_mode == AuthMode::Cookie {
            // the web's own login page takes over
            self.session.end(ctx);
            self.phase = AppPhase::SignIn;
            ctx.render(Surface::App);
            return;
        }
        self.accounts.token_rejected(ctx, account_id);
        if self.session.account_id() == Some(account_id) {
            self.session.end(ctx);
            self.settle(ctx);
        }
    }

    fn view(&self, surface: &Surface) -> serde_json::Value {
        let json = match surface {
            Surface::App => serde_json::to_value(AppView {
                phase: self.phase,
                active_account: self.session.account_id().map(str::to_string),
            }),
            Surface::Servers => serde_json::to_value(self.servers.view()),
            Surface::Accounts => serde_json::to_value(self.accounts.view(&self.servers)),
            Surface::SignIn => serde_json::to_value(self.accounts.sign_in_view()),
            Surface::Devices => serde_json::to_value(self.accounts.devices_view()),
            Surface::PairingApproval => serde_json::to_value(self.accounts.approval_view()),
            Surface::Session => serde_json::to_value(self.session.view()),
            Surface::Markdown(source) => serde_json::to_value(markdown::parse(source)),
        };
        json.expect("view models always serialize")
    }
}
