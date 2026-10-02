//! UI strings: `contract/i18n/{locale}.json` (inlang message format, also read by Paraglide on the
//! web) -> an Apple string catalog with typed accessors and Android string resources.
//!
//! Supported messages are plain patterns (`"{count} items"`) and single-selector plurals in the
//! shape Paraglide compiles:
//! `[{"declarations": ["input n", "local nPlural = n: plural"], "selectors": ["nPlural"],
//!    "match": {"nPlural=one": "...", "nPlural=*": "..."}}]`.
//! Admin-only strings (by key prefix) are left out of the native outputs.

use std::collections::{BTreeMap, BTreeSet};
use std::fmt::Write as _;
use std::path::{Path, PathBuf};

use serde_json::{Map, Value, json};

use crate::out;

const LOCALES: [&str; 2] = ["en", "cs"];
const BASE: &str = "en";
const ADMIN_PREFIXES: [&str; 6] =
    ["admin_", "jobs_", "library_", "settings_", "uploads_", "users_"];
const SOURCE: &str = "contract/i18n";

const APPLE_DIR: &str =
    "clients/apple/Packages/CouchverseDesign/Sources/CouchverseDesign/Generated";
const ANDROID_RES: &str = "clients/android/design/src/generated/res";

#[derive(Debug, Clone, PartialEq)]
enum Part {
    Text(String),
    Param(String),
}

type Pattern = Vec<Part>;

#[derive(Debug)]
enum Message {
    Text(Pattern),
    /// `variants` maps CLDR categories (`one`, `few`, `many`, `other`) to patterns; the inlang
    /// catch-all `*` is stored as `other`.
    Plural {
        input: String,
        variants: BTreeMap<String, Pattern>,
    },
}

impl Message {
    fn params(&self) -> BTreeSet<String> {
        let patterns: Vec<&Pattern> = match self {
            Message::Text(p) => vec![p],
            Message::Plural { variants, .. } => variants.values().collect(),
        };
        patterns
            .into_iter()
            .flatten()
            .filter_map(|part| match part {
                Part::Param(name) => Some(name.clone()),
                Part::Text(_) => None,
            })
            .collect()
    }

    /// The pattern that decides parameter order: the text, or a plural's `other` form.
    fn order_pattern(&self) -> &Pattern {
        match self {
            Message::Text(p) => p,
            Message::Plural { variants, .. } => &variants["other"],
        }
    }
}

type Catalog = BTreeMap<String, Message>;

pub fn generate(root: &Path, written: &mut Vec<PathBuf>) -> Result<(), String> {
    let mut catalogs = BTreeMap::new();
    for locale in LOCALES {
        let path = root.join(SOURCE).join(format!("{locale}.json"));
        let catalog = parse_catalog(&out::read_json(&path)?)
            .map_err(|e| format!("{}: {e}", path.display()))?;
        catalogs.insert(locale, catalog);
    }
    validate(&catalogs)?;

    let native: Vec<&String> = catalogs[BASE]
        .keys()
        .filter(|key| !ADMIN_PREFIXES.iter().any(|p| key.starts_with(p)))
        .collect();
    let layouts: BTreeMap<&String, Layout> =
        native.iter().map(|key| (*key, Layout::of(&catalogs[BASE][*key]))).collect();

    let apple = root.join(APPLE_DIR);
    out::write(&apple.join("Localizable.xcstrings"), &xcstrings(&catalogs, &layouts), written)?;
    out::write(&apple.join("L10n.swift"), &swift_accessors(&layouts), written)?;

    for locale in LOCALES {
        let dir = if locale == BASE { "values".to_string() } else { format!("values-{locale}") };
        let xml = android_strings(&catalogs[locale], &layouts);
        out::write(&root.join(ANDROID_RES).join(dir).join("strings.xml"), &xml, written)?;
    }
    Ok(())
}

fn parse_catalog(value: &Value) -> Result<Catalog, String> {
    let object = value.as_object().ok_or("expected an object")?;
    let mut catalog = Catalog::new();
    for (key, value) in object {
        if key.starts_with('$') {
            continue;
        }
        let message = parse_message(value).map_err(|e| format!("{key}: {e}"))?;
        catalog.insert(key.clone(), message);
    }
    Ok(catalog)
}

