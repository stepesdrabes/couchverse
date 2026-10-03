//! Couch sessions (synced watch parties) from a client's side: hosting, joining by code, the
//! session socket with reconnect and backoff, participants and reactions, the host broadcasting
//! its play state, a follower kept within a few seconds of the host, and the host's other
//! devices steering its player as remotes.

use std::collections::VecDeque;

use couchverse_api::couch::{
    ClientFrame, CouchEmojiCommand, CouchHostStateCommand, CouchPausedCommand, CouchRemoteCommand,
    CouchRemoteCommandAction, ServerFrame,
};
use couchverse_api::ops::{
    CreateCouchQuery, GetCouchPlaybackQuery, JoinCouchQuery, ResolveCouchPlaybackQuery,
};
use couchverse_api::types::{
    CouchHostState, CouchMediaRef, CouchMediaRefKind, CouchParticipant, CouchPlayback,
    CouchSession, CouchSessionRole, CouchStart, CouchStartKind, CreateCouchDelivery,
    JoinCouchDelivery,
};
use couchverse_api::{Call, ops};
use serde::{Deserialize, Serialize};
use typeshare::typeshare;

use crate::api::{Endpoint, Failure, decode};
use crate::core::Pending;
use crate::effects::Ctx;
use crate::messages::{
    EffectOutput, HttpHeader, LoadStatus, PlayerCommand, PlayerSeek, Problem, Surface, U53,
};
use crate::modules::catalog::{PlayKind, PlayTarget};
use crate::modules::images::{Image, Images, Size};
use crate::modules::playback::Playback;

/// A follower further than this from the host is snapped back to it.
const DRIFT_SECONDS: f64 = 3.0;
/// The host repeats its play state this often, so a follower's estimate never wanders far.
const HEARTBEAT_MS: U53 = 2_000;
const FIRST_BACKOFF_MS: U53 = 500;
const MAX_BACKOFF_MS: U53 = 8_000;
/// How long a reaction floats on screen, matching the shells' rise-and-fade animation.
const REACTION_MS: U53 = 2_200;
/// How long "resynced" stays up after a follower is snapped back.
const RESYNC_NOTE_MS: U53 = 2_500;
const RECENT_EMOJIS: usize = 3;
const RECENT_KEY: &str = "couch.recentEmojis";
/// Where the participant token travels for clients without a cookie jar.
const TOKEN_HEADER: &str = "X-Couch-Token";

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CouchCode {
    /// The six-digit share code.
    pub code: String,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CouchReaction {
    pub emoji: String,
}

#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CouchPause {
    pub paused: bool,
}

#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum RemoteAction {
    Play,
    Pause,
    Seek,
    Next,
    Previous,
}

#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RemoteControl {
    pub action: RemoteAction,
    /// Where to seek; only for `seek`.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub position_seconds: Option<f64>,
}

#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum CouchRole {
    Host,
    Follower,
    /// The host's account on another device, steering the host's player.
    Remote,
}

#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum CouchStatus {
    #[default]
    Idle,
    /// Creating or joining, or opening the socket for the first time.
    Connecting,
    Open,
    /// The socket dropped; the core retries with backoff.
    Reconnecting,
    /// The session is over (`CouchView.ended` says why).
    Ended,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
