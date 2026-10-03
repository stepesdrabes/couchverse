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
use crate::modules::catalog::{self, Catalog, CatalogChange, CatalogPending, Env};
use crate::modules::couch::{self, Couch, CouchChange, CouchPending};
use crate::modules::downloads::{self, Downloads, DownloadsChange, DownloadsPending};
use crate::modules::images::Images;
use crate::modules::markdown;
use crate::modules::notices::Notices;
use crate::modules::playback::{self, Playback, PlaybackChange, PlaybackPending};
use crate::modules::profile::{Profile, ProfileChange, ProfilePending};
use crate::modules::ranks::{self, Ranks, RanksChange, RanksPending};
use crate::modules::servers::{Servers, ServersPending};
use crate::modules::session::{Session, SessionChange, SessionPending};
use crate::modules::theme;

/// What a module is waiting for from an effect it issued.
#[derive(Debug, Clone, PartialEq)]
pub enum Pending {
    Servers(ServersPending),
    Accounts(AccountsPending),
    Session(SessionPending),
    Catalog(CatalogPending),
    Ranks(RanksPending),
    Profile(ProfilePending),
    Playback(PlaybackPending),
    Couch(CouchPending),
    Downloads(DownloadsPending),
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
    catalog: Catalog,
    ranks: Ranks,
    profile: Profile,
    playback: Playback,
    couch: Couch,
    downloads: Downloads,
    notices: Notices,
    phase: AppPhase,
    /// Persisted reads still outstanding at start-up; the phase is decided when they land.
    boot_reads: usize,
    /// The web's requests without a signed-in session: a couch guest rides on the couch
    /// cookie alone. `None` for native clients, which need an account for everything.
    guest: Option<Endpoint>,
}

impl Core {
    pub fn new(config: CoreConfig) -> Self {
        let session = Session::new(&config.locale);
        let guest = (config.auth_mode == AuthMode::Cookie)
            .then(|| Endpoint { base: String::new(), token: None });
        let model = Model {
            config,
            servers: Servers::default(),
            accounts: Accounts::default(),
            session,
            catalog: Catalog::default(),
            ranks: Ranks::default(),
            profile: Profile::default(),
            playback: Playback::default(),
            couch: Couch::default(),
            downloads: Downloads::default(),
            notices: Notices::default(),
            phase: AppPhase::Starting,
            boot_reads: 0,
            guest,
        };
        Self { registry: Registry::default(), model }
    }

    pub fn send(&mut self, message: Message) -> Vec<EffectRequest> {
        let mut ctx = Ctx::new(message.now_ms, &mut self.registry);
        ctx.wall = message.wall_ms;
        self.model.send(&mut ctx, message.event);
        self.registry.drain()
    }

    pub fn resolve(&mut self, resolution: Resolution) -> Vec<EffectRequest> {
        // a cancelled timer's late tick, a fire-and-forget write: nothing is waiting
        if let Some(pending) = self.registry.take(resolution.id) {
            let terminal = resolution.output.is_terminal();
            let mut ctx = Ctx::new(resolution.now_ms, &mut self.registry);
            self.model.resolve(&mut ctx, pending, resolution.output);
            if terminal {
                self.registry.forget(resolution.id);
            }
        }
        self.registry.drain()
    }

    /// The surface's view model as JSON.
    pub fn view(&self, surface: &Surface) -> String {
        self.model.view(surface)
    }

    #[cfg(test)]
    pub fn pending_effects(&self) -> usize {
        self.registry.pending_count()
    }
}

/// The active session's request context, built from the session field alone so the catalog can
/// be borrowed mutably next to it.
fn env(session: &Session) -> Option<Env<'_>> {
    session.endpoint().map(|endpoint| Env { endpoint, language: session.language() })
}

fn playback_env<'a>(
    session: &'a Session,
    guest: Option<&'a Endpoint>,
    images: Images,
) -> Option<playback::Env<'a>> {
    session.endpoint().or(guest).map(|endpoint| playback::Env {
        endpoint,
        language: session.language(),
        images,
    })
}

fn couch_env<'a>(
    session: &'a Session,
    guest: Option<&'a Endpoint>,
    config: &'a CoreConfig,
    playback: &'a Playback,
) -> Option<couch::Env<'a>> {
    session.endpoint().or(guest).map(|endpoint| couch::Env {
        endpoint,
        language: session.language(),
        cookie: config.auth_mode == AuthMode::Cookie,
        origin: &config.origin,
        profile: playback.profile(),
    })
}

