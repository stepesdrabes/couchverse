//! Downloads for offline viewing (iOS and Android, plan 10.8). The server prepares an MP4 for this
//! device; the core polls until it is ready and fetches it, with its artwork, through the
//! download effect. The account's offline library and the progress saved while offline live in
//! the store, so both survive a relaunch and work without a network; the progress is replayed,
//! with the time it was watched, once the server answers again.

use std::collections::HashMap;

use couchverse_api::Call;
use couchverse_api::ops::{self, ListDownloadsQuery, RequestDownloadQuery};
use couchverse_api::types::{
    Download, DownloadList, DownloadRequest, DownloadRequestKind, DownloadRequestQuality,
    DownloadStatus, ProgressReport,
};
use serde::{Deserialize, Serialize};
use typeshare::typeshare;

use crate::api::{Endpoint, Failure, decode};
use crate::core::Pending;
use crate::effects::Ctx;
use crate::messages::{EffectOutput, LoadStatus, Problem, Surface, U53};
use crate::modules::catalog::{EpisodeNumber, PlayKind, PlayTarget};
use crate::modules::images::{Image, Images, Size};
use crate::modules::playback::LocalTitle;

/// The account's downloads and unsaved progress live under this prefix and the account id.
const KEY_PREFIX: &str = "downloads.";
/// How often the server is asked about downloads it is still preparing.
const POLL_MS: U53 = 5_000;
/// The most watched seconds the server counts from one progress report.
const MAX_REPORT_WATCHED: i64 = 600;

/// A movie or an episode to keep on the device.
#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DownloadAsk {
    pub target: PlayTarget,
    pub quality: DownloadQuality,
    /// Audio languages to keep, in order; empty for the default track.
    #[serde(default)]
    pub audio: Vec<String>,
}

/// A download by its id from `DownloadsView`.
#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DownloadRef {
    pub id: String,
}

/// How big a download is: the source picture, or a ladder rung (smaller, H.264).
#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum DownloadQuality {
    Original,
    #[serde(rename = "1080p")]
    Hd1080,
    #[serde(rename = "720p")]
    Hd720,
    #[serde(rename = "480p")]
    Sd480,
}

#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum DownloadState {
    /// The server has not started preparing it.
    Queued,
    /// The server is making the MP4.
    Preparing,
    /// The device is fetching it.
    Fetching,
    /// On the device, playable offline.
    Ready,
    Failed,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DownloadsView {
    /// `loading` until the device's list is read, then `loaded`.
    pub status: LoadStatus,
    /// Newest first.
    pub items: Vec<DownloadItem>,
    /// Bytes the finished downloads take on the device.
    pub used_bytes: U53,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DownloadItem {
    pub id: String,
    pub target: PlayTarget,
    pub title: String,
    pub title_slug: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub episode: Option<EpisodeNumber>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub episode_name: Option<String>,
    pub quality: DownloadQuality,
    pub state: DownloadState,
    /// 0 to 1 while preparing or fetching.
    pub progress: f64,
    /// The MP4's size; 0 until known.
    pub size_bytes: U53,
    /// The episode still or the poster, from the server.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub image: Option<Image>,
    /// The same artwork kept on the device: its name in the downloads directory. Prefer it
    /// offline.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub artwork: Option<String>,
    /// Why it failed: `unsupported`, `expired`, `prepare_failed`, `fetch_failed`, `no_space`.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub problem: Option<Problem>,
}

/// One download as the device keeps it.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct Entry {
    id: String,
    target: PlayTarget,
    title: String,
    title_id: String,
    title_slug: String,
    #[serde(default)]
    episode: Option<EpisodeNumber>,
    #[serde(default)]
    episode_name: Option<String>,
    quality: DownloadQuality,
    /// The languages asked for, to ask again after a failure.
    #[serde(default)]
    languages: Vec<String>,
    duration: f64,
    #[serde(default)]
    audio: Vec<(String, String)>,
    #[serde(default)]
    subtitles: Vec<(String, String, bool)>,
    /// Artwork id and version: the episode still, else the poster.
    #[serde(default)]
    art: Option<(String, Option<i64>)>,
    #[serde(default)]
    artwork: Option<String>,
    stage: Stage,
    #[serde(default)]
    size: U53,
    /// Where offline playback stopped.
    #[serde(default)]
    position: f64,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(tag = "stage", rename_all = "camelCase")]
