//! Couch TV from a viewer's side (plan 7.1): tuning in, signed in or as a guest who joined
//! through the channel link; the live player, held on the channel's clock; what is on by the
//! picture's program date, with the overlays' cues; the room's socket (presence, reactions,
//! schedule changes and the viewer's reports of what they watch); the guide; and the home
//! screen's "On now" card.
//!
//! The hold (plan 6.6): every viewer shows the same moment by keeping the picture's program
//! date three target durations behind the server's clock, playing up to 10 % faster or slower
//! for a drift under 2 s and seeking beyond. The server's clock comes with tuning in and with
//! the socket's hello, against the shell's monotonic clock, so the device's own wall clock never
//! matters.

mod views;

use std::collections::VecDeque;

use couchverse_api::couchtv::{
    ClientFrame, CouchTvPresence, CouchTvReactionCommand, CouchTvReport, ServerFrame,
};
use couchverse_api::ops::{
    GetCouchTvGuideQuery, JoinCouchTvQuery, TuneCouchTvGuestQuery, TuneCouchTvQuery,
};
use couchverse_api::types::{
    CouchTvGuest, CouchTvGuide, CouchTvJoin as JoinBody, CouchTvListing, CouchTvTune,
    JoinCouchTvDelivery,
};
use couchverse_api::{Call, ops};
use serde::{Deserialize, Serialize};
use typeshare::typeshare;

pub use views::*;

use crate::api::{Endpoint, Failure, decode};
use crate::core::Pending;
use crate::effects::Ctx;
use crate::messages::{
    AudioRendition, EffectOutput, HttpHeader, LiveStream, LoadStatus, NowPlaying, PlayerCommand,
    PlayerLoad, PlayerRate, PlayerReport, PlayerSeek, PlayerSource, PlayerSubtitle, Problem,
    SubtitleSelection, Surface, U53,
};
use crate::modules::couch::socket_base;
use crate::modules::images::{Images, Size};
use crate::time;

/// The hold keeps the picture this many target durations behind the server's clock.
const LATENCY_TARGET_DURATIONS: f64 = 3.0;
/// A drift beyond this is seeked away; a smaller one is played away.
const SEEK_DRIFT_MS: i64 = 2_000;
/// A drift this small plays in real time.
const IN_STEP_MS: i64 = 100;
/// The rate moves by the drift over `RATE_DRIFT_MS` (1 % per 50 ms, the S1 spike's hold),
/// at most `MAX_RATE_CHANGE` either way.
const RATE_DRIFT_MS: f64 = 5_000.0;
const MAX_RATE_CHANGE: f64 = 0.1;
/// After a load or a seek the player settles before it is held again.
const SETTLE_MS: U53 = 3_000;
/// How often the picture's program date is reported to the room.
const REPORT_EVERY_MS: U53 = 10_000;
/// The "Now" caption shows for the first seconds of a program; the "Next" caption for a while,
/// starting this long before the program leaves the air.
pub(crate) const NOW_CAPTION_MS: i64 = 8_000;
pub(crate) const NEXT_CAPTION_LEAD_MS: i64 = 120_000;
pub(crate) const NEXT_CAPTION_MS: i64 = 12_000;
const FIRST_BACKOFF_MS: U53 = 500;
const MAX_BACKOFF_MS: U53 = 8_000;
/// How long a reaction floats on screen, matching the shells' rise-and-fade animation.
const REACTION_MS: U53 = 2_200;
/// A player that fails is tuned in again (a fresh grant) and reloaded this many times in a row.
const MAX_RELOADS: u8 = 3;
/// What is on is asked again at most this often when the picture runs past what is known.
const LINEUP_REFRESH_MS: U53 = 60_000;
/// The guide covers the day before (today's broadcast day began within it) to a week ahead.
const GUIDE_BACK_MS: i64 = 24 * 3_600_000;
const GUIDE_AHEAD_MS: i64 = 7 * 24 * 3_600_000;
/// The home card asks again this long after the program on it ends, at the soonest.
const ON_NOW_MIN_MS: i64 = 5_000;
/// Where a guest's token travels for clients without a cookie jar.
const TOKEN_HEADER: &str = "X-Couch-Tv-Token";

/// What a viewer without an account types on the channel link's page.
#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CouchTvJoin {
    /// The code from the channel link (`/tv/join/<code>`).
    pub code: String,
    pub name: String,
}

/// Everything the module needs from the active session.
pub struct Env<'a> {
    pub endpoint: &'a Endpoint,
    pub language: &'a str,
    /// A signed-in viewer; otherwise the guest this device joined as.
    pub signed_in: bool,
    /// The web: the channel cookie carries a guest's token, and URLs are origin-relative.
    pub cookie: bool,
}

#[derive(Debug, Clone, PartialEq)]
pub struct CouchTvPending {
    generation: u64,
    request: Request,
}

