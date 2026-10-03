//! Scenario tests: a fake shell drives the core through whole flows with explicit time, answers
//! its effects and asserts on the effects and view models, the way a real shell would see them.

mod boot;
mod catalog;
mod couch;
mod pairing;
mod playback;
mod ranks;
mod session;
mod sign_in;

use std::collections::HashMap;

use serde::de::DeserializeOwned;
use serde_json::{Value, json};

use crate::core::{AppPhase, AppView, Core};
use crate::messages::{
    AuthMode, CoreConfig, Effect, EffectOutput, EffectRequest, Event, HttpFailure, HttpFailureKind,
    HttpRequest, HttpResponse, LoadStatus, Message, Platform, PlayerCommand, Resolution,
    SocketCommand, SocketOpen, SocketText, StoreOp, StoredValue, Surface, U53, UploadRequest,
};

pub const SERVER_ID: &str = "4f6c0a5e-6a43-4c0e-9d4b-2b8f8d0b7a11";
pub const HTTPS: &str = "https://media.example.com";

/// A shell with in-memory stores. Writes and deletes are applied as they are issued (they are
/// fire-and-forget); reads, HTTP calls and timers stay outstanding until a test answers them.
pub struct Shell {
    pub core: Core,
    pub now: U53,
    pub store: HashMap<String, String>,
    pub secure: HashMap<String, String>,
    outstanding: Vec<EffectRequest>,
    /// Surfaces named by `Render` effects since the last `take_renders`.
    renders: Vec<Surface>,
    cancelled: Vec<U53>,
    /// Every command the core gave the player, oldest first.
    pub player: Vec<PlayerCommand>,
    /// Sockets the core opened, by the open effect's id.
    pub sockets: Vec<(U53, SocketOpen)>,
    /// Text the core sent, with the socket it went on.
    pub sent: Vec<(U53, String)>,
    pub closed_sockets: Vec<U53>,
}

impl Shell {
    pub fn new(platform: Platform) -> Self {
        let auth_mode = if platform == Platform::Web { AuthMode::Cookie } else { AuthMode::Bearer };
        Self::with_config(CoreConfig {
            platform,
            auth_mode,
            device_name: "Living Room".into(),
            locale: "cs-CZ".into(),
            origin: String::new(),
        })
    }

    pub fn with_config(config: CoreConfig) -> Self {
        Self {
            core: Core::new(config),
            now: 1_000,
            store: HashMap::new(),
            secure: HashMap::new(),
            outstanding: vec![],
            renders: vec![],
            cancelled: vec![],
            player: vec![],
            sockets: vec![],
            sent: vec![],
            closed_sockets: vec![],
        }
    }

    /// A fresh core over this shell's stores, as after the app was killed and relaunched.
    pub fn relaunch(&self, platform: Platform) -> Self {
        let mut shell = Self::new(platform);
        shell.store.clone_from(&self.store);
        shell.secure.clone_from(&self.secure);
        shell
    }

    pub fn send(&mut self, event: Event) {
        let effects = self.core.send(Message { now_ms: self.now, event });
        self.absorb(effects);
    }

    pub fn resolve(&mut self, id: U53, output: EffectOutput) {
        self.outstanding.retain(|e| e.id != id);
        let effects = self.core.resolve(Resolution { now_ms: self.now, id, output });
        self.absorb(effects);
    }

    /// Re-delivers an output for an effect that already finished (a late timer tick).
    pub fn resolve_late(&mut self, id: U53, output: EffectOutput) -> Vec<EffectRequest> {
        self.core.resolve(Resolution { now_ms: self.now, id, output })
    }

    fn absorb(&mut self, effects: Vec<EffectRequest>) {
        for effect in effects {
            match &effect.effect {
                Effect::Render(render) => {
                    for surface in &render.surfaces {
                        if !self.renders.contains(surface) {
                            self.renders.push(surface.clone());
                        }
                    }
                }
                Effect::CancelTimer(timer) => {
                    self.cancelled.push(timer.id);
                    self.outstanding.retain(|e| e.id != timer.id);
                }
                Effect::Store(req) | Effect::SecureStore(req) => {
                    let secure = matches!(effect.effect, Effect::SecureStore(_));
                    let map = if secure { &mut self.secure } else { &mut self.store };
                    match &req.op {
                        StoreOp::Write(value) => {
                            map.insert(req.key.clone(), value.clone());
                        }
                        StoreOp::Delete => {
                            map.remove(&req.key);
                        }
                        StoreOp::Read => self.outstanding.push(effect),
                    }
                }
                Effect::Player(command) => self.player.push(command.clone()),
                Effect::Socket(SocketCommand::Open(open)) => {
                    self.sockets.push((effect.id, open.clone()));
                }
                Effect::Socket(SocketCommand::Send(send)) => {
                    self.sent.push((send.socket, send.text.clone()));
                }
                Effect::Socket(SocketCommand::Close(socket)) => self.closed_sockets.push(socket.id),
                Effect::Http(_) | Effect::Timer(_) | Effect::Upload(_) => {
                    self.outstanding.push(effect);
                }
            }
        }
    }

