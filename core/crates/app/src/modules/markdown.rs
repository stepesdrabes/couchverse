//! User-authored markdown (profile bios) as a safe document tree that every client renders with
//! its own native views (D19). This is the security boundary: raw HTML is kept as plain text,
//! images are not fetched (they become links), only http(s) and mailto links survive, and the
//! tree has no way to express anything else.

use pulldown_cmark::{Event, HeadingLevel, Options, Parser, Tag};
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

/// Parses `source` (`CommonMark` with tables and strikethrough) into a safe document.
pub fn parse(source: &str) -> MarkdownDoc {
    let options =
        Options::ENABLE_TABLES | Options::ENABLE_STRIKETHROUGH | Options::ENABLE_SMART_PUNCTUATION;
    let mut builder = Builder::default();
    for event in Parser::new_ext(source, options) {
        builder.event(event);
    }
    MarkdownDoc { blocks: builder.finish() }
}

/// One open container while the event stream is folded into a tree.
enum Frame {
    Blocks(Vec<Block>),
    Paragraph(Vec<Inline>),
    Heading(u8, Vec<Inline>),
    Quote(Vec<Block>),
    List(Option<u32>, Vec<ListItem>),
    Item(Vec<Block>, Vec<Inline>),
    Code(String),
    Table(Vec<TableCell>, Vec<TableRow>),
    Row(Vec<TableCell>),
    Cell(Vec<Inline>),
    Strong(Vec<Inline>),
    Emphasis(Vec<Inline>),
    Strike(Vec<Inline>),
    /// A link (or an image shown as one); `None` when its URL is not allowed.
    Link(Option<String>, Vec<Inline>),
}

struct Builder {
    stack: Vec<Frame>,
}

impl Default for Builder {
    fn default() -> Self {
        Self { stack: vec![Frame::Blocks(vec![])] }
    }
}

impl Builder {
    fn finish(mut self) -> Vec<Block> {
        while self.stack.len() > 1 {
            self.close();
        }
        match self.stack.pop() {
            Some(Frame::Blocks(blocks)) => blocks,
            _ => vec![],
        }
    }

    fn event(&mut self, event: Event) {
        match event {
            Event::Start(tag) => self.open(tag),
            Event::End(_) => self.close(),
            Event::Text(text) => self.text(&text),
            Event::Code(code) => self.inline(Inline::Code(code.into_string())),
            // HTML is shown as written, never interpreted
            Event::Html(html) | Event::InlineHtml(html) => self.text(&html),
            Event::SoftBreak | Event::HardBreak => self.inline(Inline::Break),
            Event::Rule => self.block(Block::Rule),
            _ => {}
        }
    }

    fn open(&mut self, tag: Tag) {
        let frame = match tag {
            Tag::Heading { level, .. } => Frame::Heading(heading_level(level), vec![]),
            Tag::BlockQuote(_) => Frame::Quote(vec![]),
            Tag::List(start) => {
                Frame::List(start.map(|n| u32::try_from(n).unwrap_or(u32::MAX)), vec![])
            }
            Tag::Item => Frame::Item(vec![], vec![]),
            Tag::CodeBlock(_) => Frame::Code(String::new()),
            Tag::Table(_) => Frame::Table(vec![], vec![]),
            Tag::TableHead | Tag::TableRow => Frame::Row(vec![]),
            Tag::TableCell => Frame::Cell(vec![]),
            Tag::Strong => Frame::Strong(vec![]),
            Tag::Emphasis => Frame::Emphasis(vec![]),
            Tag::Strikethrough => Frame::Strike(vec![]),
            Tag::Link { dest_url, .. } | Tag::Image { dest_url, .. } => {
                Frame::Link(safe_href(&dest_url), vec![])
            }
            // paragraphs, and anything unsupported (footnotes, metadata) read as one
            _ => Frame::Paragraph(vec![]),
        };
        self.stack.push(frame);
    }

