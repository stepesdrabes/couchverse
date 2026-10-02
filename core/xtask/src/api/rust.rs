//! Renders the model as `core/crates/api/src/generated.rs` (see the `couchverse-api` docs).

use std::fmt::Write as _;
use std::io::Write as _;
use std::process::{Command, Stdio};

use super::model::{
    Field, FrameUnion, Item, Operation, Param, Response, Ty, pascal, snake, variant,
};
use crate::out;

pub fn render(
    source: &str,
    types: &[&Item],
    ops: &[Operation],
    couch: &[&Item],
    shared: &[&str],
    frames: &[FrameUnion],
) -> Result<String, String> {
    let mut s = out::header("//", source);
    s.push('\n');
    s.push_str(&items_module("types", types, &[], &[]));
    s.push_str(&ops_module(ops));
    s.push_str(&items_module("couch", couch, shared, frames));
    rustfmt(&s)
}

fn rustfmt(code: &str) -> Result<String, String> {
    // rustfmt reading stdin looks for its config from the working directory, not the file
    let config = out::repo_root().join("core/rustfmt.toml");
    let mut child = Command::new("rustfmt")
        .args(["--edition", "2024", "--emit", "stdout", "--config-path"])
        .arg(config)
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .map_err(|e| format!("rustfmt: {e}"))?;
    child
        .stdin
        .take()
        .expect("piped stdin")
        .write_all(code.as_bytes())
        .map_err(|e| format!("rustfmt: {e}"))?;
    let output = child.wait_with_output().map_err(|e| format!("rustfmt: {e}"))?;
    if !output.status.success() {
        return Err(format!(
            "rustfmt rejected the generated code:\n{}",
            String::from_utf8_lossy(&output.stderr)
        ));
    }
    String::from_utf8(output.stdout).map_err(|e| format!("rustfmt: {e}"))
}

fn ty(t: &Ty) -> String {
    match t {
        Ty::String => "String".to_string(),
        Ty::Bool => "bool".to_string(),
        Ty::I32 => "i32".to_string(),
        Ty::I64 => "i64".to_string(),
        Ty::F64 => "f64".to_string(),
        Ty::Named(name) => name.clone(),
        Ty::List(item) => format!("Vec<{}>", ty(item)),
        Ty::Map(value) => format!("BTreeMap<String, {}>", ty(value)),
    }
}

const KEYWORDS: [&str; 12] =
    ["type", "ref", "match", "move", "use", "crate", "self", "mod", "fn", "impl", "loop", "where"];

/// A Rust identifier for a JSON name; anything outside `[a-z0-9_]` (`tmdb.api_key`) becomes `_`.
fn field_name(json: &str) -> String {
    let name: String = snake(json)
        .chars()
        .map(|c| if c.is_ascii_alphanumeric() || c == '_' { c } else { '_' })
        .collect();
    if KEYWORDS.contains(&name.as_str()) { format!("r#{name}") } else { name }
}

fn write_doc(s: &mut String, doc: Option<&str>) {
    for line in doc.into_iter().flat_map(str::lines) {
        let _ = writeln!(s, "/// {line}");
    }
}

fn items_module(module: &str, items: &[&Item], shared: &[&str], frames: &[FrameUnion]) -> String {
    let mut s = format!(
        "\npub mod {module} {{\n#![allow(clippy::doc_markdown)]\n#[allow(unused_imports)]\nuse std::collections::BTreeMap;\nuse serde::{{Deserialize, Serialize}};\n"
    );
    if !shared.is_empty() {
        let _ = writeln!(s, "pub use super::types::{{{}}};", shared.join(", "));
    }
    for item in items {
        s.push('\n');
        match item {
            Item::Struct { name, doc, fields } => {
                write_struct(&mut s, name, doc.as_deref(), fields);
            }
            Item::Enum { name, doc, values } => write_enum(&mut s, name, doc.as_deref(), values),
            Item::Alias { name, doc, target } => {
                write_doc(&mut s, doc.as_deref());
                let _ = writeln!(s, "pub type {name} = {};", ty(target));
            }
        }
    }
    for union in frames {
        write_frames(&mut s, union);
    }
    s.push_str("}\n");
    s
}

