//! Playing a movie or an episode: fetching how to play it for this device, loading the shell's
//! player, resuming, accounting watched time and saving progress, keeping an instant-play
//! (JIT) transcode alive and stopping it, waiting for one that is still being prepared,
//! switching quality, audio and subtitles, and deciding what plays next. Shells only run the
//! player and report what it does.

mod views;

use couchverse_api::ops::{GetPlaybackQuery, ResolvePlaybackQuery};
use couchverse_api::types::{
    GetPlaybackKind, PlaybackAudioTrackSource, PlaybackInfo, PlaybackInfoMode, PlaybackInfoTier,
    ProgressReport, ResolvePlaybackKind, StreamSession, StreamSessionStart,
};
use couchverse_api::{Call, ops};
use serde::{Deserialize, Serialize};
use serde_json::{Value, json};
use typeshare::typeshare;

pub use views::*;

use crate::api::{Endpoint, Failure, decode};
use crate::core::Pending;
use crate::effects::Ctx;
use crate::messages::{
    AudioRendition, EffectOutput, LoadStatus, NowPlaying, PlayerCommand, PlayerLoad, PlayerReport,
    PlayerSource, PlayerSubtitle, Problem, SubtitleSelection, Surface, U53,
};
use crate::modules::catalog::{PlayKind, PlayTarget};
use crate::modules::images::Images;
use crate::time;

/// Starting over is pointless in the first seconds, as on the web.
const RESUME_AFTER_SECONDS: f64 = 5.0;
/// Progress is saved every this many seconds of playback, and on pause.
const REPORT_EVERY_SECONDS: f64 = 10.0;
/// The next episode is offered this close to the end.
const NEXT_UP_SECONDS: f64 = 20.0;
/// A JIT transcode is reaped by the server without a sign of life this often.
const KEEPALIVE_MS: U53 = 15_000;
/// How often a transcode being prepared is checked on.
const PREPARING_POLL_MS: U53 = 3_000;
const PREFS_KEY: &str = "player.prefs";

/// What this device can play, measured by the shell once per launch (plan 7.6). It mirrors the
/// server's device profile, which is authoritative: the server never offers what this leaves out.
#[typeshare]
#[derive(Debug, Clone, PartialEq, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DeviceProfile {
    /// Progressive containers played directly.
    pub containers: Vec<Container>,
    pub video: Vec<VideoSupport>,
    /// Audio decoded, or passed through to a receiver.
    pub audio: Vec<AudioSupport>,
    /// HDR formats shown; SDR is always assumed.
    #[serde(default)]
    pub hdr: Vec<HdrFormat>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_width: Option<u32>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_height: Option<u32>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_frame_rate: Option<f64>,
    /// Bits per second.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_bitrate: Option<U53>,
    /// HLS segment formats played; empty for none.
    pub hls: Vec<HlsFormat>,
    /// Subtitles rendered beside a progressive file; empty when they need HLS renditions
    /// (`AVPlayer`).
    #[serde(default)]
    pub sidecar_subtitles: Vec<SubtitleFormat>,
    /// Audio tracks inside a progressive file can be switched.
    #[serde(default)]
    pub audio_track_switching: bool,
}

#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum Container {
    Mp4,
    Mov,
    Mkv,
    Webm,
    Ts,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct VideoSupport {
    pub codec: VideoCodec,
    /// Empty for every profile within `max_bit_depth`.
    #[serde(default)]
    pub profiles: Vec<VideoProfile>,
    /// The highest level as written on the box (5.1).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_level: Option<f64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_bit_depth: Option<u8>,
}

#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum VideoCodec {
    H264,
    Hevc,
    Av1,
    Vp9,
}

#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum VideoProfile {
    Baseline,
    Main,
    High,
    High10,
    High422,
    High444,
    Main10,
    Rext,
    Professional,
    Profile0,
    Profile1,
    Profile2,
    Profile3,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AudioSupport {
    pub codec: AudioCodec,
    /// Two when absent.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_channels: Option<u8>,
    /// Dolby Atmos reaches the output as Atmos.
    #[serde(default)]
    pub atmos: bool,
}

#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum AudioCodec {
    Aac,
    Mp3,
    Ac3,
    Eac3,
    Truehd,
    Dts,
    Flac,
    Opus,
    Vorbis,
    Alac,
    Pcm,
}

#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum HdrFormat {
    Hdr10,
    Hdr10plus,
    Hlg,
    DolbyVision5,
    DolbyVision7,
    DolbyVision8,
    DolbyVision10,
}

#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum HlsFormat {
    Ts,
    Fmp4,
}

#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum SubtitleFormat {
    Webvtt,
}

impl DeviceProfile {
    /// The API's profile: the same JSON, so serde maps one onto the other.
    pub fn to_api(&self) -> Option<couchverse_api::types::DeviceProfile> {
        serde_json::to_value(self).ok().and_then(|v| serde_json::from_value(v).ok())
    }
}

/// A quality from `PlayerView.qualities`, by its key.
#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct QualityChoice {
    pub key: String,
}

/// A track by id; no id turns subtitles off.
#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct TrackChoice {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub id: Option<String>,
}