#[allow(clippy::struct_excessive_bools)] // a view model mirrors the flags a screen shows
pub struct CouchMember {
    pub id: String,
    pub display_name: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub avatar: Option<Image>,
    /// Seeds a generated avatar for members without one.
    pub seed: String,
    pub host: bool,
    pub anonymous: bool,
    /// Paused their own playback.
    pub paused: bool,
    pub me: bool,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Reaction {
    pub id: U53,
    pub participant_id: String,
    pub emoji: String,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
#[allow(clippy::struct_excessive_bools)] // a view model mirrors the flags a screen shows
pub struct CouchView {
    pub status: CouchStatus,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub role: Option<CouchRole>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub code: Option<String>,
    /// The public join page, for the QR a host shows.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub share_url: Option<String>,
    pub members: Vec<CouchMember>,
    /// What the host is watching; absent while they choose.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub media: Option<PlayTarget>,
    pub playing: bool,
    /// The host's position when last heard, at `positionAtMs` on the shell's clock: a remote's
    /// scrubber extrapolates from it while `playing`.
    pub position_seconds: f64,
    pub position_at_ms: U53,
    pub host_away: bool,
    /// A follower waits: the host is choosing what to watch, or away.
    pub waiting: bool,
    pub local_paused: bool,
    pub reactions: Vec<Reaction>,
    pub recent_emojis: Vec<String>,
    /// Briefly true after a follower was snapped back to the host.
    pub resynced: bool,
    /// Why the session ended: `host_ended`, `host_left`, `host_timeout`, `idle`,
    /// `server_shutdown` or `left` (this device left).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub ended: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub problem: Option<Problem>,
}

/// Everything the module needs from the active session.
pub struct Env<'a> {
    pub endpoint: &'a Endpoint,
    pub language: &'a str,
    /// The web: the couch cookie authenticates, and URLs are relative to `origin`.
    pub cookie: bool,
    pub origin: &'a str,
    /// What this device can play, for a follower's payload.
    pub profile: Option<&'a couchverse_api::types::DeviceProfile>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct CouchPending {
    generation: u64,
    request: Request,
}

#[derive(Debug, Clone, PartialEq)]
enum Request {
    Session(Call<CouchSession>),
    Player(Call<CouchPlayback>),
    /// The socket: resolved on open, for every frame and once when it closes.
    Socket,
    Reconnect,
    Heartbeat,
    ReactionGone(U53),
    ResyncNoteGone,
    Recent,
    Ignored,
}

/// What the rest of the core should do after a couch output.
#[derive(Debug, PartialEq)]
pub enum CouchChange {
    None,
    Unauthorized,
    /// A follower plays this payload, locked to the host's timeline.
    Follow(PlayTarget, Box<couchverse_api::types::PlaybackInfo>),
    /// The host's playing device was asked to change track.
    Next,
    Previous,
    /// The follower's session ended: close its player.
    StopFollowing,
}

struct Live {
    code: String,
    role: CouchRole,
    token: Option<String>,
    me: String,
    participants: Vec<CouchParticipant>,
    state: CouchHostState,
    /// When `state` arrived, on the shell's clock.
    received_at: U53,
    socket: Option<U53>,
    status: CouchStatus,
    backoff: U53,
    heartbeat: Option<U53>,
    local_paused: bool,
    resync_timer: Option<U53>,
}

#[derive(Default)]
pub struct Couch {
    generation: u64,
    live: Option<Live>,
    joining: bool,
    /// The recent emojis are read once, when the first session of a launch starts.
    recent_read: bool,
    ended: Option<String>,
    problem: Option<Problem>,
    reactions: VecDeque<Reaction>,
    next_reaction: U53,
    recent: Vec<String>,
}

impl Couch {
    pub fn is_follower(&self) -> bool {
        self.live.as_ref().is_some_and(|l| l.role == CouchRole::Follower)
    }

    pub fn is_host(&self) -> bool {
        self.live.as_ref().is_some_and(|l| l.role == CouchRole::Host)
    }

    fn pending(&self, request: Request) -> Pending {
        Pending::Couch(CouchPending { generation: self.generation, request })
    }

    fn begin(&mut self, ctx: &mut Ctx) {
        if !self.recent_read {
            self.recent_read = true;
            ctx.store_read(RECENT_KEY, self.pending(Request::Recent));
        }
        self.teardown(ctx);
        self.generation += 1;
        self.joining = true;
        self.ended = None;
        self.problem = None;
        ctx.render(Surface::Couch);
    }

    /// Hosts a session around what is playing.
    pub fn start(&mut self, ctx: &mut Ctx, env: &Env, target: &PlayTarget) {
        self.begin(ctx);
        let kind = match target.kind {
            PlayKind::Movie => CouchStartKind::Movie,
            PlayKind::Episode => CouchStartKind::Episode,
        };
        let delivery = (!env.cookie).then_some(CreateCouchDelivery::Body);
        let call = ops::create_couch(
            &CreateCouchQuery { delivery },
            &CouchStart { id: target.id.clone(), kind },
        );
        ctx.http(env.endpoint.request(&call.request), self.pending(Request::Session(call)));
    }