#[derive(Debug, Clone, PartialEq)]
enum Request {
    /// Tuning in; `reload` loads the player with the answer (the first tune, or after the
    /// player failed), otherwise only what is on and the clock are read again.
    Tune {
        call: Call<CouchTvTune>,
        asked_at: U53,
        reload: bool,
    },
    Join(Call<CouchTvGuest>),
    Guide(Call<CouchTvGuide>),
    OnNow {
        call: Call<CouchTvTune>,
        asked_at: U53,
    },
    OnNowDue,
    /// The room's socket: resolved on open, for every frame and once when it closes.
    Socket,
    Reconnect,
    ReactionGone(U53),
}

/// What the rest of the core should do after a Couch TV output.
#[derive(Debug, Clone, Copy, PartialEq)]
pub enum CouchTvChange {
    None,
    /// A signed-in viewer's request came back 401.
    Unauthorized,
}

/// The tuned stream.
struct Stream {
    url: String,
    latency_ms: i64,
    target_seconds: f64,
}

/// What the player shows, as it last reported.
struct Picture {
    position: f64,
    /// The program date on screen, and when the shell reported it.
    program_date: Option<(i64, U53)>,
    playing: bool,
    buffering: bool,
    rate: f64,
    settle_until: U53,
}

impl Picture {
    fn loaded(now: U53) -> Self {
        Self {
            position: 0.0,
            program_date: None,
            playing: false,
            buffering: false,
            rate: 1.0,
            settle_until: now + SETTLE_MS,
        }
    }
}

/// The live screen, from tuning in until it closes.
struct Live {
    status: CouchTvStatus,
    signed_in: bool,
    stream: Option<Stream>,
    picture: Option<Picture>,
    reloads: u8,
    socket: Option<U53>,
    connection: CouchTvConnection,
    backoff: U53,
    me: Option<String>,
    presence: CouchTvPresence,
    reported_at: Option<U53>,
    refreshed_at: Option<U53>,
    /// What is on screen and the cues, as last rendered.
    shown: (Option<String>, CouchTvCues),
}

impl Live {
    fn new(signed_in: bool) -> Self {
        Self {
            status: CouchTvStatus::Tuning,
            signed_in,
            stream: None,
            picture: None,
            reloads: 0,
            socket: None,
            connection: CouchTvConnection::Offline,
            backoff: FIRST_BACKOFF_MS,
            me: None,
            presence: CouchTvPresence { count: 0, viewers: vec![] },
            reported_at: None,
            refreshed_at: None,
            shown: (None, CouchTvCues::default()),
        }
    }
}

/// A guest who joined through the channel link. A browser holds their token as the channel
/// cookie; a native client keeps it here.
struct Guest {
    name: String,
    token: Option<String>,
}

#[derive(Default)]
struct Guide {
    status: LoadStatus,
    view: Option<CouchTvGuide>,
    /// The server's clock when the guide was asked for: the days that ended before it go.
    asked: Option<i64>,
    problem: Option<Problem>,
}

#[derive(Default)]
struct OnNow {
    open: bool,
    status: LoadStatus,
    now: Option<Listing>,
    next: Option<Listing>,
    problem: Option<Problem>,
    due: Option<U53>,
}

#[derive(Default)]
pub struct CouchTv {
    /// Bumped when the live screen closes or the account changes: answers for a previous one
    /// are dropped.
    generation: u64,
    live: Option<Live>,
    guest: Option<Guest>,
    joining: bool,
    problem: Option<Problem>,
    /// The server's clock: its Unix milliseconds minus the shell's monotonic milliseconds.
    clock: Option<i64>,
    /// What is on and next, from the last tune-in, in order.
    lineup: Vec<Listing>,
    languages: Vec<String>,
    audio: Option<String>,
    subtitle: Option<String>,
    /// Lets a guest load artwork without an account.
    artwork_grant: Option<String>,
    reactions: VecDeque<CouchTvReaction>,
    next_reaction: U53,
    guide: Guide,
    on_now: OnNow,
}

impl CouchTv {
    fn pending(&self, request: Request) -> Pending {
        Pending::CouchTv(CouchTvPending { generation: self.generation, request })
    }

    /// Whether the channel has the shell's player: its reports come here.
    pub fn plays(&self) -> bool {
        self.live.as_ref().is_some_and(|l| l.picture.is_some())
    }

    /// The artwork grant of a guest without an account.
    pub fn artwork_grant(&self) -> Option<&str> {
        self.artwork_grant.as_deref()
    }

    /// Whether this module shows a surface.
    pub fn owns(surface: &Surface) -> bool {
        matches!(surface, Surface::CouchTv | Surface::CouchTvGuide | Surface::CouchTvOnNow)
    }

    /// A request with the guest's token where a browser sends the channel cookie.
    fn authorized(
        &self,
        env: &Env,
        request: &couchverse_api::Request,
    ) -> crate::messages::HttpRequest {
        let mut http = env.endpoint.request(request);
        if let Some(token) =
            self.guest.as_ref().and_then(|g| g.token.as_ref()).filter(|_| !env.signed_in)
        {
            http.headers.push(HttpHeader { name: TOKEN_HEADER.into(), value: token.clone() });
        }
        http
    }