/// Choices that carry over from one title to the next on this device.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct Prefs {
    subtitle_lang: Option<String>,
    audio_lang: Option<String>,
    shuffle: bool,
}

/// Everything the module needs from the active session for a request.
pub struct Env<'a> {
    pub endpoint: &'a Endpoint,
    pub language: &'a str,
    /// For the artwork the system's Now Playing shows.
    pub images: Images,
}

#[derive(Debug, Clone, PartialEq)]
pub struct PlaybackPending {
    /// Bumped by every new title, so answers about the previous one are dropped.
    generation: u64,
    request: Request,
}

#[derive(Debug, Clone, PartialEq)]
enum Request {
    Info(Call<PlaybackInfo>),
    Jit(Call<StreamSession>),
    PreparingTick,
    KeepaliveTick,
    /// A progress save; one that never reached the server is handed back to be kept for
    /// later, stamped with when it was made.
    Progress {
        report: Box<ProgressReport>,
        wall: Option<U53>,
    },
    /// Fire-and-forget calls: keepalives, stopping a transcode.
    Ignored,
    Prefs,
}

#[derive(Debug, Clone, PartialEq)]
pub enum PlaybackChange {
    None,
    Unauthorized,
    /// Something was watched: a good moment to look for achievements (forced at the end).
    Watched {
        finished: bool,
    },
    /// A progress save never reached the server (offline): keep it and send it later.
    Unsaved(Box<ProgressReport>),
    /// A couch follower's player failed: its payload comes from the couch, which fetches a
    /// fresh one.
    Refollow,
}

/// A finished download, played from the device.
pub struct LocalTitle {
    pub target: PlayTarget,
    /// The file's name in the shell's downloads directory.
    pub file: String,
    pub title: String,
    /// The episode line; empty for a movie.
    pub subtitle: String,
    pub title_id: String,
    pub title_slug: String,
    pub duration: f64,
    /// Where offline playback stopped last time.
    pub resume: f64,
    /// The tracks inside the file: language and label.
    pub audio: Vec<(String, String)>,
    /// The subtitles inside the file: language, label and whether forced.
    pub subtitles: Vec<(String, String, bool)>,
}

/// Which source the player has, so a switch knows what it is switching from.
#[derive(Debug, Clone, PartialEq, Eq)]
enum Quality {
    /// The source file or the copied source video, outside the ladder.
    Original,
    /// The transcoded ladder, adapting.
    Auto,
    /// The ladder pinned to one rendition, by its name.
    Rendition(String),
}

impl Quality {
    fn key(&self) -> String {
        match self {
            Quality::Original => "original".into(),
            Quality::Auto => "auto".into(),
            Quality::Rendition(name) => name.clone(),
        }
    }
}

struct Jit {
    grant: String,
    session: String,
    keepalive: U53,
}

/// What follows this title, decided once near the end so the countdown and the jump agree.
#[derive(Debug, Clone, PartialEq)]
enum Next {
    Undecided,
    Decided(NextUp),
    /// The viewer dismissed the countdown: playback stops at the end.
    Cancelled,
}

impl Next {
    fn up(&self) -> Option<&NextUp> {
        match self {
            Next::Decided(next) => Some(next),
            Next::Undecided | Next::Cancelled => None,
        }
    }
}

/// One title on screen.
struct Session {
    target: PlayTarget,
    images: Images,
    info: Option<PlaybackInfo>,
    status: LoadStatus,
    problem: Option<Problem>,
    preparing_timer: Option<U53>,
    jit: Option<Jit>,
    quality: Quality,
    audio: Option<String>,
    subtitle: Option<String>,
    /// A couch follower's session: the host's progress counts, not this one's.
    linear: bool,
    /// The last report from the player, and when it came.
    position: f64,
    duration: f64,
    playing: bool,
    reported_at: Option<U53>,
    /// Played seconds not saved yet; seeks and pauses add nothing.
    watched: f64,
    saved_position: f64,
    next: Next,
    /// The player failed once already and was reloaded with a fresh payload.
    /// Reloads after a player failure; the first one fetches a fresh payload.
    retries: u8,
    buffering: bool,
    /// A finished download plays from this file in the downloads directory.
    local: Option<String>,
}

impl Session {
    fn new(target: PlayTarget, images: Images, linear: bool) -> Self {
        Self {
            target,
            images,
            info: None,
            status: LoadStatus::Loading,
            problem: None,
            preparing_timer: None,
            jit: None,
            quality: Quality::Auto,
            audio: None,
            subtitle: None,
            linear,
            position: 0.0,
            duration: 0.0,
            playing: false,
            reported_at: None,
            watched: 0.0,
            saved_position: 0.0,
            next: Next::Undecided,
            retries: 0,
            buffering: false,
            local: None,
        }
    }
}

#[derive(Default)]
pub struct Playback {
    generation: u64,
    /// The last report was a play, a pause or a seek: a couch host tells its followers at once.
    moved: bool,
    prefs: Prefs,
    /// The API form of the device's profile, once the shell reported it.
    profile: Option<couchverse_api::types::DeviceProfile>,
    session: Option<Session>,
    /// Draws for shuffle; any sequence will do, the core only needs no clock or OS randomness.
    seed: u64,
}

