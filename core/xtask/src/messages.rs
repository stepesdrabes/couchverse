//! The bridge's message types for the shells: Swift, Kotlin and TypeScript renderings of every
//! `#[typeshare]` type in the core (`core/crates/app/src`), so all three decode the same JSON.

use std::collections::{BTreeMap, HashMap, HashSet};
use std::fs;
use std::path::{Path, PathBuf};

use typeshare_core::context::{ParseContext, ParseFileContext};
use typeshare_core::language::{
    GenericConstraints, Kotlin, Language, SINGLE_FILE_CRATE_NAME, Swift, TypeScript,
};
use typeshare_core::parser::{self, ParsedData};
use typeshare_core::reconcile::reconcile_aliases;

use crate::out;

const SOURCE: &str = "core/crates/app/src";
const SWIFT: &str =
    "clients/apple/Packages/CouchverseCore/Sources/CouchverseCore/Generated/Messages.swift";
const KOTLIN: &str =
    "clients/android/core/src/generated/kotlin/io/stepes/couchverse/core/Messages.kt";
const KOTLIN_PACKAGE: &str = "io.stepes.couchverse.core";
const TYPESCRIPT: &str = "clients/web/src/lib/generated/core.ts";

pub fn generate(root: &Path, written: &mut Vec<PathBuf>) -> Result<(), String> {
    let files = sources(&root.join(SOURCE))?;

    let swift = Swift {
        // Swift 6 strict concurrency: the shells hand view models across actors
        default_decorators: vec!["Sendable".into(), "Hashable".into()],
        default_generic_constraints: GenericConstraints::from_config(vec![
            "Sendable".into(),
            "Hashable".into(),
        ]),
        no_version_header: true,
        ..Swift::default()
    };
    let kotlin =
        Kotlin { package: KOTLIN_PACKAGE.into(), no_version_header: true, ..Kotlin::default() };
    let typescript = TypeScript { no_version_header: true, ..TypeScript::default() };

    let swift = render(Box::new(swift), &files, "//")?;
    out::write(&root.join(SWIFT), &swift_optional_defaults(&swift), written)?;
    let kotlin = render(Box::new(kotlin), &files, "//")?;
    out::write(&root.join(KOTLIN), &kotlin_sealed(&kotlin_generics(&kotlin)), written)?;
    let typescript = render(Box::new(typescript), &files, "//")?;
    out::write(&root.join(TYPESCRIPT), &typescript, written)?;
    Ok(())
}

/// The core's sources in a stable order, so the output does not depend on the file system.
fn sources(dir: &Path) -> Result<Vec<(PathBuf, String)>, String> {
    let mut files = Vec::new();
    let mut pending = vec![dir.to_path_buf()];
    while let Some(dir) = pending.pop() {
        let entries = fs::read_dir(&dir).map_err(|e| format!("read {}: {e}", dir.display()))?;
        for entry in entries {
            let path = entry.map_err(|e| format!("read {}: {e}", dir.display()))?.path();
            if path.is_dir() {
                pending.push(path);
            } else if path.extension().is_some_and(|e| e == "rs") {
                let text = fs::read_to_string(&path)
                    .map_err(|e| format!("read {}: {e}", path.display()))?;
                files.push((path, text));
            }
        }
    }
    files.sort();
    Ok(files)
}

fn render(
    mut lang: Box<dyn Language>,
    files: &[(PathBuf, String)],
    comment: &str,
) -> Result<String, String> {
    let context = ParseContext {
        ignored_types: lang.ignored_reference_types(),
        multi_file: false,
        target_os: vec![],
    };
    let mut data = ParsedData::default();
    for (path, source) in files {
        let file = ParseFileContext {
            source_code: source.clone(),
            crate_name: SINGLE_FILE_CRATE_NAME,
            file_name: String::new(),
            file_path: path.clone(),
        };
        let parsed =
            parser::parse(&context, file).map_err(|e| format!("{}: {e}", path.display()))?;
        if let Some(parsed) = parsed {
            if let Some(error) = parsed.errors.first() {
                return Err(format!("{}: {error:?}", path.display()));
            }
            data += parsed;
        }
    }
    let mut crates = BTreeMap::from([(SINGLE_FILE_CRATE_NAME, data)]);
    reconcile_aliases(&mut crates);
    let data = crates.remove(&SINGLE_FILE_CRATE_NAME).expect("the single crate");

    let mut output = Vec::new();
    lang.generate_types(&mut output, &HashMap::new(), data).map_err(|e| e.to_string())?;
    let body = String::from_utf8(output).map_err(|e| e.to_string())?;
    Ok(format!("{}{body}", out::header(comment, SOURCE)))
}

