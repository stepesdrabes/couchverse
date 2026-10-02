//! Inline text: code spans, emphasis, strong and strikethrough through `CommonMark`'s delimiter
//! algorithm, inline links and autolinks, backslash escapes, entities, and every newline as a
//! line break. Bare URLs and email addresses are linked afterwards.

use super::{Inline, LinkInline, safe_href};

/// Emphasis matching looks back over the open delimiters; beyond this many runs the rest stay
/// literal, so hostile input cannot make it quadratic in a meaningful way.
const MAX_DELIMITERS: usize = 1000;

enum Item {
    Inline(Inline),
    Delim(Delim),
    /// An unmatched `[` (or `![`) that may still open a link.
    Bracket {
        image: bool,
        active: bool,
    },
}

struct Delim {
    ch: char,
    count: usize,
    /// The run's original length, for the rule of three.
    original: usize,
    open: bool,
    close: bool,
}

pub fn parse(text: &str) -> Vec<Inline> {
    let chars: Vec<char> = text.chars().collect();
    let mut parser = Parser {
        chars: &chars,
        items: Vec::new(),
        text: String::new(),
        brackets: Vec::new(),
        delims: 0,
    };
    parser.run();
    let mut items = parser.items;
    process_emphasis(&mut items);
    linkify(finish(items))
}

struct Parser<'a> {
    chars: &'a [char],
    items: Vec<Item>,
    /// Plain text not yet pushed as an item.
    text: String,
    /// Indexes of the `Bracket` items still waiting for their `]`.
    brackets: Vec<usize>,
    delims: usize,
}

