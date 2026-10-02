//! Block structure, line by line: the `CommonMark` containers and leaves a bio uses (paragraphs,
//! ATX and setext headings, quotes, lists, fenced and indented code, rules) plus GFM tables. Raw
//! HTML is not a block here; it stays paragraph text.

use super::inlines;
use super::{Block, HeadingBlock, ListBlock, ListItem, TableBlock, TableCell, TableRow};

/// Quotes and lists nest by recursion; deeper input is read as plain paragraphs.
const MAX_DEPTH: usize = 16;

pub fn parse(source: &str) -> Vec<Block> {
    let lines: Vec<String> = source.lines().map(expand_leading_tabs).collect();
    blocks(&lines, 0)
}

fn blocks(lines: &[String], depth: usize) -> Vec<Block> {
    let mut out = Vec::new();
    let mut i = 0;
    while i < lines.len() {
        if is_blank(&lines[i]) {
            i += 1;
            continue;
        }
        let (block, next) = fenced_code(lines, i)
            .or_else(|| atx_heading(&lines[i]).map(|b| (b, i + 1)))
            .or_else(|| thematic_break(&lines[i]).then_some((Block::Rule, i + 1)))
            .or_else(|| (depth < MAX_DEPTH).then(|| quote(lines, i, depth)).flatten())
            .or_else(|| (depth < MAX_DEPTH).then(|| list(lines, i, depth)).flatten())
            .or_else(|| table(lines, i))
            .or_else(|| indented_code(lines, i))
            .unwrap_or_else(|| paragraph(lines, i));
        out.push(block);
        i = next;
    }
    out
}

fn paragraph(lines: &[String], start: usize) -> (Block, usize) {
    let mut end = start + 1;
    while end < lines.len() && !is_blank(&lines[end]) {
        if let Some(level) = setext_underline(&lines[end]) {
            return (heading(level, &join(&lines[start..end])), end + 1);
        }
        if interrupts(lines, end) {
            break;
        }
        end += 1;
    }
    (Block::Paragraph(inlines::parse(&join(&lines[start..end]))), end)
}

fn join(lines: &[String]) -> String {
    let trimmed: Vec<&str> = lines.iter().map(|l| l.trim_start()).collect();
    trimmed.join("\n").trim_end().to_string()
}

fn heading(level: u8, text: &str) -> Block {
    Block::Heading(HeadingBlock { level, inlines: inlines::parse(text) })
}

/// Whether line `i` starts a block that may cut a paragraph short.
fn interrupts(lines: &[String], i: usize) -> bool {
    let line = &lines[i];
    fence_open(line).is_some()
        || atx_heading(line).is_some()
        || thematic_break(line)
        || quote_marker(line).is_some()
        || list_marker(line).is_some_and(|m| !m.empty && (!m.ordered || m.start == 1))
        || table_header(lines, i).is_some()
}

fn fenced_code(lines: &[String], start: usize) -> Option<(Block, usize)> {
    let (indent, fence, len) = fence_open(&lines[start])?;
    let mut code = Vec::new();
    let mut i = start + 1;
    while i < lines.len() {
        let line = &lines[i];
        let trimmed = line.trim();
        let closes =
            indent_of(line) < 4 && trimmed.len() >= len && trimmed.chars().all(|c| c == fence);
        if closes {
            return Some((Block::Code(code.join("\n")), i + 1));
        }
        let strip = indent_of(line).min(indent);
        code.push(&line[strip..]);
        i += 1;
    }
    // an unclosed fence runs to the end of its container
    Some((Block::Code(code.join("\n")), i))
}

/// The indent, fence character and fence length of an opening code fence.
fn fence_open(line: &str) -> Option<(usize, char, usize)> {
    let indent = indent_of(line);
    if indent > 3 {
        return None;
    }
    let rest = &line[indent..];
    let fence = rest.chars().next().filter(|c| *c == '`' || *c == '~')?;
    let len = rest.chars().take_while(|c| *c == fence).count();
    // a backtick fence's info string cannot hold backticks (it would be inline code)
    let valid = len >= 3 && (fence == '~' || !rest[len..].contains('`'));
    valid.then_some((indent, fence, len))
}

