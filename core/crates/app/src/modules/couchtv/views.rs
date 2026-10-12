//! Couch TV's view models: the live channel, the guide and the home screen's card. Every time in
//! them is Unix milliseconds; shells format them in the viewer's time zone.

use couchverse_api::types::{CouchTvListing, CouchTvListingKind, CouchTvListingTitleKind};
use serde::{Deserialize, Serialize};
use typeshare::typeshare;

use crate::messages::{LoadStatus, Problem, U53};
use crate::modules::catalog::{PlayKind, PlayTarget};
use crate::modules::images::{Image, Images, Size};
use crate::time;

#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum CouchTvStatus {
    #[default]
    Idle,
    /// Tuning in, or the player has not shown a picture yet.
    Tuning,
    /// The picture plays; `CouchTvView.now` says what it shows.
    OnAir,
    /// Nothing is on air.
    OffAir,
    /// The channel cannot be played (`CouchTvView.problem` says why).
    Failed,
}

/// The room's socket: presence and reactions.
#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum CouchTvConnection {
    #[default]
    Offline,
    Connecting,
    Open,
    /// The socket dropped; the core reconnects with backoff.
    Reconnecting,
}

#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum CouchTvItemKind {
    Program,
    /// Between two programs: idents, promos and up next, drawn by the broadcast itself.
    Break,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CouchTvEpisode {
    pub season: u32,
    pub number: u32,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub name: Option<String>,
}

/// An item of the schedule: a program with its title, or a break.
#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CouchTvProgram {
    pub id: String,
    pub kind: CouchTvItemKind,
    pub starts_at_ms: U53,
    pub ends_at_ms: U53,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub title: Option<String>,
    /// The title page.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub title_slug: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub episode: Option<CouchTvEpisode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub overview: Option<String>,
    /// The rule that chose the program (`premiere`, `saga`, `strip`...), for a badge.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub fill: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub poster: Option<Image>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub backdrop: Option<Image>,
    /// "Watch from the start" in the on-demand player: signed-in viewers only.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub play: Option<PlayTarget>,
}