impl Parser<'_> {
    fn run(&mut self) {
        let mut i = 0;
        while i < self.chars.len() {
            i = match self.chars[i] {
                '\\' => self.escape(i),
                '`' => self.code_span(i),
                c @ ('*' | '_' | '~') => self.delimiter_run(i, c),
                '[' => self.open_bracket(i, false),
                '!' if self.chars.get(i + 1) == Some(&'[') => self.open_bracket(i + 1, true),
                ']' => self.close_bracket(i),
                '<' => self.autolink(i),
                '&' => self.entity(i),
                '\n' => self.line_break(i),
                c => {
                    self.text.push(c);
                    i + 1
                }
            };
        }
        self.flush();
    }

    fn flush(&mut self) {
        if !self.text.is_empty() {
            self.items.push(Item::Inline(Inline::Text(std::mem::take(&mut self.text))));
        }
    }

    fn push(&mut self, item: Item) {
        self.flush();
        self.items.push(item);
    }

    fn escape(&mut self, i: usize) -> usize {
        match self.chars.get(i + 1) {
            Some('\n') => {
                self.push(Item::Inline(Inline::Break));
                i + 2
            }
            Some(c) if c.is_ascii_punctuation() => {
                self.text.push(*c);
                i + 2
            }
            _ => {
                self.text.push('\\');
                i + 1
            }
        }
    }

    fn line_break(&mut self, i: usize) -> usize {
        let trimmed = self.text.trim_end_matches(' ').len();
        self.text.truncate(trimmed);
        self.push(Item::Inline(Inline::Break));
        let mut next = i + 1;
        while self.chars.get(next) == Some(&' ') {
            next += 1;
        }
        next
    }

    fn code_span(&mut self, i: usize) -> usize {
        let run = self.run_length(i, '`');
        let mut j = i + run;
        while j < self.chars.len() {
            if self.chars[j] == '`' {
                let close = self.run_length(j, '`');
                if close == run {
                    let code: String = self.chars[i + run..j]
                        .iter()
                        .map(|c| if *c == '\n' { ' ' } else { *c })
                        .collect();
                    let strip = code.len() >= 2
                        && code.starts_with(' ')
                        && code.ends_with(' ')
                        && !code.trim().is_empty();
                    let code = if strip { code[1..code.len() - 1].to_string() } else { code };
                    self.push(Item::Inline(Inline::Code(code)));
                    return j + close;
                }
                j += close;
            } else {
                j += 1;
            }
        }
        // no closing run: the backticks are literal
        self.text.extend(std::iter::repeat_n('`', run));
        i + run
    }

    fn run_length(&self, i: usize, c: char) -> usize {
        self.chars[i..].iter().take_while(|x| **x == c).count()
    }

    fn delimiter_run(&mut self, i: usize, ch: char) -> usize {
        let count = self.run_length(i, ch);
        // strikethrough is exactly two tildes, as in markdown-it
        if (ch == '~' && count != 2) || self.delims >= MAX_DELIMITERS {
            self.text.extend(std::iter::repeat_n(ch, count));
            return i + count;
        }
        let before = i.checked_sub(1).map(|p| self.chars[p]);
        let after = self.chars.get(i + count).copied();
        let left = !is_space(after) && (!is_punct(after) || is_space(before) || is_punct(before));
        let right = !is_space(before) && (!is_punct(before) || is_space(after) || is_punct(after));
        let (open, close) = if ch == '_' {
            // intraword underscores (snake_case) never emphasize
            (left && (!right || is_punct(before)), right && (!left || is_punct(after)))
        } else {
            (left, right)
        };
        self.delims += 1;
        self.push(Item::Delim(Delim { ch, count, original: count, open, close }));
        i + count
    }

    fn open_bracket(&mut self, i: usize, image: bool) -> usize {
        self.push(Item::Bracket { image, active: true });
        self.brackets.push(self.items.len() - 1);
        i + 1
    }

    fn close_bracket(&mut self, i: usize) -> usize {
        self.flush();
        let Some(opener) = self.brackets.pop() else {
            self.text.push(']');
            return i + 1;
        };
        let Item::Bracket { image, active } = self.items[opener] else { unreachable!() };
        let target = if active { link_target(self.chars, i + 1) } else { None };
        let Some((href, next)) = target else {
            self.items[opener] = Item::Inline(Inline::Text(bracket_text(image)));
            self.text.push(']');
            return i + 1;
        };

        let mut children: Vec<Item> = self.items.split_off(opener + 1);
        self.items.pop();
        process_emphasis(&mut children);
        let mut children = finish(children);
        if !image {
            // links cannot contain links: earlier openers can no longer form one
            for &b in &self.brackets {
                if let Item::Bracket { image: false, active } = &mut self.items[b] {
                    *active = false;
                }
            }
        }
        match safe_href(&href) {
            Some(href) => {
                if children.is_empty() {
                    children.push(Inline::Text(href.clone()));
                }
                self.items.push(Item::Inline(Inline::Link(LinkInline { href, children })));
            }
            // a disallowed link keeps its text and loses the link
            None => self.items.extend(children.into_iter().map(Item::Inline)),
        }
        next
    }

    fn autolink(&mut self, i: usize) -> usize {
        let end =
            self.chars[i + 1..].iter().position(|c| *c == '>' || *c == '<' || c.is_whitespace());
        let Some(end) = end.map(|e| i + 1 + e).filter(|e| self.chars[*e] == '>') else {
            self.text.push('<');
            return i + 1;
        };
        let target: String = self.chars[i + 1..end].iter().collect();
        let href = if is_uri(&target) {
            safe_href(&target)
        } else if is_email(&target) {
            Some(format!("mailto:{target}"))
        } else {
            // anything else, including HTML tags, is text
            self.text.push('<');
            return i + 1;
        };
        match href {
            Some(href) => self.push(Item::Inline(Inline::Link(LinkInline {
                href,
                children: vec![Inline::Text(target)],
            }))),
            None => self.text.push_str(&target),
        }
        end + 1
    }

    fn entity(&mut self, i: usize) -> usize {
        let end = self.chars[i + 1..].iter().take(32).position(|c| *c == ';');
        let decoded = end.and_then(|end| {
            let name: String = self.chars[i + 1..i + 1 + end].iter().collect();
            decode_entity(&name).map(|c| (c, i + end + 2))
        });
        if let Some((c, next)) = decoded {
            self.text.push(c);
            next
        } else {
            self.text.push('&');
            i + 1
        }
    }
}

fn bracket_text(image: bool) -> String {
    if image { "![".into() } else { "[".into() }
}