impl Playback {
    pub fn load_prefs(&mut self, ctx: &mut Ctx) {
        ctx.store_read(PREFS_KEY, self.pending(Request::Prefs));
    }

    pub fn set_profile(&mut self, profile: &DeviceProfile) {
        self.profile = profile.to_api();
    }

    /// The device's profile for the server, when the shell reported one.
    pub fn profile(&self) -> Option<&couchverse_api::types::DeviceProfile> {
        self.profile.as_ref()
    }

    /// Opens a title, ending whatever was playing.
    pub fn play(&mut self, ctx: &mut Ctx, env: &Env, target: PlayTarget, linear: bool) {
        self.end(ctx, env.endpoint);
        self.generation += 1;
        self.seed ^= ctx.now;
        self.session = Some(Session::new(target, env.images.clone(), linear));
        self.fetch(ctx, env);
        ctx.render(Surface::Player);
    }

    fn fetch(&mut self, ctx: &mut Ctx, env: &Env) {
        let Some(session) = &self.session else { return };
        let lang = Some(env.language.to_string());
        let id = &session.target.id;
        // the profile decides how to play; without one, the browser baseline does
        let call = match (&self.profile, session.target.kind) {
            (Some(profile), kind) => {
                let kind = match kind {
                    PlayKind::Movie => ResolvePlaybackKind::Movie,
                    PlayKind::Episode => ResolvePlaybackKind::Episode,
                };
                ops::resolve_playback(kind, id, &ResolvePlaybackQuery { lang }, profile)
            }
            (None, PlayKind::Movie) => ops::get_playback(
                GetPlaybackKind::Movie,
                id,
                &GetPlaybackQuery { lang, caps: None },
            ),
            (None, PlayKind::Episode) => ops::get_playback(
                GetPlaybackKind::Episode,
                id,
                &GetPlaybackQuery { lang, caps: None },
            ),
        };
        let request = env.endpoint.request(&call.request);
        ctx.http(request, self.pending(Request::Info(call)));
    }

    /// The player screen went away: save where the viewer got to and free the transcode.
    pub fn close(&mut self, ctx: &mut Ctx, endpoint: &Endpoint) {
        self.end(ctx, endpoint);
        self.generation += 1;
        ctx.render(Surface::Player);
    }

    fn end(&mut self, ctx: &mut Ctx, endpoint: &Endpoint) {
        let Some(mut session) = self.session.take() else { return };
        if session.status == LoadStatus::Loaded {
            ctx.player(PlayerCommand::Stop);
        }
        if let Some(report) = progress(&mut session) {
            let call = ops::save_progress_beacon(&report);
            let request = Request::Progress { report: Box::new(report), wall: ctx.wall };
            ctx.http(endpoint.request(&call.request), self.pending(request));
        }
        if let Some(timer) = session.preparing_timer {
            ctx.cancel_timer(timer);
        }
        if let Some(jit) = session.jit {
            ctx.cancel_timer(jit.keepalive);
            let call = ops::stop_stream_session(&jit.grant, &jit.session);
            ctx.http(endpoint.request(&call.request), self.pending(Request::Ignored));
        }
    }

    /// Account switch or sign-out: drop everything without talking to the server.
    pub fn reset(&mut self, ctx: &mut Ctx) {
        if let Some(session) = self.session.take() {
            if let Some(timer) = session.preparing_timer {
                ctx.cancel_timer(timer);
            }
            if let Some(jit) = session.jit {
                ctx.cancel_timer(jit.keepalive);
            }
            ctx.player(PlayerCommand::Stop);
        }
        self.generation += 1;
        ctx.render(Surface::Player);
    }

    pub fn report(&mut self, ctx: &mut Ctx, env: &Env, report: &PlayerReport) -> PlaybackChange {
        let Some(session) = self.session.as_mut() else { return PlaybackChange::None };
        if session.status != LoadStatus::Loaded {
            return PlaybackChange::None;
        }
        if let Some(reason) = &report.failed {
            return self.failed(ctx, env, reason);
        }
        // a forward step no longer than the time that passed (at up to double speed) is
        // playback; anything else is a seek
        let mut seeked = false;
        if let Some(at) = session.reported_at {
            #[allow(clippy::cast_precision_loss)] // milliseconds between reports
            let elapsed = ctx.now.saturating_sub(at) as f64 / 1000.0;
            let step = report.position_seconds - session.position;
            let window = if session.playing { elapsed * 2.0 + 1.0 } else { 1.0 };
            if session.playing && step > 0.0 && step <= window {
                session.watched += step;
            }
            seeked = !(-1.0..=window).contains(&step);
        }
        let paused = session.playing && !report.playing;
        self.moved = paused || seeked || (!session.playing && report.playing);
        session.buffering = report.buffering;
        session.position = report.position_seconds;
        session.duration = report.duration_seconds.max(session.duration);
        session.playing = report.playing;
        session.reported_at = Some(ctx.now);

        let due = (session.position - session.saved_position).abs() >= REPORT_EVERY_SECONDS;
        let mut change = PlaybackChange::None;
        if report.ended || paused || due {
            change = self.save(ctx, env.endpoint, report.ended);
        }
        if self.next_up(ctx, report.ended) {
            ctx.render(Surface::Player);
        }
        if report.ended {
            change = PlaybackChange::Watched { finished: true };
            let advance = self.session.as_ref().and_then(|s| s.next.up().cloned());
            if let Some(next) = advance {
                self.play(ctx, env, next.target, false);
            }
        }
        change
    }