    /// The live screen opened: tune in and play.
    pub fn tune(&mut self, ctx: &mut Ctx, env: &Env) {
        self.teardown(ctx);
        self.generation += 1;
        self.problem = None;
        self.live = Some(Live::new(env.signed_in));
        self.ask_tune(ctx, env, true);
        ctx.render(Surface::CouchTv);
    }

    fn ask_tune(&self, ctx: &mut Ctx, env: &Env, reload: bool) {
        let lang = Some(env.language.to_string());
        let call = if env.signed_in {
            ops::tune_couch_tv(&TuneCouchTvQuery { lang })
        } else {
            ops::tune_couch_tv_guest(&TuneCouchTvGuestQuery { lang })
        };
        let request = self.authorized(env, &call.request);
        ctx.http(request, self.pending(Request::Tune { call, asked_at: ctx.now, reload }));
    }

    /// The live screen went away: stop the player and leave the room.
    pub fn close(&mut self, ctx: &mut Ctx) {
        if self.live.is_none() {
            return;
        }
        self.teardown(ctx);
        self.generation += 1;
        ctx.render(Surface::CouchTv);
    }

    /// Account switch or sign-out: forget everything, the guest included.
    pub fn reset(&mut self, ctx: &mut Ctx) {
        self.teardown(ctx);
        if let Some(timer) = self.on_now.due.take() {
            ctx.cancel_timer(timer);
        }
        let on_now_open = self.on_now.open;
        *self = Self { generation: self.generation + 1, ..Self::default() };
        self.on_now.open = on_now_open;
        ctx.render(Surface::CouchTv);
        ctx.render(Surface::CouchTvGuide);
        ctx.render(Surface::CouchTvOnNow);
    }

    fn teardown(&mut self, ctx: &mut Ctx) {
        let Some(live) = self.live.take() else { return };
        if live.picture.is_some() {
            ctx.player(PlayerCommand::Stop);
        }
        if let Some(socket) = live.socket {
            ctx.socket_close(socket);
        }
        self.reactions.clear();
    }

    /// A viewer without an account joins through the channel link under the name they typed.
    pub fn join(&mut self, ctx: &mut Ctx, env: &Env, join: &CouchTvJoin) {
        self.joining = true;
        self.problem = None;
        let delivery = (!env.cookie).then_some(JoinCouchTvDelivery::Body);
        let call = ops::join_couch_tv(
            join.code.trim(),
            &JoinCouchTvQuery { delivery },
            &JoinBody { name: join.name.trim().to_string() },
        );
        ctx.http(env.endpoint.request(&call.request), self.pending(Request::Join(call)));
        ctx.render(Surface::CouchTv);
    }

    pub fn react(&mut self, ctx: &mut Ctx, emoji: &str) {
        let emoji = emoji.trim();
        if !emoji.is_empty() {
            self.send(ctx, &ClientFrame::Reaction(CouchTvReactionCommand { emoji: emoji.into() }));
        }
    }

    fn send(&self, ctx: &mut Ctx, frame: &ClientFrame) -> bool {
        let Some(socket) = self.live.as_ref().and_then(|l| l.socket) else { return false };
        let Ok(text) = serde_json::to_string(frame) else { return false };
        ctx.socket_send(socket, text);
        true
    }

    pub fn choose_audio(&mut self, ctx: &mut Ctx, id: Option<&str>) {
        let Some(lang) = id.filter(|l| self.languages.iter().any(|x| x == l)) else { return };
        self.audio = Some(lang.to_string());
        if self.plays() {
            ctx.player(PlayerCommand::SelectAudio(AudioRendition {
                lang: lang.into(),
                index: None,
            }));
        }
        ctx.render(Surface::CouchTv);
    }

    /// Subtitles stay as chosen across programs: they are one rendition of the stream.
    pub fn choose_subtitles(&mut self, ctx: &mut Ctx, id: Option<&str>) {
        let id = id.filter(|l| self.languages.iter().any(|x| x == l));
        self.subtitle = id.map(str::to_string);
        if self.plays() {
            ctx.player(PlayerCommand::SelectSubtitles(SubtitleSelection {
                id: self.subtitle.clone(),
            }));
        }
        ctx.render(Surface::CouchTv);
    }

    /// What the shell's player does: the hold, the room's reports and what is on screen.
    pub fn report(&mut self, ctx: &mut Ctx, env: &Env, report: &PlayerReport) -> CouchTvChange {
        let Some(live) = self.live.as_mut() else { return CouchTvChange::None };
        let Some(picture) = live.picture.as_mut() else { return CouchTvChange::None };
        if let Some(reason) = &report.failed {
            return self.player_failed(ctx, env, reason);
        }
        picture.position = report.position_seconds;
        picture.playing = report.playing;
        picture.buffering = report.buffering;
        if let Some(date) = report.program_date_ms.and_then(|d| i64::try_from(d).ok()) {
            picture.program_date = Some((date, ctx.now));
            live.reloads = 0;
            if live.status != CouchTvStatus::OnAir {
                live.status = CouchTvStatus::OnAir;
                ctx.render(Surface::CouchTv);
            }
        }
        self.hold(ctx);
        self.report_to_room(ctx);
        self.follow_picture(ctx, env);
        CouchTvChange::None
    }