    /// Joins by code, as a follower or as a remote for the host's own player.
    pub fn join(&mut self, ctx: &mut Ctx, env: &Env, code: &str, remote: bool) {
        self.begin(ctx);
        let delivery = (!env.cookie).then_some(JoinCouchDelivery::Body);
        let query = JoinCouchQuery { delivery, remote: remote.then_some(true) };
        let call = ops::join_couch(code.trim(), &query);
        ctx.http(env.endpoint.request(&call.request), self.pending(Request::Session(call)));
    }

    /// Leaves the session; a host leaving ends it for everyone.
    pub fn leave(&mut self, ctx: &mut Ctx, env: &Env, end: bool) -> CouchChange {
        let Some(live) = &self.live else { return CouchChange::None };
        let call = if end { ops::end_couch(&live.code) } else { ops::leave_couch(&live.code) };
        let request = Self::authorized(env, &call.request, live.token.as_deref());
        ctx.http(request, self.pending(Request::Ignored));
        let follower = live.role == CouchRole::Follower;
        self.teardown(ctx);
        self.generation += 1;
        self.ended = Some(if end { "host_ended".into() } else { "left".into() });
        ctx.render(Surface::Couch);
        if follower { CouchChange::StopFollowing } else { CouchChange::None }
    }

    /// Account switch or sign-out: forget the session without telling the server.
    pub fn reset(&mut self, ctx: &mut Ctx) {
        self.teardown(ctx);
        self.generation += 1;
        self.ended = None;
        self.joining = false;
        ctx.render(Surface::Couch);
    }

    fn teardown(&mut self, ctx: &mut Ctx) {
        if let Some(live) = self.live.take() {
            if let Some(socket) = live.socket {
                ctx.socket_close(socket);
            }
            for timer in [live.heartbeat, live.resync_timer].into_iter().flatten() {
                ctx.cancel_timer(timer);
            }
        }
        self.reactions.clear();
    }

    /// A request carrying the participant token where the cookie would be.
    fn authorized(
        env: &Env,
        request: &couchverse_api::Request,
        token: Option<&str>,
    ) -> crate::messages::HttpRequest {
        let mut http = env.endpoint.request(request);
        if let Some(token) = token {
            http.headers.push(HttpHeader { name: TOKEN_HEADER.into(), value: token.to_string() });
        }
        http
    }

    pub fn send_emoji(&mut self, ctx: &mut Ctx, emoji: &str) {
        let emoji = emoji.trim();
        if emoji.is_empty()
            || !self.send(ctx, &ClientFrame::Emoji(CouchEmojiCommand { emoji: emoji.to_string() }))
        {
            return;
        }
        self.recent.retain(|e| e != emoji);
        self.recent.insert(0, emoji.to_string());
        self.recent.truncate(RECENT_EMOJIS);
        if let Ok(json) = serde_json::to_string(&self.recent) {
            ctx.store_write(RECENT_KEY, json);
        }
        ctx.render(Surface::Couch);
    }

    /// A follower paused or resumed their own playback; resuming jumps back to the host.
    pub fn local_pause(&mut self, ctx: &mut Ctx, playback: &Playback, paused: bool) {
        let Some(live) = self.live.as_mut().filter(|l| l.role == CouchRole::Follower) else {
            return;
        };
        if live.local_paused == paused {
            return;
        }
        live.local_paused = paused;
        self.send(ctx, &ClientFrame::Paused(CouchPausedCommand { paused }));
        if paused {
            playback.command(ctx, PlayerCommand::Pause);
        } else {
            self.resync(ctx, playback, true);
        }
        ctx.render(Surface::Couch);
    }

    pub fn remote(&mut self, ctx: &mut Ctx, control: RemoteControl) {
        if !self.live.as_ref().is_some_and(|l| l.role == CouchRole::Remote) {
            return;
        }
        let action = match control.action {
            RemoteAction::Play => CouchRemoteCommandAction::Play,
            RemoteAction::Pause => CouchRemoteCommandAction::Pause,
            RemoteAction::Seek => CouchRemoteCommandAction::Seek,
            RemoteAction::Next => CouchRemoteCommandAction::Next,
            RemoteAction::Previous => CouchRemoteCommandAction::Previous,
        };
        let position_seconds =
            control.position_seconds.filter(|_| control.action == RemoteAction::Seek);
        self.send(
            ctx,
            &ClientFrame::RemoteCommand(CouchRemoteCommand { action, position_seconds }),
        );
    }

