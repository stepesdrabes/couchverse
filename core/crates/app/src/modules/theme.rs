//! Accent colours: every client derives the same palette from an artwork or site accent, so a
//! title page reads the same on a TV, a phone and the web. The parameters come from the shared
//! design tokens (`contract/design/tokens.json`).

use std::sync::OnceLock;

use serde::{Deserialize, Serialize};
use typeshare::typeshare;

/// The colours a surface derives from one accent, as `#rrggbb` (or `#rrggbbaa` for `soft`).
#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AccentPalette {
    pub accent: String,
    /// Darker, for pressed states and gradients.
    pub strong: String,
    /// A translucent tint for backgrounds.
    pub soft: String,
    /// Readable text on top of `accent` (near-white or near-black).
    pub on_accent: String,
    /// The accent as text on the app's surfaces: lightened just enough to reach the
    /// `inkContrast` ratio (WCAG AA) on the lightest of them.
    pub ink: String,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct AccentTokens {
    strong_shade: f64,
    soft_alpha: f64,
    on_accent_light: String,
    on_accent_dark: String,
    luminance_threshold: f64,
    ink_contrast: f64,
}

#[derive(Deserialize)]
struct Tokens {
    accent: AccentTokens,
    color: Colors,
}

#[derive(Deserialize)]
struct Colors {
    accent: String,
    #[serde(rename = "surface-2")]
    surface_2: String,
}

fn tokens() -> &'static Tokens {
    static TOKENS: OnceLock<Tokens> = OnceLock::new();
    TOKENS.get_or_init(|| {
        serde_json::from_str(include_str!("../../../../../contract/design/tokens.json"))
            .expect("contract/design/tokens.json is valid")
    })
}

/// The site's default accent, for servers that never set one.
pub fn default_accent() -> &'static str {
    &tokens().color.accent
}

/// The palette for `hex`; an unreadable colour falls back to the default accent.
pub fn palette(hex: &str) -> AccentPalette {
    let t = &tokens().accent;
    let rgb =
        parse_hex(hex).or_else(|| parse_hex(default_accent())).expect("default accent parses");
    let alpha = channel(t.soft_alpha * 255.0);
    AccentPalette {
        accent: to_hex(rgb),
        strong: to_hex(shade(rgb, t.strong_shade)),
        soft: format!("{}{alpha:02x}", to_hex(rgb)),
        on_accent: if luminance(rgb) > t.luminance_threshold {
            t.on_accent_dark.clone()
        } else {
            t.on_accent_light.clone()
        },
        ink: to_hex(ink(rgb, t.ink_contrast)),
    }
}

/// The least lightening (in steps of 1%) that gives `rgb` the contrast `ratio` on the
/// lightest surface; white when nothing short of it does.
fn ink(rgb: [u8; 3], ratio: f64) -> [u8; 3] {
    let surface = parse_hex(&tokens().color.surface_2).expect("surface-2 parses");
    (0..=100)
        .map(|step| shade(rgb, f64::from(step) / 100.0))
        .find(|c| contrast(*c, surface) >= ratio)
        .unwrap_or([255, 255, 255])
}

/// The WCAG contrast ratio between two colours (1 to 21).
fn contrast(a: [u8; 3], b: [u8; 3]) -> f64 {
    let (la, lb) = (luminance(a), luminance(b));
    (la.max(lb) + 0.05) / (la.min(lb) + 0.05)
}

fn parse_hex(hex: &str) -> Option<[u8; 3]> {
    let digits = hex.trim().strip_prefix('#').unwrap_or(hex.trim());
    if digits.len() != 6 {
        return None;
    }
    let value = u32::from_str_radix(digits, 16).ok()?;
    let [_, r, g, b] = value.to_be_bytes();
    Some([r, g, b])
}

fn to_hex([r, g, b]: [u8; 3]) -> String {
    format!("#{r:02x}{g:02x}{b:02x}")
}

/// Mixes toward black (`amount` < 0) or white (`amount` > 0), like the web's `shade`.
fn shade(rgb: [u8; 3], amount: f64) -> [u8; 3] {
    let target = if amount < 0.0 { 0.0 } else { 255.0 };
    let t = amount.abs();
    rgb.map(|c| {
        let c = f64::from(c);
        channel(c + (target - c) * t)
    })
}

#[allow(clippy::cast_possible_truncation, clippy::cast_sign_loss)]
fn channel(value: f64) -> u8 {
    // rounded and clamped into 0..=255 first, so the cast is exact
    value.round().clamp(0.0, 255.0) as u8
}

/// WCAG relative luminance (0 = black, 1 = white).
fn luminance(rgb: [u8; 3]) -> f64 {
    let lin = |c: u8| {
        let s = f64::from(c) / 255.0;
        if s <= 0.039_28 { s / 12.92 } else { ((s + 0.055) / 1.055).powf(2.4) }
    };
    let [r, g, b] = rgb;
    0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn the_default_red_matches_the_web() {
        // the web's applyAccent('#e50914') sets #b30710 and white text
        let p = palette("#e50914");
        assert_eq!(p.accent, "#e50914");
        assert_eq!(p.strong, "#b30710");
        assert_eq!(p.soft, "#e5091429");
        assert_eq!(p.on_accent, "#ffffff");
    }

    #[test]
    fn ink_reads_on_every_surface() {
        // the default accent's ink is the token every client starts with
        assert_eq!(palette("#e50914").ink, "#ec474f");
        assert_eq!(palette(default_accent()).ink, "#ec474f");
        assert_eq!(palette("#3a6ea5").ink, "#5b87b4");
        // light enough already: unchanged
        assert_eq!(palette("#facc15").ink, "#facc15");
    }

    #[test]
    fn bright_accents_get_dark_text() {
        assert_eq!(palette("#facc15").on_accent, "#0b0c10");
        assert_eq!(palette("#204060").on_accent, "#ffffff");
    }

    #[test]
    fn unreadable_colours_fall_back_to_the_default() {
        assert_eq!(palette("red").accent, "#e50914");
        assert_eq!(palette("#12").accent, "#e50914");
        assert_eq!(palette("3a6ea5").accent, "#3a6ea5");
    }
}