enum Stage {
    /// On the server: queued or being prepared.
    Waiting {
        preparing: bool,
        progress: f64,
    },
    Fetching {
        received: U53,
        total: Option<U53>,
    },
    Ready,
    Failed {
        code: String,
    },
}

impl Entry {
    fn file(&self) -> String {
        format!("{}.mp4", self.id)
    }

    fn artwork_file(&self) -> String {
        format!("{}.jpg", self.id)
    }

    fn item(&self, images: &Images) -> DownloadItem {
        let (state, progress, problem) = match &self.stage {
            Stage::Waiting { preparing: false, .. } => (DownloadState::Queued, 0.0, None),
            Stage::Waiting { preparing: true, progress } => {
                (DownloadState::Preparing, *progress, None)
            }
            Stage::Fetching { received, total } => {
                #[allow(clippy::cast_precision_loss)] // bytes, as a fraction
                let progress =
                    total.filter(|t| *t > 0).map_or(0.0, |t| *received as f64 / t as f64);
                (DownloadState::Fetching, progress.min(1.0), None)
            }
            Stage::Ready => (DownloadState::Ready, 1.0, None),
            Stage::Failed { code } => {
                (DownloadState::Failed, 0.0, Some(Problem::new(code, "the download failed")))
            }
        };
        let size = match self.episode {
            Some(_) => Size::Medium,
            None => Size::Small,
        };
        DownloadItem {
            id: self.id.clone(),
            target: self.target.clone(),
            title: self.title.clone(),
            title_slug: self.title_slug.clone(),
            episode: self.episode,
            episode_name: self.episode_name.clone(),
            quality: self.quality,
            state,
            progress,
            size_bytes: self.size,
            image: self
                .art
                .as_ref()
                .map(|(id, ver)| images.image(id, ver.map(|v| v.to_string()), size, None)),
            artwork: self.artwork.clone(),
            problem,
        }
    }

    /// Takes what the server says about the download; the device's own progress (fetching,
    /// fetched) is not the server's to undo.
    fn update(&mut self, download: &Download) {
        self.title.clone_from(&download.title);
        self.episode_name.clone_from(&download.episode_name);
        self.duration = download.duration_seconds;
        self.audio = download.audio.iter().map(|t| (t.lang.clone(), t.label.clone())).collect();
        self.subtitles = download
            .subtitles
            .iter()
            .map(|s| (s.lang.clone(), s.label.clone(), s.forced))
            .collect();
        self.art = art(download);
        self.size = download.size_bytes.and_then(|b| U53::try_from(b).ok()).unwrap_or(self.size);
        if matches!(self.stage, Stage::Fetching { .. } | Stage::Ready) {
            return;
        }
        #[allow(clippy::cast_precision_loss)] // percent
        let progress = download.progress as f64 / 100.0;
        self.stage = match download.status {
            DownloadStatus::Queued | DownloadStatus::Unknown => {
                Stage::Waiting { preparing: false, progress: 0.0 }
            }
            DownloadStatus::Preparing => Stage::Waiting { preparing: true, progress },
            // fetched next; until then it waits, done on the server
            DownloadStatus::Ready => Stage::Waiting { preparing: true, progress: 1.0 },
            DownloadStatus::Failed => Stage::Failed { code: "prepare_failed".into() },
        };
    }
}