    /// The host's player changed: followers hear about it at once rather than at the next
    /// heartbeat.
    pub fn host_moved(&mut self, ctx: &mut Ctx, playback: &Playback) {
        if self.is_host() {
            self.broadcast(ctx, playback);
        }
    }

    fn broadcast(&mut self, ctx: &mut Ctx, playback: &Playback) {
        let media = match playback.target() {
            Some(PlayTarget { kind: PlayKind::Movie, id }) => CouchMediaRef {
                kind: CouchMediaRefKind::Movie,
                title_id: Some(id.clone()),
                episode_id: None,
            },
            Some(PlayTarget { kind: PlayKind::Episode, id }) => CouchMediaRef {
                kind: CouchMediaRefKind::Episode,
                title_id: None,
                episode_id: Some(id.clone()),
            },
            // the host is choosing what to watch
            None => {
                CouchMediaRef { kind: CouchMediaRefKind::Empty, title_id: None, episode_id: None }
            }
        };
        let (position_seconds, playing) = playback.position_at(ctx.now).unwrap_or((0.0, false));
        self.send(
            ctx,
            &ClientFrame::HostState(CouchHostStateCommand { media, playing, position_seconds }),
        );
    }

    fn send(&self, ctx: &mut Ctx, frame: &ClientFrame) -> bool {
        let Some(socket) = self.live.as_ref().and_then(|l| l.socket) else { return false };
        let Ok(text) = serde_json::to_string(frame) else { return false };
        ctx.socket_send(socket, text);
        true
    }

    pub fn resolve(
        &mut self,
        ctx: &mut Ctx,
        env: Option<&Env>,
        playback: &Playback,
        pending: CouchPending,
        output: EffectOutput,
    ) -> CouchChange {
        match pending.request {
            Request::Recent => {
                if let EffectOutput::Stored(stored) = output
                    && let Some(recent) = stored.value.and_then(|j| serde_json::from_str(&j).ok())
                {
                    self.recent = recent;
                    ctx.render(Surface::Couch);
                }
                CouchChange::None
            }
            Request::Ignored => CouchChange::None,
            _ if pending.generation != self.generation => CouchChange::None,
            Request::Session(call) => match decode(&call, output) {
                Ok(session) => self.joined(ctx, env, session),
                Err(failure) => self.failed(ctx, &failure),
            },
            Request::Player(call) => match decode(&call, output) {
                Ok(CouchPlayback { player: Some(info), media }) => match target(&media) {
                    Some(target) => CouchChange::Follow(target, Box::new(info)),
                    None => CouchChange::StopFollowing,
                },
                // the host is choosing: nothing to play yet
                Ok(CouchPlayback { player: None, .. }) => CouchChange::StopFollowing,
                Err(failure) => self.failed(ctx, &failure),
            },
            Request::Socket => self.socket_output(ctx, env, playback, output),
            Request::Reconnect => {
                if let Some(env) = env {
                    self.open_socket(ctx, env);
                }
                CouchChange::None
            }
            Request::Heartbeat => {
                self.broadcast(ctx, playback);
                CouchChange::None
            }
            Request::ReactionGone(id) => {
                self.reactions.retain(|r| r.id != id);
                ctx.render(Surface::Couch);
                CouchChange::None
            }
            Request::ResyncNoteGone => {
                if let Some(live) = self.live.as_mut() {
                    live.resync_timer = None;
                    ctx.render(Surface::Couch);
                }
                CouchChange::None
            }
        }
    }

    fn failed(&mut self, ctx: &mut Ctx, failure: &Failure) -> CouchChange {
        self.joining = false;
        self.problem = Some(failure.problem());
        ctx.render(Surface::Couch);
        // a dead couch cookie is not a dead account: only a 401 on joining one signs out
        if failure.unauthorized() && self.live.is_none() {
            CouchChange::Unauthorized
        } else {
            CouchChange::None
        }
    }

