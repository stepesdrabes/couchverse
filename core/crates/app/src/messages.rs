//! The bridge vocabulary: every message a shell sends to the core or receives from it. All of it
//! is plain data, generated for Swift, Kotlin and TypeScript by typeshare, so the three shells
//! decode it identically. Enums carrying data are adjacently tagged (`{"type", "content"}`);
//! data-less enums are plain strings.

use serde::{Deserialize, Serialize};
use typeshare::typeshare;

use crate::modules::{accounts, catalog, notices, profile, ranks, servers, session};

/// Effect ids and monotonic milliseconds: typeshare maps the name to a 53-bit-safe integer in
/// every language.
pub type U53 = u64;

/// How the shell authenticates API requests.
#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum AuthMode {
    /// Native clients: the core attaches each account's device token.
    Bearer,
    /// The web app: the browser sends its session cookie to the one server it was loaded from.
    Cookie,
}

#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum Platform {
    Ios,
    Ipados,
    Tvos,
    Android,
    Androidtv,
    Web,
}

impl Platform {
    /// The platform as the server's device APIs name it; the web has no device sessions.
    pub fn api_name(self) -> &'static str {
        match self {
            Platform::Ios => "ios",
            Platform::Ipados => "ipados",
            Platform::Tvos => "tvos",
            Platform::Android => "android",
            Platform::Androidtv => "androidtv",
            Platform::Web => "web",
        }
    }
}

/// What the shell tells the core once, when it creates it.
#[typeshare]
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CoreConfig {
    pub platform: Platform,
    pub auth_mode: AuthMode,
    /// The name new device sessions get, e.g. the system's device name.
    pub device_name: String,
    /// The system language (BCP 47), which seeds a new account's display language.
    pub locale: String,
    /// The server the web app was served from (`AuthMode::Cookie`); empty for native clients.
    #[serde(default)]
    pub origin: String,
}

/// Something that happened in the shell: a user intent or a lifecycle change.
#[typeshare]
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Message {
    pub now_ms: U53,
    pub event: Event,
}

/// The output of an effect the core asked for. Streaming effects (sockets, repeating timers)
/// resolve the same id several times and end with a terminal output.
#[typeshare]
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Resolution {
    pub now_ms: U53,
    pub id: U53,
    pub output: EffectOutput,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(tag = "type", content = "content", rename_all = "camelCase")]
pub enum Event {
    /// The shell is up; the core loads what it persisted.
    AppStarted,
    /// The app came to the foreground; stale data is refreshed.
    AppBecameActive,
    /// The web signed in through its own login form (`AuthMode::Cookie`); the core loads the
    /// new session. Native clients sign in through the core and never send it.
    SessionStarted,
    /// The user typed a server address on the add-server screen.
    ServerAddressSubmitted(servers::ServerAddress),
    ServerRemoved(servers::ServerRef),
    PasswordSignInSubmitted(accounts::PasswordSignIn),
    PairingStarted(servers::ServerRef),
    PairingCancelled,
    /// The shell opened a `couchverse://` link (a scanned QR code or a tapped link).
    LinkOpened(accounts::Link),
    AccountSelected(accounts::AccountRef),
    SignOutRequested(accounts::AccountRef),
    DevicesOpened,
    DeviceRevoked(accounts::DeviceRef),
    PairingApprovalOpened(accounts::UserCode),
    PairingApproved(accounts::PairingApproval),
    PairingDenied(accounts::UserCode),
    DisplayLanguageChanged(session::LanguageChoice),
    /// A screen showing this surface appeared; the core loads it or, when cached, shows it
    /// and refreshes it.
    ScreenOpened(Surface),
    ScreenClosed(Surface),
    /// Pull to refresh.
    RefreshRequested(Surface),
    /// The user scrolled near the end of a listing.
    BrowseMoreRequested(catalog::BrowseKey),
    /// The search field changed; the core waits for a pause in typing before searching.
    SearchChanged(catalog::SearchText),
    WatchlistChanged(catalog::WatchlistChange),
    NoticeDismissed(notices::NoticeRef),
    /// Something that may have earned an achievement happened (playback finished a title).
    AchievementsCheckRequested(ranks::CheckRequest),
    /// The celebration on screen was seen.
    CelebrationDismissed,
    /// The viewer's profile appears on public pages and leaderboards, or not.
    ProfileVisibilityChanged(ranks::PublicChoice),
    ProfileEditSubmitted(profile::ProfileEdit),
    PasswordChangeSubmitted(profile::PasswordForm),
    /// The user picked an image; the core asks the shell to upload it.
    ImageChosen(profile::ImageChoice),
    ImageRemoved(profile::ImageSlotRef),
}