/// Optional initializer parameters default to `nil` in Swift, as they do in Kotlin, so a new
/// optional field (`#[serde(default)]`) does not break the shells' call sites.
fn swift_optional_defaults(source: &str) -> String {
    source
        .lines()
        .map(|line| {
            let Some(open) = line.find("public init(") else { return line.to_string() };
            let params_start = open + "public init(".len();
            let Some(close) = line.rfind(") {") else { return line.to_string() };
            let params = split_params(&line[params_start..close])
                .into_iter()
                .map(|p| if p.ends_with('?') { format!("{p} = nil") } else { p.to_string() })
                .collect::<Vec<_>>()
                .join(", ");
            format!("{}{params}{}", &line[..params_start], &line[close..])
        })
        .collect::<Vec<_>>()
        .join("\n")
        + if source.ends_with('\n') { "\n" } else { "" }
}

/// Splits `a: [String: Int], b: Foo?` at the commas outside brackets.
fn split_params(params: &str) -> Vec<&str> {
    let mut out = Vec::new();
    let (mut depth, mut start) = (0i32, 0);
    for (i, c) in params.char_indices() {
        match c {
            '[' | '<' | '(' => depth += 1,
            ']' | '>' | ')' => depth -= 1,
            ',' if depth == 0 => {
                out.push(params[start..i].trim());
                start = i + 1;
            }
            _ => {}
        }
    }
    if !params[start..].trim().is_empty() {
        out.push(params[start..].trim());
    }
    out
}

/// typeshare emits `object Loading: LoadState<T>()` for a unit variant of a generic enum, which
/// is not valid Kotlin (an object cannot see `T`): the class becomes covariant and its unit
/// variants extend it at `Nothing`.
fn kotlin_generics(source: &str) -> String {
    let mut generic = Vec::new();
    let mut out = String::with_capacity(source.len());
    for line in source.lines() {
        let mut line = line.to_string();
        if let Some(rest) = line.strip_prefix("sealed class ")
            && let Some((name, tail)) = rest.split_once("<T>")
        {
            generic.push(name.to_string());
            line = format!("sealed class {name}<out T>{tail}");
        }
        for name in &generic {
            let unit = format!(": {name}<T>()");
            if line.trim_start().starts_with("object ") && line.contains(&unit) {
                line = line.replace(&unit, &format!(": {name}<Nothing>()"));
            }
        }
        out.push_str(&line);
        out.push('\n');
    }
    out
}

/// typeshare nests the variants of a data-carrying enum in its sealed class, where a variant
/// shadows every type of the same name: inside `Block`, `List<Inline>` names the `Block.List`
/// variant and does not compile. A variant's content that names a sibling variant is written
/// fully qualified instead. Unit variants become `data object`s so they print their name.
fn kotlin_sealed(source: &str) -> String {
    let lines: Vec<&str> = source.lines().collect();
    let top_level: HashSet<&str> = lines.iter().filter_map(|l| declared_name(l)).collect();
    let mut out = String::with_capacity(source.len());
    let mut i = 0;
    while i < lines.len() {
        let line = lines[i];
        out.push_str(line);
        out.push('\n');
        i += 1;
        if !line.starts_with("sealed class ") {
            continue;
        }
        let end = lines[i..].iter().position(|l| *l == "}").map_or(lines.len(), |p| i + p);
        let body = &lines[i..end];
        let variants: HashSet<&str> =
            body.iter().filter_map(|l| declared_name(l.trim_start())).collect();
        for line in body {
            out.push_str(&sealed_variant(line, &variants, &top_level));
            out.push('\n');
        }
        i = end;
    }
    out
}