fn art(download: &Download) -> Option<(String, Option<i64>)> {
    match (&download.thumb_id, &download.poster_id) {
        (Some(thumb), _) => Some((thumb.clone(), download.thumb_ver)),
        (None, Some(poster)) => Some((poster.clone(), download.poster_ver)),
        (None, None) => None,
    }
}

fn entry(download: &Download, ask: &DownloadAsk) -> Entry {
    let episode = match (download.season_number, download.episode_number) {
        (Some(season), Some(episode)) => Some(EpisodeNumber {
            season: u32::try_from(season).unwrap_or_default(),
            episode: u32::try_from(episode).unwrap_or_default(),
        }),
        _ => None,
    };
    let mut entry = Entry {
        id: download.id.clone(),
        target: ask.target.clone(),
        title: String::new(),
        title_id: download.title_id.clone(),
        title_slug: download.title_slug.clone(),
        episode,
        episode_name: None,
        quality: ask.quality,
        languages: ask.audio.clone(),
        duration: 0.0,
        audio: vec![],
        subtitles: vec![],
        art: None,
        artwork: None,
        stage: Stage::Waiting { preparing: false, progress: 0.0 },
        size: 0,
        position: 0.0,
    };
    entry.update(download);
    entry
}

/// What the device keeps for one account.
#[derive(Debug, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct Saved {
    #[serde(default)]
    entries: Vec<Entry>,
    /// Progress that never reached the server, oldest first.
    #[serde(default)]
    unsaved: Vec<ProgressReport>,
}

/// What a request needs from the active session.
pub struct Env<'a> {
    pub endpoint: &'a Endpoint,
    pub language: &'a str,
    /// The device's profile; the server plans the MP4 for it.
    pub profile: Option<&'a couchverse_api::types::DeviceProfile>,
    /// For the artwork kept next to a download.
    pub images: Images,
}

#[derive(Debug, Clone, PartialEq)]
pub struct DownloadsPending {
    /// Bumped by every account switch, so answers for the previous account are dropped.
    generation: u64,
    request: Request,
}

#[derive(Debug, Clone, PartialEq)]
enum Request {
    Load,
    /// Another account's list, read to delete its files after signing out of it.
    Purge(String),
    Ask(Call<Download>, DownloadAsk),
    List(Call<DownloadList>),
    Poll,
    Fetch(String),
    /// Picking up, after a relaunch, a transfer the shell may still run or have finished.
    Attach(String),
    Artwork(String),
    Replayed(Box<ProgressReport>),
    Removed,
}

/// What a resolved output means for the rest of the core.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum DownloadsChange {
    None,
    Unauthorized,
    /// Tell the user: a notice code.
    Notice(&'static str),
}

#[derive(Default)]
pub struct Downloads {
    account: Option<String>,
    generation: u64,
    loaded: bool,
    entries: Vec<Entry>,
    unsaved: Vec<ProgressReport>,
    /// Transfers in flight, by download id.
    fetching: HashMap<String, U53>,
    poll: Option<U53>,
    /// A list request is out; another would only repeat it.
    listing: bool,
    /// Replays still waiting for an answer; no new round starts until they land.
    replaying: usize,
}

impl Downloads {
    /// Reads the account's downloads; until they land, nothing else happens.
    pub fn activate(&mut self, ctx: &mut Ctx, account_id: &str) {
        self.reset(ctx);
        self.account = Some(account_id.to_string());
        ctx.store_read(&key(account_id), self.pending(Request::Load));
    }

    /// The account is no longer active: transfers carry on in the shell and are picked up
    /// again when it is.
    pub fn reset(&mut self, ctx: &mut Ctx) {
        if let Some(poll) = self.poll.take() {
            ctx.cancel_timer(poll);
        }
        for (_, transfer) in self.fetching.drain() {
            ctx.forget(transfer);
        }
        self.generation += 1;
        self.account = None;
        self.loaded = false;
        self.entries.clear();
        self.unsaved.clear();
        self.listing = false;
        self.replaying = 0;
        ctx.render(Surface::Downloads);
    }