    /// Keeps the picture's program date `latency` behind the server's clock.
    fn hold(&mut self, ctx: &mut Ctx) {
        let Some(clock) = self.clock else { return };
        let Some(live) = self.live.as_mut() else { return };
        let (Some(stream), Some(picture)) = (&live.stream, live.picture.as_mut()) else { return };
        let Some((date, at)) = picture.program_date else { return };
        if !picture.playing || picture.buffering || ctx.now < picture.settle_until || at != ctx.now
        {
            return;
        }
        let target = now_ms(ctx.now) + clock - stream.latency_ms;
        // positive when the picture runs ahead of everyone else's
        let drift = date - target;
        if drift.abs() > SEEK_DRIFT_MS {
            #[allow(clippy::cast_precision_loss)] // milliseconds of drift
            let seconds = (picture.position - drift as f64 / 1000.0).max(0.0);
            ctx.player(PlayerCommand::Seek(PlayerSeek { seconds }));
            picture.settle_until = ctx.now + SETTLE_MS;
            set_rate(ctx, picture, 1.0);
            return;
        }
        let rate = if drift.abs() < IN_STEP_MS {
            1.0
        } else {
            #[allow(clippy::cast_precision_loss)] // milliseconds of drift
            let change = (drift as f64 / RATE_DRIFT_MS).clamp(-MAX_RATE_CHANGE, MAX_RATE_CHANGE);
            ((1.0 - change) * 100.0).round() / 100.0
        };
        set_rate(ctx, picture, rate);
    }

    /// Tells the room what the picture shows, every `REPORT_EVERY_MS` while it plays.
    fn report_to_room(&mut self, ctx: &mut Ctx) {
        let Some(live) = self.live.as_ref() else { return };
        let Some(picture) = &live.picture else { return };
        let Some((date, _)) = picture.program_date.filter(|_| picture.playing) else { return };
        if live.connection != CouchTvConnection::Open
            || live.reported_at.is_some_and(|at| ctx.now < at + REPORT_EVERY_MS)
        {
            return;
        }
        if self.send(ctx, &ClientFrame::Report(CouchTvReport { program_date: date }))
            && let Some(live) = self.live.as_mut()
        {
            live.reported_at = Some(ctx.now);
        }
    }

    /// Renders when what is on screen or an overlay changes, and asks for more of the lineup
    /// when the picture runs past what is known.
    fn follow_picture(&mut self, ctx: &mut Ctx, env: &Env) {
        let Some(at) = self.picture_date() else { return };
        let (now, coming) = on_screen(&self.lineup, at);
        let shown = (now.as_ref().map(|n| n.api.id.clone()), cues(now.as_ref(), at));
        let exhausted = coming.is_empty();
        let Some(live) = self.live.as_mut() else { return };
        if live.shown != shown {
            live.shown = shown;
            ctx.render(Surface::CouchTv);
        }
        if exhausted && live.refreshed_at.is_none_or(|t| ctx.now >= t + LINEUP_REFRESH_MS) {
            live.refreshed_at = Some(ctx.now);
            self.ask_tune(ctx, env, false);
        }
    }

    /// The picture's program date when last reported.
    fn picture_date(&self) -> Option<i64> {
        self.live.as_ref()?.picture.as_ref()?.program_date.map(|(date, _)| date)
    }

    /// A player that fails (an expired grant, a link that ended, a stall it gave up on) gets a
    /// fresh tune-in and a reload, a few times in a row.
    fn player_failed(&mut self, ctx: &mut Ctx, env: &Env, reason: &str) -> CouchTvChange {
        let Some(live) = self.live.as_mut() else { return CouchTvChange::None };
        live.reloads += 1;
        if live.reloads > MAX_RELOADS {
            live.status = CouchTvStatus::Failed;
            live.picture = None;
            self.problem = Some(Problem::new("couchtv_playback_failed", reason));
            ctx.player(PlayerCommand::Stop);
            ctx.render(Surface::CouchTv);
            return CouchTvChange::None;
        }
        live.status = CouchTvStatus::Tuning;
        ctx.render(Surface::CouchTv);
        self.ask_tune(ctx, env, true);
        CouchTvChange::None
    }

    /// A guide or the home card appeared (or is pulled to refresh).
    pub fn opened(&mut self, ctx: &mut Ctx, env: Option<&Env>, surface: &Surface) {
        match surface {
            Surface::CouchTvGuide => {
                if let Some(env) = env {
                    self.ask_guide(ctx, env);
                }
            }
            Surface::CouchTvOnNow => {
                self.on_now.open = true;
                if let Some(env) = env {
                    self.ask_on_now(ctx, env);
                }
            }
            _ => {}
        }
    }