fn atx_heading(line: &str) -> Option<Block> {
    let indent = indent_of(line);
    if indent > 3 {
        return None;
    }
    let rest = &line[indent..];
    let level = rest.chars().take_while(|c| *c == '#').count();
    let text = &rest[level..];
    if !(1..=6).contains(&level) || !(text.is_empty() || text.starts_with(' ')) {
        return None;
    }
    let mut text = text.trim();
    // an optional closing run of #s, separated by a space
    let without_closing = text.trim_end_matches('#');
    if without_closing.is_empty() || without_closing.ends_with(' ') {
        text = without_closing.trim_end();
    }
    Some(heading(u8::try_from(level).unwrap_or(6), text))
}

fn setext_underline(line: &str) -> Option<u8> {
    let indent = indent_of(line);
    let rest = line[indent..].trim_end();
    if indent > 3 || rest.is_empty() {
        return None;
    }
    if rest.chars().all(|c| c == '=') {
        Some(1)
    } else if rest.chars().all(|c| c == '-') {
        Some(2)
    } else {
        None
    }
}

fn thematic_break(line: &str) -> bool {
    if indent_of(line) > 3 {
        return false;
    }
    let marks: Vec<char> = line.chars().filter(|c| !c.is_whitespace()).collect();
    marks.len() >= 3 && matches!(marks[0], '-' | '*' | '_') && marks.iter().all(|c| *c == marks[0])
}

fn quote(lines: &[String], start: usize, depth: usize) -> Option<(Block, usize)> {
    quote_marker(&lines[start])?;
    let mut content: Vec<String> = Vec::new();
    let mut i = start;
    while i < lines.len() {
        let line = &lines[i];
        if let Some(rest) = quote_marker(line) {
            content.push(rest.to_string());
        } else if lazy_continuation(&content, lines, i) {
            content.push(line.clone());
        } else {
            break;
        }
        i += 1;
    }
    Some((Block::Quote(blocks(&content, depth + 1)), i))
}

/// The text after a `>` marker (and one optional space).
fn quote_marker(line: &str) -> Option<&str> {
    let indent = indent_of(line);
    let rest = line[indent..].strip_prefix('>')?;
    (indent <= 3).then(|| rest.strip_prefix(' ').unwrap_or(rest))
}

/// A line that continues the paragraph a container's last line belongs to, without the
/// container's marker or indent.
fn lazy_continuation(content: &[String], lines: &[String], i: usize) -> bool {
    let line = &lines[i];
    !is_blank(line)
        && content.last().is_some_and(|last| !is_blank(last) && fence_open(last).is_none())
        && !interrupts(lines, i)
        && list_marker(line).is_none()
}

struct Marker {
    ordered: bool,
    /// The bullet, or the delimiter after the number.
    symbol: char,
    start: u32,
    /// Where the item's content begins; continuation lines are indented this far.
    content: usize,
    empty: bool,
}

fn list_marker(line: &str) -> Option<Marker> {
    let indent = indent_of(line);
    if indent > 3 {
        return None;
    }
    let rest = &line[indent..];
    let (ordered, symbol, start, width) = match rest.chars().next()? {
        c @ ('-' | '+' | '*') => (false, c, 0, 1),
        c if c.is_ascii_digit() => {
            let digits = rest.chars().take_while(char::is_ascii_digit).count();
            let symbol = rest[digits..].chars().next().filter(|c| *c == '.' || *c == ')')?;
            if digits > 9 {
                return None;
            }
            (true, symbol, rest[..digits].parse().ok()?, digits + 1)
        }
        _ => return None,
    };
    let after = &rest[width..];
    if !(after.is_empty() || after.starts_with(' ')) {
        return None;
    }
    let spaces = after.chars().take_while(|c| *c == ' ').count();
    let empty = after.trim().is_empty();
    // five or more spaces start indented code inside the item, which keeps one
    let gap = if empty || spaces > 4 { 1 } else { spaces };
    Some(Marker { ordered, symbol, start, content: indent + width + gap, empty })
}