    fn save(&mut self, ctx: &mut Ctx, endpoint: &Endpoint, finished: bool) -> PlaybackChange {
        let Some(session) = self.session.as_mut() else { return PlaybackChange::None };
        let Some(report) = progress(session) else { return PlaybackChange::None };
        let call = ops::save_progress(&report);
        let request = Request::Progress { report: Box::new(report), wall: ctx.wall };
        ctx.http(endpoint.request(&call.request), self.pending(request));
        PlaybackChange::Watched { finished }
    }

    /// Shows the next episode near the end; returns whether the countdown changed.
    fn next_up(&mut self, _ctx: &mut Ctx, ended: bool) -> bool {
        let Some(session) = self.session.as_mut() else { return false };
        let Some(info) = &session.info else { return false };
        let remaining = session.duration - session.position;
        // a viewer who skipped straight to the end still gets the next episode
        let near_end = ended || (remaining > 0.0 && remaining <= NEXT_UP_SECONDS);
        if session.linear || session.next == Next::Cancelled || !near_end {
            return false;
        }
        if session.next == Next::Undecided {
            let candidates = next_candidates(info, &session.target, self.prefs.shuffle);
            if candidates.is_empty() {
                return false;
            }
            self.seed = splitmix(self.seed);
            #[allow(clippy::cast_possible_truncation)] // the remainder is below the list length
            let pick = (self.seed % candidates.len() as u64) as usize;
            session.next = Next::Decided(candidates[pick].clone());
        }
        #[allow(clippy::cast_possible_truncation, clippy::cast_sign_loss)] // 0 to 20 seconds
        let countdown = remaining.ceil() as u32;
        let Next::Decided(next) = &mut session.next else { return false };
        let changed = next.countdown_seconds != countdown;
        next.countdown_seconds = countdown;
        changed
    }

    pub fn next_now(&mut self, ctx: &mut Ctx, env: &Env) {
        let Some(session) = self.session.as_ref() else { return };
        let next = session.next.up().cloned().or_else(|| {
            let info = session.info.as_ref()?;
            next_candidates(info, &session.target, self.prefs.shuffle).into_iter().next()
        });
        if let Some(next) = next {
            self.save(ctx, env.endpoint, false);
            self.play(ctx, env, next.target, false);
        }
    }

    pub fn cancel_next(&mut self, ctx: &mut Ctx) {
        if let Some(session) = self.session.as_mut() {
            session.next = Next::Cancelled;
            ctx.render(Surface::Player);
        }
    }

    pub fn toggle_shuffle(&mut self, ctx: &mut Ctx) {
        self.prefs.shuffle = !self.prefs.shuffle;
        if let Some(session) = self.session.as_mut() {
            // decided again with the new rule, unless dismissed
            if session.next != Next::Cancelled {
                session.next = Next::Undecided;
            }
        }
        self.next_up(ctx, false);
        self.save_prefs(ctx);
        ctx.render(Surface::Player);
    }

    pub fn choose_quality(&mut self, ctx: &mut Ctx, key: &str) {
        let Some(session) = self.session.as_mut() else { return };
        let Some(info) = &session.info else { return };
        let quality = match key {
            "original" => Quality::Original,
            "auto" => Quality::Auto,
            name if info.variants.iter().flatten().any(|v| v.name == name) => {
                Quality::Rendition(name.to_string())
            }
            _ => return,
        };
        if quality == session.quality {
            return;
        }
        session.quality = quality;
        let start = session.position;
        let autoplay = session.playing;
        self.load(ctx, start, autoplay);
        ctx.render(Surface::Player);
    }

    pub fn choose_audio(&mut self, ctx: &mut Ctx, id: Option<&str>) {
        let Some(session) = self.session.as_mut() else { return };
        let Some(track) = session
            .info
            .as_ref()
            .and_then(|i| i.audio.as_ref()?.iter().find(|t| Some(t.id.as_str()) == id).cloned())
        else {
            return;
        };
        if session.audio.as_deref() == Some(track.id.as_str()) {
            return;
        }
        session.audio = Some(track.id.clone());
        self.prefs.audio_lang = Some(track.lang.clone());
        if track.source == PlaybackAudioTrackSource::Embedded {
            // a rendition inside the stream switches in place
            let index = session.info.as_ref().and_then(|i| {
                i.audio
                    .iter()
                    .flatten()
                    .filter(|t| t.source == PlaybackAudioTrackSource::Embedded)
                    .position(|t| t.id == track.id)
                    .and_then(|p| u32::try_from(p).ok())
            });
            ctx.player(PlayerCommand::SelectAudio(AudioRendition { lang: track.lang, index }));
        } else {
            // another language is another file: reload it where playback was
            let (start, autoplay) = (session.position, session.playing);
            self.load(ctx, start, autoplay);
        }
        self.save_prefs(ctx);
        ctx.render(Surface::Player);
    }