    /// Signing out of an account deletes its downloads from the device.
    pub fn purge(&mut self, ctx: &mut Ctx, account_id: &str) {
        if self.account.as_deref() == Some(account_id) {
            for (_, transfer) in self.fetching.drain() {
                ctx.download_cancel(transfer);
            }
            for entry in &self.entries {
                ctx.download_remove(entry.file());
                ctx.download_remove(entry.artwork_file());
            }
            ctx.store_delete(&key(account_id));
            self.reset(ctx);
        } else {
            let pending = Pending::Downloads(DownloadsPending {
                generation: self.generation,
                request: Request::Purge(account_id.to_string()),
            });
            ctx.store_read(&key(account_id), pending);
        }
    }

    pub fn view(&self, images: &Images) -> DownloadsView {
        let used = self.entries.iter().filter(|e| e.stage == Stage::Ready).map(|e| e.size).sum();
        DownloadsView {
            status: if self.loaded { LoadStatus::Loaded } else { LoadStatus::Loading },
            items: self.entries.iter().map(|e| e.item(images)).collect(),
            used_bytes: used,
        }
    }

    /// Asks the server to prepare a download; asking again for one already kept does nothing.
    pub fn ask(&mut self, ctx: &mut Ctx, env: &Env, ask: DownloadAsk) -> DownloadsChange {
        let Some(profile) = env.profile else { return DownloadsChange::Notice("download_failed") };
        let known = self.entries.iter().any(|e| {
            e.target == ask.target
                && e.quality == ask.quality
                && !matches!(e.stage, Stage::Failed { .. })
        });
        if known {
            return DownloadsChange::None;
        }
        let body = DownloadRequest {
            audio: Some(ask.audio.clone()).filter(|a| !a.is_empty()),
            id: ask.target.id.clone(),
            kind: match ask.target.kind {
                PlayKind::Movie => DownloadRequestKind::Movie,
                PlayKind::Episode => DownloadRequestKind::Episode,
            },
            profile: profile.clone(),
            quality: match ask.quality {
                DownloadQuality::Original => DownloadRequestQuality::Original,
                DownloadQuality::Hd1080 => DownloadRequestQuality::V1080p,
                DownloadQuality::Hd720 => DownloadRequestQuality::V720p,
                DownloadQuality::Sd480 => DownloadRequestQuality::V480p,
            },
        };
        let query = RequestDownloadQuery { lang: Some(env.language.to_string()) };
        let call = ops::request_download(&query, &body);
        ctx.http(env.endpoint.request(&call.request), self.pending(Request::Ask(call, ask)));
        DownloadsChange::None
    }

    /// Asks again for a download that failed, as it was first asked for.
    pub fn retry(&mut self, ctx: &mut Ctx, env: &Env, id: &str) -> DownloadsChange {
        let Some(entry) = self.entries.iter().find(|e| e.id == id) else {
            return DownloadsChange::None;
        };
        if !matches!(entry.stage, Stage::Failed { .. }) {
            return DownloadsChange::None;
        }
        let ask = DownloadAsk {
            target: entry.target.clone(),
            quality: entry.quality,
            audio: entry.languages.clone(),
        };
        self.ask(ctx, env, ask)
    }

    /// Deletes a download from the device and from the account's list on the server.
    pub fn remove(&mut self, ctx: &mut Ctx, endpoint: Option<&Endpoint>, id: &str) {
        let Some(index) = self.entries.iter().position(|e| e.id == id) else { return };
        let entry = self.entries.remove(index);
        if let Some(transfer) = self.fetching.remove(id) {
            ctx.download_cancel(transfer);
        }
        ctx.download_remove(entry.file());
        ctx.download_remove(entry.artwork_file());
        // offline, the server keeps its row until its retention runs out
        if let Some(endpoint) = endpoint {
            let call = ops::delete_download(id);
            ctx.http(endpoint.request(&call.request), self.pending(Request::Removed));
        }
        self.save(ctx);
        ctx.render(Surface::Downloads);
    }