    /// Answers every outstanding store read from the in-memory stores, until none are left.
    pub fn answer_reads(&mut self) {
        while let Some((id, value)) = self.outstanding.iter().find_map(|e| match &e.effect {
            Effect::Store(r) if r.op == StoreOp::Read => Some((e.id, self.store.get(&r.key))),
            Effect::SecureStore(r) if r.op == StoreOp::Read => {
                Some((e.id, self.secure.get(&r.key)))
            }
            _ => None,
        }) {
            let value = value.cloned();
            self.resolve(id, EffectOutput::Stored(StoredValue { value }));
        }
    }

    /// The outstanding HTTP request for `method` and `url`; panics with what is outstanding.
    pub fn request(&self, method: &str, url: &str) -> (U53, HttpRequest) {
        self.find_request(method, url).unwrap_or_else(|| {
            panic!("no outstanding {method} {url}; outstanding: {:#?}", self.http_summary())
        })
    }

    pub fn find_request(&self, method: &str, url: &str) -> Option<(U53, HttpRequest)> {
        self.outstanding.iter().find_map(|e| match &e.effect {
            Effect::Http(r) | Effect::Upload(UploadRequest { request: r, .. })
                if r.method == method && r.url == url =>
            {
                Some((e.id, r.clone()))
            }
            _ => None,
        })
    }

    /// Every outstanding request for `method` and `url`, oldest first.
    pub fn requests(&self, method: &str, url: &str) -> Vec<(U53, HttpRequest)> {
        self.outstanding
            .iter()
            .filter_map(|e| match &e.effect {
                Effect::Http(r) | Effect::Upload(UploadRequest { request: r, .. })
                    if r.method == method && r.url == url =>
                {
                    Some((e.id, r.clone()))
                }
                _ => None,
            })
            .collect()
    }

    pub fn http_summary(&self) -> Vec<String> {
        self.outstanding
            .iter()
            .filter_map(|e| match &e.effect {
                Effect::Http(r) => Some(format!("{} {}", r.method, r.url)),
                Effect::Upload(u) => Some(format!("UPLOAD {} {}", u.request.method, u.request.url)),
                _ => None,
            })
            .collect()
    }

    /// Answers the outstanding `method url` with `status` and a JSON body.
    pub fn respond(&mut self, method: &str, url: &str, status: u16, body: Value) {
        let (id, _) = self.request(method, url);
        let body = match body {
            Value::Null => String::new(),
            json => json.to_string(),
        };
        self.resolve(id, EffectOutput::Http(HttpResponse { status, body }));
    }

    pub fn respond_raw(&mut self, method: &str, url: &str, status: u16, body: &str) {
        let (id, _) = self.request(method, url);
        self.resolve(id, EffectOutput::Http(HttpResponse { status, body: body.to_string() }));
    }

    pub fn fail(&mut self, method: &str, url: &str, kind: HttpFailureKind) {
        let (id, _) = self.request(method, url);
        let failure = HttpFailure { kind, message: "simulated".into() };
        self.resolve(id, EffectOutput::HttpFailed(failure));
    }

    /// The socket opened to `url`, the latest first.
    pub fn socket(&self, url: &str) -> U53 {
        let found = self.sockets.iter().rev().find(|(_, open)| open.url == url);
        let opened: Vec<&String> = self.sockets.iter().map(|(_, o)| &o.url).collect();
        found.map_or_else(|| panic!("no socket to {url}; opened: {opened:?}"), |(id, _)| *id)
    }

    /// The server says something on a socket.
    pub fn frame(&mut self, socket: U53, frame: &Value) {
        let text = frame.to_string();
        self.resolve(socket, EffectOutput::SocketText(SocketText { text }));
    }

    /// The JSON frames the core sent on a socket, oldest first.
    pub fn sent_frames(&self, socket: U53) -> Vec<Value> {
        self.sent
            .iter()
            .filter(|(s, _)| *s == socket)
            .map(|(_, t)| serde_json::from_str(t).expect("JSON frame"))
            .collect()
    }

    /// Every effect still waiting for an answer.
    pub fn effects_waiting(&self) -> Vec<EffectRequest> {
        self.outstanding.clone()
    }

    /// The outstanding timers as `(id, after_ms, repeat)`.
    pub fn timers(&self) -> Vec<(U53, U53, bool)> {
        self.outstanding
            .iter()
            .filter_map(|e| match &e.effect {
                Effect::Timer(t) => Some((e.id, t.after_ms, t.repeat)),
                _ => None,
            })
            .collect()
    }