fn list(lines: &[String], start: usize, depth: usize) -> Option<(Block, usize)> {
    if thematic_break(&lines[start]) {
        return None;
    }
    let first = list_marker(&lines[start])?;
    let (ordered, symbol, number) = (first.ordered, first.symbol, first.start);
    let mut items = Vec::new();
    let mut marker = first;
    let mut i = start;
    loop {
        let line = &lines[i];
        let mut content = vec![line.get(marker.content..).unwrap_or("").to_string()];
        i += 1;
        while i < lines.len() {
            let line = &lines[i];
            if is_blank(line) {
                content.push(String::new());
            } else if indent_of(line) >= marker.content {
                content.push(line[marker.content..].to_string());
            } else if lazy_continuation(&content, lines, i) {
                content.push(line.trim_start().to_string());
            } else {
                break;
            }
            i += 1;
        }
        while content.last().is_some_and(|l| is_blank(l)) {
            content.pop();
        }
        items.push(ListItem { blocks: blocks(&content, depth + 1) });

        let next = (i < lines.len() && !thematic_break(&lines[i]))
            .then(|| list_marker(&lines[i]))
            .flatten()
            .filter(|m| m.ordered == ordered && m.symbol == symbol);
        match next {
            Some(next) => marker = next,
            None => break,
        }
    }
    let start = ordered.then_some(number);
    Some((Block::List(ListBlock { start, items }), i))
}

fn table(lines: &[String], start: usize) -> Option<(Block, usize)> {
    let columns = table_header(lines, start)?;
    let cells = |line: &str| -> Vec<TableCell> {
        let mut cells: Vec<TableCell> =
            split_row(line).iter().map(|c| TableCell { inlines: inlines::parse(c) }).collect();
        cells.resize_with(columns, || TableCell { inlines: vec![] });
        cells
    };
    let header = cells(&lines[start]);
    let mut rows = Vec::new();
    let mut i = start + 2;
    while i < lines.len() && !is_blank(&lines[i]) && !interrupts(lines, i) {
        rows.push(TableRow { cells: cells(&lines[i]) });
        i += 1;
    }
    Some((Block::Table(TableBlock { header, rows }), i))
}

/// The column count when line `i` is a table header followed by its delimiter row.
fn table_header(lines: &[String], i: usize) -> Option<usize> {
    let (header, delimiter) = (lines.get(i)?, lines.get(i + 1)?);
    if !header.contains('|') || !delimiter.contains('|') || indent_of(header) > 3 {
        return None;
    }
    let columns = split_row(header).len();
    let delimiters = split_row(delimiter);
    let valid = delimiters.len() == columns
        && delimiters.iter().all(|cell| {
            let dashes = cell.trim_start_matches(':').trim_end_matches(':');
            !dashes.is_empty() && dashes.chars().all(|c| c == '-')
        });
    valid.then_some(columns)
}

/// The cells of a table row, without the optional outer pipes; `\|` is a literal pipe.
fn split_row(line: &str) -> Vec<String> {
    let line = line.trim();
    let line = line.strip_prefix('|').unwrap_or(line);
    let line = match line.strip_suffix('|') {
        Some(inner) if !inner.ends_with('\\') => inner,
        _ => line,
    };
    let mut cells = vec![String::new()];
    let mut chars = line.chars().peekable();
    while let Some(c) = chars.next() {
        match c {
            '\\' if chars.peek() == Some(&'|') => {
                cells.last_mut().expect("a cell").push('|');
                chars.next();
            }
            '|' => cells.push(String::new()),
            c => cells.last_mut().expect("a cell").push(c),
        }
    }
    cells.iter().map(|c| c.trim().to_string()).collect()
}

fn indented_code(lines: &[String], start: usize) -> Option<(Block, usize)> {
    if indent_of(&lines[start]) < 4 {
        return None;
    }
    let mut code = Vec::new();
    let mut i = start;
    while i < lines.len() && (is_blank(&lines[i]) || indent_of(&lines[i]) >= 4) {
        code.push(lines[i].get(4..).unwrap_or(""));
        i += 1;
    }
    while code.last().is_some_and(|l| l.trim().is_empty()) {
        code.pop();
    }
    Some((Block::Code(code.join("\n")), i))
}

fn is_blank(line: &str) -> bool {
    line.trim().is_empty()
}

fn indent_of(line: &str) -> usize {
    line.chars().take_while(|c| *c == ' ').count()
}

/// Tabs in the indent count as four columns, so nesting reads the same as spaces.
fn expand_leading_tabs(line: &str) -> String {
    let indent = line.len() - line.trim_start_matches([' ', '\t']).len();
    let mut out = String::with_capacity(line.len() + 8);
    let mut column = 0;
    for c in line[..indent].chars() {
        let width = if c == '\t' { 4 - column % 4 } else { 1 };
        out.extend(std::iter::repeat_n(' ', width));
        column += width;
    }
    out.push_str(&line[indent..]);
    out
}