    fn joined(&mut self, ctx: &mut Ctx, env: Option<&Env>, session: CouchSession) -> CouchChange {
        let role = match session.role {
            CouchSessionRole::Host => CouchRole::Host,
            CouchSessionRole::Remote => CouchRole::Remote,
            _ => CouchRole::Follower,
        };
        self.joining = false;
        self.live = Some(Live {
            code: session.share_token,
            role,
            token: session.participant_token,
            me: session.my_participant_id,
            participants: session.participants,
            state: session.state,
            received_at: ctx.now,
            socket: None,
            status: CouchStatus::Connecting,
            backoff: FIRST_BACKOFF_MS,
            heartbeat: None,
            local_paused: false,
            resync_timer: None,
        });
        ctx.render(Surface::Couch);
        let Some(env) = env else { return CouchChange::None };
        self.open_socket(ctx, env);
        if role == CouchRole::Follower {
            self.fetch_player(ctx, env);
        }
        CouchChange::None
    }

    fn fetch_player(&mut self, ctx: &mut Ctx, env: &Env) {
        let Some(live) = &self.live else { return };
        let lang = Some(env.language.to_string());
        let call = match env.profile {
            Some(profile) => ops::resolve_couch_playback(
                &live.code,
                &ResolveCouchPlaybackQuery { lang },
                profile,
            ),
            None => {
                ops::get_couch_playback(&live.code, &GetCouchPlaybackQuery { lang, caps: None })
            }
        };
        let request = Self::authorized(env, &call.request, live.token.as_deref());
        ctx.http(request, self.pending(Request::Player(call)));
    }

    fn open_socket(&mut self, ctx: &mut Ctx, env: &Env) {
        let pending = self.pending(Request::Socket);
        let Some(live) = self.live.as_mut() else { return };
        let path = ops::couch_socket(&live.code).path_and_query();
        // the web resolves a path against the page; native clients get the server's address
        let url = format!("{}/api/v1{path}", socket_base(&env.endpoint.base));
        let headers = live
            .token
            .as_ref()
            .map(|t| vec![HttpHeader { name: TOKEN_HEADER.into(), value: t.clone() }])
            .unwrap_or_default();
        live.socket = Some(ctx.socket_open(url, headers, pending));
    }

    fn socket_output(
        &mut self,
        ctx: &mut Ctx,
        env: Option<&Env>,
        playback: &Playback,
        output: EffectOutput,
    ) -> CouchChange {
        match output {
            EffectOutput::SocketOpened => {
                let heartbeat = self.pending(Request::Heartbeat);
                let Some(live) = self.live.as_mut() else { return CouchChange::None };
                live.status = CouchStatus::Open;
                live.backoff = FIRST_BACKOFF_MS;
                if live.role == CouchRole::Host && live.heartbeat.is_none() {
                    live.heartbeat = Some(ctx.every(HEARTBEAT_MS, heartbeat));
                }
                ctx.render(Surface::Couch);
                CouchChange::None
            }
            EffectOutput::SocketText(text) => match serde_json::from_str::<ServerFrame>(&text.text)
            {
                Ok(frame) => self.frame(ctx, env, playback, frame),
                Err(_) => CouchChange::None,
            },
            EffectOutput::SocketClosed(_) => {
                let reconnect = self.pending(Request::Reconnect);
                let Some(live) = self.live.as_mut() else { return CouchChange::None };
                live.socket = None;
                live.status = CouchStatus::Reconnecting;
                ctx.after(live.backoff, reconnect);
                live.backoff = (live.backoff * 2).min(MAX_BACKOFF_MS);
                ctx.render(Surface::Couch);
                CouchChange::None
            }
            _ => CouchChange::None,
        }
    }

