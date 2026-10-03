//! The bridge vocabulary: every message a shell sends to the core or receives from it. All of it
//! is plain data, generated for Swift, Kotlin and TypeScript by typeshare, so the three shells
//! decode it identically. Enums carrying data are adjacently tagged (`{"type", "content"}`);
//! data-less enums are plain strings.

use serde::{Deserialize, Serialize};
use typeshare::typeshare;

use crate::modules::{
    accounts, catalog, couch, notices, playback, profile, ranks, servers, session,
};

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
    /// The web changed what the session shows through its own API calls (a profile edit, the
    /// admin settings) or had one rejected as signed out; the core reads the session again.
    /// Native clients make those calls through the core and never send it.
    SessionChanged,
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
    /// The player screen opened for a movie or an episode (or switched to another episode).
    PlayRequested(catalog::PlayTarget),
    /// What the shell's player is doing: sent on every state change and about once a second
    /// while playing.
    PlayerReported(PlayerReport),
    /// The player screen went away; the core saves progress and stops the stream.
    PlayerClosed,
    QualityChosen(playback::QualityChoice),
    AudioChosen(playback::TrackChoice),
    SubtitlesChosen(playback::TrackChoice),
    /// Play the next episode now instead of waiting for the countdown.
    NextEpisodeRequested,
    /// Hide the next-episode countdown; playback stops at the end.
    NextEpisodeCancelled,
    ShuffleToggled,
    /// What this device can play, measured by the shell once per launch.
    CapabilitiesReported(playback::DeviceProfile),
    /// Host a couch session around what is playing.
    CouchStartRequested,
    /// Join a couch session by its code (a typed code, a scanned QR or a link).
    CouchJoinRequested(couch::CouchCode),
    /// Join the session as a remote for this account's own player on another device.
    CouchRemoteRequested(couch::CouchCode),
    /// Leave the session; a host leaving ends it.
    CouchLeft,
    /// The host ends the session for everyone.
    CouchEndRequested,
    CouchEmojiSent(couch::CouchReaction),
    /// A follower paused or resumed their own playback.
    CouchLocalPauseChanged(couch::CouchPause),
    /// A remote's play, pause, seek, next or previous.
    CouchRemoteCommanded(couch::RemoteControl),
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
    /// The player screen: sources, tracks, qualities, episodes and the next-episode countdown.
    Player,
    /// The couch session: members, reactions, the host's state and the follower's sync.
    Couch,
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
    /// Drive the shell's video player; fire-and-forget. It reports back with `PlayerReported`.
    Player(PlayerCommand),
    /// Open, write to or close a WebSocket. An open resolves `socketOpened`, then
    /// `socketText` for every frame, and ends with `socketClosed`; send and close are
    /// fire-and-forget.
    Socket(SocketCommand),
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(tag = "type", content = "content", rename_all = "camelCase")]
pub enum SocketCommand {
    Open(SocketOpen),
    Send(SocketSend),
    Close(EffectRef),
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SocketOpen {
    /// `ws://` or `wss://`; a path alone is relative to the page (the web picks the scheme).
    pub url: String,
    pub headers: Vec<HttpHeader>,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SocketSend {
    /// The id of the open effect.
    pub socket: U53,
    pub text: String,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(tag = "type", content = "content", rename_all = "camelCase")]
pub enum PlayerCommand {
    /// Replace what is playing. Sent again for a quality or audio-file switch, starting where
    /// playback was.
    Load(PlayerLoad),
    Play,
    Pause,
    Seek(PlayerSeek),
    /// Switch to the stream's embedded audio rendition in this language.
    SelectAudio(AudioRendition),
    /// Show this subtitle track, or none.
    SelectSubtitles(SubtitleSelection),
    /// Stop and release the player.
    Stop,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PlayerLoad {
    pub url: String,
    pub source: PlayerSource,
    pub start_seconds: f64,
    pub autoplay: bool,
    /// Caps an HLS stream at this height (a quality the user pinned); absent to adapt freely.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_height: Option<u32>,
    /// Sidecar WebVTT tracks the player can show.
    pub subtitles: Vec<PlayerSubtitle>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub subtitle: Option<String>,
    /// The embedded audio rendition to start with.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub audio_lang: Option<String>,
    /// A couch follower's player: no seeking or pausing of the shared timeline.
    pub linear: bool,
    /// For the system's Now Playing and lock-screen controls.
    pub now_playing: NowPlaying,
}

#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum PlayerSource {
    /// A media file the player reads progressively.
    File,
    /// An HLS multivariant playlist.
    Hls,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PlayerSubtitle {
    pub id: String,
    pub lang: String,
    pub label: String,
    pub url: String,
    pub forced: bool,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct NowPlaying {
    pub title: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub subtitle: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub artwork: Option<String>,
    pub duration_seconds: f64,
}

#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PlayerSeek {
    pub seconds: f64,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AudioRendition {
    pub lang: String,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SubtitleSelection {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub id: Option<String>,
}

/// The shell's player state. `failed` carries a short reason when the player gave up.
#[typeshare]
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PlayerReport {
    pub position_seconds: f64,
    pub duration_seconds: f64,
    pub playing: bool,
    #[serde(default)]
    pub buffering: bool,
    #[serde(default)]
    pub ended: bool,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub failed: Option<String>,
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
    SocketOpened,
    /// A text frame from the socket.
    SocketText(SocketText),
    /// Terminal: the socket closed or could not open.
    SocketClosed(SocketClosed),
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SocketText {
    pub text: String,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SocketClosed {
    /// The WebSocket close code; 1006 when the connection failed.
    pub code: u16,
    #[serde(default)]
    pub reason: String,
}

impl EffectOutput {
    /// Whether this output ends a streaming effect, so nothing more arrives for its id.
    pub fn is_terminal(&self) -> bool {
        matches!(self, EffectOutput::SocketClosed(_))
    }
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