/// The destination of an inline link `(url "title")` starting at `i`, and the index after it.
fn link_target(chars: &[char], i: usize) -> Option<(String, usize)> {
    if chars.get(i) != Some(&'(') {
        return None;
    }
    let mut j = skip_space(chars, i + 1);
    let mut href = String::new();
    if chars.get(j) == Some(&'<') {
        j += 1;
        loop {
            match chars.get(j)? {
                '>' => break,
                '<' | '\n' => return None,
                '\\' if chars.get(j + 1).is_some_and(char::is_ascii_punctuation) => {
                    href.push(chars[j + 1]);
                    j += 1;
                }
                c => href.push(*c),
            }
            j += 1;
        }
        j += 1;
    } else {
        let mut depth = 0usize;
        while let Some(&c) = chars.get(j) {
            match c {
                c if c.is_whitespace() || c.is_control() => break,
                '\\' if chars.get(j + 1).is_some_and(char::is_ascii_punctuation) => {
                    href.push(chars[j + 1]);
                    j += 1;
                }
                '(' => {
                    depth += 1;
                    href.push(c);
                }
                ')' if depth == 0 => break,
                ')' => {
                    depth -= 1;
                    href.push(c);
                }
                c => href.push(c),
            }
            j += 1;
        }
    }
    let after = skip_space(chars, j);
    j = match chars.get(after) {
        // an optional title, which bios have no use for
        Some(&quote @ ('"' | '\'' | '(')) if after > j => {
            let close = if quote == '(' { ')' } else { quote };
            let end = chars[after + 1..].iter().position(|c| *c == close)?;
            skip_space(chars, after + 1 + end + 1)
        }
        _ => after,
    };
    (chars.get(j) == Some(&')')).then_some((href, j + 1))
}

fn skip_space(chars: &[char], mut i: usize) -> usize {
    while chars.get(i).is_some_and(|c| *c == ' ' || *c == '\n') {
        i += 1;
    }
    i
}

/// `CommonMark`'s "process emphasis": pairs each closing run with the nearest compatible opener
/// and wraps what lies between.
fn process_emphasis(items: &mut Vec<Item>) {
    let mut i = 0;
    while i < items.len() {
        let Item::Delim(closer) = &items[i] else {
            i += 1;
            continue;
        };
        if !closer.close {
            i += 1;
            continue;
        }
        let opener = items[..i].iter().rposition(|item| match item {
            Item::Delim(opener) => opener.ch == closer.ch && opener.open && pairs(opener, closer),
            _ => false,
        });
        let Some(o) = opener else {
            i += 1;
            continue;
        };

        let (ch, use_count) = match (&items[o], &items[i]) {
            (Item::Delim(a), Item::Delim(b)) if a.ch != '~' && a.count >= 2 && b.count >= 2 => {
                (a.ch, 2)
            }
            (Item::Delim(a), _) if a.ch == '~' => ('~', 2),
            (Item::Delim(a), _) => (a.ch, 1),
            _ => unreachable!(),
        };
        let inner: Vec<Item> = items.drain(o + 1..i).collect();
        let children = finish(inner);
        let node = match (ch, use_count) {
            ('~', _) => Inline::Strike(children),
            (_, 2) => Inline::Strong(children),
            _ => Inline::Emphasis(children),
        };
        items.insert(o + 1, Item::Inline(node));
        let mut closer_at = o + 2;
        if let Item::Delim(d) = &mut items[closer_at] {
            d.count -= use_count;
            if d.count == 0 {
                items.remove(closer_at);
            }
        }
        let opener_left = match &mut items[o] {
            Item::Delim(d) => {
                d.count -= use_count;
                d.count > 0
            }
            _ => false,
        };
        if !opener_left {
            items.remove(o);
            closer_at -= 1;
        }
        // a closer with characters left may close an earlier opener too
        i = closer_at;
    }
}

/// The rule of three: a run that can both open and close only pairs with one whose length
/// keeps the sum off a multiple of three, unless both are multiples of three.
fn pairs(opener: &Delim, closer: &Delim) -> bool {
    let both = opener.close || closer.open;
    let sum = opener.original + closer.original;
    let threes = opener.original.is_multiple_of(3) && closer.original.is_multiple_of(3);
    !(both && sum.is_multiple_of(3) && !threes)
}

/// Unmatched delimiters and brackets become text; adjacent text runs merge.
fn finish(items: Vec<Item>) -> Vec<Inline> {
    let mut out: Vec<Inline> = Vec::with_capacity(items.len());
    for item in items {
        let inline = match item {
            Item::Inline(inline) => inline,
            Item::Delim(d) => Inline::Text(std::iter::repeat_n(d.ch, d.count).collect()),
            Item::Bracket { image, .. } => Inline::Text(bracket_text(image)),
        };
        match (out.last_mut(), inline) {
            (Some(Inline::Text(last)), Inline::Text(more)) => last.push_str(&more),
            (_, Inline::Text(t)) if t.is_empty() => {}
            (_, inline) => out.push(inline),
        }
    }
    out
}