/// The type a Kotlin declaration line introduces.
fn declared_name(line: &str) -> Option<&str> {
    let rest = ["data class ", "sealed class ", "enum class ", "data object ", "object "]
        .iter()
        .find_map(|keyword| line.strip_prefix(keyword))?;
    let end = rest.find(|c: char| !c.is_alphanumeric() && c != '_').unwrap_or(rest.len());
    (end > 0).then(|| &rest[..end])
}

fn sealed_variant(line: &str, variants: &HashSet<&str>, top_level: &HashSet<&str>) -> String {
    let declaration = line.trim_start();
    let indent = &line[..line.len() - declaration.len()];
    if let Some(rest) = declaration.strip_prefix("object ") {
        return format!("{indent}data object {rest}");
    }
    let (Some(open), Some(close)) = (line.find('('), line.rfind("): ")) else {
        return line.to_string();
    };
    if !declaration.starts_with("data class ") || close < open {
        return line.to_string();
    }
    let qualify = |name: &str| {
        if !variants.contains(name) {
            name.to_string()
        } else if top_level.contains(name) {
            format!("{KOTLIN_PACKAGE}.{name}")
        } else if matches!(name, "List" | "Map" | "HashMap" | "Set") {
            format!("kotlin.collections.{name}")
        } else {
            format!("kotlin.{name}")
        }
    };
    let params = &line[open + 1..close];
    let mut qualified = String::with_capacity(params.len());
    let mut word_start = None;
    for (at, c) in params.char_indices() {
        let in_word = c.is_alphanumeric() || c == '_';
        match (in_word, word_start) {
            (true, None) => word_start = Some(at),
            (false, Some(start)) => {
                qualified.push_str(&qualify(&params[start..at]));
                qualified.push(c);
                word_start = None;
            }
            (false, None) => qualified.push(c),
            (true, Some(_)) => {}
        }
    }
    if let Some(start) = word_start {
        qualified.push_str(&qualify(&params[start..]));
    }
    format!("{}{qualified}{}", &line[..=open], &line[close..])
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn swift_optionals_default_to_nil() {
        let source = "\tpublic init(nowMs: UInt64, event: Event, wallMs: UInt64?) {\n\
\tpublic init(headers: [String: String]?, items: [Item], next: Next?) {\n\
\tpublic init() {}\n";
        let expected = "\tpublic init(nowMs: UInt64, event: Event, wallMs: UInt64? = nil) {\n\
\tpublic init(headers: [String: String]? = nil, items: [Item], next: Next? = nil) {\n\
\tpublic init() {}\n";
        assert_eq!(swift_optional_defaults(source), expected);
    }

    #[test]
    fn kotlin_variants_do_not_shadow_the_types_they_carry() {
        let source = "\
data class Link (
\tval items: List<String>
)

sealed class Inline {
\t@SerialName(\"text\")
\tdata class Text(val content: String): Inline()
\t@SerialName(\"strong\")
\tdata class Strong(val content: List<Inline>): Inline()
\t@SerialName(\"list\")
\tdata class List(val content: Link): Inline()
\t@SerialName(\"link\")
\tdata class Link(val content: Link): Inline()
\t@SerialName(\"break\")
\tobject Break: Inline()
}
";
        let lines: Vec<String> = kotlin_sealed(source).lines().map(String::from).collect();
        let expected = [
            "\tval items: List<String>",
            "\tdata class Text(val content: String): Inline()",
            "\tdata class Strong(val content: kotlin.collections.List<Inline>): Inline()",
            "\tdata class List(val content: io.stepes.couchverse.core.Link): Inline()",
            "\tdata class Link(val content: io.stepes.couchverse.core.Link): Inline()",
            "\tdata object Break: Inline()",
        ];
        for line in expected {
            assert!(lines.iter().any(|l| l == line), "missing {line:?} in {lines:#?}");
        }
    }
}
