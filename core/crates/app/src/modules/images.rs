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
    crate::time::unix_seconds(created_at).map(|s| s.to_string())
}

#[cfg(test)]
mod tests {
    use super::*;

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