fn write_struct(s: &mut String, name: &str, doc: Option<&str>, fields: &[Field]) {
    write_doc(s, doc);
    s.push_str("#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]\n");
    s.push_str("#[serde(rename_all = \"camelCase\")]\n");
    let _ = writeln!(s, "pub struct {name} {{");
    for field in fields {
        write_doc(s, field.doc.as_deref());
        let rust = field_name(&field.json);
        if out::camel(rust.trim_start_matches("r#")) != field.json {
            let _ = writeln!(s, "#[serde(rename = \"{}\")]", field.json);
        }
        let t = ty(&field.ty);
        if field.optional {
            s.push_str("#[serde(default, skip_serializing_if = \"Option::is_none\")]\n");
            let _ = writeln!(s, "pub {rust}: Option<{t}>,");
        } else if field.nullable {
            let _ = writeln!(s, "pub {rust}: Option<{t}>,");
        } else {
            let _ = writeln!(s, "pub {rust}: {t},");
        }
    }
    s.push_str("}\n");
}

fn write_enum(s: &mut String, name: &str, doc: Option<&str>, values: &[String]) {
    write_doc(s, doc);
    s.push_str("#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]\n");
    let _ = writeln!(s, "pub enum {name} {{");
    for value in values {
        let _ = writeln!(s, "#[serde(rename = \"{value}\")]\n{},", variant(value));
    }
    s.push_str("/// A value this client does not know yet.\n#[serde(other)]\nUnknown,\n}\n\n");
    let _ = writeln!(s, "impl {name} {{\npub fn as_str(self) -> &'static str {{\nmatch self {{");
    for value in values {
        let _ = writeln!(s, "{name}::{} => \"{value}\",", variant(value));
    }
    let _ = writeln!(s, "{name}::Unknown => \"unknown\",\n}}\n}}\n}}\n");
    let _ = writeln!(
        s,
        "impl std::fmt::Display for {name} {{\nfn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {{\nf.write_str(self.as_str())\n}}\n}}"
    );
}

/// Frame unions serialize as `{"type", "data"}`; decoding is written out by hand because serde's
/// `other` cannot absorb an unknown frame that carries data.
fn write_frames(s: &mut String, union: &FrameUnion) {
    let name = &union.name;
    let _ = writeln!(s, "\n/// {}", union.doc);
    s.push_str("#[derive(Debug, Clone, PartialEq, Serialize)]\n");
    s.push_str("#[serde(tag = \"type\", content = \"data\")]\n");
    let _ = writeln!(s, "pub enum {name} {{");
    for v in &union.variants {
        write_doc(s, v.doc.as_deref());
        let _ = writeln!(s, "#[serde(rename = \"{}\")]", v.tag);
        match &v.payload {
            Some(payload) => {
                let _ = writeln!(s, "{}({payload}),", variant(&v.tag));
            }
            None => {
                let _ = writeln!(s, "{},", variant(&v.tag));
            }
        }
    }
    s.push_str("/// A frame this client does not know yet.\nUnknown,\n}\n");

    let _ = writeln!(
        s,
        "\nimpl<'de> Deserialize<'de> for {name} {{\nfn deserialize<D: serde::Deserializer<'de>>(deserializer: D) -> Result<Self, D::Error> {{"
    );
    s.push_str("#[derive(Deserialize)]\nstruct Envelope {\nr#type: String,\n#[serde(default)]\ndata: serde_json::Value,\n}\n");
    s.push_str("let envelope = Envelope::deserialize(deserializer)?;\nOk(match envelope.r#type.as_str() {\n");
    for v in &union.variants {
        let ctor = format!("{name}::{}", variant(&v.tag));
        if v.payload.is_some() {
            let _ = writeln!(
                s,
                "\"{}\" => {ctor}(serde_json::from_value(envelope.data).map_err(serde::de::Error::custom)?),",
                v.tag
            );
        } else {
            let _ = writeln!(s, "\"{}\" => {ctor},", v.tag);
        }
    }
    let _ = writeln!(s, "_ => {name}::Unknown,\n}})\n}}\n}}");
}