    pub fn choose_subtitles(&mut self, ctx: &mut Ctx, id: Option<&str>) {
        let Some(session) = self.session.as_mut() else { return };
        let Some(info) = &session.info else { return };
        let track = id.and_then(|id| info.subtitles.iter().find(|s| s.id == id));
        if id.is_some() && track.is_none() {
            return;
        }
        session.subtitle = track.map(|t| t.id.clone());
        self.prefs.subtitle_lang = track.map(|t| t.lang.clone());
        ctx.player(PlayerCommand::SelectSubtitles(SubtitleSelection {
            id: session.subtitle.clone(),
        }));
        self.save_prefs(ctx);
        ctx.render(Surface::Player);
    }

    fn save_prefs(&self, ctx: &mut Ctx) {
        if let Ok(json) = serde_json::to_string(&self.prefs) {
            ctx.store_write(PREFS_KEY, json);
        }
    }

    fn pending(&self, request: Request) -> Pending {
        Pending::Playback(PlaybackPending { generation: self.generation, request })
    }

    pub fn resolve(
        &mut self,
        ctx: &mut Ctx,
        env: Option<&Env>,
        pending: PlaybackPending,
        output: EffectOutput,
    ) -> PlaybackChange {
        match pending.request {
            Request::Prefs => {
                if let EffectOutput::Stored(stored) = output
                    && let Some(prefs) = stored.value.and_then(|j| serde_json::from_str(&j).ok())
                {
                    self.prefs = prefs;
                }
                return PlaybackChange::None;
            }
            Request::Ignored => {
                return match decode_status(&output) {
                    Some(401) => PlaybackChange::Unauthorized,
                    _ => PlaybackChange::None,
                };
            }
            Request::Progress { mut report, wall } => {
                return match decode_status(&output) {
                    Some(401) => PlaybackChange::Unauthorized,
                    // offline, or a server that could not take it: kept for later
                    None | Some(500..) => {
                        report.watched_at = report.watched_at.or(wall.map(time::rfc3339));
                        PlaybackChange::Unsaved(report)
                    }
                    Some(_) => PlaybackChange::None,
                };
            }
            _ if pending.generation != self.generation => return PlaybackChange::None,
            Request::Info(call) => match decode(&call, output) {
                Ok(info) => self.info(ctx, env, info),
                Err(failure) => return self.load_failed(ctx, &failure),
            },
            Request::Jit(call) => match decode(&call, output) {
                Ok(started) => self.jit_started(ctx, started),
                Err(failure) => return self.load_failed(ctx, &failure),
            },
            Request::PreparingTick => {
                if let Some(session) = self.session.as_mut() {
                    session.preparing_timer = None;
                }
                if let Some(env) = env {
                    self.fetch(ctx, env);
                }
            }
            Request::KeepaliveTick => {
                if let (Some(env), Some(jit)) =
                    (env, self.session.as_ref().and_then(|s| s.jit.as_ref()))
                {
                    let call = ops::keep_stream_session_alive(&jit.grant, &jit.session);
                    ctx.http(env.endpoint.request(&call.request), self.pending(Request::Ignored));
                }
            }
        }
        PlaybackChange::None
    }

    fn info(&mut self, ctx: &mut Ctx, env: Option<&Env>, info: PlaybackInfo) {
        let Some(session) = self.session.as_mut() else { return };
        ctx.render(Surface::Player);
        // a fresh payload for a title already playing (an expired grant) picks up where it was
        let refresh = session.info.is_some()
            && matches!(session.status, LoadStatus::Loaded | LoadStatus::Stale);
        let resume = if refresh {
            session.position
        } else if session.linear {
            0.0
        } else {
            #[allow(clippy::cast_precision_loss)] // seconds
            let saved = info.resume_position as f64;
            if saved > RESUME_AFTER_SECONDS { saved } else { 0.0 }
        };
        match info.mode {
            PlaybackInfoMode::Preparing => {
                session.info = Some(info);
                session.status = LoadStatus::Loading;
                let pending = Pending::Playback(PlaybackPending {
                    generation: self.generation,
                    request: Request::PreparingTick,
                });
                session.preparing_timer = Some(ctx.after(PREPARING_POLL_MS, pending));
            }
            PlaybackInfoMode::Jit => {
                let call = ops::create_stream_session(
                    &info.grant,
                    &StreamSessionStart { start_at: Some(resume), plan: info.jit.clone() },
                );
                session.position = resume;
                session.info = Some(info);
                if let Some(env) = env {
                    let request = env.endpoint.request(&call.request);
                    ctx.http(
                        request,
                        Pending::Playback(PlaybackPending {
                            generation: self.generation,
                            request: Request::Jit(call),
                        }),
                    );
                }
            }
            PlaybackInfoMode::Direct | PlaybackInfoMode::Hls => {
                session.quality = starting_quality(&info);
                if !refresh {
                    session.audio = default_audio(&info, self.prefs.audio_lang.as_deref());
                    session.subtitle = self.prefs.subtitle_lang.as_ref().and_then(|lang| {
                        info.subtitles.iter().find(|s| &s.lang == lang).map(|s| s.id.clone())
                    });
                }
                #[allow(clippy::cast_precision_loss)] // seconds
                let saved = info.resume_position as f64;
                session.saved_position = if session.linear { 0.0 } else { saved };
                session.duration = info.duration_seconds;
                session.info = Some(info);
                session.status = LoadStatus::Loaded;
                session.problem = None;
                self.load(ctx, resume, true);
            }
            PlaybackInfoMode::Unsupported | PlaybackInfoMode::Unknown => {
                session.info = Some(info);
                session.status = LoadStatus::Failed;
                session.problem = Some(Problem::new("unsupported", "nothing this device can play"));
            }
        }
    }