    pub fn closed(&mut self, ctx: &mut Ctx, surface: &Surface) {
        if *surface == Surface::CouchTvOnNow {
            self.on_now.open = false;
            if let Some(timer) = self.on_now.due.take() {
                ctx.cancel_timer(timer);
            }
        }
    }

    /// The display language changed: titles and overviews are read again.
    pub fn language_changed(&mut self, ctx: &mut Ctx, env: &Env) {
        if self.live.is_some() {
            self.ask_tune(ctx, env, false);
        }
        if self.guide.view.is_some() {
            self.ask_guide(ctx, env);
        }
        if self.on_now.open {
            self.ask_on_now(ctx, env);
        }
    }

    /// The server's clock now, else the shell's wall clock.
    fn server_now(&self, ctx: &Ctx) -> Option<i64> {
        self.clock
            .map(|c| now_ms(ctx.now) + c)
            .or_else(|| ctx.wall.and_then(|w| i64::try_from(w).ok()))
    }

    fn ask_guide(&mut self, ctx: &mut Ctx, env: &Env) {
        let now = self.server_now(ctx);
        let at = |offset: i64| now.and_then(|n| U53::try_from(n + offset).ok()).map(time::rfc3339);
        let query = GetCouchTvGuideQuery {
            lang: Some(env.language.to_string()),
            from: at(-GUIDE_BACK_MS),
            to: at(GUIDE_AHEAD_MS),
        };
        let call = ops::get_couch_tv_guide(&query);
        ctx.http(env.endpoint.request(&call.request), self.pending(Request::Guide(call)));
        self.guide.asked = now;
        self.guide.status =
            if self.guide.view.is_some() { LoadStatus::Stale } else { LoadStatus::Loading };
        ctx.render(Surface::CouchTvGuide);
    }

    fn ask_on_now(&mut self, ctx: &mut Ctx, env: &Env) {
        if let Some(timer) = self.on_now.due.take() {
            ctx.cancel_timer(timer);
        }
        let call = ops::tune_couch_tv(&TuneCouchTvQuery { lang: Some(env.language.to_string()) });
        let request = env.endpoint.request(&call.request);
        ctx.http(request, self.pending(Request::OnNow { call, asked_at: ctx.now }));
        if self.on_now.status != LoadStatus::Loaded {
            self.on_now.status = LoadStatus::Loading;
            ctx.render(Surface::CouchTvOnNow);
        }
    }

    /// Learns the server's clock from an answer stamped somewhere between asking and hearing.
    fn learn_clock(&mut self, server_time: i64, asked_at: U53, now: U53) {
        self.clock = Some(server_time - i64::midpoint(now_ms(asked_at), now_ms(now)));
    }

    pub fn resolve(
        &mut self,
        ctx: &mut Ctx,
        env: Option<&Env>,
        pending: CouchTvPending,
        output: EffectOutput,
    ) -> CouchTvChange {
        if pending.generation != self.generation {
            return CouchTvChange::None;
        }
        match pending.request {
            Request::Tune { call, asked_at, reload } => match decode(&call, output) {
                Ok(tune) => {
                    if let Some(env) = env {
                        self.tuned(ctx, env, &tune, asked_at, reload);
                    }
                    CouchTvChange::None
                }
                Err(failure) => self.tune_failed(ctx, &failure, reload),
            },
            Request::Join(call) => {
                self.joining = false;
                match decode(&call, output) {
                    Ok(guest) => self.guest = Some(Guest { name: guest.name, token: guest.token }),
                    Err(failure) => self.problem = Some(failure.problem()),
                }
                ctx.render(Surface::CouchTv);
                CouchTvChange::None
            }
            Request::Guide(call) => {
                match decode(&call, output) {
                    Ok(guide) => {
                        self.guide.status = LoadStatus::Loaded;
                        self.guide.view = Some(guide);
                        self.guide.problem = None;
                    }
                    Err(failure) => {
                        self.guide.status = if self.guide.view.is_some() {
                            LoadStatus::Stale
                        } else {
                            LoadStatus::Failed
                        };
                        self.guide.problem = Some(failure.problem());
                    }
                }
                ctx.render(Surface::CouchTvGuide);
                CouchTvChange::None
            }
            Request::OnNow { call, asked_at } => {
                self.on_now_answered(ctx, decode(&call, output), asked_at)
            }
            Request::OnNowDue => {
                self.on_now.due = None;
                if let Some(env) = env.filter(|_| self.on_now.open) {
                    self.ask_on_now(ctx, env);
                }
                CouchTvChange::None
            }
            Request::Socket => {
                self.socket_output(ctx, env, output);
                CouchTvChange::None
            }
            Request::Reconnect => {
                // the tune-in says whether the channel (and a guest's link) still works first
                if let Some(env) = env.filter(|_| self.live.is_some()) {
                    self.ask_tune(ctx, env, false);
                }
                CouchTvChange::None
            }
            Request::ReactionGone(id) => {
                self.reactions.retain(|r| r.id != id);
                ctx.render(Surface::CouchTv);
                CouchTvChange::None
            }
        }
    }