/// A screen, panel or piece of state a shell renders from a view model.
#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(tag = "type", content = "content", rename_all = "camelCase")]
pub enum Surface {
    /// Where the app is: which account is active and what the shell should show at the root.
    App,
    /// The known servers and the add-server flow.
    Servers,
    /// Every account on every server: the "Who's watching?" picker and the account switcher.
    Accounts,
    /// Signing in to a server: password and pairing.
    SignIn,
    /// The signed-in account's devices.
    Devices,
    /// A pairing request this account is approving for another device.
    PairingApproval,
    /// The active account's session: user, features, display language and theme.
    Session,
    /// A markdown document rendered safely; the content is the source.
    Markdown(String),
    Home,
    /// A listing of movies, series or a genre.
    Browse(catalog::BrowseKey),
    /// A title's detail page; the content is its slug.
    Title(String),
    Genres,
    MyList,
    Search,
    /// Transient notices for a toast or banner.
    Notices,
    /// The viewer's rank badge and the achievement celebrations.
    Rank,
    /// A member's profile; the content is the username.
    Profile(String),
    Leaderboard(ranks::LeaderboardKey),
    /// The viewer's profile, password and image saves.
    ProfileEditor,
}

/// Something the core asks the shell to do. One-shot effects resolve once; streaming ones
/// resolve until a terminal output and can be cancelled.
#[typeshare]
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct EffectRequest {
    pub id: U53,
    pub effect: Effect,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(tag = "type", content = "content", rename_all = "camelCase")]
pub enum Effect {
    Http(HttpRequest),
    /// Fire once after a delay, or repeatedly at an interval until cancelled.
    Timer(TimerRequest),
    /// Stop a running timer; fire-and-forget.
    CancelTimer(EffectRef),
    /// Read, write or delete a secret (device tokens): Keychain, Keystore.
    SecureStore(StoreRequest),
    /// Read, write or delete non-secret state (servers, accounts, warm-start data).
    Store(StoreRequest),
    /// These view models changed; re-read them with `view`. Fire-and-forget.
    Render(RenderRequest),
    /// Upload a file the shell holds as a multipart form; resolves like `Http`.
    Upload(UploadRequest),
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct EffectRef {
    pub id: U53,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct HttpRequest {
    pub method: String,
    /// Absolute for native clients; origin-relative (`/api/v1/...`) for the web.
    pub url: String,
    pub headers: Vec<HttpHeader>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub body: Option<String>,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct UploadRequest {
    /// The request without a body; the shell sends the form as its body.
    pub request: HttpRequest,
    /// The handle the shell gave the core for the picked file.
    pub file: String,
    /// The form part the file goes in, with the file's own name (servers type images by its
    /// extension).
    pub field: String,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct HttpHeader {
    pub name: String,
    pub value: String,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct TimerRequest {
    pub after_ms: U53,
    /// Fire again every `after_ms` until cancelled.
    #[serde(default)]
    pub repeat: bool,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct StoreRequest {
    pub key: String,
    pub op: StoreOp,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(tag = "type", content = "content", rename_all = "camelCase")]
pub enum StoreOp {
    Read,
    Write(String),
    Delete,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RenderRequest {
    pub surfaces: Vec<Surface>,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(tag = "type", content = "content", rename_all = "camelCase")]
pub enum EffectOutput {
    Http(HttpResponse),
    /// The request never got an HTTP response (offline, DNS, TLS, timeout).
    HttpFailed(HttpFailure),
    TimerFired,
    /// A store read; `None` when the key holds nothing.
    Stored(StoredValue),
    /// A store write or delete finished.
    StoreDone,
    StoreFailed(StoreFailure),
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct HttpResponse {
    pub status: u16,
    pub body: String,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct HttpFailure {
    pub kind: HttpFailureKind,
    pub message: String,
}

#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum HttpFailureKind {
    Offline,
    Timeout,
    /// The certificate was rejected: an https address that needs http, or an invalid cert.
    Tls,
    Other,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct StoredValue {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub value: Option<String>,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct StoreFailure {
    pub message: String,
}

/// Where a view model's data stands. `Stale` keeps the previous content while a refresh runs,
/// so a screen never blanks out (stale beats blank).
#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum LoadStatus {
    #[default]
    Idle,
    Loading,
    Loaded,
    Stale,
    NotFound,
    Failed,
}

/// A failure the shell shows; `code` is stable and localized by the shell, never by the core.
#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Problem {
    pub code: String,
    /// An English hint for logs and debugging, never shown as is.
    pub detail: String,
}

impl Problem {
    pub fn new(code: &str, detail: impl Into<String>) -> Self {
        Self { code: code.to_string(), detail: detail.into() }
    }
}