    /// A finished download to play from the device.
    pub fn local(&self, id: &str) -> Option<LocalTitle> {
        self.entries.iter().find(|e| e.id == id && e.stage == Stage::Ready).map(local)
    }

    /// The finished download of a title, for playing it while the server is out of reach.
    pub fn local_for(&self, target: &PlayTarget) -> Option<LocalTitle> {
        self.entries.iter().find(|e| &e.target == target && e.stage == Stage::Ready).map(local)
    }

    /// Remembers where offline playback of a download got to.
    pub fn remember(&mut self, ctx: &mut Ctx, target: &PlayTarget, position: f64) {
        let mut changed = false;
        for entry in self.entries.iter_mut().filter(|e| &e.target == target) {
            changed |= (entry.position - position).abs() >= 1.0;
            entry.position = position;
        }
        if changed {
            self.save(ctx);
        }
    }

    /// Keeps progress that never reached the server; only the latest per title counts.
    pub fn keep(&mut self, ctx: &mut Ctx, report: ProgressReport) {
        self.merge(report);
        self.save(ctx);
    }

    fn merge(&mut self, mut report: ProgressReport) {
        // the latest report for a title carries its position; the watched time of the ones it
        // replaces rides along, up to what the server takes from one report
        let last = self
            .unsaved
            .iter()
            .rposition(|r| r.title_id == report.title_id && r.episode_id == report.episode_id);
        if let Some(index) = last {
            let watched = self.unsaved[index].watched_seconds.unwrap_or(0)
                + report.watched_seconds.unwrap_or(0);
            if watched <= MAX_REPORT_WATCHED {
                report.watched_seconds = Some(watched);
                self.unsaved.remove(index);
            }
        }
        self.unsaved.push(report);
    }

    /// The server answers again: sends the kept progress and catches up on what it prepared.
    pub fn reconnected(&mut self, ctx: &mut Ctx, env: &Env) {
        if !self.loaded {
            return;
        }
        if self.replaying == 0 {
            for report in self.unsaved.clone() {
                let call = ops::save_progress(&report);
                let request = Request::Replayed(Box::new(report));
                ctx.http(env.endpoint.request(&call.request), self.pending(request));
                self.replaying += 1;
            }
        }
        if self.entries.iter().any(|e| self.wants_server(e)) {
            self.list(ctx, env);
        }
    }

    /// The downloads screen opened: what the server prepared since shows at once.
    pub fn opened(&mut self, ctx: &mut Ctx, env: Option<&Env>) {
        if let Some(env) = env
            && self.loaded
            && self.entries.iter().any(|e| self.wants_server(e))
        {
            self.list(ctx, env);
        }
        ctx.render(Surface::Downloads);
    }