    fn tuned(&mut self, ctx: &mut Ctx, env: &Env, tune: &CouchTvTune, asked_at: U53, reload: bool) {
        self.learn_clock(tune.server_time, asked_at, ctx.now);
        self.lineup = lineup(tune);
        if self.on_now.open {
            self.on_now_from(ctx, tune);
        }
        self.languages.clone_from(&tune.languages);
        if self.audio.as_ref().is_none_or(|a| !self.languages.contains(a)) {
            self.audio = self
                .languages
                .iter()
                .find(|l| env.language.starts_with(l.as_str()))
                .or(self.languages.first())
                .cloned();
        }
        if tune.artwork_grant.is_some() {
            self.artwork_grant.clone_from(&tune.artwork_grant);
        }
        if let (Some(guest), Some(known)) = (&tune.guest, self.guest.as_mut()) {
            known.name.clone_from(&guest.name);
        } else if let Some(guest) = &tune.guest {
            // a browser that joined earlier: the channel cookie remembered the guest
            self.guest = Some(Guest { name: guest.name.clone(), token: None });
        }
        #[allow(clippy::cast_precision_loss)] // a handful of seconds
        let target_seconds = tune.target_duration as f64;
        #[allow(clippy::cast_possible_truncation)] // milliseconds of latency
        let stream = Stream {
            url: env.endpoint.absolute(&tune.stream_url),
            latency_ms: (LATENCY_TARGET_DURATIONS * target_seconds * 1000.0) as i64,
            target_seconds,
        };
        let title = self.lineup.first().and_then(|l| l.api.title_name.clone());
        let load = (reload).then(|| self.player_load(&stream, title));
        let Some(live) = self.live.as_mut() else { return };
        if let Some(load) = load {
            ctx.player(PlayerCommand::Load(load));
            live.picture = Some(Picture::loaded(ctx.now));
            live.status =
                if tune.now.is_some() { CouchTvStatus::Tuning } else { CouchTvStatus::OffAir };
        }
        live.stream = Some(stream);
        if live.socket.is_none() {
            self.open_socket(ctx, env);
        }
        ctx.render(Surface::CouchTv);
    }

    fn player_load(&self, stream: &Stream, title: Option<String>) -> PlayerLoad {
        PlayerLoad {
            url: stream.url.clone(),
            source: PlayerSource::Hls,
            start_seconds: 0.0,
            autoplay: true,
            max_height: None,
            subtitles: self
                .languages
                .iter()
                .map(|l| PlayerSubtitle {
                    id: l.clone(),
                    lang: l.clone(),
                    label: l.clone(),
                    url: None,
                    forced: false,
                })
                .collect(),
            subtitle: self.subtitle.clone(),
            audio_lang: self.audio.clone(),
            audio_index: None,
            linear: true,
            now_playing: NowPlaying {
                // the channel's name is a brand, the same in every language
                title: title.clone().unwrap_or_else(|| "Couch TV".into()),
                subtitle: title.map(|_| "Couch TV".into()),
                artwork: None,
                duration_seconds: 0.0,
            },
            live: Some(LiveStream {
                #[allow(clippy::cast_precision_loss)] // milliseconds of latency
                latency_seconds: stream.latency_ms as f64 / 1000.0,
                target_duration_seconds: stream.target_seconds,
            }),
        }
    }

    fn tune_failed(&mut self, ctx: &mut Ctx, failure: &Failure, reload: bool) -> CouchTvChange {
        let Some(live) = self.live.as_ref() else { return CouchTvChange::None };
        let signed_in = live.signed_in;
        let gone = matches!(failure, Failure::Api(e) if matches!(e.status, 401 | 403 | 404));
        // a refresh that fails for a while (offline, a restart) tries again later while the
        // socket is down; the picture plays on meanwhile
        if !reload && !gone {
            if live.socket.is_none() {
                self.retry_later(ctx);
            }
            return CouchTvChange::None;
        }
        if signed_in && failure.unauthorized() {
            return CouchTvChange::Unauthorized;
        }
        if failure.unauthorized() {
            // the channel link was rotated or turned off
            self.guest = None;
        }
        self.problem = Some(failure.problem());
        self.teardown(ctx);
        let mut failed = Live::new(signed_in);
        failed.status = CouchTvStatus::Failed;
        self.live = Some(failed);
        ctx.render(Surface::CouchTv);
        CouchTvChange::None
    }

    fn on_now_answered(
        &mut self,
        ctx: &mut Ctx,
        result: Result<CouchTvTune, Failure>,
        asked_at: U53,
    ) -> CouchTvChange {
        match result {
            Ok(tune) => {
                self.learn_clock(tune.server_time, asked_at, ctx.now);
                self.on_now_from(ctx, &tune);
                CouchTvChange::None
            }
            Err(failure) => {
                self.on_now.status = LoadStatus::Failed;
                self.on_now.problem = Some(failure.problem());
                ctx.render(Surface::CouchTvOnNow);
                if failure.unauthorized() {
                    CouchTvChange::Unauthorized
                } else {
                    CouchTvChange::None
                }
            }
        }
    }