fn parse_message(value: &Value) -> Result<Message, String> {
    if let Some(text) = value.as_str() {
        return Ok(Message::Text(parse_pattern(text)?));
    }
    let [complex] = value.as_array().map(Vec::as_slice).unwrap_or_default() else {
        return Err("expected a string or a one-element array".into());
    };
    let strings = |field: &str| -> Vec<&str> {
        complex[field].as_array().into_iter().flatten().filter_map(Value::as_str).collect()
    };
    let declarations = strings("declarations");
    let selectors = strings("selectors");
    let [selector] = selectors.as_slice() else {
        return Err("exactly one selector is supported".into());
    };
    let input = declarations
        .iter()
        .find_map(|d| {
            let rest = d.strip_prefix("local ")?;
            let (name, value) = rest.split_once(" = ")?;
            (name == *selector).then_some(value.strip_suffix(": plural")?.to_string())
        })
        .ok_or_else(|| {
            format!("selector {selector} must be declared as `local {selector} = <input>: plural`")
        })?;
    if !declarations.contains(&format!("input {input}").as_str()) {
        return Err(format!("missing declaration `input {input}`"));
    }

    let mut variants = BTreeMap::new();
    for (key, pattern) in complex["match"].as_object().ok_or("missing match")? {
        let category = key
            .strip_prefix(&format!("{selector}="))
            .ok_or_else(|| format!("match key {key} must start with {selector}="))?;
        let category = if category == "*" { "other" } else { category };
        if !["zero", "one", "two", "few", "many", "other"].contains(&category) {
            return Err(format!("unknown plural category {category}"));
        }
        let pattern = pattern.as_str().ok_or("match values must be strings")?;
        variants.insert(category.to_string(), parse_pattern(pattern)?);
    }
    if !variants.contains_key("other") {
        return Err("a plural needs a catch-all `*` variant".into());
    }
    Ok(Message::Plural { input, variants })
}

fn parse_pattern(text: &str) -> Result<Pattern, String> {
    let mut parts = Vec::new();
    let mut rest = text;
    while let Some(start) = rest.find('{') {
        if start > 0 {
            parts.push(Part::Text(rest[..start].to_string()));
        }
        let end = rest[start..].find('}').ok_or_else(|| format!("unclosed {{ in {text:?}"))?;
        let name = &rest[start + 1..start + end];
        if name.is_empty() || !name.chars().all(|c| c.is_ascii_alphanumeric() || c == '_') {
            return Err(format!("invalid parameter {{{name}}} in {text:?}"));
        }
        parts.push(Part::Param(name.to_string()));
        rest = &rest[start + end + 1..];
    }
    if !rest.is_empty() {
        parts.push(Part::Text(rest.to_string()));
    }
    Ok(parts)
}

/// Every locale must define the same keys with the same shape and parameters, so no client can
/// ever look up a string that one language lacks.
fn validate(catalogs: &BTreeMap<&str, Catalog>) -> Result<(), String> {
    let base = &catalogs[BASE];
    let mut problems = Vec::new();
    for (locale, catalog) in catalogs {
        for key in base.keys().filter(|k| !catalog.contains_key(*k)) {
            problems.push(format!("{locale}: missing {key}"));
        }
        for key in catalog.keys().filter(|k| !base.contains_key(*k)) {
            problems.push(format!("{locale}: {key} is not in {BASE}"));
        }
        for (key, message) in catalog {
            let Some(reference) = base.get(key) else {
                continue;
            };
            match (reference, message) {
                (Message::Text(_), Message::Text(_)) => {}
                (Message::Plural { input: a, .. }, Message::Plural { input: b, .. }) if a == b => {}
                _ => problems.push(format!(
                    "{locale}: {key} must be a plural over the same input as {BASE}"
                )),
            }
            if reference.params() != message.params() {
                problems.push(format!("{locale}: {key} parameters differ from {BASE}"));
            }
        }
    }
    if problems.is_empty() {
        Ok(())
    } else {
        Err(format!("i18n catalogs disagree:\n  {}", problems.join("\n  ")))
    }
}

/// Positional argument order and types, fixed by the base language so every locale (and the
/// generated accessors) agrees.
struct Layout {
    params: Vec<String>,
    plural_input: Option<String>,
}