    pub fn resolve(
        &mut self,
        ctx: &mut Ctx,
        env: Option<&Env>,
        pending: DownloadsPending,
        output: EffectOutput,
    ) -> DownloadsChange {
        if let Request::Purge(account) = &pending.request {
            if let EffectOutput::Stored(stored) = output {
                let saved: Saved =
                    stored.value.and_then(|j| serde_json::from_str(&j).ok()).unwrap_or_default();
                for entry in &saved.entries {
                    ctx.download_remove(entry.file());
                    ctx.download_remove(entry.artwork_file());
                }
            }
            ctx.store_delete(&key(account));
            return DownloadsChange::None;
        }
        if pending.generation != self.generation {
            return DownloadsChange::None;
        }
        match pending.request {
            Request::Load => self.loaded(ctx, env, output),
            Request::Ask(call, ask) => return self.answered(ctx, env, &call, &ask, output),
            Request::List(call) => {
                self.listing = false;
                match decode(&call, output) {
                    Ok(list) => self.listed(ctx, env, &list),
                    Err(failure) if failure.unauthorized() => {
                        return DownloadsChange::Unauthorized;
                    }
                    Err(_) => {}
                }
            }
            Request::Poll => {
                if let Some(env) = env {
                    self.list(ctx, env);
                }
            }
            Request::Fetch(id) => self.fetched(ctx, env, &id, output),
            Request::Attach(id) => {
                if let EffectOutput::DownloadFailed(_) = output {
                    // the shell lost it: fetch it again with a fresh URL
                    self.fetching.remove(&id);
                    if let Some(env) = env {
                        self.list(ctx, env);
                    }
                } else {
                    self.fetched(ctx, env, &id, output);
                }
            }
            Request::Artwork(id) => {
                if let EffectOutput::DownloadFinished(_) = output
                    && let Some(entry) = self.entries.iter_mut().find(|e| e.id == id)
                {
                    entry.artwork = Some(entry.artwork_file());
                    self.save(ctx);
                    ctx.render(Surface::Downloads);
                }
            }
            Request::Replayed(report) => {
                self.replaying = self.replaying.saturating_sub(1);
                let status = match output {
                    EffectOutput::Http(response) => Some(response.status),
                    _ => None,
                };
                match status {
                    Some(401) => return DownloadsChange::Unauthorized,
                    // still out of reach: kept for the next try
                    None | Some(500..) => {}
                    Some(_) => {
                        self.unsaved.retain(|r| r != report.as_ref());
                        self.save(ctx);
                    }
                }
            }
            Request::Removed | Request::Purge(_) => {}
        }
        DownloadsChange::None
    }

    fn loaded(&mut self, ctx: &mut Ctx, env: Option<&Env>, output: EffectOutput) {
        let saved: Saved = match output {
            EffectOutput::Stored(stored) => {
                stored.value.and_then(|j| serde_json::from_str(&j).ok()).unwrap_or_default()
            }
            _ => Saved::default(),
        };
        self.entries = saved.entries;
        // transfers that were running when the app went away may have finished since
        for entry in &self.entries {
            if matches!(entry.stage, Stage::Fetching { .. }) {
                let pending = Pending::Downloads(DownloadsPending {
                    generation: self.generation,
                    request: Request::Attach(entry.id.clone()),
                });
                let transfer = ctx.download_start(String::new(), entry.file(), pending);
                self.fetching.insert(entry.id.clone(), transfer);
            }
        }
        // kept progress first: anything saved while this read was out came later
        let later = std::mem::take(&mut self.unsaved);
        self.unsaved = saved.unsaved;
        for report in later {
            self.merge(report);
        }
        self.loaded = true;
        if let Some(env) = env {
            self.reconnected(ctx, env);
        }
        ctx.render(Surface::Downloads);
    }

    fn answered(
        &mut self,
        ctx: &mut Ctx,
        env: Option<&Env>,
        call: &Call<Download>,
        ask: &DownloadAsk,
        output: EffectOutput,
    ) -> DownloadsChange {
        let download = match decode(call, output) {
            Ok(download) => download,
            Err(failure) if failure.unauthorized() => return DownloadsChange::Unauthorized,
            Err(Failure::Api(e)) if e.status == 422 => {
                return DownloadsChange::Notice("download_unsupported");
            }
            Err(_) => return DownloadsChange::Notice("download_failed"),
        };
        match self.entries.iter_mut().find(|e| e.id == download.id) {
            Some(existing) => {
                // a retry: the server's state takes over again
                existing.stage = Stage::Waiting { preparing: false, progress: 0.0 };
                existing.update(&download);
            }
            None => self.entries.insert(0, entry(&download, ask)),
        }
        if let (Some(env), Some(url)) = (env, &download.url)
            && download.status == DownloadStatus::Ready
        {
            self.fetch(ctx, env, &download.id, url);
        }
        self.save(ctx);
        self.schedule(ctx);
        ctx.render(Surface::Downloads);
        DownloadsChange::None
    }