    fn close(&mut self) {
        // the root frame is only taken by `finish`, whatever the event stream says
        if self.stack.len() < 2 {
            return;
        }
        let Some(frame) = self.stack.pop() else {
            return;
        };
        match frame {
            Frame::Blocks(blocks) => blocks.into_iter().for_each(|b| self.block(b)),
            Frame::Paragraph(inlines) => {
                if !inlines.is_empty() {
                    self.block(Block::Paragraph(inlines));
                }
            }
            Frame::Heading(level, inlines) => {
                self.block(Block::Heading(HeadingBlock { level, inlines }));
            }
            Frame::Quote(blocks) => self.block(Block::Quote(blocks)),
            Frame::List(start, items) => self.block(Block::List(ListBlock { start, items })),
            Frame::Item(mut blocks, inlines) => {
                // tight list items hold text directly, without a paragraph
                if !inlines.is_empty() {
                    blocks.insert(0, Block::Paragraph(inlines));
                }
                if let Some(Frame::List(_, items)) = self.stack.last_mut() {
                    items.push(ListItem { blocks });
                }
            }
            Frame::Code(text) => self.block(Block::Code(text.trim_end_matches('\n').to_string())),
            Frame::Table(header, rows) => self.block(Block::Table(TableBlock { header, rows })),
            Frame::Row(cells) => match self.stack.last_mut() {
                Some(Frame::Table(header, _)) if header.is_empty() => *header = cells,
                Some(Frame::Table(_, rows)) => rows.push(TableRow { cells }),
                _ => {}
            },
            Frame::Cell(inlines) => {
                if let Some(Frame::Row(cells)) = self.stack.last_mut() {
                    cells.push(TableCell { inlines });
                }
            }
            Frame::Strong(children) => self.inline(Inline::Strong(children)),
            Frame::Emphasis(children) => self.inline(Inline::Emphasis(children)),
            Frame::Strike(children) => self.inline(Inline::Strike(children)),
            Frame::Link(Some(href), children) => {
                self.inline(Inline::Link(LinkInline { href, children }));
            }
            // a disallowed link keeps its text and loses the link
            Frame::Link(None, children) => children.into_iter().for_each(|c| self.inline(c)),
        }
    }

    fn block(&mut self, block: Block) {
        match self.stack.last_mut() {
            Some(Frame::Blocks(blocks) | Frame::Quote(blocks)) => blocks.push(block),
            Some(Frame::Item(blocks, inlines)) => {
                if !inlines.is_empty() {
                    blocks.push(Block::Paragraph(std::mem::take(inlines)));
                }
                blocks.push(block);
            }
            _ => {}
        }
    }

    fn inline(&mut self, inline: Inline) {
        match self.stack.last_mut() {
            Some(
                Frame::Paragraph(inlines)
                | Frame::Heading(_, inlines)
                | Frame::Item(_, inlines)
                | Frame::Cell(inlines)
                | Frame::Strong(inlines)
                | Frame::Emphasis(inlines)
                | Frame::Strike(inlines)
                | Frame::Link(_, inlines),
            ) => match (inlines.last_mut(), inline) {
                // the parser splits text at every entity and HTML tag; shells get one run
                (Some(Inline::Text(last)), Inline::Text(more)) => last.push_str(&more),
                (_, inline) => inlines.push(inline),
            },
            Some(Frame::Code(text)) => {
                if let Inline::Text(t) = inline {
                    text.push_str(&t);
                }
            }
            _ => {}
        }
    }

    /// Plain text; outside links and code, bare URLs and email addresses become links.
    fn text(&mut self, text: &str) {
        let in_link = self.stack.iter().any(|f| matches!(f, Frame::Link(..)));
        if in_link || matches!(self.stack.last(), Some(Frame::Code(_))) {
            self.inline(Inline::Text(text.to_string()));
            return;
        }
        for span in linkify::LinkFinder::new().spans(text) {
            let piece = span.as_str().to_string();
            let href = match span.kind() {
                Some(linkify::LinkKind::Url) => safe_href(&piece),
                Some(linkify::LinkKind::Email) => Some(format!("mailto:{piece}")),
                _ => None,
            };
            match href {
                Some(href) => self
                    .inline(Inline::Link(LinkInline { href, children: vec![Inline::Text(piece)] })),
                None => self.inline(Inline::Text(piece)),
            }
        }
    }
}

fn heading_level(level: HeadingLevel) -> u8 {
    match level {
        HeadingLevel::H1 => 1,
        HeadingLevel::H2 => 2,
        HeadingLevel::H3 => 3,
        HeadingLevel::H4 => 4,
        HeadingLevel::H5 => 5,
        HeadingLevel::H6 => 6,
    }
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
}