impl Layout {
    fn of(message: &Message) -> Self {
        let mut params: Vec<String> = Vec::new();
        for part in message.order_pattern() {
            if let Part::Param(name) = part
                && !params.contains(name)
            {
                params.push(name.clone());
            }
        }
        for name in message.params() {
            if !params.contains(&name) {
                params.push(name);
            }
        }
        let plural_input = match message {
            Message::Plural { input, .. } => Some(input.clone()),
            Message::Text(_) => None,
        };
        Self { params, plural_input }
    }

    fn position(&self, name: &str) -> usize {
        self.params.iter().position(|p| p == name).expect("validated parameter") + 1
    }

    fn is_number(&self, name: &str) -> bool {
        self.plural_input.as_deref() == Some(name)
    }
}

/// Renders a pattern as a printf-style format; `param` formats one parameter and literal `%`
/// is doubled whenever the string takes arguments.
fn format_pattern(pattern: &Pattern, layout: &Layout, param: impl Fn(&str) -> String) -> String {
    let mut s = String::new();
    for part in pattern {
        match part {
            Part::Text(text) if layout.params.is_empty() => s.push_str(text),
            Part::Text(text) => s.push_str(&text.replace('%', "%%")),
            Part::Param(name) => s.push_str(&param(name)),
        }
    }
    s
}

fn apple_param(layout: &Layout, name: &str) -> String {
    let spec = if layout.is_number(name) { "lld" } else { "@" };
    format!("%{}${spec}", layout.position(name))
}

fn xcstrings(catalogs: &BTreeMap<&str, Catalog>, layouts: &BTreeMap<&String, Layout>) -> String {
    let mut strings = Map::new();
    for (key, layout) in layouts {
        let mut localizations = Map::new();
        for (locale, catalog) in catalogs {
            let unit =
                |value: String| json!({"stringUnit": {"state": "translated", "value": value}});
            let localization = match &catalog[*key] {
                Message::Text(pattern) => {
                    unit(format_pattern(pattern, layout, |n| apple_param(layout, n)))
                }
                Message::Plural { input, variants } => {
                    // the whole sentence varies, so it lives inside one substitution; inside a
                    // variation the varied argument itself is written %arg
                    let mut plural = Map::new();
                    for (category, pattern) in variants {
                        let text = format_pattern(pattern, layout, |n| {
                            if n == input { "%arg".to_string() } else { apple_param(layout, n) }
                        });
                        plural.insert(category.clone(), unit(text));
                    }
                    json!({
                        "stringUnit": {"state": "translated", "value": format!("%#@{input}@")},
                        "substitutions": {
                            input: {
                                "argNum": layout.position(input),
                                "formatSpecifier": "lld",
                                "variations": {"plural": plural},
                            }
                        }
                    })
                }
            };
            localizations.insert((*locale).to_string(), localization);
        }
        strings.insert(
            (*key).clone(),
            json!({"extractionState": "manual", "localizations": localizations}),
        );
    }
    let catalog = json!({"sourceLanguage": BASE, "strings": strings, "version": "1.0"});
    serde_json::to_string_pretty(&catalog).expect("serializable") + "\n"
}

fn swift_accessors(layouts: &BTreeMap<&String, Layout>) -> String {
    let mut s = out::header("//", SOURCE);
    s.push_str(
        "\n// L10n.string(_:) and L10n.format(_:_:) resolve keys in the display language.\n",
    );
    s.push_str("extension L10n {\n");
    for (key, layout) in layouts {
        let name = out::camel(key);
        if layout.params.is_empty() {
            let _ = writeln!(s, "    public static var {name}: String {{ string(\"{key}\") }}");
            continue;
        }
        let args: Vec<String> = layout
            .params
            .iter()
            .map(|p| {
                let ty = if layout.is_number(p) { "Int" } else { "String" };
                format!("{}: {ty}", out::camel(p))
            })
            .collect();
        let values: Vec<String> = layout.params.iter().map(|p| out::camel(p)).collect();
        let _ = writeln!(
            s,
            "    public static func {name}({}) -> String {{ format(\"{key}\", [{}]) }}",
            args.join(", "),
            values.join(", ")
        );
    }
    s.push_str("}\n");
    s
}

