//! The player screen's view model.

use couchverse_api::types::{PlaybackInfo, PlaybackInfoMode};
use serde::{Deserialize, Serialize};
use typeshare::typeshare;

use super::{Quality, Session};
use crate::messages::{LoadStatus, Problem};
use crate::modules::catalog::PlayTarget;
use crate::modules::images::{Image, Images, Size};

/// The episode that plays when this one ends.
#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct NextUp {
    pub target: PlayTarget,
    pub season: u32,
    pub episode: u32,
    pub name: String,
    /// Seconds until it starts; shown in the last stretch of the current one.
    pub countdown_seconds: u32,
    /// Picked at random among the series' episodes.
    pub shuffled: bool,
}

#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum QualityKind {
    /// The source as it is, outside the adaptive ladder.
    Original,
    /// The ladder, adapting to the connection.
    Auto,
    /// One rung of the ladder, pinned.
    Rendition,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct QualityOption {
    /// What `QualityChosen` takes.
    pub key: String,
    pub kind: QualityKind,
    /// The rendition's height, for its label (`1080p`).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub height: Option<u32>,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct TrackOption {
    pub id: String,
    pub lang: String,
    pub label: String,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PlayerEpisode {
    pub id: String,
    pub number: u32,
    pub name: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub still: Option<Image>,
    pub current: bool,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PlayerSeason {
    pub number: u32,
    pub episodes: Vec<PlayerEpisode>,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PlayerView {
    /// `loading` while fetching or preparing, `loaded` once the player has a source, `stale`
    /// while a failed player reloads, `notFound`/`failed` with a problem.
    pub status: LoadStatus,
    /// What is on screen; absent with the player closed. The web keeps its URL in step.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub target: Option<PlayTarget>,
    pub title: String,
    /// The episode's label, empty for a movie.
    pub subtitle: String,
    /// The title page to go back to.
    pub title_slug: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub backdrop: Option<Image>,
    /// A transcode is being prepared; its progress in percent.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub preparing: Option<u32>,
    pub qualities: Vec<QualityOption>,
    pub quality: String,
    pub audio: Vec<TrackOption>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub audio_selected: Option<String>,
    pub subtitles: Vec<TrackOption>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub subtitle_selected: Option<String>,
    /// The series' playable episodes for the switcher, by season.
    pub seasons: Vec<PlayerSeason>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub next_up: Option<NextUp>,
    /// The series allows random playback, so a shuffle switch makes sense.
    pub shuffle_available: bool,
    pub shuffle: bool,
    /// A still for the seek-bar preview: append `?t=<seconds>`.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub frame_url: Option<String>,
    /// A couch follower's player: hide timeline controls.
    pub linear: bool,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub problem: Option<Problem>,
}

pub(super) fn player(session: Option<&Session>, shuffle: bool, images: &Images) -> PlayerView {
    let Some(session) = session else {
        return PlayerView {
            status: LoadStatus::Idle,
            target: None,
            title: String::new(),
            subtitle: String::new(),
            title_slug: String::new(),
            backdrop: None,
            preparing: None,
            qualities: vec![],
            quality: String::new(),
            audio: vec![],
            audio_selected: None,
            subtitles: vec![],
            subtitle_selected: None,
            seasons: vec![],
            next_up: None,
            shuffle_available: false,
            shuffle,
            frame_url: None,
            linear: false,
            problem: None,
        };
    };
    let info = session.info.as_ref();
    let preparing = info
        .filter(|i| i.mode == PlaybackInfoMode::Preparing)
        .map(|i| u32::try_from(i.job_progress.unwrap_or(0).clamp(0, 100)).unwrap_or(0));
    PlayerView {
        status: session.status,
        target: Some(session.target.clone()),
        title: info.map(|i| i.display.title.clone()).unwrap_or_default(),
        subtitle: info.map(|i| i.display.subtitle.clone()).unwrap_or_default(),
        title_slug: info.map(|i| i.display.title_slug.clone()).unwrap_or_default(),
        backdrop: info.and_then(|i| backdrop(images, i)),
        preparing,
        qualities: info.map(qualities).unwrap_or_default(),
        quality: session.quality.key(),
        audio: info
            .and_then(|i| i.audio.as_ref())
            .map(|tracks| {
                tracks
                    .iter()
                    .map(|t| TrackOption {
                        id: t.id.clone(),
                        lang: t.lang.clone(),
                        label: t.label.clone(),
                    })
                    .collect()
            })
            .unwrap_or_default(),
        audio_selected: session.audio.clone(),
        subtitles: info
            .map(|i| {
                i.subtitles
                    .iter()
                    .map(|s| TrackOption {
                        id: s.id.clone(),
                        lang: s.lang.clone(),
                        label: s.label.clone(),
                    })
                    .collect()
            })
            .unwrap_or_default(),
        subtitle_selected: session.subtitle.clone(),
        seasons: info.map(|i| seasons(i, &session.target, images)).unwrap_or_default(),
        next_up: session.next.up().cloned(),
        shuffle_available: info.is_some_and(|i| {
            i.allow_random_playback && i.episodes.as_ref().is_some_and(|e| e.len() > 1)
        }),
        shuffle,
        frame_url: info.map(|i| i.frame_url.clone()).filter(|u| !u.is_empty()),
        linear: session.linear,
        problem: session.problem.clone(),
    }
}

/// A backdrop for the loading screen and Now Playing.
pub(super) fn backdrop(images: &Images, info: &PlaybackInfo) -> Option<Image> {
    let d = &info.display;
    images.optional(d.backdrop_id.as_ref(), d.backdrop_ver, Size::Full, d.backdrop_accent.as_ref())
}

fn qualities(info: &PlaybackInfo) -> Vec<QualityOption> {
    let mut options = Vec::new();
    if info.mode == PlaybackInfoMode::Direct && info.stream_url.is_some() {
        options.push(QualityOption {
            key: Quality::Original.key(),
            kind: QualityKind::Original,
            height: None,
        });
    }
    let variants = info.variants.as_deref().unwrap_or_default();
    let ladder = info.hls_url.is_some() || info.mode == PlaybackInfoMode::Hls;
    if ladder && !variants.is_empty() {
        options.push(QualityOption {
            key: Quality::Auto.key(),
            kind: QualityKind::Auto,
            height: None,
        });
        for v in variants {
            options.push(QualityOption {
                key: v.name.clone(),
                kind: QualityKind::Rendition,
                height: u32::try_from(v.height).ok(),
            });
        }
    }
    options
}

fn seasons(info: &PlaybackInfo, current: &PlayTarget, images: &Images) -> Vec<PlayerSeason> {
    let mut seasons: Vec<PlayerSeason> = Vec::new();
    for e in info.episodes.as_deref().unwrap_or_default() {
        let number = u32::try_from(e.season_number).unwrap_or(0);
        let episode = PlayerEpisode {
            id: e.episode_id.clone(),
            number: u32::try_from(e.episode_number).unwrap_or(0),
            name: e.name.clone(),
            still: images.optional(e.thumb_id.as_ref(), e.thumb_ver, Size::Medium, None),
            current: e.episode_id == current.id,
        };
        match seasons.iter_mut().find(|s| s.number == number) {
            Some(season) => season.episodes.push(episode),
            None => seasons.push(PlayerSeason { number, episodes: vec![episode] }),
        }
    }
    seasons.sort_by_key(|s| s.number);
    seasons
}
