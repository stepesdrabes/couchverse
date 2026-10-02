//! The bridge's message types for the shells: Swift, Kotlin and TypeScript renderings of every
//! `#[typeshare]` type in the core (`core/crates/app/src`), so all three decode the same JSON.

use std::collections::{BTreeMap, HashMap};
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
    let kotlin = Kotlin {
        package: "io.stepes.couchverse.core".into(),
        no_version_header: true,
        ..Kotlin::default()
    };
    let typescript = TypeScript { no_version_header: true, ..TypeScript::default() };

    let swift = render(Box::new(swift), &files, "//")?;
    out::write(&root.join(SWIFT), &swift, written)?;
    let kotlin = render(Box::new(kotlin), &files, "//")?;
    out::write(&root.join(KOTLIN), &kotlin_generics(&kotlin), written)?;
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