    /// The home card from a tune-in, asked again when what is on ends.
    fn on_now_from(&mut self, ctx: &mut Ctx, tune: &CouchTvTune) {
        if let Some(timer) = self.on_now.due.take() {
            ctx.cancel_timer(timer);
        }
        let listings = lineup(tune);
        let now = tune.now.as_ref().and_then(|_| listings.first().cloned());
        let next = listings
            .iter()
            .find(|l| l.program() && now.as_ref().is_none_or(|n| l.starts >= n.ends))
            .cloned();
        self.on_now = OnNow {
            open: self.on_now.open,
            status: LoadStatus::Loaded,
            now,
            next,
            problem: None,
            due: None,
        };
        let ends = self.on_now.now.as_ref().map(|n| n.ends).filter(|_| self.on_now.open);
        if let (Some(ends), Some(server_now)) = (ends, self.server_now(ctx)) {
            let wait = U53::try_from((ends - server_now).max(ON_NOW_MIN_MS)).unwrap_or_default();
            self.on_now.due = Some(ctx.after(wait, self.pending(Request::OnNowDue)));
        }
        ctx.render(Surface::CouchTvOnNow);
    }

    fn open_socket(&mut self, ctx: &mut Ctx, env: &Env) {
        let pending = self.pending(Request::Socket);
        let header = self
            .guest
            .as_ref()
            .and_then(|g| g.token.clone())
            .filter(|_| !env.signed_in)
            .map(|value| HttpHeader { name: TOKEN_HEADER.into(), value });
        let Some(live) = self.live.as_mut() else { return };
        let path = ops::couch_tv_socket().path_and_query();
        // the web resolves a path against the page; native clients get the server's address
        let url = format!("{}/api/v1{path}", socket_base(&env.endpoint.base));
        live.socket = Some(ctx.socket_open(url, header.into_iter().collect(), pending));
        live.connection = CouchTvConnection::Connecting;
    }

    fn socket_output(&mut self, ctx: &mut Ctx, env: Option<&Env>, output: EffectOutput) {
        let Some(live) = self.live.as_mut() else { return };
        match output {
            EffectOutput::SocketOpened => {
                live.connection = CouchTvConnection::Open;
                live.backoff = FIRST_BACKOFF_MS;
                live.reported_at = None;
                ctx.render(Surface::CouchTv);
            }
            EffectOutput::SocketText(text) => {
                if let Ok(frame) = serde_json::from_str::<ServerFrame>(&text.text) {
                    self.frame(ctx, env, frame);
                }
            }
            EffectOutput::SocketClosed(_) => {
                live.socket = None;
                live.connection = CouchTvConnection::Reconnecting;
                self.retry_later(ctx);
                ctx.render(Surface::CouchTv);
            }
            _ => {}
        }
    }

    /// Schedules the next reconnect, backing off up to `MAX_BACKOFF_MS`.
    fn retry_later(&mut self, ctx: &mut Ctx) {
        let reconnect = self.pending(Request::Reconnect);
        let Some(live) = self.live.as_mut() else { return };
        ctx.after(live.backoff, reconnect);
        live.backoff = (live.backoff * 2).min(MAX_BACKOFF_MS);
    }

    fn frame(&mut self, ctx: &mut Ctx, env: Option<&Env>, frame: ServerFrame) {
        match frame {
            ServerFrame::Hello(hello) => {
                // the socket's clock is the freshest: its frame was made as it opened
                self.clock = Some(hello.server_time - now_ms(ctx.now));
                if let Some(live) = self.live.as_mut() {
                    live.me = Some(hello.me.id);
                    live.presence = hello.presence;
                }
            }
            ServerFrame::Presence(presence) => {
                if let Some(live) = self.live.as_mut() {
                    live.presence = presence;
                }
            }
            ServerFrame::Reaction(reaction) => {
                self.next_reaction += 1;
                let id = self.next_reaction;
                self.reactions.push_back(CouchTvReaction {
                    id,
                    viewer_id: reaction.viewer_id,
                    emoji: reaction.emoji,
                });
                ctx.after(REACTION_MS, self.pending(Request::ReactionGone(id)));
            }
            ServerFrame::ScheduleChanged => {
                if let Some(env) = env {
                    self.ask_tune(ctx, env, false);
                    if self.guide.view.is_some() {
                        self.ask_guide(ctx, env);
                    }
                    if self.on_now.open {
                        self.ask_on_now(ctx, env);
                    }
                }
            }
            ServerFrame::Unknown => return,
        }
        ctx.render(Surface::CouchTv);
    }

