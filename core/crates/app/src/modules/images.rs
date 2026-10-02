//! Artwork URLs. Shells load images with their platform's image loader, which sends no session,
//! so a native client's URLs carry the account's artwork grant; the web's carry none and ride on
//! the cookie. The core picks the size for each role so every shell asks for the same files.

use couchverse_api::ops::{self, GetArtworkQuery};
use couchverse_api::types::GetArtworkSize;
use serde::{Deserialize, Serialize};
use typeshare::typeshare;

/// An image ready to load, with the accent colour extracted from it when known.
#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Image {
    pub url: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub accent: Option<String>,
}

/// How big an image is shown, which decides the file asked for.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Size {
    /// Poster cards and avatars.
    Small,
    /// Landscape cards, episode stills and the detail poster.
    Medium,
    /// Full-screen backdrops, where only the original is sharp on a 4K TV.
    Full,
}

/// Where artwork comes from for the active account.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Images {
    /// The server's base URL; empty for the web, whose URLs are origin-relative.
    pub base: String,
    pub grant: Option<String>,
}

impl Images {
    /// `version` busts caches when the artwork is replaced (the browser keeps versioned URLs
    /// forever).
    pub fn image(
        &self,
        id: &str,
        version: Option<String>,
        size: Size,
        accent: Option<String>,
    ) -> Image {
        let size = match size {
            Size::Small => Some(GetArtworkSize::W342),
            Size::Medium => Some(GetArtworkSize::W780),
            Size::Full => None,
        };
        let query = GetArtworkQuery { size, v: version, g: self.grant.clone() };
        let request = ops::get_artwork(id, &query);
        Image { url: format!("{}/api/v1{}", self.base, request.path_and_query()), accent }
    }

    /// For API payloads that carry an id, a numeric version and an accent side by side.
    pub fn optional(
        &self,
        id: Option<&String>,
        version: Option<i64>,
        size: Size,
        accent: Option<&String>,
    ) -> Option<Image> {
        id.map(|id| self.image(id, version.map(|v| v.to_string()), size, accent.cloned()))
    }
}

/// The artwork version token for a full artwork row: its `createdAt` in unix seconds, as the
/// web has always sent it.
pub fn version_of(created_at: &str) -> Option<String> {
    unix_seconds(created_at).map(|s| s.to_string())
}

/// Seconds since the epoch for an RFC 3339 timestamp (`2026-10-02T12:00:00.123+02:00`).
fn unix_seconds(timestamp: &str) -> Option<i64> {
    let (date, rest) = timestamp.split_once(['T', 't', ' '])?;
    let mut date = date.splitn(3, '-').map(str::parse::<i64>);
    let (year, month, day) = (date.next()?.ok()?, date.next()?.ok()?, date.next()?.ok()?);
    let time_end = rest.find(['Z', 'z', '+', '-']).unwrap_or(rest.len());
    let (time, zone) = rest.split_at(time_end);
    let time = time.split('.').next()?;
    let mut time = time.splitn(3, ':').map(str::parse::<i64>);
    let (hour, minute, second) = (time.next()?.ok()?, time.next()?.ok()?, time.next()?.ok()?);
    let offset = match zone.chars().next() {
        Some(sign @ ('+' | '-')) => {
            let (h, m) = zone[1..].split_once(':')?;
            let minutes = h.parse::<i64>().ok()? * 60 + m.parse::<i64>().ok()?;
            if sign == '+' { minutes * 60 } else { -minutes * 60 }
        }
        _ => 0,
    };
    Some(days_from_civil(year, month, day) * 86_400 + hour * 3600 + minute * 60 + second - offset)
}

/// Days since 1970-01-01 in the proleptic Gregorian calendar (Howard Hinnant's algorithm).
fn days_from_civil(year: i64, month: i64, day: i64) -> i64 {
    let year = if month <= 2 { year - 1 } else { year };
    let era = year.div_euclid(400);
    let year_of_era = year - era * 400;
    let month_index = (month + 9) % 12;
    let day_of_year = (153 * month_index + 2) / 5 + day - 1;
    let day_of_era = year_of_era * 365 + year_of_era / 4 - year_of_era / 100 + day_of_year;
    era * 146_097 + day_of_era - 719_468
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn timestamps_become_unix_seconds() {
        assert_eq!(unix_seconds("1970-01-01T00:00:00Z"), Some(0));
        assert_eq!(unix_seconds("2026-10-02T12:00:00Z"), Some(1_790_942_400));
        assert_eq!(unix_seconds("2026-10-02T14:00:00.987654+02:00"), Some(1_790_942_400));
        assert_eq!(unix_seconds("2000-02-29T23:59:59-01:30"), Some(951_874_199));
        assert_eq!(unix_seconds("not a date"), None);
    }

    #[test]
    fn urls_carry_size_version_and_grant() {
        let native = Images { base: "https://tv.home".into(), grant: Some("g1".into()) };
        let image = native.image("a1", Some("7".into()), Size::Small, Some("#112233".into()));
        assert_eq!(image.url, "https://tv.home/api/v1/artwork/a1?size=w342&v=7&g=g1");
        assert_eq!(image.accent.as_deref(), Some("#112233"));
        let web = Images::default();
        assert_eq!(web.image("a1", None, Size::Full, None).url, "/api/v1/artwork/a1");
    }
}