    fn listed(&mut self, ctx: &mut Ctx, env: Option<&Env>, list: &DownloadList) {
        let mut ready = vec![];
        for entry in &mut self.entries {
            if matches!(entry.stage, Stage::Ready | Stage::Failed { .. }) {
                continue;
            }
            let Some(download) = list.downloads.iter().find(|d| d.id == entry.id) else {
                // gone from the server before the device fetched it
                if !self.fetching.contains_key(&entry.id) {
                    entry.stage = Stage::Failed { code: "expired".into() };
                }
                continue;
            };
            entry.update(download);
            if download.status == DownloadStatus::Ready
                && !self.fetching.contains_key(&entry.id)
                && let Some(url) = &download.url
            {
                ready.push((entry.id.clone(), url.clone()));
            }
        }
        if let Some(env) = env {
            for (id, url) in ready {
                self.fetch(ctx, env, &id, &url);
            }
        }
        self.save(ctx);
        self.schedule(ctx);
        ctx.render(Surface::Downloads);
    }

    /// Starts fetching (or, after a relaunch, re-attaches to the shell's transfer of) a
    /// download the server has ready.
    fn fetch(&mut self, ctx: &mut Ctx, env: &Env, id: &str, url: &str) {
        let Some(entry) = self.entries.iter_mut().find(|e| e.id == id) else { return };
        let total = Some(entry.size).filter(|s| *s > 0);
        if !matches!(entry.stage, Stage::Fetching { .. }) {
            entry.stage = Stage::Fetching { received: 0, total };
        }
        let pending = Pending::Downloads(DownloadsPending {
            generation: self.generation,
            request: Request::Fetch(id.to_string()),
        });
        let transfer = ctx.download_start(absolute(env.endpoint, url), entry.file(), pending);
        self.fetching.insert(id.to_string(), transfer);
    }

    fn fetched(&mut self, ctx: &mut Ctx, env: Option<&Env>, id: &str, output: EffectOutput) {
        let generation = self.generation;
        let Some(entry) = self.entries.iter_mut().find(|e| e.id == id) else { return };
        match output {
            EffectOutput::DownloadProgress(progress) => {
                let before = entry.item(&Images::default()).progress;
                entry.stage = Stage::Fetching {
                    received: progress.received_bytes,
                    total: progress.total_bytes.or(Some(entry.size).filter(|s| *s > 0)),
                };
                // a render per percent, not per chunk
                if (entry.item(&Images::default()).progress - before).abs() >= 0.01 {
                    ctx.render(Surface::Downloads);
                }
                return;
            }
            EffectOutput::DownloadFinished(done) => {
                self.fetching.remove(id);
                entry.stage = Stage::Ready;
                entry.size = done.bytes;
                if entry.artwork.is_none()
                    && let (Some(art), Some(env)) = (&entry.art, env)
                {
                    // the image's own URL, with the account's grant, as the screen shows it
                    let version = art.1.map(|v| v.to_string());
                    let image = env.images.image(&art.0, version, Size::Medium, None);
                    let pending = Pending::Downloads(DownloadsPending {
                        generation,
                        request: Request::Artwork(id.to_string()),
                    });
                    ctx.download_start(image.url, entry.artwork_file(), pending);
                }
            }
            EffectOutput::DownloadFailed(failure) => {
                self.fetching.remove(id);
                let code = if failure.no_space { "no_space" } else { "fetch_failed" };
                entry.stage = Stage::Failed { code: code.into() };
            }
            _ => return,
        }
        self.save(ctx);
        ctx.render(Surface::Downloads);
    }

    /// Whether the server still has something to say about a download.
    fn wants_server(&self, entry: &Entry) -> bool {
        match entry.stage {
            Stage::Waiting { .. } => true,
            // a transfer the shell lost (killed, or a grant that expired) needs a fresh URL
            Stage::Fetching { .. } => !self.fetching.contains_key(&entry.id),
            Stage::Ready | Stage::Failed { .. } => false,
        }
    }