    pub fn view(&self, images: &Images) -> CouchTvView {
        let tracks = |selected: Option<&String>| -> Vec<CouchTvTrack> {
            self.languages
                .iter()
                .map(|l| CouchTvTrack {
                    id: l.clone(),
                    lang: l.clone(),
                    selected: selected == Some(l),
                })
                .collect()
        };
        let guest = self.guest.as_ref().map(|g| g.name.clone());
        let Some(live) = &self.live else {
            return CouchTvView {
                status: if self.problem.is_some() {
                    CouchTvStatus::Failed
                } else {
                    CouchTvStatus::Idle
                },
                connection: CouchTvConnection::Offline,
                now: None,
                next: None,
                later: vec![],
                program_date_ms: None,
                program_date_at_ms: 0,
                cues: CouchTvCues::default(),
                viewers: 0,
                members: vec![],
                reactions: vec![],
                audio: tracks(self.audio.as_ref()),
                subtitles: tracks(self.subtitle.as_ref()),
                guest,
                joining: self.joining,
                problem: self.problem.clone(),
            };
        };
        let date = live.picture.as_ref().and_then(|p| p.program_date);
        let (now, coming) = match date {
            Some((at, _)) => on_screen(&self.lineup, at),
            // before the picture shows, what the server said is on
            None => (
                self.lineup.first().cloned(),
                self.lineup.iter().skip(1).filter(|l| l.program()).collect(),
            ),
        };
        let signed_in = live.signed_in;
        CouchTvView {
            status: live.status,
            connection: live.connection,
            cues: date.map_or_else(CouchTvCues::default, |(at, _)| cues(now.as_ref(), at)),
            now: now.as_ref().map(|n| n.view(images, signed_in)),
            next: coming.first().map(|n| n.view(images, signed_in)),
            later: coming.iter().skip(1).map(|l| l.view(images, signed_in)).collect(),
            program_date_ms: date.and_then(|(at, _)| U53::try_from(at).ok()),
            program_date_at_ms: date.map_or(0, |(_, seen)| seen),
            viewers: u32::try_from(live.presence.count).unwrap_or_default(),
            members: live
                .presence
                .viewers
                .iter()
                .map(|v| CouchTvMember {
                    id: v.id.clone(),
                    display_name: v.display_name.clone(),
                    avatar: v
                        .avatar_id
                        .as_ref()
                        .map(|id| images.image(id, None, Size::Small, None)),
                    seed: v.seed.clone(),
                    guest: v.guest,
                    me: live.me.as_ref() == Some(&v.id),
                })
                .collect(),
            reactions: self.reactions.iter().cloned().collect(),
            audio: tracks(self.audio.as_ref()),
            subtitles: tracks(self.subtitle.as_ref()),
            guest: guest.filter(|_| !signed_in),
            joining: self.joining,
            problem: self.problem.clone(),
        }
    }

    pub fn guide_view(&self, images: &Images) -> CouchTvGuideView {
        let Some(guide) = &self.guide.view else {
            return CouchTvGuideView {
                status: self.guide.status,
                days: vec![],
                time_zone: String::new(),
                problem: self.guide.problem.clone(),
            };
        };
        let programs: Vec<Listing> =
            guide.programs.iter().cloned().filter_map(Listing::new).collect();
        let days = guide
            .days
            .iter()
            .filter_map(|day| {
                let starts = time::unix_ms(&day.starts_at)?;
                let ends = time::unix_ms(&day.ends_at)?;
                // the stretch asked for starts a day back, before today's broadcast day
                if self.guide.asked.is_some_and(|asked| ends <= asked) {
                    return None;
                }
                Some(CouchTvGuideDay {
                    date: day.date.clone(),
                    starts_at_ms: U53::try_from(starts).ok()?,
                    ends_at_ms: U53::try_from(ends).ok()?,
                    programs: programs
                        .iter()
                        .filter(|p| p.starts < ends && p.ends > starts)
                        .map(|p| p.view(images, true))
                        .collect(),
                })
            })
            .collect();
        CouchTvGuideView {
            status: self.guide.status,
            days,
            time_zone: guide.time_zone.clone(),
            problem: self.guide.problem.clone(),
        }
    }

    pub fn on_now_view(&self, images: &Images) -> CouchTvOnNowView {
        CouchTvOnNowView {
            status: self.on_now.status,
            now: self.on_now.now.as_ref().map(|n| n.view(images, true)),
            next: self.on_now.next.as_ref().map(|n| n.view(images, true)),
            problem: self.on_now.problem.clone(),
        }
    }
}

fn set_rate(ctx: &mut Ctx, picture: &mut Picture, rate: f64) {
    if (rate - picture.rate).abs() > f64::EPSILON {
        picture.rate = rate;
        ctx.player(PlayerCommand::Rate(PlayerRate { rate }));
    }
}

/// The shell's monotonic milliseconds as a signed number, to subtract from.
fn now_ms(now: U53) -> i64 {
    i64::try_from(now).unwrap_or(i64::MAX)
}

/// What is on, next and later from a tune-in, in order.
fn lineup(tune: &CouchTvTune) -> Vec<Listing> {
    let on_air = [&tune.now, &tune.next].into_iter().flatten().filter_map(|l| {
        // the API's "on air" listing is the same JSON as a listing
        serde_json::to_value(l).ok().and_then(|v| serde_json::from_value::<CouchTvListing>(v).ok())
    });
    on_air.chain(tune.later.iter().cloned()).filter_map(Listing::new).collect()
}