    fn jit_started(&mut self, ctx: &mut Ctx, started: StreamSession) {
        let pending = self.pending(Request::KeepaliveTick);
        let Some(session) = self.session.as_mut() else { return };
        let Some(info) = &session.info else { return };
        let keepalive = ctx.every(KEEPALIVE_MS, pending);
        session.jit =
            Some(Jit { grant: info.grant.clone(), session: started.session_id, keepalive });
        session.status = LoadStatus::Loaded;
        session.quality = Quality::Auto;
        session.audio = default_audio(info, self.prefs.audio_lang.as_deref());
        #[allow(clippy::cast_precision_loss)] // seconds
        let saved = info.resume_position as f64;
        session.saved_position = saved;
        session.duration = info.duration_seconds;
        let url = started.playlist_url;
        let start = session.position;
        let load = self.player_load(url, PlayerSource::Hls, None, start, true);
        if let Some(load) = load {
            ctx.player(PlayerCommand::Load(load));
        }
        ctx.render(Surface::Player);
    }

    /// Loads the session's chosen source into the player.
    fn load(&mut self, ctx: &mut Ctx, start: f64, autoplay: bool) {
        let Some(session) = self.session.as_ref() else { return };
        let Some(info) = &session.info else { return };
        if let Some(file) = session.local.clone() {
            if let Some(load) =
                self.player_load(file, PlayerSource::Download, None, start, autoplay)
            {
                ctx.player(PlayerCommand::Load(load));
            }
            return;
        }
        let file_audio = session.audio.as_ref().and_then(|id| {
            info.audio
                .as_ref()?
                .iter()
                .find(|t| &t.id == id && t.source != PlaybackAudioTrackSource::Embedded)
        });
        let (url, source, max_height) = match (&session.quality, file_audio) {
            // another language's file plays in place of the main one
            (Quality::Original, Some(track)) if track.stream_url.is_some() => {
                (track.stream_url.clone(), PlayerSource::File, None)
            }
            (_, Some(track)) if track.hls_url.is_some() => {
                (track.hls_url.clone(), PlayerSource::Hls, None)
            }
            (Quality::Original, _) => {
                let (url, source) = original(info);
                (url, source, None)
            }
            (Quality::Rendition(name), _) => {
                let height =
                    info.variants.iter().flatten().find(|v| &v.name == name).map(|v| v.height);
                (ladder(info), PlayerSource::Hls, height.and_then(|h| u32::try_from(h).ok()))
            }
            (Quality::Auto, _) => (ladder(info), PlayerSource::Hls, None),
        };
        let Some(url) = url else { return };
        if let Some(load) = self.player_load(url, source, max_height, start, autoplay) {
            ctx.player(PlayerCommand::Load(load));
        }
    }

    fn player_load(
        &self,
        url: String,
        source: PlayerSource,
        max_height: Option<u32>,
        start: f64,
        autoplay: bool,
    ) -> Option<PlayerLoad> {
        let session = self.session.as_ref()?;
        let info = session.info.as_ref()?;
        let audio_lang = session.audio.as_ref().and_then(|id| {
            info.audio.as_ref()?.iter().find(|t| &t.id == id).map(|t| t.lang.clone())
        });
        Some(PlayerLoad {
            url,
            source,
            start_seconds: start,
            autoplay,
            max_height,
            subtitles: info
                .subtitles
                .iter()
                .map(|s| PlayerSubtitle {
                    id: s.id.clone(),
                    lang: s.lang.clone(),
                    label: s.label.clone(),
                    url: Some(s.url.clone()).filter(|u| !u.is_empty()),
                    forced: s.forced,
                })
                .collect(),
            subtitle: session.subtitle.clone(),
            audio_lang,
            linear: session.linear,
            now_playing: NowPlaying {
                title: info.display.title.clone(),
                subtitle: Some(info.display.subtitle.clone()).filter(|s| !s.is_empty()),
                artwork: views::backdrop(&session.images, info).map(|i| i.url),
                duration_seconds: info.duration_seconds,
            },
        })
    }

    fn load_failed(&mut self, ctx: &mut Ctx, failure: &Failure) -> PlaybackChange {
        if let Some(session) = self.session.as_mut() {
            session.status = if matches!(failure, Failure::Api(e) if e.status == 404) {
                LoadStatus::NotFound
            } else {
                LoadStatus::Failed
            };
            session.problem = Some(failure.problem());
            ctx.render(Surface::Player);
        }
        if failure.unauthorized() { PlaybackChange::Unauthorized } else { PlaybackChange::None }
    }