    fn frame(
        &mut self,
        ctx: &mut Ctx,
        env: Option<&Env>,
        playback: &Playback,
        frame: ServerFrame,
    ) -> CouchChange {
        let Some(live) = self.live.as_mut() else { return CouchChange::None };
        ctx.render(Surface::Couch);
        match frame {
            ServerFrame::Hello(hello) => {
                live.me = hello.my_participant_id;
                live.participants = hello.participants;
                live.state = hello.state;
                live.received_at = ctx.now;
                if live.role == CouchRole::Follower {
                    self.resync(ctx, playback, true);
                }
            }
            ServerFrame::HostState(state) => {
                // stale or out of order
                if state.seq <= live.state.seq {
                    return CouchChange::None;
                }
                live.state = state;
                live.received_at = ctx.now;
                if live.role == CouchRole::Follower {
                    self.resync(ctx, playback, false);
                }
            }
            ServerFrame::Participants(update) => live.participants = update.participants,
            ServerFrame::MediaChanged(_) => {
                if live.role == CouchRole::Follower {
                    // a new title starts unpaused for everyone
                    live.local_paused = false;
                    if let Some(env) = env {
                        self.fetch_player(ctx, env);
                    }
                }
            }
            ServerFrame::HostAway(_) => {
                live.state.away = true;
                if live.role == CouchRole::Follower {
                    playback.command(ctx, PlayerCommand::Pause);
                }
            }
            ServerFrame::HostReturned => {
                live.state.away = false;
                if live.role == CouchRole::Follower {
                    self.resync(ctx, playback, true);
                }
            }
            ServerFrame::Emoji(emoji) => {
                self.next_reaction += 1;
                let id = self.next_reaction;
                self.reactions.push_back(Reaction {
                    id,
                    participant_id: emoji.from_participant_id,
                    emoji: emoji.emoji,
                });
                ctx.after(REACTION_MS, self.pending(Request::ReactionGone(id)));
            }
            ServerFrame::SessionEnded(ended) => {
                let follower = live.role == CouchRole::Follower;
                self.teardown(ctx);
                self.generation += 1;
                self.ended = Some(ended.reason.as_str().to_string());
                return if follower { CouchChange::StopFollowing } else { CouchChange::None };
            }
            ServerFrame::RemoteCommand(command) => {
                return Self::apply_remote(ctx, playback, &command);
            }
            ServerFrame::Unknown => {}
        }
        CouchChange::None
    }

    /// The host's playing device follows a remote's command; its next state broadcast tells
    /// everyone.
    fn apply_remote(
        ctx: &mut Ctx,
        playback: &Playback,
        command: &CouchRemoteCommand,
    ) -> CouchChange {
        match command.action {
            CouchRemoteCommandAction::Play => playback.command(ctx, PlayerCommand::Play),
            CouchRemoteCommandAction::Pause => playback.command(ctx, PlayerCommand::Pause),
            CouchRemoteCommandAction::Seek => {
                if let Some(seconds) = command.position_seconds {
                    playback.command(ctx, PlayerCommand::Seek(PlayerSeek { seconds }));
                }
            }
            CouchRemoteCommandAction::Next => return CouchChange::Next,
            CouchRemoteCommandAction::Previous => return CouchChange::Previous,
            CouchRemoteCommandAction::Unknown => {}
        }
        CouchChange::None
    }

    /// Keeps a follower on the host's timeline: the same play state, and within the drift
    /// threshold of where the host is now. `hard` jumps even when close (joining, unpausing).
    pub fn resync(&mut self, ctx: &mut Ctx, playback: &Playback, hard: bool) {
        let note = self.pending(Request::ResyncNoteGone);
        let Some(live) = self.live.as_mut().filter(|l| l.role == CouchRole::Follower) else {
            return;
        };
        if live.local_paused {
            return;
        }
        let state = &live.state;
        let Some((position, playing)) = playback.position_at(ctx.now) else { return };
        if state.away || !state.playing {
            if playing {
                playback.command(ctx, PlayerCommand::Pause);
            }
            return;
        }
        if !playing {
            playback.command(ctx, PlayerCommand::Play);
        }
        // a seeking or buffering player is left alone unless this is a hard resync: seeking a
        // player that is not ready is what makes it jump around
        if !hard && playback.buffering() {
            return;
        }
        let expected = expected_position(state, live.received_at, ctx.now);
        let drift = (position - expected).abs();
        if hard || drift > DRIFT_SECONDS {
            playback.command(ctx, PlayerCommand::Seek(PlayerSeek { seconds: expected }));
        }
        if drift > DRIFT_SECONDS {
            if let Some(timer) = live.resync_timer.take() {
                ctx.cancel_timer(timer);
            }
            live.resync_timer = Some(ctx.after(RESYNC_NOTE_MS, note));
            ctx.render(Surface::Couch);
        }
    }