fn ops_module(ops: &[Operation]) -> String {
    let mut s = String::from(
        "\npub mod ops {\n#![allow(clippy::doc_markdown, clippy::too_many_lines, clippy::wildcard_imports)]\nuse super::types::*;\n#[allow(unused_imports)]\nuse std::collections::BTreeMap;\nuse crate::{build, Call, Method, NoContent, Request};\n",
    );
    for op in ops {
        write_query_struct(&mut s, op);
        write_operation(&mut s, op);
    }
    s.push_str("}\n");
    s
}

fn write_query_struct(s: &mut String, op: &Operation) {
    let query = op.query();
    if query.is_empty() {
        return;
    }
    let _ = writeln!(s, "\n/// Query parameters of [`{}`].", snake(&op.id));
    if query.iter().any(|p| p.required) {
        s.push_str("#[derive(Debug, Clone, PartialEq)]\n");
    } else {
        s.push_str("#[derive(Debug, Clone, Default, PartialEq)]\n");
    }
    let _ = writeln!(s, "pub struct {}Query {{", pascal(&op.id));
    for p in &query {
        write_doc(s, p.doc.as_deref());
        let t = if p.required { ty(&p.ty) } else { format!("Option<{}>", ty(&p.ty)) };
        let _ = writeln!(s, "pub {}: {t},", field_name(&p.name));
    }
    s.push_str("}\n");
}

fn path_arg(p: &Param) -> String {
    let t = if p.ty == Ty::String { "&str".to_string() } else { ty(&p.ty) };
    format!("{}: {t}", field_name(&p.name))
}

fn write_operation(s: &mut String, op: &Operation) {
    let query = op.query();
    let path_params = op.path_params();

    let mut args: Vec<String> = path_params.iter().map(|p| path_arg(p)).collect();
    if !query.is_empty() {
        args.push(format!("query: &{}Query", pascal(&op.id)));
    }
    if let Some(body) = &op.body {
        args.push(format!("body: &{}", ty(body)));
    }
    let (ret, wrap) = match &op.response {
        Response::Json(t) => (format!("Call<{}>", ty(t)), "build::json"),
        Response::NoContent => ("Call<NoContent>".to_string(), "build::no_content"),
        Response::Raw => ("Request".to_string(), ""),
    };

    s.push('\n');
    if let Some(summary) = &op.summary {
        let _ = writeln!(s, "/// {summary}\n///");
    }
    let _ = writeln!(s, "/// `{} {}`", op.method.to_uppercase(), op.path);
    let _ = writeln!(s, "pub fn {}({}) -> {ret} {{", snake(&op.id), args.join(", "));

    let mut path = op.path.clone();
    let mut path_args = Vec::new();
    for p in &path_params {
        path = path.replace(&format!("{{{}}}", p.name), "{}");
        let name = field_name(&p.name);
        path_args.push(if p.ty == Ty::String {
            format!("build::segment({name})")
        } else {
            format!("build::segment(&{name}.to_string())")
        });
    }
    let path_expr = if path_args.is_empty() {
        format!("\"{path}\".to_string()")
    } else {
        format!("format!(\"{path}\", {})", path_args.join(", "))
    };

    s.push_str(if query.is_empty() {
        "let q = Vec::new();\n"
    } else {
        "let mut q = Vec::new();\n"
    });
    for p in &query {
        let name = field_name(&p.name);
        // arrays travel as one comma-separated value (huma's explode: false)
        let list = matches!(p.ty, Ty::List(_));
        let value = match (p.required, list) {
            (true, false) => format!("Some(&query.{name})"),
            (true, true) => format!("Some(build::csv(&query.{name}))"),
            (false, false) => format!("query.{name}.as_ref()"),
            (false, true) => format!("query.{name}.as_deref().map(build::csv)"),
        };
        let _ = writeln!(s, "build::push(&mut q, \"{}\", {value});", p.name);
    }
    let body = if op.body.is_some() { "Some(body)" } else { "None::<&()>" };
    let request = format!("build::request(Method::{}, {path_expr}, q, {body})", op.method);
    if wrap.is_empty() {
        let _ = writeln!(s, "{request}\n}}");
    } else {
        let _ = writeln!(s, "{wrap}({request})\n}}");
    }
}
