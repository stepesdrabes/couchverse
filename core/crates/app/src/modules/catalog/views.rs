//! The catalog's view models and how they are built from API payloads. Views are built when a
//! shell reads them, from the cached payloads and the current artwork grant, so a renewed grant
//! or a My List change shows up without refetching.

use std::collections::{HashMap, HashSet};

use couchverse_api::types::{
    BrowseTitlesSort, CardItem, ContinueItem, FeaturedItem, Genre, Home, HomeRow,
    HomeRowKind as ApiRowKind, MediaFileVideoRange, TitleDetail,
};
use serde::{Deserialize, Serialize};
use typeshare::typeshare;

use crate::messages::{LoadStatus, Problem, U53};
use crate::modules::images::{Image, Images, Size, version_of};
use crate::modules::theme::{self, AccentPalette};

#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum TitleKind {
    Movie,
    Series,
}

impl TitleKind {
    pub fn parse(kind: &str) -> Self {
        if kind == "series" { TitleKind::Series } else { TitleKind::Movie }
    }
}

/// What a play button starts.
#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PlayTarget {
    pub kind: PlayKind,
    pub id: String,
}

#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum PlayKind {
    Movie,
    Episode,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Card {
    pub title_id: String,
    pub slug: String,
    pub name: String,
    pub kind: TitleKind,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub year: Option<i32>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub poster: Option<Image>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub backdrop: Option<Image>,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ContinueCard {
    pub title_id: String,
    pub slug: String,
    pub name: String,
    pub kind: TitleKind,
    /// The server's episode label (`S1 E3`); absent for a movie.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub episode_label: Option<String>,
    pub position_seconds: U53,
    pub duration_seconds: U53,
    /// How far in, from 0 to 1.
    pub progress: f64,
    pub play: PlayTarget,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub poster: Option<Image>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub backdrop: Option<Image>,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct FeaturedCard {
    pub title_id: String,
    pub slug: String,
    pub name: String,
    pub kind: TitleKind,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub year: Option<i32>,
    pub overview: String,
    /// Genre labels in the display language.
    pub genres: Vec<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub content_rating: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub runtime_minutes: Option<u32>,
    /// The full-size backdrop for a hero.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub backdrop: Option<Image>,
    pub in_list: bool,
}

#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum HomeRowKind {
    ContinueWatching,
    RecentlyAdded,
    Genre,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct HomeRowView {
    /// Stable within one home, for list identity.
    pub id: String,
    pub kind: HomeRowKind,
    pub label: String,
    pub cards: Vec<Card>,
    pub continue_watching: Vec<ContinueCard>,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct HomeView {
    pub status: LoadStatus,
    pub featured: Vec<FeaturedCard>,
    pub rows: Vec<HomeRowView>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub problem: Option<Problem>,
}

/// The highest resolution a title is available in.
#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum Quality {
    Sd,
    Hd720,
    Hd1080,
    Uhd,
}

impl Quality {
    fn of_height(height: i64) -> Option<Self> {
        match height {
            h if h >= 2000 => Some(Quality::Uhd),
            h if h >= 1000 => Some(Quality::Hd1080),
            h if h >= 700 => Some(Quality::Hd720),
            h if h > 0 => Some(Quality::Sd),
            _ => None,
        }
    }
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PlayAction {
    pub target: PlayTarget,
    /// Where playback resumes; absent to start from the beginning.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub resume_seconds: Option<U53>,
    /// The episode the button plays, for its label.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub episode: Option<EpisodeNumber>,
}

#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct EpisodeNumber {
    pub season: u32,
    pub episode: u32,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct EpisodeView {
    pub id: String,
    pub number: u32,
    pub name: String,
    pub overview: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub runtime_minutes: Option<u32>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub air_date: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub still: Option<Image>,
    /// How far in, from 0 to 1.
    pub progress: f64,
    pub completed: bool,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SeasonView {
    pub id: String,
    pub number: u32,
    pub name: String,
    pub overview: String,
    pub episodes: Vec<EpisodeView>,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct TitleDetailView {
    pub id: String,
    pub slug: String,
    pub name: String,
    pub kind: TitleKind,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub year: Option<i32>,
    pub overview: String,
    pub genres: Vec<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub content_rating: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub runtime_minutes: Option<u32>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub poster: Option<Image>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub backdrop: Option<Image>,
    /// The page's colours, from the backdrop.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub accent: Option<AccentPalette>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub quality: Option<Quality>,
    pub hdr: bool,
    pub in_list: bool,
    /// Absent for a series without a playable episode.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub play: Option<PlayAction>,
    /// A series that allows playing a random episode.
    pub shuffle: bool,
    /// Only seasons and episodes that have something to play.
    pub seasons: Vec<SeasonView>,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct TitleView {
    pub slug: String,
    pub status: LoadStatus,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub detail: Option<TitleDetailView>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub problem: Option<Problem>,
}

#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum BrowseSort {
    #[default]
    Added,
    Name,
    Year,
}

impl BrowseSort {
    pub fn api(self) -> BrowseTitlesSort {
        match self {
            BrowseSort::Added => BrowseTitlesSort::Added,
            BrowseSort::Name => BrowseTitlesSort::Name,
            BrowseSort::Year => BrowseTitlesSort::Year,
        }
    }
}

/// One browse listing: a kind, a genre (by its English name), or both.
#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Hash, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct BrowseKey {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub kind: Option<TitleKind>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub genre: Option<String>,
    /// Required, so every shell builds the same key the core echoes back in renders.
    pub sort: BrowseSort,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct BrowseView {
    pub key: BrowseKey,
    pub status: LoadStatus,
    pub cards: Vec<Card>,
    pub total: U53,
    /// More pages can be loaded with `BrowseMoreRequested`.
    pub more: bool,
    pub loading_more: bool,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub problem: Option<Problem>,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct GenreView {
    /// The English name, which identifies the genre in a `BrowseKey`.
    pub name: String,
    pub label: String,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct GenresView {
    pub status: LoadStatus,
    pub genres: Vec<GenreView>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub problem: Option<Problem>,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct MyListView {
    pub status: LoadStatus,
    pub cards: Vec<Card>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub problem: Option<Problem>,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SearchView {
    pub query: String,
    pub status: LoadStatus,
    pub cards: Vec<Card>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub problem: Option<Problem>,
}

/// My List changes not yet confirmed by the server, by title id.
pub type Listed = HashMap<String, bool>;

pub fn card(item: &CardItem, images: &Images) -> Card {
    Card {
        title_id: item.title_id.clone(),
        slug: item.slug.clone(),
        name: item.name.clone(),
        kind: TitleKind::parse(item.kind.as_str()),
        year: year(item.year),
        poster: images.optional(
            item.poster_id.as_ref(),
            item.poster_ver,
            Size::Small,
            item.poster_accent.as_ref(),
        ),
        backdrop: images.optional(
            item.backdrop_id.as_ref(),
            item.backdrop_ver,
            Size::Medium,
            item.backdrop_accent.as_ref(),
        ),
    }
}

pub fn cards(items: &[CardItem], images: &Images) -> Vec<Card> {
    items.iter().map(|item| card(item, images)).collect()
}

fn continue_card(item: &ContinueItem, images: &Images) -> ContinueCard {
    let position = u64::try_from(item.position_seconds).unwrap_or(0);
    let duration = u64::try_from(item.duration_seconds).unwrap_or(0);
    let play_kind =
        if item.playback_kind.as_str() == "episode" { PlayKind::Episode } else { PlayKind::Movie };
    ContinueCard {
        title_id: item.title_id.clone(),
        slug: item.slug.clone(),
        name: item.name.clone(),
        kind: TitleKind::parse(item.kind.as_str()),
        episode_label: Some(item.episode_label.clone()).filter(|l| !l.is_empty()),
        position_seconds: position,
        duration_seconds: duration,
        progress: fraction(position, duration),
        play: PlayTarget { kind: play_kind, id: item.playback_id.clone() },
        poster: images.optional(
            item.poster_id.as_ref(),
            item.poster_ver,
            Size::Small,
            item.poster_accent.as_ref(),
        ),
        backdrop: images.optional(
            item.backdrop_id.as_ref(),
            item.backdrop_ver,
            Size::Medium,
            item.backdrop_accent.as_ref(),
        ),
    }
}

fn featured(item: &FeaturedItem, images: &Images, listed: &Listed) -> FeaturedCard {
    FeaturedCard {
        title_id: item.id.clone(),
        slug: item.slug.clone(),
        name: item.name.clone(),
        kind: TitleKind::parse(item.kind.as_str()),
        year: year(item.year),
        overview: item.overview.clone(),
        genres: item.genre_labels.clone(),
        content_rating: Some(item.content_rating.clone()).filter(|r| !r.is_empty()),
        runtime_minutes: minutes(item.runtime_minutes),
        backdrop: images.optional(
            item.backdrop_id.as_ref(),
            item.backdrop_ver,
            Size::Full,
            item.backdrop_accent.as_ref(),
        ),
        in_list: listed.get(&item.id).copied().unwrap_or(item.in_list),
    }
}

fn row(index: usize, row: &HomeRow, images: &Images) -> Option<HomeRowView> {
    let kind = match row.kind {
        ApiRowKind::ContinueWatching => HomeRowKind::ContinueWatching,
        ApiRowKind::RecentlyAdded => HomeRowKind::RecentlyAdded,
        ApiRowKind::Genre => HomeRowKind::Genre,
        // a row kind from a newer server: skip it rather than show it wrong
        ApiRowKind::Unknown => return None,
    };
    Some(HomeRowView {
        id: format!("{index}-{}", row.label),
        kind,
        label: row.label.clone(),
        cards: cards(&row.items, images),
        continue_watching: row.continue_watching.iter().map(|c| continue_card(c, images)).collect(),
    })
}

pub fn home(
    status: LoadStatus,
    home: Option<&Home>,
    problem: Option<Problem>,
    images: &Images,
    listed: &Listed,
) -> HomeView {
    let (featured, rows) = home.map_or_else(
        || (vec![], vec![]),
        |h| {
            let featured = h.featured.iter().map(|f| featured(f, images, listed)).collect();
            let rows = h
                .rows
                .iter()
                .enumerate()
                .filter_map(|(i, r)| row(i, r, images))
                .filter(|r| !r.cards.is_empty() || !r.continue_watching.is_empty())
                .collect();
            (featured, rows)
        },
    );
    HomeView { status, featured, rows, problem }
}

pub fn title(detail: &TitleDetail, images: &Images, listed: &Listed) -> TitleDetailView {
    let t = &detail.title;
    let artwork = |kind: &str| detail.artwork.iter().find(|a| a.kind.as_str() == kind);
    let image = |kind: &str, size: Size| {
        artwork(kind)
            .map(|a| images.image(&a.id, version_of(&a.created_at), size, a.accent.clone()))
    };
    let backdrop = image("backdrop", Size::Full);
    let kind = TitleKind::parse(t.kind.as_str());

    // only episodes with a file to play are shown, and seasons left empty by that are dropped
    let playable: HashSet<&str> =
        detail.media_files.iter().filter_map(|f| f.episode_id.as_deref()).collect();
    let seasons: Vec<SeasonView> = detail
        .seasons
        .iter()
        .map(|s| SeasonView {
            id: s.id.clone(),
            number: count(s.season_number),
            name: s.name.clone(),
            overview: s.overview.clone(),
            episodes: s
                .episodes
                .iter()
                .filter(|e| playable.contains(e.id.as_str()))
                .map(|e| {
                    let progress = detail.episode_progress.get(&e.id);
                    EpisodeView {
                        id: e.id.clone(),
                        number: count(e.episode_number),
                        name: e.name.clone(),
                        overview: e.overview.clone(),
                        runtime_minutes: minutes(e.runtime_minutes),
                        air_date: e.air_date.clone(),
                        still: images.optional(
                            e.thumb_id.as_ref(),
                            e.thumb_ver,
                            Size::Medium,
                            None,
                        ),
                        progress: progress.map_or(0.0, |p| {
                            if p.completed {
                                1.0
                            } else {
                                fraction(seconds(p.position_seconds), seconds(p.duration_seconds))
                            }
                        }),
                        completed: progress.is_some_and(|p| p.completed),
                    }
                })
                .collect(),
        })
        .filter(|s| !s.episodes.is_empty())
        .collect();

    let play = match kind {
        TitleKind::Movie => Some(PlayAction {
            target: PlayTarget { kind: PlayKind::Movie, id: t.id.clone() },
            resume_seconds: detail.progress.as_ref().and_then(|p| resume(p.position_seconds)),
            episode: None,
        }),
        // the first episode not watched to the end
        TitleKind::Series => seasons.iter().find_map(|s| {
            s.episodes.iter().find(|e| !e.completed).map(|e| PlayAction {
                target: PlayTarget { kind: PlayKind::Episode, id: e.id.clone() },
                resume_seconds: detail
                    .episode_progress
                    .get(&e.id)
                    .and_then(|p| resume(p.position_seconds)),
                episode: Some(EpisodeNumber { season: s.number, episode: e.number }),
            })
        }),
    };
    let max_height = detail.media_files.iter().map(|f| f.height).max().unwrap_or(0);

    TitleDetailView {
        id: t.id.clone(),
        slug: t.slug.clone(),
        name: t.name.clone(),
        kind,
        year: year(t.year),
        overview: t.overview.clone(),
        genres: t.genre_labels.clone(),
        content_rating: Some(t.content_rating.clone()).filter(|r| !r.is_empty()),
        runtime_minutes: minutes(t.runtime_minutes),
        poster: image("poster", Size::Medium),
        accent: backdrop.as_ref().and_then(|b| b.accent.as_deref()).map(theme::palette),
        backdrop,
        quality: Quality::of_height(max_height),
        hdr: detail.media_files.iter().any(|f| {
            !matches!(f.video_range, MediaFileVideoRange::Sdr | MediaFileVideoRange::Unknown)
        }),
        in_list: listed.get(&t.id).copied().unwrap_or(detail.in_watchlist),
        shuffle: kind == TitleKind::Series
            && t.allow_random_playback
            && seasons.iter().any(|s| !s.episodes.is_empty()),
        play,
        seasons,
    }
}

pub fn genres(
    status: LoadStatus,
    genres: Option<&[Genre]>,
    problem: Option<Problem>,
) -> GenresView {
    GenresView {
        status,
        genres: genres
            .unwrap_or_default()
            .iter()
            .map(|g| GenreView { name: g.name.clone(), label: g.label.clone() })
            .collect(),
        problem,
    }
}

/// Resuming only makes sense past the first few seconds, as on the web.
fn resume(position: i64) -> Option<U53> {
    u64::try_from(position).ok().filter(|p| *p > 10)
}

fn fraction(position: u64, duration: u64) -> f64 {
    if duration == 0 {
        return 0.0;
    }
    #[allow(clippy::cast_precision_loss)] // seconds of video stay far below 2^52
    let fraction = position as f64 / duration as f64;
    fraction.clamp(0.0, 1.0)
}

fn seconds(value: i64) -> u64 {
    u64::try_from(value).unwrap_or(0)
}

fn count(value: i64) -> u32 {
    u32::try_from(value).unwrap_or(0)
}

fn minutes(value: Option<i64>) -> Option<u32> {
    value.and_then(|m| u32::try_from(m).ok()).filter(|m| *m > 0)
}

fn year(value: Option<i64>) -> Option<i32> {
    value.and_then(|y| i32::try_from(y).ok())
}
