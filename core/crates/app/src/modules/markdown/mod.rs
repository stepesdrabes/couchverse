//! User-authored markdown (profile bios) as a safe document tree that every client renders with
//! its own native views (D19). This is the security boundary: raw HTML is kept as plain text,
//! images are not fetched (they become links), only http(s) and mailto links survive, and the
//! tree has no way to express anything else.

mod blocks;
mod inlines;

use serde::{Deserialize, Serialize};
use typeshare::typeshare;

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct MarkdownDoc {
    pub blocks: Vec<Block>,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(tag = "type", content = "content", rename_all = "camelCase")]
pub enum Block {
    Paragraph(Vec<Inline>),
    Heading(HeadingBlock),
    Quote(Vec<Block>),
    List(ListBlock),
    /// Preformatted text, shown monospaced.
    Code(String),
    Rule,
    Table(TableBlock),
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct HeadingBlock {
    /// 1 to 6.
    pub level: u8,
    pub inlines: Vec<Inline>,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ListBlock {
    /// The first number of an ordered list; absent for a bulleted one.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub start: Option<u32>,
    pub items: Vec<ListItem>,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ListItem {
    pub blocks: Vec<Block>,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct TableBlock {
    pub header: Vec<TableCell>,
    pub rows: Vec<TableRow>,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct TableRow {
    pub cells: Vec<TableCell>,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct TableCell {
    pub inlines: Vec<Inline>,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(tag = "type", content = "content", rename_all = "camelCase")]
pub enum Inline {
    Text(String),
    Code(String),
    Strong(Vec<Inline>),
    Emphasis(Vec<Inline>),
    Strike(Vec<Inline>),
    /// Opens outside the app; shells add their equivalent of rel="nofollow noopener".
    Link(LinkInline),
    /// A line break: a single newline in a bio breaks the line, as in a chat message.
    Break,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct LinkInline {
    pub href: String,
    pub children: Vec<Inline>,
}

/// Parses `source` into a safe document: `CommonMark` blocks and inlines with GFM tables and
/// strikethrough, every newline a line break and bare URLs linked (the web's markdown-it preset
/// before the core took over), without raw HTML, images or reference links.
pub fn parse(source: &str) -> MarkdownDoc {
    MarkdownDoc { blocks: blocks::parse(source) }
}

/// Only absolute http(s) and mailto links leave a bio; `javascript:`, `data:`, relative paths
/// and everything else lose their link.
fn safe_href(url: &str) -> Option<String> {
    let url = url.trim();
    let lower = url.to_ascii_lowercase();
    let allowed = ["https://", "http://", "mailto:"].iter().any(|p| lower.starts_with(p));
    (allowed && !url.chars().any(char::is_control)).then(|| url.to_string())
}

#[cfg(test)]
mod tests {
    use super::*;

    fn text(s: &str) -> Inline {
        Inline::Text(s.to_string())
    }

    #[test]
    fn paragraphs_keep_single_newlines_as_breaks() {
        let doc = parse("I watch **sci-fi**\nand _horror_.");
        assert_eq!(
            doc.blocks,
            [Block::Paragraph(vec![
                text("I watch "),
                Inline::Strong(vec![text("sci-fi")]),
                Inline::Break,
                text("and "),
                Inline::Emphasis(vec![text("horror")]),
                text("."),
            ])]
        );
    }

    #[test]
    fn raw_html_stays_text() {
        let doc = parse("<script>alert(1)</script>\n\nhi <b>there</b>");
        let json = serde_json::to_string(&doc).unwrap();
        assert!(json.contains("<script>alert(1)</script>"), "{json}");
        assert!(!doc.blocks.is_empty());
        assert_eq!(doc.blocks[1], Block::Paragraph(vec![text("hi <b>there</b>")]));
    }

    #[test]
    fn unsafe_links_lose_their_href() {
        let doc = parse("[click](javascript:alert(1)) [ok](https://example.com) [rel](/admin)");
        assert_eq!(
            doc.blocks,
            [Block::Paragraph(vec![
                text("click "),
                Inline::Link(LinkInline {
                    href: "https://example.com".into(),
                    children: vec![text("ok")]
                }),
                text(" rel"),
            ])]
        );
    }

    #[test]
    fn images_become_links_and_are_never_fetched() {
        let doc = parse("![my cat](https://example.com/cat.jpg)");
        let Block::Paragraph(inlines) = &doc.blocks[0] else { panic!() };
        assert_eq!(
            inlines[0],
            Inline::Link(LinkInline {
                href: "https://example.com/cat.jpg".into(),
                children: vec![text("my cat")]
            })
        );
    }

    #[test]
    fn bare_urls_and_emails_are_linked() {
        let doc = parse("see https://couchverse.app or mail me@example.com");
        let Block::Paragraph(inlines) = &doc.blocks[0] else { panic!() };
        assert!(matches!(&inlines[1], Inline::Link(l) if l.href == "https://couchverse.app"));
        assert!(matches!(&inlines[3], Inline::Link(l) if l.href == "mailto:me@example.com"));
    }

    #[test]
    fn lists_quotes_headings_code_and_tables_nest() {
        let doc = parse(
            "# Hi\n\n> quoted\n\n1. one\n2. two\n\n```\nlet x = 1;\n```\n\n| a | b |\n|---|---|\n| 1 | 2 |",
        );
        assert!(matches!(&doc.blocks[0], Block::Heading(h) if h.level == 1));
        assert!(matches!(&doc.blocks[1], Block::Quote(q) if q.len() == 1));
        assert!(
            matches!(&doc.blocks[2], Block::List(l) if l.start == Some(1) && l.items.len() == 2)
        );
        assert_eq!(doc.blocks[3], Block::Code("let x = 1;".into()));
        assert!(
            matches!(&doc.blocks[4], Block::Table(t) if t.header.len() == 2 && t.rows.len() == 1)
        );
    }

    fn para(inlines: Vec<Inline>) -> Block {
        Block::Paragraph(inlines)
    }

    fn strong(children: Vec<Inline>) -> Inline {
        Inline::Strong(children)
    }

    fn em(children: Vec<Inline>) -> Inline {
        Inline::Emphasis(children)
    }

    fn link(href: &str, children: Vec<Inline>) -> Inline {
        Inline::Link(LinkInline { href: href.into(), children })
    }

    fn inlines(source: &str) -> Vec<Inline> {
        match parse(source).blocks.as_slice() {
            [Block::Paragraph(inlines)] => inlines.clone(),
            other => panic!("expected one paragraph, got {other:?}"),
        }
    }

    #[test]
    fn emphasis_follows_the_delimiter_rules() {
        assert_eq!(
            inlines("*a **b** c*"),
            [em(vec![text("a "), strong(vec![text("b")]), text(" c")])]
        );
        assert_eq!(inlines("***both***"), [em(vec![strong(vec![text("both")])])]);
        assert_eq!(inlines("**open*"), [text("*"), em(vec![text("open")])]);
        assert_eq!(inlines("snake_case_name"), [text("snake_case_name")]);
        assert_eq!(inlines("*foo*bar"), [em(vec![text("foo")]), text("bar")]);
        assert_eq!(inlines("a * b * c"), [text("a * b * c")]);
        // the rule of three keeps these two runs apart
        assert_eq!(
            inlines("*foo**bar**baz*"),
            [em(vec![text("foo"), strong(vec![text("bar")]), text("baz")])]
        );
        assert_eq!(
            inlines("__bold__ _it_"),
            [strong(vec![text("bold")]), text(" "), em(vec![text("it")])]
        );
    }

    #[test]
    fn strikethrough_takes_exactly_two_tildes() {
        assert_eq!(
            inlines("~~gone~~ ~kept~ ~~~three~~~"),
            [Inline::Strike(vec![text("gone")]), text(" ~kept~ ~~~three~~~")]
        );
    }

    #[test]
    fn code_spans_are_literal() {
        assert_eq!(inlines("`a *b* c`"), [Inline::Code("a *b* c".into())]);
        assert_eq!(inlines("`` a ` b ``"), [Inline::Code("a ` b".into())]);
        assert_eq!(inlines("`unclosed"), [text("`unclosed")]);
    }

    #[test]
    fn escapes_and_entities() {
        assert_eq!(inlines(r"\*not em\* \# a\b"), [text(r"*not em* # a\b")]);
        assert_eq!(
            inlines("&amp; &copy; &#65; &#x42; &bogus; & x"),
            [text("& \u{a9} A B &bogus; & x")]
        );
        assert_eq!(inlines("&#0;"), [text("\u{fffd}")]);
    }

    #[test]
    fn links_take_balanced_parens_titles_and_inner_markup() {
        assert_eq!(
            inlines("[**wiki**](https://en.wikipedia.org/wiki/Foo_(bar) \"title\")"),
            [link("https://en.wikipedia.org/wiki/Foo_(bar)", vec![strong(vec![text("wiki")])])]
        );
        assert_eq!(inlines("[a](<https://x.y/a b>)"), [link("https://x.y/a b", vec![text("a")])]);
        assert_eq!(inlines("[](https://x.y)"), [link("https://x.y", vec![text("https://x.y")])]);
        assert_eq!(inlines("[no target] [ref][x]"), [text("[no target] [ref][x]")]);
        // links cannot nest: the inner one wins
        assert_eq!(
            inlines("[a [b](https://b.c)](https://a.c)"),
            [text("[a "), link("https://b.c", vec![text("b")]), text("](https://a.c)")]
        );
    }

    #[test]
    fn autolinks_are_checked_like_links() {
        assert_eq!(
            inlines("<https://x.y/z> <me@x.y> <javascript:alert(1)> <b>"),
            [
                link("https://x.y/z", vec![text("https://x.y/z")]),
                text(" "),
                link("mailto:me@x.y", vec![text("me@x.y")]),
                text(" javascript:alert(1) <b>"),
            ]
        );
    }

    #[test]
    fn blocks_cover_headings_quotes_rules_and_code() {
        let doc = parse(
            "Title\n=====\n\nSub\n---\n\n## Closed ##\n#nospace\n\n***\n\n> a\nlazy\n> > deep\n\n~~~rust\nlet a = 1;\n~~~\n\n    indented\n    code",
        );
        assert_eq!(
            doc.blocks,
            [
                Block::Heading(HeadingBlock { level: 1, inlines: vec![text("Title")] }),
                Block::Heading(HeadingBlock { level: 2, inlines: vec![text("Sub")] }),
                Block::Heading(HeadingBlock { level: 2, inlines: vec![text("Closed")] }),
                para(vec![text("#nospace")]),
                Block::Rule,
                Block::Quote(vec![
                    para(vec![text("a"), Inline::Break, text("lazy")]),
                    Block::Quote(vec![para(vec![text("deep")])]),
                ]),
                Block::Code("let a = 1;".into()),
                Block::Code("indented\ncode".into()),
            ]
        );
        assert_eq!(parse("```\nnever closed\n").blocks, [Block::Code("never closed".into())]);
    }

    #[test]
    fn lists_nest_and_keep_their_start() {
        let doc = parse("3. three\n4. four\n   - nested\n     more\n\n5. loose\n\n- other list");
        let item = |blocks: Vec<Block>| ListItem { blocks };
        assert_eq!(
            doc.blocks,
            [
                Block::List(ListBlock {
                    start: Some(3),
                    items: vec![
                        item(vec![para(vec![text("three")])]),
                        item(vec![
                            para(vec![text("four")]),
                            Block::List(ListBlock {
                                start: None,
                                items: vec![item(vec![para(vec![
                                    text("nested"),
                                    Inline::Break,
                                    text("more")
                                ])])],
                            }),
                        ]),
                        item(vec![para(vec![text("loose")])]),
                    ],
                }),
                Block::List(ListBlock {
                    start: None,
                    items: vec![item(vec![para(vec![text("other list")])])]
                }),
            ]
        );
    }

    #[test]
    fn only_lists_starting_at_one_interrupt_a_paragraph() {
        assert_eq!(
            parse("In\n2019. was great").blocks,
            [para(vec![text("In"), Inline::Break, text("2019. was great")])]
        );
        assert!(matches!(&parse("Top\n1. one").blocks[1], Block::List(_)));
        assert!(matches!(&parse("- - -").blocks[0], Block::Rule));
    }

    #[test]
    fn tables_align_cells_to_the_header() {
        let doc = parse("| a | b \\| c |\n|:--|--:|\n| 1 | 2 | 3 |\n| **x** |");
        let cell =
            |s: &str| TableCell { inlines: if s.is_empty() { vec![] } else { vec![text(s)] } };
        assert_eq!(
            doc.blocks,
            [Block::Table(TableBlock {
                header: vec![cell("a"), cell("b | c")],
                rows: vec![
                    TableRow { cells: vec![cell("1"), cell("2")] },
                    TableRow {
                        cells: vec![TableCell { inlines: vec![strong(vec![text("x")])] }, cell("")]
                    },
                ],
            })]
        );
        // a delimiter row without pipes is a setext heading, not a table
        assert!(matches!(&parse("a | b\n---").blocks[0], Block::Heading(_)));
    }

    #[test]
    fn windows_newlines_and_tabs_read_the_same() {
        assert_eq!(parse("- a\r\n\t- b").blocks, parse("- a\n    - b").blocks);
    }

    #[test]
    fn hostile_input_stays_cheap_and_bounded() {
        for source in [
            "*".repeat(20_000),
            "*a".repeat(10_000),
            "[".repeat(10_000) + &"]".repeat(10_000),
            ">".repeat(10_000) + " deep",
            "- ".repeat(5_000) + "x",
            "`".repeat(10_000),
            "<".repeat(10_000),
        ] {
            let started = std::time::Instant::now();
            let doc = parse(&source);
            assert!(started.elapsed().as_secs() < 2, "slow on {}", &source[..10]);
            assert!(!doc.blocks.is_empty());
        }
    }
}