    /// Advances the clock and fires a timer; a repeating timer stays outstanding.
    pub fn fire(&mut self, id: U53, advance_ms: U53) {
        self.now += advance_ms;
        let repeat = self.timers().iter().any(|(t, _, r)| *t == id && *r);
        let kept = repeat.then(|| self.outstanding.iter().find(|e| e.id == id).cloned()).flatten();
        self.resolve(id, EffectOutput::TimerFired);
        if let Some(timer) = kept {
            self.outstanding.push(timer);
        }
    }

    pub fn was_cancelled(&self, id: U53) -> bool {
        self.cancelled.contains(&id)
    }

    pub fn take_renders(&mut self) -> Vec<Surface> {
        std::mem::take(&mut self.renders)
    }

    pub fn view<T: DeserializeOwned>(&self, surface: &Surface) -> T {
        serde_json::from_str(&self.core.view(surface)).expect("view model decodes")
    }

    pub fn phase(&self) -> AppPhase {
        self.view::<AppView>(&Surface::App).phase
    }

    pub fn active_account(&self) -> Option<String> {
        self.view::<AppView>(&Surface::App).active_account
    }

    /// Answers the four session loads of a freshly activated account.
    pub fn answer_session(&mut self, base: &str, user: Value, language: Option<&str>) {
        self.respond("GET", &format!("{base}/api/v1/auth/me"), 200, user);
        self.respond(
            "GET",
            &format!("{base}/api/v1/features"),
            200,
            json!({"couchEnabled": true, "rankingsEnabled": false}),
        );
        let prefs = language.map_or_else(|| json!({}), |l| json!({ "language": l }));
        self.respond("GET", &format!("{base}/api/v1/me/preferences"), 200, prefs);
        self.respond("GET", &format!("{base}/api/v1/server"), 200, server_info("#3a6ea5"));
    }
}

/// An empty list to compare against, so a failing assertion shows what was there.
pub fn empty<T>() -> Vec<T> {
    Vec::new()
}

pub fn header<'a>(request: &'a HttpRequest, name: &str) -> Option<&'a str> {
    request.headers.iter().find(|h| h.name == name).map(|h| h.value.as_str())
}

pub fn body(request: &HttpRequest) -> Value {
    serde_json::from_str(request.body.as_deref().expect("request has a body")).expect("JSON body")
}

pub fn server_info(accent: &str) -> Value {
    json!({
        "id": SERVER_ID,
        "name": "Home Media",
        "version": "1.4.0",
        "apiLevel": 1,
        "accent": accent,
    })
}

pub fn user(id: i64, username: &str) -> Value {
    json!({
        "id": id,
        "username": username,
        "displayName": username.to_uppercase(),
        "role": if id == 1 { "admin" } else { "user" },
        "bio": "",
        "disabled": false,
        "createdAt": "2026-01-01T00:00:00Z",
        "avatarId": "av-1",
    })
}

pub fn device_token(token: &str, user_id: i64, username: &str) -> Value {
    json!({ "deviceId": format!("dev-{user_id}"), "token": token, "user": user(user_id, username) })
}

pub fn account_id(user_id: i64) -> String {
    format!("{SERVER_ID}/{user_id}")
}

/// A shell whose stores already hold `server` and signed-in `users` (`(id, name, token)`),
/// with `active` as the last used account.
pub fn returning(platform: Platform, users: &[(i64, &str, Option<&str>)], active: i64) -> Shell {
    let mut shell = Shell::new(platform);
    let server = json!([{
        "id": SERVER_ID, "url": HTTPS, "name": "Home Media", "version": "1.4.0",
        "apiLevel": 1, "accent": "#3a6ea5", "insecure": false,
    }]);
    let accounts: Vec<Value> = users
        .iter()
        .map(|(id, name, _)| {
            json!({
                "id": account_id(*id), "serverId": SERVER_ID, "userId": id,
                "username": name, "displayName": name.to_uppercase(),
                "avatarId": null, "artworkGrant": "g-art",
            })
        })
        .collect();
    shell.store.insert("servers".into(), server.to_string());
    shell.store.insert(
        "accounts".into(),
        json!({ "accounts": accounts, "active": account_id(active) }).to_string(),
    );
    for (id, _, token) in users {
        if let Some(token) = token {
            shell.secure.insert(format!("token.{}", account_id(*id)), (*token).to_string());
        }
    }
    shell
}

/// A launched shell whose start-up reads have all been answered.
pub fn launched(mut shell: Shell) -> Shell {
    shell.send(Event::AppStarted);
    shell.answer_reads();
    shell
}