    /// The player gave up. Grants expire after hours, so the first failure fetches a fresh
    /// payload and resumes; a second one is reported.
    fn failed(&mut self, ctx: &mut Ctx, env: &Env, reason: &str) -> PlaybackChange {
        let Some(session) = self.session.as_mut() else { return PlaybackChange::None };
        if session.retries > 0 {
            session.status = LoadStatus::Failed;
            session.problem = Some(Problem::new("playback_failed", reason));
            ctx.render(Surface::Player);
            return PlaybackChange::None;
        }
        session.retries += 1;
        if session.linear {
            session.status = LoadStatus::Stale;
            ctx.render(Surface::Player);
            return PlaybackChange::Refollow;
        }
        if session.local.is_some() {
            // nothing to fetch: the file is on the device
            let start = session.position;
            self.load(ctx, start, true);
            return PlaybackChange::None;
        }
        session.status = LoadStatus::Stale;
        self.fetch(ctx, env);
        ctx.render(Surface::Player);
        PlaybackChange::None
    }

    pub fn view(&self, images: &Images) -> PlayerView {
        views::player(self.session.as_ref(), self.prefs.shuffle, images)
    }

    /// The couch steers a follower's player: these do nothing outside a session.
    pub fn command(&self, ctx: &mut Ctx, command: PlayerCommand) {
        if self.session.as_ref().is_some_and(|s| s.status == LoadStatus::Loaded) {
            ctx.player(command);
        }
    }

    /// Where the player is now: its last report, moved on by the time since while playing.
    pub fn position_at(&self, now: U53) -> Option<(f64, bool)> {
        let session = self.session.as_ref().filter(|s| s.status == LoadStatus::Loaded)?;
        let mut position = session.position;
        if session.playing
            && let Some(at) = session.reported_at
        {
            #[allow(clippy::cast_precision_loss)] // milliseconds since the report
            let elapsed = now.saturating_sub(at) as f64 / 1000.0;
            position += elapsed;
        }
        Some((position, session.playing))
    }

    pub fn buffering(&self) -> bool {
        self.session.as_ref().is_some_and(|s| s.buffering)
    }

    pub fn moved(&self) -> bool {
        self.moved
    }

    /// A couch follower plays what the host plays, from the payload the couch fetched, locked
    /// to the host's timeline.
    pub fn follow(&mut self, ctx: &mut Ctx, env: &Env, target: PlayTarget, info: PlaybackInfo) {
        // a fresh payload after a failure keeps the count, so a second failure is reported
        let retries = self.session.as_ref().filter(|s| s.target == target).map_or(0, |s| s.retries);
        self.end(ctx, env.endpoint);
        self.generation += 1;
        let mut session = Session::new(target, env.images.clone(), true);
        session.retries = retries;
        self.session = Some(session);
        self.info(ctx, Some(env), info);
        ctx.render(Surface::Player);
    }

    /// The episode before this one, for a remote's "previous".
    pub fn previous(&mut self, ctx: &mut Ctx, env: &Env) {
        let Some(session) = self.session.as_ref() else { return };
        let Some(episodes) = session.info.as_ref().and_then(|i| i.episodes.as_ref()) else {
            return;
        };
        let Some(index) = episodes.iter().position(|e| e.episode_id == session.target.id) else {
            return;
        };
        if let Some(before) = index.checked_sub(1).and_then(|i| episodes.get(i)) {
            let target = PlayTarget { kind: PlayKind::Episode, id: before.episode_id.clone() };
            self.save(ctx, env.endpoint, false);
            self.play(ctx, env, target, false);
        }
    }

    pub fn target(&self) -> Option<&PlayTarget> {
        self.session.as_ref().map(|s| &s.target)
    }

    /// Plays a finished download from the device: the same session as a streamed title (resume,
    /// progress saves, tracks), from a payload made up from what the download recorded.
    pub fn play_download(&mut self, ctx: &mut Ctx, env: &Env, local: &LocalTitle) {
        self.end(ctx, env.endpoint);
        self.generation += 1;
        self.seed ^= ctx.now;
        let mut session = Session::new(local.target.clone(), env.images.clone(), false);
        session.local = Some(local.file.clone());
        self.session = Some(session);
        match local_info(local) {
            Some(info) => self.info(ctx, Some(env), info),
            None => {
                if let Some(session) = self.session.as_mut() {
                    session.status = LoadStatus::Failed;
                    session.problem = Some(Problem::new("playback_failed", "unreadable download"));
                }
            }
        }
        ctx.render(Surface::Player);
    }

    /// The download playing and where it is, so offline playback resumes there next time.
    pub fn local_position(&self) -> Option<(&PlayTarget, f64)> {
        let session = self.session.as_ref().filter(|s| s.local.is_some())?;
        Some((&session.target, session.position))
    }
}

