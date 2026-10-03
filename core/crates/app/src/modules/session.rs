//! The active account's session: who is signed in, which optional features the server has on,
//! the display language (the account's preference, seeded from the device's language on first
//! sign-in, D21) and the server's accent palette.

use couchverse_api::types::{FeatureFlags, Preferences, ServerInfo, User, UserRole};
use couchverse_api::{Call, ops};
use serde::{Deserialize, Serialize};
use typeshare::typeshare;

use crate::api::{Endpoint, Failure, decode};
use crate::core::Pending;
use crate::effects::Ctx;
use crate::messages::{EffectOutput, LoadStatus, Problem, Surface};
use crate::modules::theme::{self, AccentPalette};

/// Display languages the clients ship strings for.
pub const LANGUAGES: [&str; 2] = ["en", "cs"];

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct LanguageChoice {
    /// ISO 639-1, one of the supported display languages.
    pub code: String,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SessionView {
    pub status: LoadStatus,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub account_id: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub user: Option<SessionUser>,
    pub features: Features,
    /// The display language for UI strings and catalog text.
    pub language: String,
    pub accent: AccentPalette,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub problem: Option<Problem>,
    /// The server is out of reach (no network, or it is down): show what works offline,
    /// the downloads.
    pub offline: bool,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SessionUser {
    pub username: String,
    pub display_name: String,
    pub admin: bool,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub avatar_id: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub banner_id: Option<String>,
    /// Markdown; shells render it through the `Markdown` surface.
    pub bio: String,
    /// RFC 3339.
    pub created_at: String,
}

/// Optional server features; on until the server says otherwise, so nothing flickers away.
#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Features {
    pub couch: bool,
    pub rankings: bool,
    /// Downloads for offline viewing (native apps).
    pub downloads: bool,
}

impl Default for Features {
    fn default() -> Self {
        Self { couch: true, rankings: true, downloads: true }
    }
}

/// A session call, tagged with the account it was made for: answers that arrive after the user
/// switched accounts are dropped, so nothing leaks between accounts.
#[derive(Debug, Clone, PartialEq)]
pub struct SessionPending {
    account_id: String,
    call: SessionCall,
}

#[derive(Debug, Clone, PartialEq)]
enum SessionCall {
    Me(Call<User>),
    Features(Call<FeatureFlags>),
    Preferences(Call<Preferences>),
    Server(Call<ServerInfo>),
    LanguageSaved,
    LoggedOut,
}

/// What a resolved output means for the rest of the core.
#[derive(Debug, PartialEq)]
pub enum SessionChange {
    None,
    /// The signed-in user is known (and fresh, for the account's card).
    Ready(User),
    /// The server said which optional features are on.
    Features,
    /// The server rejected the active account's credentials.
    Unauthorized(String),
}

pub struct Session {
    view: SessionView,
    endpoint: Option<Endpoint>,
    /// The device's language, for seeding a new account's preference.
    locale: String,
}

impl Session {
    pub fn new(locale: &str) -> Self {
        Self {
            view: SessionView {
                status: LoadStatus::Idle,
                account_id: None,
                user: None,
                features: Features::default(),
                language: supported_language(locale),
                accent: theme::palette(theme::default_accent()),
                problem: None,
                offline: false,
            },
            endpoint: None,
            locale: locale.to_string(),
        }
    }

    pub fn view(&self) -> SessionView {
        self.view.clone()
    }

    pub fn account_id(&self) -> Option<&str> {
        self.view.account_id.as_deref()
    }

    pub fn username(&self) -> Option<&str> {
        self.view.user.as_ref().map(|u| u.username.as_str())
    }

    /// Whether the server has couch sessions on (assumed until it says otherwise).
    pub fn couch(&self) -> bool {
        self.view.features.couch
    }

    /// Whether the server has rankings on (assumed until it says otherwise).
    pub fn downloads(&self) -> bool {
        self.view.features.downloads
    }

    pub fn rankings(&self) -> bool {
        self.view.features.rankings
    }

    /// The signed-in user changed their profile.
    pub fn user_updated(&mut self, ctx: &mut Ctx, user: &User) {
        if let Some(current) = self.view.user.as_mut() {
            current.display_name.clone_from(&user.display_name);
            current.avatar_id.clone_from(&user.avatar_id);
            current.banner_id.clone_from(&user.banner_id);
            current.bio.clone_from(&user.bio);
            ctx.render(Surface::Session);
        }
    }

    pub fn language(&self) -> &str {
        &self.view.language
    }

    /// Where the active session's API calls go; `None` without one.
    pub fn endpoint(&self) -> Option<&Endpoint> {
        self.endpoint.as_ref()
    }

    /// The accent colour showing now, as `#rrggbb`.
    pub fn accent(&self) -> &str {
        &self.view.accent.accent
    }

    /// Makes `account_id` the session and loads it. The previous account's state is dropped,
    /// so nothing leaks between accounts.
    pub fn activate(&mut self, ctx: &mut Ctx, account_id: &str, endpoint: Endpoint, accent: &str) {
        // the first account of a launch keeps the language already showing; a switch starts from
        // the device's until the new account's preference arrives
        let language = if self.view.account_id.is_none() {
            self.view.language.clone()
        } else {
            supported_language(&self.locale)
        };
        self.view = SessionView {
            status: LoadStatus::Loading,
            account_id: Some(account_id.to_string()),
            user: None,
            features: Features::default(),
            language,
            accent: theme::palette(accent),
            problem: None,
            offline: false,
        };
        self.endpoint = Some(endpoint);
        self.load(ctx);
        ctx.render(Surface::Session);
    }

    /// Re-reads the session, keeping what is showing until the answers arrive.
    pub fn refresh(&mut self, ctx: &mut Ctx) {
        if self.view.status == LoadStatus::Loaded {
            self.view.status = LoadStatus::Stale;
            ctx.render(Surface::Session);
        }
        self.load(ctx);
    }

    fn load(&mut self, ctx: &mut Ctx) {
        self.call(ctx, ops::get_me(), SessionCall::Me);
        self.call(ctx, ops::get_features(), SessionCall::Features);
        self.call(ctx, ops::get_preferences(), SessionCall::Preferences);
        self.call(ctx, ops::get_server(), SessionCall::Server);
    }

    fn call<T>(&self, ctx: &mut Ctx, call: Call<T>, tag: impl FnOnce(Call<T>) -> SessionCall) {
        let (Some(endpoint), Some(account_id)) = (&self.endpoint, &self.view.account_id) else {
            return;
        };
        let request = endpoint.request(&call.request);
        let pending = SessionPending { account_id: account_id.clone(), call: tag(call) };
        ctx.http(request, Pending::Session(pending));
    }

    pub fn end(&mut self, ctx: &mut Ctx) {
        self.endpoint = None;
        self.view.status = LoadStatus::Idle;
        self.view.account_id = None;
        self.view.user = None;
        ctx.render(Surface::Session);
    }

    /// Signs the session out on the server too (the web's cookie session), then ends it.
    pub fn log_out(&mut self, ctx: &mut Ctx) {
        self.call(ctx, ops::logout(), |_| SessionCall::LoggedOut);
        self.end(ctx);
    }

    /// Switches the display language and saves it as the account's preference.
    pub fn set_language(&mut self, ctx: &mut Ctx, code: &str) {
        if !LANGUAGES.contains(&code) || code == self.view.language {
            return;
        }
        self.view.language = code.to_string();
        self.save_language(ctx);
        ctx.render(Surface::Session);
    }

    fn save_language(&self, ctx: &mut Ctx) {
        let body = Preferences {
            language: Some(self.view.language.clone()),
            public_profile: None,
            subtitles: None,
        };
        self.call(ctx, ops::update_preferences(&body), |_| SessionCall::LanguageSaved);
    }

    pub fn resolve(
        &mut self,
        ctx: &mut Ctx,
        pending: SessionPending,
        output: EffectOutput,
    ) -> SessionChange {
        let current = self.view.account_id.as_deref();
        let stale = match pending.call {
            // the server's colours outlive a sign-out, so the sign-in screen keeps them
            SessionCall::Server(_) => current.is_some_and(|id| id != pending.account_id),
            _ => current != Some(pending.account_id.as_str()),
        };
        if stale {
            return SessionChange::None;
        }
        let change = match pending.call {
            SessionCall::Me(call) => match decode(&call, output) {
                Ok(user) => {
                    self.view.user = Some(SessionUser {
                        username: user.username.clone(),
                        display_name: user.display_name.clone(),
                        admin: user.role == UserRole::Admin,
                        avatar_id: user.avatar_id.clone(),
                        banner_id: user.banner_id.clone(),
                        bio: user.bio.clone(),
                        created_at: user.created_at.clone(),
                    });
                    self.view.status = LoadStatus::Loaded;
                    self.view.problem = None;
                    self.view.offline = false;
                    SessionChange::Ready(user)
                }
                Err(failure) if failure.unauthorized() => {
                    return SessionChange::Unauthorized(pending.account_id);
                }
                Err(failure) => {
                    self.failed(&failure);
                    SessionChange::None
                }
            },
            SessionCall::Features(call) => {
                if let Ok(flags) = decode(&call, output) {
                    self.view.features = Features {
                        couch: flags.couch_enabled,
                        rankings: flags.rankings_enabled,
                        downloads: flags.downloads_enabled,
                    };
                    SessionChange::Features
                } else {
                    SessionChange::None
                }
            }
            SessionCall::Preferences(call) => {
                if let Ok(prefs) = decode(&call, output) {
                    match prefs.language.filter(|l| LANGUAGES.contains(&l.as_str())) {
                        Some(language) => self.view.language = language,
                        // first sign-in on any device: the account adopts this device's language
                        None => self.save_language(ctx),
                    }
                }
                SessionChange::None
            }
            SessionCall::Server(call) => {
                if let Ok(info) = decode(&call, output) {
                    self.view.accent = theme::palette(&info.accent);
                }
                SessionChange::None
            }
            SessionCall::LanguageSaved | SessionCall::LoggedOut => return SessionChange::None,
        };
        ctx.render(Surface::Session);
        change
    }

    fn failed(&mut self, failure: &Failure) {
        self.view.status =
            if self.view.user.is_some() { LoadStatus::Stale } else { LoadStatus::Failed };
        self.view.problem = Some(failure.problem());
        self.view.offline = matches!(failure, Failure::Network(_))
            || matches!(failure, Failure::Api(e) if e.status >= 500);
    }

    /// Whether the server was out of reach the last time the session asked.
    pub fn offline(&self) -> bool {
        self.view.offline
    }
}

/// The display language for a BCP 47 locale (`cs-CZ` -> `cs`), English when unsupported.
pub fn supported_language(locale: &str) -> String {
    let primary = locale.split(['-', '_']).next().unwrap_or_default().to_ascii_lowercase();
    if LANGUAGES.contains(&primary.as_str()) { primary } else { "en".to_string() }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn locales_map_to_supported_languages() {
        assert_eq!(supported_language("cs-CZ"), "cs");
        assert_eq!(supported_language("CS"), "cs");
        assert_eq!(supported_language("en_GB"), "en");
        assert_eq!(supported_language("de-DE"), "en");
        assert_eq!(supported_language(""), "en");
    }
}