fn android_strings(catalog: &Catalog, layouts: &BTreeMap<&String, Layout>) -> String {
    let mut s = String::from("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n");
    let _ = writeln!(s, "<!-- {} -->", out::header("", SOURCE).trim());
    s.push_str("<resources>\n");
    for (key, layout) in layouts {
        let param = |name: &str| {
            let spec = if layout.is_number(name) { 'd' } else { 's' };
            format!("%{}${spec}", layout.position(name))
        };
        match &catalog[*key] {
            Message::Text(pattern) => {
                let value = android_escape(&format_pattern(pattern, layout, param));
                let attr = if layout.params.is_empty() && value.contains('%') {
                    " formatted=\"false\""
                } else {
                    ""
                };
                let _ = writeln!(s, "    <string name=\"{key}\"{attr}>{value}</string>");
            }
            Message::Plural { variants, .. } => {
                let _ = writeln!(s, "    <plurals name=\"{key}\">");
                for (category, pattern) in variants {
                    let value = android_escape(&format_pattern(pattern, layout, param));
                    let _ = writeln!(s, "        <item quantity=\"{category}\">{value}</item>");
                }
                s.push_str("    </plurals>\n");
            }
        }
    }
    s.push_str("</resources>\n");
    s
}

fn android_escape(text: &str) -> String {
    let mut s = String::with_capacity(text.len());
    for (i, c) in text.chars().enumerate() {
        match c {
            '&' => s.push_str("&amp;"),
            '<' => s.push_str("&lt;"),
            '>' => s.push_str("&gt;"),
            '\'' => s.push_str("\\'"),
            '"' => s.push_str("\\\""),
            '\\' => s.push_str("\\\\"),
            '\n' => s.push_str("\\n"),
            '@' | '?' if i == 0 => {
                s.push('\\');
                s.push(c);
            }
            _ => s.push(c),
        }
    }
    s
}

#[cfg(test)]
mod tests {
    use super::*;

    fn plural(input: &str, forms: &[(&str, &str)]) -> Value {
        let sel = format!("{input}Plural");
        let mut matches = Map::new();
        for (cat, text) in forms {
            matches.insert(format!("{sel}={cat}"), json!(text));
        }
        json!([{
            "declarations": [format!("input {input}"), format!("local {sel} = {input}: plural")],
            "selectors": [sel],
            "match": matches,
        }])
    }

    #[test]
    fn plain_patterns_split_into_text_and_parameters() {
        let pattern = parse_pattern("Delete “{name}” in {n}%").unwrap();
        assert_eq!(
            pattern,
            vec![
                Part::Text("Delete “".into()),
                Part::Param("name".into()),
                Part::Text("” in ".into()),
                Part::Param("n".into()),
                Part::Text("%".into()),
            ]
        );
    }

    #[test]
    fn plurals_map_the_catch_all_to_other() {
        let message =
            parse_message(&plural("count", &[("one", "{count} den"), ("*", "{count} dní")]))
                .unwrap();
        let Message::Plural { input, variants } = message else { panic!("not a plural") };
        assert_eq!(input, "count");
        assert_eq!(variants.keys().collect::<Vec<_>>(), ["one", "other"]);
    }

    #[test]
    fn plurals_without_a_catch_all_are_rejected() {
        assert!(parse_message(&plural("count", &[("one", "{count} day")])).is_err());
    }

    #[test]
    fn locales_must_agree_on_parameters() {
        let mut catalogs = BTreeMap::new();
        catalogs.insert("en", parse_catalog(&json!({"a": "{x} items"})).unwrap());
        catalogs.insert("cs", parse_catalog(&json!({"a": "{y} položek"})).unwrap());
        let err = validate(&catalogs).unwrap_err();
        assert!(err.contains("cs: a parameters differ"), "{err}");
    }

    #[test]
    fn plural_strings_become_typed_positional_formats() {
        let message = parse_message(&plural(
            "count",
            &[("one", "Delete {count} {kind}?"), ("*", "Delete {count} {kind}s?")],
        ))
        .unwrap();
        let layout = Layout::of(&message);
        assert_eq!(layout.params, ["count", "kind"]);
        let Message::Plural { variants, .. } = &message else { unreachable!() };
        let text = format_pattern(&variants["one"], &layout, |n| apple_param(&layout, n));
        assert_eq!(text, "Delete %1$lld %2$@?");
    }

    #[test]
    fn android_escapes_quotes_and_leading_at() {
        assert_eq!(android_escape("@home it's \"x\" & y"), "\\@home it\\'s \\\"x\\\" &amp; y");
    }
}