/// The payload a download plays from: the file, its tracks and what the player shows.
fn local_info(local: &LocalTitle) -> Option<PlaybackInfo> {
    let subtitles: Vec<_> = local
        .subtitles
        .iter()
        .enumerate()
        .map(|(i, (lang, label, forced))| {
            json!({ "id": format!("local:{i}"), "lang": lang, "label": label, "forced": forced, "url": "" })
        })
        .collect();
    let audio: Vec<_> = local
        .audio
        .iter()
        .enumerate()
        .map(|(i, (lang, label))| {
            json!({ "id": format!("local:{i}"), "lang": lang, "label": label, "default": i == 0, "source": "embedded" })
        })
        .collect();
    #[allow(clippy::cast_possible_truncation)] // seconds
    let resume = local.resume.floor() as i64;
    let info = json!({
        "mode": "direct",
        "tier": "direct",
        "mediaFileId": "",
        "grant": "",
        "frameUrl": "",
        "durationSeconds": local.duration,
        "resumePosition": resume,
        "allowRandomPlayback": false,
        "display": {
            "title": local.title,
            "subtitle": local.subtitle,
            "titleId": local.title_id,
            "titleSlug": local.title_slug,
            "backdropId": null,
        },
        "subtitles": subtitles,
        "audio": if audio.len() > 1 { Value::Array(audio) } else { Value::Null },
    });
    serde_json::from_value(info).ok()
}

/// The source as it is: the file for the direct tier, the copied source video in HLS for the
/// remux tier; outside the adaptive ladder either way.
fn original(info: &PlaybackInfo) -> (Option<String>, PlayerSource) {
    let direct = match info.tier {
        Some(PlaybackInfoTier::Direct) => true,
        Some(_) => false,
        None => info.mode == PlaybackInfoMode::Direct,
    };
    let url = info.original_url.clone().or_else(|| info.stream_url.clone());
    (url, if direct { PlayerSource::File } else { PlayerSource::Hls })
}

/// The transcoded ladder, or the stream itself when the payload plays HLS without one.
fn ladder(info: &PlaybackInfo) -> Option<String> {
    let stream = info.stream_url.clone().filter(|_| info.mode != PlaybackInfoMode::Direct);
    info.hls_url.clone().or(stream)
}

/// The cheapest tier the server chose plays first: the source when it can, else the ladder.
fn starting_quality(info: &PlaybackInfo) -> Quality {
    match (info.tier, info.mode) {
        (Some(PlaybackInfoTier::Direct | PlaybackInfoTier::Remux), _)
        | (None, PlaybackInfoMode::Direct) => Quality::Original,
        _ => Quality::Auto,
    }
}

/// What a progress save carries; `None` when there is nothing worth saving yet.
fn progress(session: &mut Session) -> Option<ProgressReport> {
    if session.linear
        || session.position < RESUME_AFTER_SECONDS
        || session.status != LoadStatus::Loaded
    {
        return None;
    }
    let watched = session.watched.floor();
    session.watched -= watched;
    session.saved_position = session.position;
    let (title_id, episode_id) = match session.target.kind {
        PlayKind::Movie => (Some(session.target.id.clone()), None),
        PlayKind::Episode => (None, Some(session.target.id.clone())),
    };
    #[allow(clippy::cast_possible_truncation)] // seconds of video
    Some(ProgressReport {
        title_id,
        episode_id,
        position_seconds: session.position.floor() as i64,
        duration_seconds: session.duration.floor() as i64,
        watched_seconds: Some(watched as i64),
        watched_at: None,
    })
}

/// The episodes that may play next: the following one, or with shuffle any other playable one.
fn next_candidates(info: &PlaybackInfo, current: &PlayTarget, shuffle: bool) -> Vec<NextUp> {
    let episodes = info.episodes.as_deref().unwrap_or_default();
    if shuffle && info.allow_random_playback && episodes.len() > 1 {
        return episodes
            .iter()
            .filter(|e| e.episode_id != current.id)
            .map(|e| NextUp {
                target: PlayTarget { kind: PlayKind::Episode, id: e.episode_id.clone() },
                season: u32::try_from(e.season_number).unwrap_or(0),
                episode: u32::try_from(e.episode_number).unwrap_or(0),
                name: e.name.clone(),
                countdown_seconds: 0,
                shuffled: true,
            })
            .collect();
    }
    info.next_episode
        .iter()
        .map(|e| NextUp {
            target: PlayTarget { kind: PlayKind::Episode, id: e.episode_id.clone() },
            season: u32::try_from(e.season_number).unwrap_or(0),
            episode: u32::try_from(e.episode_number).unwrap_or(0),
            name: e.name.clone(),
            countdown_seconds: 0,
            shuffled: false,
        })
        .collect()
}

fn default_audio(info: &PlaybackInfo, preferred: Option<&str>) -> Option<String> {
    let tracks = info.audio.as_deref()?;
    preferred
        .and_then(|lang| tracks.iter().find(|t| t.lang == lang))
        .or_else(|| tracks.iter().find(|t| t.default))
        .map(|t| t.id.clone())
}

/// The HTTP status of a fire-and-forget answer, to notice a revoked session.
fn decode_status(output: &EffectOutput) -> Option<u16> {
    match output {
        EffectOutput::Http(response) => Some(response.status),
        _ => None,
    }
}

/// `SplitMix64`: a tiny well-mixed sequence for picking a shuffled episode.
fn splitmix(state: u64) -> u64 {
    let mut z = state.wrapping_add(0x9e37_79b9_7f4a_7c15);
    z = (z ^ (z >> 30)).wrapping_mul(0xbf58_476d_1ce4_e5b9);
    z = (z ^ (z >> 27)).wrapping_mul(0x94d0_49bb_1331_11eb);
    z ^ (z >> 31)
}