    pub fn view(&self, images: &Images, env_origin: &str, base: &str) -> CouchView {
        let Some(live) = &self.live else {
            return CouchView {
                status: if self.joining {
                    CouchStatus::Connecting
                } else if self.ended.is_some() {
                    CouchStatus::Ended
                } else {
                    CouchStatus::Idle
                },
                role: None,
                code: None,
                share_url: None,
                members: vec![],
                media: None,
                playing: false,
                position_seconds: 0.0,
                position_at_ms: 0,
                host_away: false,
                waiting: false,
                local_paused: false,
                reactions: vec![],
                recent_emojis: self.recent.clone(),
                resynced: false,
                ended: self.ended.clone(),
                problem: self.problem.clone(),
            };
        };
        let server = if base.is_empty() { env_origin } else { base };
        let media = target(&live.state.media);
        CouchView {
            status: live.status,
            role: Some(live.role),
            code: Some(live.code.clone()),
            share_url: Some(format!("{server}/couch/{}", live.code)),
            members: live
                .participants
                .iter()
                .map(|p| CouchMember {
                    id: p.id.clone(),
                    display_name: p.display_name.clone(),
                    avatar: p
                        .avatar_id
                        .as_ref()
                        .map(|id| images.image(id, None, Size::Small, None)),
                    seed: p.seed.clone().unwrap_or_default(),
                    host: p.is_host,
                    anonymous: p.is_anonymous,
                    paused: p.paused,
                    me: p.id == live.me,
                })
                .collect(),
            waiting: live.role == CouchRole::Follower && (live.state.away || media.is_none()),
            media,
            playing: live.state.playing,
            position_seconds: live.state.position_seconds,
            position_at_ms: live.received_at,
            host_away: live.state.away,
            local_paused: live.local_paused,
            reactions: self.reactions.iter().cloned().collect(),
            recent_emojis: self.recent.clone(),
            resynced: live.resync_timer.is_some(),
            ended: self.ended.clone(),
            problem: self.problem.clone(),
        }
    }

    pub fn status(&self) -> LoadStatus {
        if self.live.is_some() { LoadStatus::Loaded } else { LoadStatus::Idle }
    }
}

/// Where the host is now: its last position, moved on by the time since it arrived when playing.
/// The server stamps states with its own clock, but only the shell's clock is comparable here.
fn expected_position(state: &CouchHostState, received_at: U53, now: U53) -> f64 {
    if !state.playing {
        return state.position_seconds;
    }
    #[allow(clippy::cast_precision_loss)] // milliseconds since the frame arrived
    let elapsed = now.saturating_sub(received_at) as f64 / 1000.0;
    state.position_seconds + elapsed
}

fn target(media: &CouchMediaRef) -> Option<PlayTarget> {
    match media.kind {
        CouchMediaRefKind::Movie => {
            media.title_id.clone().map(|id| PlayTarget { kind: PlayKind::Movie, id })
        }
        CouchMediaRefKind::Episode => {
            media.episode_id.clone().map(|id| PlayTarget { kind: PlayKind::Episode, id })
        }
        _ => None,
    }
}

/// The socket's scheme for a server address: wss for https, ws for http, nothing for the web.
fn socket_base(base: &str) -> String {
    if let Some(rest) = base.strip_prefix("https://") {
        format!("wss://{rest}")
    } else if let Some(rest) = base.strip_prefix("http://") {
        format!("ws://{rest}")
    } else {
        base.to_string()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn state(position: f64, playing: bool) -> CouchHostState {
        CouchHostState {
            away: false,
            media: CouchMediaRef {
                kind: CouchMediaRefKind::Movie,
                title_id: Some("t".into()),
                episode_id: None,
            },
            playing,
            position_seconds: position,
            seq: 1,
            server_timestamp: 0,
        }
    }

    #[test]
    fn the_host_moves_on_while_playing_and_stays_put_when_paused() {
        assert!((expected_position(&state(100.0, true), 1_000, 3_500) - 102.5).abs() < 1e-9);
        assert!((expected_position(&state(100.0, false), 1_000, 3_500) - 100.0).abs() < 1e-9);
        // a clock that went backwards does not rewind the host
        assert!((expected_position(&state(100.0, true), 5_000, 3_000) - 100.0).abs() < 1e-9);
    }

    #[test]
    fn sockets_use_the_servers_scheme() {
        assert_eq!(socket_base("https://tv.home:8443"), "wss://tv.home:8443");
        assert_eq!(socket_base("http://10.0.0.2:8080"), "ws://10.0.0.2:8080");
        assert_eq!(socket_base(""), "");
    }
}