    fn list(&mut self, ctx: &mut Ctx, env: &Env) {
        if self.listing {
            return;
        }
        self.listing = true;
        let call = ops::list_downloads(&ListDownloadsQuery { lang: Some(env.language.into()) });
        ctx.http(env.endpoint.request(&call.request), self.pending(Request::List(call)));
    }

    /// Polls while the server is preparing something; stops once nothing waits.
    fn schedule(&mut self, ctx: &mut Ctx) {
        let waiting = self.entries.iter().any(|e| matches!(e.stage, Stage::Waiting { .. }));
        match (waiting, self.poll) {
            (true, None) => self.poll = Some(ctx.every(POLL_MS, self.pending(Request::Poll))),
            (false, Some(poll)) => {
                ctx.cancel_timer(poll);
                self.poll = None;
            }
            _ => {}
        }
    }

    fn save(&self, ctx: &mut Ctx) {
        let Some(account) = &self.account else { return };
        if !self.loaded {
            return;
        }
        let saved = Saved { entries: self.entries.clone(), unsaved: self.unsaved.clone() };
        if let Ok(json) = serde_json::to_string(&saved) {
            ctx.store_write(&key(account), json);
        }
    }

    fn pending(&self, request: Request) -> Pending {
        Pending::Downloads(DownloadsPending { generation: self.generation, request })
    }
}

fn key(account_id: &str) -> String {
    format!("{KEY_PREFIX}{account_id}")
}

/// The server hands out paths; transfers need the whole URL.
fn absolute(endpoint: &Endpoint, url: &str) -> String {
    if url.starts_with("http://") || url.starts_with("https://") {
        url.to_string()
    } else {
        format!("{}{url}", endpoint.base)
    }
}

fn local(entry: &Entry) -> LocalTitle {
    let subtitle = match (&entry.episode, &entry.episode_name) {
        // as the server writes it for a streamed episode
        (Some(n), Some(name)) if !name.is_empty() => {
            format!("S{} E{} \u{b7} {name}", n.season, n.episode)
        }
        (Some(n), _) => format!("S{} E{}", n.season, n.episode),
        _ => String::new(),
    };
    LocalTitle {
        target: entry.target.clone(),
        file: entry.file(),
        title: entry.title.clone(),
        subtitle,
        title_id: entry.title_id.clone(),
        title_slug: entry.title_slug.clone(),
        duration: entry.duration,
        resume: entry.position,
        audio: entry.audio.clone(),
        subtitles: entry.subtitles.clone(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn report(episode: &str, position: i64, watched: i64) -> ProgressReport {
        ProgressReport {
            title_id: None,
            episode_id: Some(episode.into()),
            position_seconds: position,
            duration_seconds: 2400,
            watched_seconds: Some(watched),
            watched_at: Some(format!("2026-10-02T12:{:02}:00Z", position / 60)),
        }
    }

    #[test]
    fn kept_progress_folds_per_title_without_losing_watched_time() {
        let mut downloads = Downloads::default();
        downloads.merge(report("e1", 100, 10));
        downloads.merge(report("e2", 50, 10));
        downloads.merge(report("e1", 110, 10));
        let kept: Vec<_> = downloads
            .unsaved
            .iter()
            .map(|r| (r.episode_id.clone().unwrap(), r.position_seconds, r.watched_seconds))
            .collect();
        assert_eq!(kept, [("e2".into(), 50, Some(10)), ("e1".into(), 110, Some(20))]);

        // past what one report may carry, a new one starts
        downloads.merge(report("e1", 700, 590));
        assert_eq!(downloads.unsaved.len(), 3);
        assert_eq!(downloads.unsaved[2].watched_seconds, Some(590));
    }
}