/// Links bare URLs and email addresses in text, except inside links and code.
fn linkify(inlines: Vec<Inline>) -> Vec<Inline> {
    let mut out = Vec::with_capacity(inlines.len());
    for inline in inlines {
        match inline {
            Inline::Text(text) => {
                for span in linkify::LinkFinder::new().spans(&text) {
                    let piece = span.as_str().to_string();
                    let href = match span.kind() {
                        Some(linkify::LinkKind::Url) => safe_href(&piece),
                        Some(linkify::LinkKind::Email) => Some(format!("mailto:{piece}")),
                        _ => None,
                    };
                    out.push(match href {
                        Some(href) => {
                            Inline::Link(LinkInline { href, children: vec![Inline::Text(piece)] })
                        }
                        None => Inline::Text(piece),
                    });
                }
            }
            Inline::Strong(children) => out.push(Inline::Strong(linkify(children))),
            Inline::Emphasis(children) => out.push(Inline::Emphasis(linkify(children))),
            Inline::Strike(children) => out.push(Inline::Strike(linkify(children))),
            other => out.push(other),
        }
    }
    out
}

fn is_space(c: Option<char>) -> bool {
    c.is_none_or(char::is_whitespace)
}

fn is_punct(c: Option<char>) -> bool {
    c.is_some_and(|c| c.is_ascii_punctuation() || (!c.is_alphanumeric() && !c.is_whitespace()))
}

fn is_uri(target: &str) -> bool {
    let Some((scheme, rest)) = target.split_once(':') else { return false };
    (2..=32).contains(&scheme.len())
        && scheme.starts_with(|c: char| c.is_ascii_alphabetic())
        && scheme.chars().all(|c| c.is_ascii_alphanumeric() || matches!(c, '+' | '.' | '-'))
        && !rest.is_empty()
}

fn is_email(target: &str) -> bool {
    let Some((local, domain)) = target.split_once('@') else { return false };
    !local.is_empty()
        && local.chars().all(|c| c.is_ascii_alphanumeric() || ".!#$%&'*+/=?^_`{|}~-".contains(c))
        && domain.split('.').count() >= 2
        && domain
            .split('.')
            .all(|l| !l.is_empty() && l.chars().all(|c| c.is_ascii_alphanumeric() || c == '-'))
}

/// Numeric references and the named entities people actually type; anything else stays text.
fn decode_entity(name: &str) -> Option<char> {
    if let Some(number) = name.strip_prefix('#') {
        let value = match number.strip_prefix(['x', 'X']) {
            Some(hex) if (1..=6).contains(&hex.len()) => u32::from_str_radix(hex, 16).ok()?,
            None if (1..=7).contains(&number.len()) => number.parse().ok()?,
            _ => return None,
        };
        // NUL and invalid code points become the replacement character, as CommonMark says
        return Some(char::from_u32(value).filter(|c| *c != '\0').unwrap_or('\u{fffd}'));
    }
    let c = match name {
        "amp" => '&',
        "lt" => '<',
        "gt" => '>',
        "quot" => '"',
        "apos" => '\'',
        "nbsp" => '\u{a0}',
        "copy" => '\u{a9}',
        "reg" => '\u{ae}',
        "trade" => '\u{2122}',
        "hellip" => '\u{2026}',
        "ndash" => '\u{2013}',
        "mdash" => '\u{2014}',
        "lsquo" => '\u{2018}',
        "rsquo" => '\u{2019}',
        "ldquo" => '\u{201c}',
        "rdquo" => '\u{201d}',
        "laquo" => '\u{ab}',
        "raquo" => '\u{bb}',
        "bull" => '\u{2022}',
        "middot" => '\u{b7}',
        "deg" => '\u{b0}',
        "times" => '\u{d7}',
        "divide" => '\u{f7}',
        "plusmn" => '\u{b1}',
        "euro" => '\u{20ac}',
        "pound" => '\u{a3}',
        "yen" => '\u{a5}',
        "cent" => '\u{a2}',
        "sect" => '\u{a7}',
        "para" => '\u{b6}',
        "frac12" => '\u{bd}',
        "frac14" => '\u{bc}',
        "frac34" => '\u{be}',
        "hearts" => '\u{2665}',
        "larr" => '\u{2190}',
        "rarr" => '\u{2192}',
        "uarr" => '\u{2191}',
        "darr" => '\u{2193}',
        _ => return None,
    };
    Some(c)
}