/// The live overlays (D14), for what the picture shows: over a program the bug stays up all the
/// time and the captions are there to show with the shell's controls; over a break there are
/// none, since breaks carry the channel's own graphics.
#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
#[allow(clippy::struct_excessive_bools)] // a view model mirrors the flags a screen shows
pub struct CouchTvCues {
    /// The corner bug with LIVE and the clock.
    pub bug: bool,
    /// The "Now" caption: the program on screen.
    pub now_caption: bool,
    /// The "Next" caption: the program after it, once the lineup knows one.
    pub next_caption: bool,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CouchTvMember {
    pub id: String,
    pub display_name: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub avatar: Option<Image>,
    /// Seeds a generated avatar for members without one.
    pub seed: String,
    /// Watches through the channel link without an account.
    pub guest: bool,
    pub me: bool,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CouchTvReaction {
    pub id: U53,
    pub viewer_id: String,
    pub emoji: String,
}

/// An audio or subtitle language of the channel; its id is the language code.
#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CouchTvTrack {
    pub id: String,
    pub lang: String,
    pub selected: bool,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CouchTvView {
    pub status: CouchTvStatus,
    pub connection: CouchTvConnection,
    /// What the picture shows, by its program date, not the wall clock.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub now: Option<CouchTvProgram>,
    /// The next program after it.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub next: Option<CouchTvProgram>,
    pub later: Vec<CouchTvProgram>,
    /// The picture's program date when last reported, at `programDateAtMs` on the shell's clock:
    /// the bug's clock and the progress line run on from it while the picture plays.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub program_date_ms: Option<U53>,
    pub program_date_at_ms: U53,
    pub cues: CouchTvCues,
    /// Everyone watching, each viewer once.
    pub viewers: u32,
    /// The first of them, in the order they tuned in.
    pub members: Vec<CouchTvMember>,
    pub reactions: Vec<CouchTvReaction>,
    pub audio: Vec<CouchTvTrack>,
    pub subtitles: Vec<CouchTvTrack>,
    /// The name this device watches under as a guest; absent when signed in or not joined.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub guest: Option<String>,
    /// Joining through the channel link.
    pub joining: bool,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub problem: Option<Problem>,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CouchTvGuideDay {
    /// The broadcast day's date in the channel's time zone (`2026-10-12`).
    pub date: String,
    /// From its first anchor (06:00) to the next day's.
    pub starts_at_ms: U53,
    pub ends_at_ms: U53,
    pub programs: Vec<CouchTvProgram>,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CouchTvGuideView {
    pub status: LoadStatus,
    /// Today's broadcast day and the ones after it.
    pub days: Vec<CouchTvGuideDay>,
    /// The channel's time zone (IANA), in which its days run.
    pub time_zone: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub problem: Option<Problem>,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CouchTvOnNowView {
    pub status: LoadStatus,
    /// What is on: a program or a break.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub now: Option<CouchTvProgram>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub next: Option<CouchTvProgram>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub problem: Option<Problem>,
}

/// A schedule item as the core keeps it: the API's listing with its times read.
#[derive(Debug, Clone, PartialEq)]
pub(super) struct Listing {
    pub api: CouchTvListing,
    pub starts: i64,
    pub ends: i64,
}

impl Listing {
    pub fn new(api: CouchTvListing) -> Option<Self> {
        let starts = time::unix_ms(&api.starts_at)?;
        let ends = time::unix_ms(&api.ends_at)?;
        Some(Self { api, starts, ends })
    }

    /// A break between two programs that the lineup does not list itself.
    pub fn between(before: &Listing, after: &Listing) -> Self {
        let api = CouchTvListing {
            id: format!("break-{}", after.api.id),
            kind: CouchTvListingKind::Break,
            starts_at: before.api.ends_at.clone(),
            ends_at: after.api.starts_at.clone(),
            backdrop_id: None,
            backdrop_ver: None,
            episode_id: None,
            episode_name: None,
            episode_number: None,
            fill: None,
            overview: None,
            poster_id: None,
            poster_ver: None,
            season_number: None,
            title_id: None,
            title_kind: None,
            title_name: None,
            title_slug: None,
        };
        Self { api, starts: before.ends, ends: after.starts }
    }

    pub fn program(&self) -> bool {
        self.api.kind != CouchTvListingKind::Break
    }

    /// The view of it; `signed_in` offers watching it from the start.
    pub fn view(&self, images: &Images, signed_in: bool) -> CouchTvProgram {
        let l = &self.api;
        let program = self.program();
        let play = match (&l.episode_id, &l.title_id, l.title_kind) {
            _ if !signed_in || !program => None,
            (Some(episode), _, _) => {
                Some(PlayTarget { kind: PlayKind::Episode, id: episode.clone() })
            }
            (None, Some(title), Some(CouchTvListingTitleKind::Movie)) => {
                Some(PlayTarget { kind: PlayKind::Movie, id: title.clone() })
            }
            _ => None,
        };
        CouchTvProgram {
            id: l.id.clone(),
            kind: if program { CouchTvItemKind::Program } else { CouchTvItemKind::Break },
            starts_at_ms: ms(self.starts),
            ends_at_ms: ms(self.ends),
            title: l.title_name.clone().filter(|_| program),
            title_slug: l.title_slug.clone(),
            episode: l.episode_id.as_ref().map(|_| CouchTvEpisode {
                season: count(l.season_number),
                number: count(l.episode_number),
                name: l.episode_name.clone().filter(|n| !n.is_empty()),
            }),
            overview: l.overview.clone().filter(|o| !o.is_empty()),
            fill: l.fill.map(|f| f.as_str().to_string()),
            poster: images.optional(l.poster_id.as_ref(), l.poster_ver, Size::Small, None),
            backdrop: images.optional(l.backdrop_id.as_ref(), l.backdrop_ver, Size::Full, None),
            play,
        }
    }
}

fn ms(value: i64) -> U53 {
    U53::try_from(value).unwrap_or_default()
}

fn count(value: Option<i64>) -> u32 {
    value.and_then(|v| u32::try_from(v).ok()).unwrap_or_default()
}

/// What the picture shows at `at` and what follows, from a lineup in order: the listing
/// covering it, else the break between the two programs around it.
pub(super) fn on_screen(lineup: &[Listing], at: i64) -> (Option<Listing>, Vec<&Listing>) {
    let now = lineup.iter().find(|l| l.starts <= at && at < l.ends).cloned().or_else(|| {
        lineup.windows(2).find_map(|pair| {
            (pair[0].ends <= at && at < pair[1].starts)
                .then(|| Listing::between(&pair[0], &pair[1]))
        })
    });
    let after = now.as_ref().map_or(at, |n| n.ends);
    let coming = lineup.iter().filter(|l| l.program() && l.starts >= after).collect();
    (now, coming)
}

/// The overlays over what is on screen; `followed` says a program comes after it.
pub(super) fn cues(now: Option<&Listing>, followed: bool) -> CouchTvCues {
    let program = now.is_some_and(Listing::program);
    CouchTvCues { bug: program, now_caption: program, next_caption: program && followed }
}

#[cfg(test)]
mod tests {
    use serde_json::json;

    use super::*;

    fn listing(id: &str, kind: &str, starts: &str, ends: &str) -> Listing {
        let api: CouchTvListing = serde_json::from_value(json!({
            "id": id, "kind": kind, "startsAt": starts, "endsAt": ends, "titleName": id,
        }))
        .expect("a listing");
        Listing::new(api).expect("its times")
    }

    fn at(timestamp: &str) -> i64 {
        time::unix_ms(timestamp).expect("a timestamp")
    }

    fn ids((now, coming): (Option<Listing>, Vec<&Listing>)) -> (Option<String>, Vec<String>) {
        (now.map(|n| n.api.id), coming.iter().map(|l| l.api.id.clone()).collect())
    }

    #[test]
    fn the_picture_finds_its_place_in_the_lineup() {
        let lineup = [
            listing("brk", "break", "2026-10-12T18:00:00Z", "2026-10-12T18:05:00Z"),
            listing("b", "program", "2026-10-12T18:05:00Z", "2026-10-12T18:27:00.500Z"),
            listing("c", "program", "2026-10-12T18:30:00Z", "2026-10-12T18:52:00Z"),
        ];
        // a break the tune-in listed, then the programs after it
        assert_eq!(
            ids(on_screen(&lineup, at("2026-10-12T18:01:00Z"))),
            (Some("brk".into()), vec!["b".into(), "c".into()])
        );
        // the break between two programs, which no listing names, to the millisecond
        assert_eq!(ids(on_screen(&lineup, at("2026-10-12T18:27:00.400Z"))).0, Some("b".into()));
        assert_eq!(
            ids(on_screen(&lineup, at("2026-10-12T18:27:00.500Z"))),
            (Some("break-c".into()), vec!["c".into()])
        );
        // past what is known: nothing to say
        assert_eq!(ids(on_screen(&lineup, at("2026-10-12T19:00:00Z"))), (None, vec![]));
    }

    #[test]
    fn the_overlays_stay_up_over_programs() {
        let b = listing("b", "program", "2026-10-12T18:05:00Z", "2026-10-12T18:27:00Z");
        let flags = |c: CouchTvCues| (c.bug, c.now_caption, c.next_caption);
        assert_eq!(flags(cues(Some(&b), true)), (true, true, true));
        // the last program the lineup knows has nothing to name next
        assert_eq!(flags(cues(Some(&b), false)), (true, true, false));
        let brk = listing("brk", "break", "2026-10-12T18:00:00Z", "2026-10-12T18:05:00Z");
        assert_eq!(cues(Some(&brk), true), CouchTvCues::default());
        assert_eq!(cues(None, true), CouchTvCues::default());
    }
}
