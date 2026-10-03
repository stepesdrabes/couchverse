//! Properties that must hold for any input, not just the cases the unit and scenario tests
//! pick: the markdown boundary stays safe, time text round-trips, accent text stays readable
//! and the bridge refuses garbage instead of panicking.

use proptest::prelude::*;

use crate::modules::markdown::{self, Block, Inline};
use crate::modules::theme;
use crate::{Bridge, time};

/// Every link the document holds, at any depth.
fn links(blocks: &[Block]) -> Vec<String> {
    fn inline(out: &mut Vec<String>, inlines: &[Inline]) {
        for i in inlines {
            match i {
                Inline::Link(link) => {
                    out.push(link.href.clone());
                    inline(out, &link.children);
                }
                Inline::Strong(c) | Inline::Emphasis(c) | Inline::Strike(c) => inline(out, c),
                Inline::Text(_) | Inline::Code(_) | Inline::Break => {}
            }
        }
    }
    fn block(out: &mut Vec<String>, blocks: &[Block]) {
        for b in blocks {
            match b {
                Block::Paragraph(i) => inline(out, i),
                Block::Heading(h) => inline(out, &h.inlines),
                Block::Quote(b) => block(out, b),
                Block::List(l) => l.items.iter().for_each(|item| block(out, &item.blocks)),
                Block::Table(t) => {
                    t.header.iter().for_each(|c| inline(out, &c.inlines));
                    t.rows.iter().flat_map(|r| &r.cells).for_each(|c| inline(out, &c.inlines));
                }
                Block::Code(_) | Block::Rule => {}
            }
        }
    }
    let mut out = vec![];
    block(&mut out, blocks);
    out
}

/// WCAG 2 contrast, written out again here so the property does not test the code with itself.
fn contrast(fg: &str, bg: &str) -> f64 {
    let luminance = |hex: &str| {
        let value = u32::from_str_radix(&hex[1..7], 16).expect("hex");
        let [_, red, green, blue] = value.to_be_bytes();
        let linear = |channel: u8| {
            let srgb = f64::from(channel) / 255.0;
            if srgb <= 0.039_28 { srgb / 12.92 } else { ((srgb + 0.055) / 1.055).powf(2.4) }
        };
        0.2126 * linear(red) + 0.7152 * linear(green) + 0.0722 * linear(blue)
    };
    let (light, dark) = (luminance(fg).max(luminance(bg)), luminance(fg).min(luminance(bg)));
    (light + 0.05) / (dark + 0.05)
}

proptest! {
    #![proptest_config(ProptestConfig::with_cases(256))]

    /// Whatever a member writes in a bio, links only ever leave for the web or mail.
    #[test]
    fn markdown_links_are_always_safe(source in "\\PC{0,400}") {
        let doc = markdown::parse(&source);
        for href in links(&doc.blocks) {
            let lower = href.to_ascii_lowercase();
            prop_assert!(
                lower.starts_with("http://") || lower.starts_with("https://") || lower.starts_with("mailto:"),
                "unsafe link {href:?} from {source:?}"
            );
        }
    }

    /// Markdown made of the characters that drive its parser never panics and keeps text.
    #[test]
    fn markdown_survives_its_own_syntax(source in "[*_~`\\[\\]()>#|\\-+0-9. \\na-z:/!<]{0,300}") {
        let doc = markdown::parse(&source);
        if !source.trim().is_empty() {
            prop_assert!(!doc.blocks.is_empty());
        }
    }

    #[test]
    fn wall_clock_text_round_trips(seconds in 0u64..253_402_300_799) {
        let text = time::rfc3339(seconds * 1000 + 999);
        prop_assert_eq!(time::unix_seconds(&text), Some(i64::try_from(seconds).unwrap()));
    }

    /// Any accent's ink reads as text on every surface, and only ever lightens the accent.
    #[test]
    fn accent_ink_is_readable(red in 0u8.., green in 0u8.., blue in 0u8..) {
        let accent = format!("#{red:02x}{green:02x}{blue:02x}");
        let palette = theme::palette(&accent);
        for surface in ["#07080d", "#11131c", "#191c27"] {
            prop_assert!(contrast(&palette.ink, surface) >= 4.5, "{} on {surface}", palette.ink);
        }
        let channels = |hex: &str| u32::from_str_radix(&hex[1..7], 16).unwrap().to_be_bytes();
        let (ink, base) = (channels(&palette.ink), channels(&accent));
        prop_assert!(ink.iter().zip(base.iter()).all(|(i, a)| i >= a));
    }

    /// A shell bug sends garbage: the bridge says so instead of panicking.
    #[test]
    fn the_bridge_refuses_garbage(message in "\\PC{0,200}") {
        let mut bridge = Bridge::new(
            r#"{"platform": "ios", "authMode": "bearer", "deviceName": "Phone", "locale": "en"}"#,
        )
        .expect("config");
        let _ = bridge.send(&message);
        let _ = bridge.resolve(&message);
        let _ = bridge.view(&message);
    }
}