fn downloads_env<'a>(
    session: &'a Session,
    playback: &'a Playback,
    images: Images,
) -> Option<downloads::Env<'a>> {
    session.endpoint().map(|endpoint| downloads::Env {
        endpoint,
        language: session.language(),
        profile: playback.profile(),
        images,
    })
}

/// Where achievement checks go: only once the server has said rankings are on, and only for a
/// signed-in viewer, since the web's cookie session can learn its flags while nobody is.
fn achievements_endpoint(session: &Session) -> Option<&Endpoint> {
    session.endpoint().filter(|_| session.rankings_confirmed() && session.username().is_some())
}

fn ranks_env(session: &Session) -> Option<ranks::Env<'_>> {
    session.endpoint().map(|endpoint| ranks::Env {
        endpoint,
        language: session.language(),
        username: session.username(),
    })
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
                // not a boot read: nothing waits for it
                self.playback.load_prefs(ctx);
            }
            Event::AppBecameActive => {
                if self.phase == AppPhase::Ready {
                    self.session.refresh(ctx);
                    if let Some(env) = env(&self.session) {
                        self.catalog.refresh_open(ctx, &env);
                    }
                    self.check_achievements(ctx, false);
                }
                // timers do not run while an app is suspended
                self.accounts.poll_pairing_now(ctx, &self.servers);
            }
            Event::SessionStarted => {
                if self.config.auth_mode == AuthMode::Cookie {
                    self.start_web_session(ctx);
                }
            }
            Event::SessionChanged => {
                if self.phase == AppPhase::Ready {
                    self.session.refresh(ctx);
                }
            }
            onboarding @ (Event::ServerAddressSubmitted(_)
            | Event::ServerRemoved(_)
            | Event::PasswordSignInSubmitted(_)
            | Event::PairingStarted(_)
            | Event::PairingCancelled
            | Event::LinkOpened(_)
            | Event::AccountSelected(_)) => self.onboard(ctx, onboarding),
            account @ (Event::SignOutRequested(_)
            | Event::DevicesOpened
            | Event::DeviceRevoked(_)
            | Event::PairingApprovalOpened(_)
            | Event::PairingApproved(_)
            | Event::PairingDenied(_)) => self.manage_account(ctx, account),
            Event::DisplayLanguageChanged(choice) => {
                let before = self.session.language().to_string();
                self.session.set_language(ctx, &choice.code);
                self.language_settled(ctx, &before);
            }
            Event::NoticeDismissed(notice) => self.notices.dismiss(ctx, notice.id),
            Event::AchievementsCheckRequested(request) => {
                self.check_achievements(ctx, request.force);
            }
            Event::CelebrationDismissed => self.ranks.celebrated(ctx),
            progress @ (Event::ProfileVisibilityChanged(_)
            | Event::ProfileEditSubmitted(_)
            | Event::PasswordChangeSubmitted(_)
            | Event::ImageChosen(_)
            | Event::ImageRemoved(_)) => self.edit_profile(ctx, progress),
            watching @ (Event::PlayRequested(_)
            | Event::PlayerReported(_)
            | Event::PlayerClosed
            | Event::QualityChosen(_)
            | Event::AudioChosen(_)
            | Event::SubtitlesChosen(_)
            | Event::NextEpisodeRequested
            | Event::NextEpisodeCancelled
            | Event::ShuffleToggled
            | Event::CapabilitiesReported(_)) => self.watch(ctx, watching),
            party @ (Event::CouchStartRequested
            | Event::CouchJoinRequested(_)
            | Event::CouchRemoteRequested(_)
            | Event::CouchLeft
            | Event::CouchEndRequested
            | Event::CouchEmojiSent(_)
            | Event::CouchLocalPauseChanged(_)
            | Event::CouchRemoteCommanded(_)) => self.couch_event(ctx, party),
            offline @ (Event::DownloadRequested(_)
            | Event::DownloadRetried(_)
            | Event::DownloadRemoved(_)
            | Event::DownloadPlayRequested(_)) => self.download_event(ctx, offline),
            browsing @ (Event::ScreenOpened(_)
            | Event::ScreenClosed(_)
            | Event::RefreshRequested(_)
            | Event::BrowseMoreRequested(_)
            | Event::SearchChanged(_)
            | Event::WatchlistChanged(_)) => self.browse(ctx, browsing),
        }
    }

    /// Where the signed-in account's own calls go: the web's cookie session, or the active
    /// account's server with its token.
    fn account_endpoint(&self) -> Option<Endpoint> {
        if self.config.auth_mode == AuthMode::Cookie {
            return self.session.endpoint().cloned();
        }
        self.accounts.active().and_then(|a| self.accounts.endpoint(&self.servers, a))
    }

    /// The signed-in account: signing out, its devices, approving another device.
    fn manage_account(&mut self, ctx: &mut Ctx, event: Event) {
        match event {
            Event::SignOutRequested(_) if self.config.auth_mode == AuthMode::Cookie => {
                self.session.log_out(ctx);
                self.catalog.reset(ctx);
                Catalog::forget(ctx, WEB_ACCOUNT);
                self.phase = AppPhase::SignIn;
                ctx.render(Surface::App);
            }
            Event::SignOutRequested(account) => {
                // the account's downloads leave the device with it
                self.downloads.purge(ctx, &account.account_id);
                if self.session.account_id() == Some(account.account_id.as_str()) {
                    self.end_session(ctx);
                }
                self.accounts.sign_out(ctx, &self.servers, &account.account_id);
                Catalog::forget(ctx, &account.account_id);
                if self.session.account_id().is_none() {
                    self.settle(ctx);
                }
            }
            Event::DevicesOpened => {
                if let Some(endpoint) = self.account_endpoint() {
                    self.accounts.open_devices(ctx, &endpoint);
                }
            }
            Event::DeviceRevoked(device) => {
                if let Some(endpoint) = self.account_endpoint() {
                    self.accounts.revoke_device(ctx, &endpoint, &device.device_id);
                }
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
            _ => {}
        }
    }

    /// Servers and accounts: adding a server, signing in, links, switching accounts.
    fn onboard(&mut self, ctx: &mut Ctx, event: Event) {
        match event {
            Event::ServerAddressSubmitted(address) => {
                self.servers.submit_address(ctx, &address.address);
            }
            Event::ServerRemoved(server) => {
                let active_server =
                    self.session.account_id().and_then(|a| self.accounts.server_of(a));
                if active_server == Some(server.server_id.as_str()) {
                    self.end_session(ctx);
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
            Event::AccountSelected(account) if self.accounts.select(ctx, &account.account_id) => {
                self.activate(ctx, &account.account_id);
            }
            _ => {}
        }
    }

    /// The catalog's events: screens opening and closing, listings, search, My List.
    fn browse(&mut self, ctx: &mut Ctx, event: Event) {
        if let Event::ScreenOpened(surface) | Event::RefreshRequested(surface) = &event
            && ranks::owns(surface)
        {
            let force = matches!(event, Event::RefreshRequested(_));
            if let Some(env) = ranks_env(&self.session) {
                self.ranks.open(ctx, &env, surface, force);
            }
            return;
        }
        if let Event::ScreenOpened(Surface::Downloads)
        | Event::RefreshRequested(Surface::Downloads) = &event
        {
            let env = downloads_env(&self.session, &self.playback, self.images());
            self.downloads.opened(ctx, env.as_ref());
            return;
        }
        let env = env(&self.session);
        match (event, env) {
            (Event::ScreenOpened(surface), env) if catalog::owns(&surface) => {
                self.catalog.opened(ctx, env.as_ref(), &surface);
            }
            (Event::ScreenClosed(surface), _) => self.catalog.closed(&surface),
            (Event::SearchChanged(text), _) => self.catalog.search_changed(ctx, &text.query),
            (Event::RefreshRequested(surface), Some(env)) => {
                self.catalog.refresh(ctx, &env, &surface);
            }
            (Event::BrowseMoreRequested(key), Some(env)) => {
                self.catalog.browse_more(ctx, &env, &key);
            }
            (Event::WatchlistChanged(change), Some(env)) => {
                self.catalog.set_listed(ctx, &env, &change.title_id, change.listed);
            }
            _ => {}
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
                    AccountsChange::SessionRejected => self.session_rejected(ctx),
                    AccountsChange::None => {}
                }
                if boot {
                    // loading the accounts queued one token read per account
                    let queued = if load { self.accounts.count() } else { 0 };
                    self.boot_read_done(ctx, queued);
                }
            }
            Pending::Session(p) => {
                let before = self.session.language().to_string();
                match self.session.resolve(ctx, p, output) {
                    SessionChange::Ready(user) => {
                        if let Some(id) = self.session.account_id() {
                            self.accounts.update_profile(ctx, id, &user);
                        }
                        // the server answers again: offline progress and downloads catch up
                        let images = self.images();
                        if let Some(env) = downloads_env(&self.session, &self.playback, images) {
                            self.downloads.reconnected(ctx, &env);
                        }
                        if self.phase != AppPhase::Ready {
                            // the web's cookie session checked out
                            self.phase = AppPhase::Ready;
                            ctx.render(Surface::App);
                        }
                        // the first check waits for both the user and the flags, whichever
                        // answers last
                        self.check_achievements(ctx, false);
                    }
                    SessionChange::Features => self.check_achievements(ctx, false),
                    SessionChange::Unauthorized(id) => self.signed_out(ctx, &id),
                    SessionChange::None => {}
                }
                // the account's saved language can differ from the one showing
                self.language_settled(ctx, &before);
            }
            Pending::Catalog(p) => {
                let env = env(&self.session);
                match self.catalog.resolve(ctx, env.as_ref(), p, output) {
                    CatalogChange::Unauthorized => self.session_rejected(ctx),
                    CatalogChange::WatchlistFailed => self.notices.push(ctx, "watchlist_failed"),
                    CatalogChange::None => {}
                }
            }
            Pending::Ranks(p) => {
                let change = self.ranks.resolve(ctx, p, output);
                self.ranks_changed(ctx, change);
            }
            Pending::Playback(p) => {
                let images = self.images();
                let env = playback_env(&self.session, self.guest.as_ref(), images);
                let change = self.playback.resolve(ctx, env.as_ref(), p, output);
                self.playback_changed(ctx, change);
            }
            Pending::Couch(p) => {
                let env =
                    couch_env(&self.session, self.guest.as_ref(), &self.config, &self.playback);
                let change = self.couch.resolve(ctx, env.as_ref(), &self.playback, p, output);
                self.couch_changed(ctx, change);
            }
            Pending::Downloads(p) => {
                let images = self.images();
                let env = downloads_env(&self.session, &self.playback, images);
                let change = self.downloads.resolve(ctx, env.as_ref(), p, output);
                self.downloads_changed(ctx, change);
            }
            Pending::Profile(p) => {
                let change = self.profile.resolve(ctx, p, output);
                self.profile_changed(ctx, change);
            }
        }
    }

    fn profile_changed(&mut self, ctx: &mut Ctx, change: ProfileChange) {
        match change {
            ProfileChange::Updated(user) => {
                self.session.user_updated(ctx, &user);
                if let Some(id) = self.session.account_id() {
                    self.accounts.update_profile(ctx, id, &user);
                }
                if let Some(env) = ranks_env(&self.session) {
                    let me = Surface::Profile(user.username.clone());
                    self.ranks.open(ctx, &env, &me, true);
                }
                // a first avatar is an achievement
                self.check_achievements(ctx, true);
            }
            ProfileChange::Unauthorized => self.session_rejected(ctx),
            ProfileChange::None => {}
        }
    }

    /// The viewer's own profile: visibility, details, password and images.
    fn edit_profile(&mut self, ctx: &mut Ctx, event: Event) {
        let Some(endpoint) = self.session.endpoint() else { return };
        match event {
            Event::ProfileVisibilityChanged(choice) => {
                self.ranks.set_public(ctx, endpoint, choice.public);
            }
            Event::ProfileEditSubmitted(edit) => self.profile.save_details(ctx, endpoint, &edit),
            Event::PasswordChangeSubmitted(form) => {
                self.profile.change_password(ctx, endpoint, &form);
            }
            Event::ImageChosen(choice) => self.profile.upload(ctx, endpoint, &choice),
            Event::ImageRemoved(slot) => self.profile.remove(ctx, endpoint, slot.slot),
            _ => {}
        }
    }

    /// The player: what plays, what the shell's player reports, tracks and what comes next.
    fn watch(&mut self, ctx: &mut Ctx, event: Event) {
        let reported = matches!(event, Event::PlayerReported(_));
        let switched = matches!(event, Event::PlayRequested(_) | Event::PlayerClosed);
        let images = self.images();
        let Some(env) = playback_env(&self.session, self.guest.as_ref(), images) else {
            if let Event::CapabilitiesReported(profile) = event {
                self.playback.set_profile(&profile);
            }
            return;
        };
        let change = match event {
            Event::PlayRequested(target) => {
                // out of reach of the server, a downloaded title plays from the device
                match self.downloads.local_for(&target).filter(|_| self.session.offline()) {
                    Some(local) => self.playback.play_download(ctx, &env, &local),
                    None => self.playback.play(ctx, &env, target, false),
                }
                PlaybackChange::None
            }
            Event::PlayerReported(report) => {
                let change = self.playback.report(ctx, &env, &report);
                if matches!(change, PlaybackChange::Watched { .. })
                    && let Some((target, position)) = self.playback.local_position()
                {
                    let target = target.clone();
                    self.downloads.remember(ctx, &target, position);
                }
                change
            }
            Event::PlayerClosed => {
                self.playback.close(ctx, env.endpoint);
                PlaybackChange::None
            }
            Event::QualityChosen(choice) => {
                self.playback.choose_quality(ctx, &choice.key);
                PlaybackChange::None
            }
            Event::AudioChosen(choice) => {
                self.playback.choose_audio(ctx, choice.id.as_deref());
                PlaybackChange::None
            }
            Event::SubtitlesChosen(choice) => {
                self.playback.choose_subtitles(ctx, choice.id.as_deref());
                PlaybackChange::None
            }
            Event::NextEpisodeRequested => {
                self.playback.next_now(ctx, &env);
                PlaybackChange::None
            }
            Event::NextEpisodeCancelled => {
                self.playback.cancel_next(ctx);
                PlaybackChange::None
            }
            Event::ShuffleToggled => {
                self.playback.toggle_shuffle(ctx);
                PlaybackChange::None
            }
            Event::CapabilitiesReported(profile) => {
                self.playback.set_profile(&profile);
                PlaybackChange::None
            }
            _ => PlaybackChange::None,
        };
        self.playback_changed(ctx, change);
        // a host's play, pause, seek or switch reaches the followers at once
        if self.couch.is_host() && ((reported && self.playback.moved()) || switched) {
            self.couch.host_moved(ctx, &self.playback);
        }
    }

    /// Couch sessions: hosting, joining, leaving, reactions and remote control.
    fn couch_event(&mut self, ctx: &mut Ctx, event: Event) {
        let Some(env) = couch_env(&self.session, self.guest.as_ref(), &self.config, &self.playback)
        else {
            return;
        };
        let change = match event {
            Event::CouchStartRequested => {
                match self.playback.target().cloned() {
                    Some(target) if self.session.couch() => self.couch.start(ctx, &env, &target),
                    Some(_) => self.notices.push(ctx, "couch_disabled"),
                    None => self.notices.push(ctx, "couch_nothing_playing"),
                }
                CouchChange::None
            }
            // a guest without an account has no flags to go by: the server refuses for it
            Event::CouchJoinRequested(_) | Event::CouchRemoteRequested(_)
                if self.session.account_id().is_some() && !self.session.couch() =>
            {
                self.notices.push(ctx, "couch_disabled");
                CouchChange::None
            }
            Event::CouchJoinRequested(code) => {
                self.couch.join(ctx, &env, &code.code, false);
                CouchChange::None
            }
            Event::CouchRemoteRequested(code) => {
                self.couch.join(ctx, &env, &code.code, true);
                CouchChange::None
            }
            Event::CouchLeft => self.couch.leave(ctx, &env, false),
            Event::CouchEndRequested => self.couch.leave(ctx, &env, true),
            Event::CouchEmojiSent(reaction) => {
                self.couch.send_emoji(ctx, &reaction.emoji);
                CouchChange::None
            }
            Event::CouchLocalPauseChanged(pause) => {
                self.couch.local_pause(ctx, &self.playback, pause.paused);
                CouchChange::None
            }
            Event::CouchRemoteCommanded(control) => {
                self.couch.remote(ctx, control);
                CouchChange::None
            }
            _ => CouchChange::None,
        };
        self.couch_changed(ctx, change);
    }

    fn couch_changed(&mut self, ctx: &mut Ctx, change: CouchChange) {
        let images = self.images();
        let Some(env) = playback_env(&self.session, self.guest.as_ref(), images) else { return };
        match change {
            CouchChange::Follow(target, info) => self.playback.follow(ctx, &env, target, *info),
            CouchChange::StopFollowing => self.playback.close(ctx, env.endpoint),
            CouchChange::Next => self.playback.next_now(ctx, &env),
            CouchChange::Previous => self.playback.previous(ctx, &env),
            CouchChange::Unauthorized => self.session_rejected(ctx),
            CouchChange::None => {}
        }
    }

    fn playback_changed(&mut self, ctx: &mut Ctx, change: PlaybackChange) {
        match change {
            // progress is saved every few seconds: the check throttles itself, and the end of
            // a title is the likeliest moment for a new badge
            PlaybackChange::Watched { finished } => self.check_achievements(ctx, finished),
            PlaybackChange::Unauthorized => self.session_rejected(ctx),
            PlaybackChange::Unsaved(report) => self.downloads.keep(ctx, *report),
            PlaybackChange::Refollow => {
                let env =
                    couch_env(&self.session, self.guest.as_ref(), &self.config, &self.playback);
                if let Some(env) = env {
                    self.couch.refetch_player(ctx, &env);
                }
            }
            PlaybackChange::None => {}
        }
    }

    /// Downloads: asking for one, trying again, deleting and playing one from the device.
    fn download_event(&mut self, ctx: &mut Ctx, event: Event) {
        let images = self.images();
        if let Event::DownloadPlayRequested(download) = &event {
            let local = self.downloads.local(&download.id);
            if let (Some(local), Some(env)) =
                (local, playback_env(&self.session, self.guest.as_ref(), images))
            {
                self.playback.play_download(ctx, &env, &local);
            }
            return;
        }
        if let Event::DownloadRemoved(download) = &event {
            self.downloads.remove(ctx, self.session.endpoint(), &download.id);
            return;
        }
        let Some(env) = downloads_env(&self.session, &self.playback, images) else { return };
        let change = match event {
            Event::DownloadRequested(_) | Event::DownloadRetried(_)
                if !self.session.downloads() =>
            {
                DownloadsChange::Notice("downloads_disabled")
            }
            Event::DownloadRequested(ask) => self.downloads.ask(ctx, &env, ask),
            Event::DownloadRetried(download) => self.downloads.retry(ctx, &env, &download.id),
            _ => DownloadsChange::None,
        };
        self.downloads_changed(ctx, change);
    }

    fn ranks_changed(&mut self, ctx: &mut Ctx, change: RanksChange) {
        match change {
            RanksChange::Unauthorized => self.session_rejected(ctx),
            RanksChange::VisibilityFailed => self.notices.push(ctx, "visibility_failed"),
            RanksChange::CheckAgain => {
                if let Some(endpoint) = achievements_endpoint(&self.session) {
                    self.ranks.retry(ctx, endpoint);
                }
            }
            RanksChange::None => {}
        }
    }

    fn downloads_changed(&mut self, ctx: &mut Ctx, change: DownloadsChange) {
        match change {
            DownloadsChange::Notice(code) => self.notices.push(ctx, code),
            DownloadsChange::Unauthorized => self.session_rejected(ctx),
            DownloadsChange::None => {}
        }
    }

    /// Asks whether anything new was earned.
    fn check_achievements(&mut self, ctx: &mut Ctx, force: bool) {
        if let Some(endpoint) = achievements_endpoint(&self.session) {
            self.ranks.check(ctx, endpoint, force);
        }
    }

    /// Reloads what is showing when the display language differs from `before`.
    fn language_settled(&mut self, ctx: &mut Ctx, before: &str) {
        if self.session.language() != before
            && let Some(env) = env(&self.session)
        {
            self.catalog.language_changed(ctx, &env);
            self.ranks.language_changed();
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
        self.start_catalog(ctx, account_id);
        self.downloads.activate(ctx, account_id);
        self.phase = AppPhase::Ready;
        ctx.render(Surface::App);
    }

    /// The web has one session: the browser's cookie for the server it was loaded from.
    fn start_web_session(&mut self, ctx: &mut Ctx) {
        let endpoint = Endpoint { base: String::new(), token: None };
        // the accent showing stays up while the session loads again, so signing in never
        // flashes the default colour
        let accent = self.session.accent().to_string();
        self.session.activate(ctx, WEB_ACCOUNT, endpoint, &accent);
        self.start_catalog(ctx, WEB_ACCOUNT);
        self.phase = AppPhase::Starting;
        ctx.render(Surface::App);
    }

    /// A new account's catalog, loading whatever its screens already have open.
    fn start_catalog(&mut self, ctx: &mut Ctx, account_id: &str) {
        self.accounts.forget_devices(ctx);
        self.ranks.reset(ctx);
        self.profile.reset(ctx);
        self.catalog.activate(ctx, account_id);
        if let Some(env) = env(&self.session) {
            self.catalog.refresh_open(ctx, &env);
        }
    }

    fn end_session(&mut self, ctx: &mut Ctx) {
        self.session.end(ctx);
        self.accounts.forget_devices(ctx);
        self.catalog.reset(ctx);
        self.ranks.reset(ctx);
        self.profile.reset(ctx);
        self.playback.reset(ctx);
        self.couch.reset(ctx);
        self.downloads.reset(ctx);
    }

    /// A module's request came back 401: the active session is no longer valid.
    fn session_rejected(&mut self, ctx: &mut Ctx) {
        if let Some(id) = self.session.account_id().map(str::to_string) {
            self.signed_out(ctx, &id);
        }
    }

    fn signed_out(&mut self, ctx: &mut Ctx, account_id: &str) {
        if self.config.auth_mode == AuthMode::Cookie {
            // the web's own login page takes over
            self.end_session(ctx);
            Catalog::forget(ctx, account_id);
            self.phase = AppPhase::SignIn;
            ctx.render(Surface::App);
            return;
        }
        self.accounts.token_rejected(ctx, account_id);
        if self.session.account_id() == Some(account_id) {
            self.end_session(ctx);
            self.settle(ctx);
        }
    }

    /// Where artwork comes from for the active account: its server, with its grant.
    fn images(&self) -> Images {
        let Some(account) = self.session.account_id() else {
            // a couch guest without an account loads artwork with the session's grant
            let grant = self.couch.artwork_grant().map(str::to_string);
            return Images { base: String::new(), grant };
        };
        let server = self.accounts.server_of(account).and_then(|s| self.servers.get(s));
        Images {
            base: server.map(|s| s.url.clone()).unwrap_or_default(),
            grant: self.accounts.artwork_grant(account).map(str::to_string),
        }
    }

    fn view(&self, surface: &Surface) -> String {
        let images = self.images();
        let catalog = &self.catalog;
        let json = match surface {
            Surface::App => serde_json::to_string(&AppView {
                phase: self.phase,
                active_account: self.session.account_id().map(str::to_string),
            }),
            Surface::Servers => serde_json::to_string(&self.servers.view()),
            Surface::Accounts => serde_json::to_string(&self.accounts.view(&self.servers)),
            Surface::SignIn => serde_json::to_string(&self.accounts.sign_in_view()),
            Surface::Devices => serde_json::to_string(&self.accounts.devices_view()),
            Surface::PairingApproval => serde_json::to_string(&self.accounts.approval_view()),
            Surface::Session => serde_json::to_string(&self.session.view()),
            Surface::Markdown(source) => serde_json::to_string(&markdown::parse(source)),
            Surface::Home => serde_json::to_string(&catalog.home_view(&images)),
            Surface::Browse(key) => serde_json::to_string(&catalog.browse_view(key, &images)),
            Surface::Title(slug) => serde_json::to_string(&catalog.title_view(slug, &images)),
            Surface::Genres => serde_json::to_string(&catalog.genres_view()),
            Surface::MyList => serde_json::to_string(&catalog.my_list_view(&images)),
            Surface::Search => serde_json::to_string(&catalog.search_view(&images)),
            Surface::Notices => serde_json::to_string(&self.notices.view()),
            Surface::Rank => serde_json::to_string(&self.ranks.rank_view()),
            Surface::Profile(username) => {
                serde_json::to_string(&self.ranks.profile_view(username, &images))
            }
            Surface::Leaderboard(key) => {
                serde_json::to_string(&self.ranks.leaderboard_view(*key, &images))
            }
            Surface::ProfileEditor => serde_json::to_string(&self.profile.view()),
            Surface::Player => serde_json::to_string(&self.playback.view(&images)),
            Surface::Couch => {
                serde_json::to_string(&self.couch.view(&images, &self.config.origin, &images.base))
            }
            Surface::Downloads => serde_json::to_string(&self.downloads.view(&images)),
        };
        json.expect("view models always serialize")
    }
}
