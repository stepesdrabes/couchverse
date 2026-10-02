//! `contract/openapi.json` -> `core/crates/api/src/generated.rs`: one Rust type per schema and
//! one typed builder per operation (see the `couchverse-api` crate docs).
//!
//! The generator covers the subset of `OpenAPI` 3.1 that huma emits for plain Go structs: objects,
//! arrays, string maps, `$ref`, string enums, `["T", "null"]` nullability and the integer/number
//! formats. Anything else is an error, so a new construct is a conscious change here.

use std::collections::{BTreeMap, BTreeSet};
use std::fmt::Write as _;
use std::io::Write as _;
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};

use serde_json::{Map, Value};

use crate::out;

const SOURCE: &str = "contract/openapi.json";
const TARGET: &str = "core/crates/api/src/generated.rs";
const METHODS: [(&str, &str); 5] = [
    ("get", "Get"),
    ("post", "Post"),
    ("put", "Put"),
    ("patch", "Patch"),
    ("delete", "Delete"),
];

pub fn generate(root: &Path, written: &mut Vec<PathBuf>) -> Result<(), String> {
    let spec = out::read_json(&root.join(SOURCE))?;
    let mut generator = Generator::new(&spec)?;
    generator.collect_schemas()?;
    let ops = generator.operations()?;
    let code = format!(
        "{}\n{}\n{}",
        out::header("//", SOURCE),
        generator.types_module(),
        ops_module(&ops)
    );
    out::write(&root.join(TARGET), &rustfmt(&code)?, written)
}

fn rustfmt(code: &str) -> Result<String, String> {
    let mut child = Command::new("rustfmt")
        .args(["--edition", "2024", "--emit", "stdout"])
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
    let output = child
        .wait_with_output()
        .map_err(|e| format!("rustfmt: {e}"))?;
    if !output.status.success() {
        return Err(format!(
            "rustfmt rejected the generated code:\n{}",
            String::from_utf8_lossy(&output.stderr)
        ));
    }
    String::from_utf8(output.stdout).map_err(|e| format!("rustfmt: {e}"))
}

/// A generated Rust item.
enum Item {
    Struct {
        name: String,
        doc: Option<String>,
        fields: Vec<Field>,
    },
    Enum {
        name: String,
        doc: Option<String>,
        values: Vec<String>,
    },
    Alias {
        name: String,
        doc: Option<String>,
        target: String,
    },
}

struct Field {
    json: String,
    rust: String,
    ty: String,
    optional: bool,
    doc: Option<String>,
}

struct Generator<'a> {
    spec: &'a Value,
    schemas: &'a Map<String, Value>,
    items: BTreeMap<String, Item>,
}

impl<'a> Generator<'a> {
    fn new(spec: &'a Value) -> Result<Self, String> {
        let schemas = spec["components"]["schemas"]
            .as_object()
            .ok_or_else(|| format!("{SOURCE}: no components.schemas"))?;
        Ok(Self {
            spec,
            schemas,
            items: BTreeMap::new(),
        })
    }

    fn collect_schemas(&mut self) -> Result<(), String> {
        for (name, schema) in self.schemas {
            let rust = type_name(name);
            let doc = description(schema);
            if schema.get("enum").is_some() {
                self.enum_item(&rust, schema)?;
            } else if schema.get("properties").is_some() {
                self.struct_item(&rust, schema)?;
            } else {
                let target = self.type_of(schema, &rust)?;
                self.items.insert(
                    rust.clone(),
                    Item::Alias {
                        name: rust,
                        doc,
                        target,
                    },
                );
            }
        }
        Ok(())
    }

    fn struct_item(&mut self, name: &str, schema: &Value) -> Result<(), String> {
        let required: BTreeSet<&str> = schema["required"]
            .as_array()
            .into_iter()
            .flatten()
            .filter_map(Value::as_str)
            .collect();
        let mut fields = Vec::new();
        for (json, prop) in schema["properties"].as_object().into_iter().flatten() {
            let hint = format!("{name}{}", pascal(json));
            let ty = self.type_of(prop, &hint)?;
            let optional = !required.contains(json.as_str()) || nullable(prop);
            fields.push(Field {
                json: json.clone(),
                rust: field_name(json),
                ty,
                optional,
                doc: description(prop),
            });
        }
        let doc = description(schema);
        self.items.insert(
            name.to_string(),
            Item::Struct {
                name: name.to_string(),
                doc,
                fields,
            },
        );
        Ok(())
    }

    fn enum_item(&mut self, name: &str, schema: &Value) -> Result<(), String> {
        let values = schema["enum"]
            .as_array()
            .into_iter()
            .flatten()
            .map(|v| {
                v.as_str()
                    .map(str::to_string)
                    .ok_or("only string enums are supported")
            })
            .collect::<Result<Vec<_>, _>>()
            .map_err(|e| format!("{name}: {e}"))?;
        let doc = description(schema);
        self.items.insert(
            name.to_string(),
            Item::Enum {
                name: name.to_string(),
                doc,
                values,
            },
        );
        Ok(())
    }

    /// The Rust type for a schema; inline enums and objects become items named after `hint`.
    fn type_of(&mut self, schema: &Value, hint: &str) -> Result<String, String> {
        if let Some(reference) = schema.get("$ref").and_then(Value::as_str) {
            let name = reference
                .strip_prefix("#/components/schemas/")
                .ok_or_else(|| format!("{hint}: unsupported $ref {reference}"))?;
            return Ok(type_name(name));
        }
        if schema.get("enum").is_some() {
            self.enum_item(hint, schema)?;
            return Ok(hint.to_string());
        }
        let ty = match base_type(schema) {
            Some("string") => "String".to_string(),
            Some("boolean") => "bool".to_string(),
            Some("integer") if schema["format"] == "int32" => "i32".to_string(),
            Some("integer") => "i64".to_string(),
            Some("number") => "f64".to_string(),
            Some("array") => {
                let item = self.type_of(&schema["items"], &format!("{hint}Item"))?;
                format!("Vec<{item}>")
            }
            Some("object") if schema.get("properties").is_some() => {
                self.struct_item(hint, schema)?;
                hint.to_string()
            }
            Some("object") => match schema.get("additionalProperties") {
                Some(value @ Value::Object(_)) => {
                    let inner = self.type_of(value, &format!("{hint}Value"))?;
                    format!("BTreeMap<String, {inner}>")
                }
                _ => return Err(format!("{hint}: free-form objects are not supported")),
            },
            other => return Err(format!("{hint}: unsupported schema type {other:?}")),
        };
        Ok(ty)
    }

    fn types_module(&self) -> String {
        let mut s = String::from(
            "pub mod types {\n#![allow(clippy::doc_markdown)]\nuse std::collections::BTreeMap;\nuse serde::{Deserialize, Serialize};\n",
        );
        for item in self.items.values() {
            s.push('\n');
            match item {
                Item::Struct { name, doc, fields } => {
                    write_struct(&mut s, name, doc.as_deref(), fields);
                }
                Item::Enum { name, doc, values } => {
                    write_enum(&mut s, name, doc.as_deref(), values);
                }
                Item::Alias { name, doc, target } => {
                    write_doc(&mut s, doc.as_deref());
                    let _ = writeln!(s, "pub type {name} = {target};");
                }
            }
        }
        s.push_str("}\n");
        s
    }

    fn operations(&mut self) -> Result<Vec<Operation>, String> {
        let mut ops = Vec::new();
        let paths = self.spec["paths"].as_object().ok_or("no paths")?.clone();
        for (path, item) in &paths {
            for (method, variant) in METHODS {
                let Some(op) = item.get(method) else { continue };
                ops.push(self.operation(path, variant, op)?);
            }
        }
        ops.sort_by(|a, b| a.id.cmp(&b.id));
        Ok(ops)
    }

    fn operation(&mut self, path: &str, method: &str, op: &Value) -> Result<Operation, String> {
        let id = op["operationId"]
            .as_str()
            .ok_or_else(|| format!("{method} {path}: no operationId"))?
            .to_string();
        let op_name = pascal(&id);
        let mut params = Vec::new();
        for param in op["parameters"].as_array().into_iter().flatten() {
            let name = param["name"]
                .as_str()
                .ok_or("parameter without a name")?
                .to_string();
            let location = param["in"].as_str().unwrap_or_default().to_string();
            if location != "path" && location != "query" {
                continue;
            }
            let hint = format!("{op_name}{}", pascal(&name));
            let ty = self.type_of(&param["schema"], &hint)?;
            params.push(Param {
                rust: field_name(&name),
                required: location == "path" || param["required"] == true,
                in_path: location == "path",
                doc: description(param),
                name,
                ty,
            });
        }
        let body = match op["requestBody"]["content"]["application/json"]["schema"] {
            Value::Null => None,
            ref schema => Some(self.type_of(schema, &format!("{op_name}Body"))?),
        };
        let mut response = Response::Raw;
        for (status, resp) in op["responses"].as_object().into_iter().flatten() {
            if !status.starts_with('2') {
                continue;
            }
            response = match &resp["content"]["application/json"]["schema"] {
                Value::Null if status == "204" => Response::NoContent,
                Value::Null => Response::Raw,
                schema => Response::Json(self.type_of(schema, &format!("{op_name}Response"))?),
            };
        }
        Ok(Operation {
            id,
            method: method.to_string(),
            path: path.to_string(),
            summary: op["summary"].as_str().map(str::to_string),
            params,
            body,
            response,
        })
    }
}

struct Param {
    name: String,
    rust: String,
    ty: String,
    required: bool,
    in_path: bool,
    doc: Option<String>,
}

enum Response {
    Json(String),
    NoContent,
    /// Byte streams, uploads and sockets: only the request is typed.
    Raw,
}

struct Operation {
    id: String,
    method: String,
    path: String,
    summary: Option<String>,
    params: Vec<Param>,
    body: Option<String>,
    response: Response,
}

fn ops_module(ops: &[Operation]) -> String {
    let mut s = String::from(
        "pub mod ops {\n#![allow(clippy::doc_markdown, clippy::too_many_lines, clippy::wildcard_imports)]\nuse super::types::*;\nuse std::collections::BTreeMap;\nuse crate::{build, Call, Method, NoContent, Request};\n",
    );
    for op in ops {
        write_query_struct(&mut s, op);
        write_operation(&mut s, op);
    }
    s.push_str("}\n");
    s
}

fn query_params(op: &Operation) -> Vec<&Param> {
    op.params.iter().filter(|p| !p.in_path).collect()
}

fn write_query_struct(s: &mut String, op: &Operation) {
    let query = query_params(op);
    if query.is_empty() {
        return;
    }
    let _ = writeln!(s, "\n/// Query parameters of [`{}`].", snake(&op.id));
    s.push_str("#[derive(Debug, Clone, Default, PartialEq)]\n");
    let _ = writeln!(s, "pub struct {}Query {{", pascal(&op.id));
    for p in &query {
        write_doc(s, p.doc.as_deref());
        let ty = if p.required {
            p.ty.clone()
        } else {
            format!("Option<{}>", p.ty)
        };
        let _ = writeln!(s, "pub {}: {ty},", p.rust);
    }
    s.push_str("}\n");
}

fn write_operation(s: &mut String, op: &Operation) {
    let query = query_params(op);
    let path_params: Vec<&Param> = op.params.iter().filter(|p| p.in_path).collect();

    let mut args: Vec<String> = path_params
        .iter()
        .map(|p| {
            format!(
                "{}: {}",
                p.rust,
                if p.ty == "String" { "&str" } else { &p.ty }
            )
        })
        .collect();
    if !query.is_empty() {
        args.push(format!("query: &{}Query", pascal(&op.id)));
    }
    if let Some(body) = &op.body {
        args.push(format!("body: &{body}"));
    }
    let (ret, wrap) = match &op.response {
        Response::Json(ty) => (format!("Call<{ty}>"), "build::json"),
        Response::NoContent => ("Call<NoContent>".to_string(), "build::no_content"),
        Response::Raw => ("Request".to_string(), ""),
    };

    s.push('\n');
    if let Some(summary) = &op.summary {
        let _ = writeln!(s, "/// {summary}\n///");
    }
    let _ = writeln!(s, "/// `{} {}`", op.method.to_uppercase(), op.path);
    let _ = writeln!(
        s,
        "pub fn {}({}) -> {ret} {{",
        snake(&op.id),
        args.join(", ")
    );

    let mut path = op.path.clone();
    let mut path_args = Vec::new();
    for p in &path_params {
        path = path.replace(&format!("{{{}}}", p.name), "{}");
        path_args.push(if p.ty == "String" {
            format!("build::segment({})", p.rust)
        } else {
            format!("build::segment(&{}.to_string())", p.rust)
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
        let value = if p.required {
            format!("Some(&query.{})", p.rust)
        } else {
            format!("query.{}.as_ref()", p.rust)
        };
        let _ = writeln!(s, "build::push(&mut q, \"{}\", {value});", p.name);
    }
    let body = if op.body.is_some() {
        "Some(body)"
    } else {
        "None::<&()>"
    };
    let request = format!(
        "build::request(Method::{}, {path_expr}, q, {body})",
        op.method
    );
    if wrap.is_empty() {
        let _ = writeln!(s, "{request}\n}}");
    } else {
        let _ = writeln!(s, "{wrap}({request})\n}}");
    }
}

fn write_doc(s: &mut String, doc: Option<&str>) {
    for line in doc.into_iter().flat_map(str::lines) {
        let _ = writeln!(s, "/// {line}");
    }
}

fn write_struct(s: &mut String, name: &str, doc: Option<&str>, fields: &[Field]) {
    write_doc(s, doc);
    s.push_str("#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]\n");
    s.push_str("#[serde(rename_all = \"camelCase\")]\n");
    let _ = writeln!(s, "pub struct {name} {{");
    for field in fields {
        write_doc(s, field.doc.as_deref());
        if camel(field.rust.trim_start_matches("r#")) != field.json {
            let _ = writeln!(s, "#[serde(rename = \"{}\")]", field.json);
        }
        if field.optional {
            s.push_str("#[serde(default, skip_serializing_if = \"Option::is_none\")]\n");
            let _ = writeln!(s, "pub {}: Option<{}>,", field.rust, field.ty);
        } else {
            let _ = writeln!(s, "pub {}: {},", field.rust, field.ty);
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
    let _ = writeln!(
        s,
        "impl {name} {{\npub fn as_str(self) -> &'static str {{\nmatch self {{"
    );
    for value in values {
        let _ = writeln!(s, "{name}::{} => \"{value}\",", variant(value));
    }
    let _ = writeln!(s, "{name}::Unknown => \"unknown\",\n}}\n}}\n}}\n");
    let _ = writeln!(
        s,
        "impl std::fmt::Display for {name} {{\nfn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {{\nf.write_str(self.as_str())\n}}\n}}"
    );
}

fn base_type(schema: &Value) -> Option<&str> {
    match &schema["type"] {
        Value::String(t) => Some(t),
        Value::Array(types) => types
            .iter()
            .filter_map(Value::as_str)
            .find(|t| *t != "null"),
        _ => None,
    }
}

fn nullable(schema: &Value) -> bool {
    schema["type"]
        .as_array()
        .is_some_and(|types| types.iter().any(|t| t == "null"))
}

fn description(schema: &Value) -> Option<String> {
    schema["description"].as_str().map(str::to_string)
}

/// Schema names are Rust type names already, except for acronym runs: `APIError` -> `ApiError`.
fn type_name(name: &str) -> String {
    let chars: Vec<char> = name.chars().collect();
    let mut out = String::with_capacity(name.len());
    for (i, c) in chars.iter().enumerate() {
        let prev_upper = i > 0 && chars[i - 1].is_uppercase();
        let next_lower = chars.get(i + 1).is_some_and(|n| n.is_lowercase());
        if c.is_uppercase() && prev_upper && !next_lower {
            out.extend(c.to_lowercase());
        } else {
            out.push(*c);
        }
    }
    out
}

/// `listGenres` / `episode_id` / `surface-2` -> `ListGenres` / `EpisodeId` / `Surface2`.
fn pascal(name: &str) -> String {
    let camel = out::camel(name);
    let mut chars = camel.chars();
    chars
        .next()
        .map(|c| c.to_uppercase().chain(chars).collect())
        .unwrap_or_default()
}

/// `getTitle` -> `get_title`, `playlistURL` -> `playlist_url`, `surface-2` -> `surface_2`.
fn snake(name: &str) -> String {
    let chars: Vec<char> = name.chars().collect();
    let mut out = String::with_capacity(name.len() + 4);
    for (i, &c) in chars.iter().enumerate() {
        if c == '-' {
            out.push('_');
            continue;
        }
        if c.is_uppercase() && i > 0 {
            let prev = chars[i - 1];
            let next_lower = chars.get(i + 1).is_some_and(|n| n.is_lowercase());
            if prev.is_lowercase() || prev.is_ascii_digit() || (prev.is_uppercase() && next_lower) {
                out.push('_');
            }
        }
        out.extend(c.to_lowercase());
    }
    out
}

fn camel(snake_name: &str) -> String {
    out::camel(snake_name)
}

const KEYWORDS: [&str; 12] = [
    "type", "ref", "match", "move", "use", "crate", "self", "mod", "fn", "impl", "loop", "where",
];

fn field_name(json: &str) -> String {
    let name = snake(json);
    if KEYWORDS.contains(&name.as_str()) {
        format!("r#{name}")
    } else {
        name
    }
}

/// `continue_watching` -> `ContinueWatching`, `1080p` -> `V1080p`, and the empty string -> `Empty`.
fn variant(value: &str) -> String {
    let name: String = value
        .split(|c: char| !c.is_ascii_alphanumeric())
        .filter(|part| !part.is_empty())
        .map(|part| {
            let mut chars = part.chars();
            chars
                .next()
                .map(|c| c.to_uppercase().chain(chars).collect::<String>())
                .unwrap_or_default()
        })
        .collect();
    match name.chars().next() {
        None => "Empty".to_string(),
        Some(c) if c.is_ascii_digit() => format!("V{name}"),
        Some(_) => name,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn names_follow_rust_conventions() {
        assert_eq!(type_name("APIError"), "ApiError");
        assert_eq!(type_name("HomeRow"), "HomeRow");
        assert_eq!(snake("adminGetTitleStorage"), "admin_get_title_storage");
        assert_eq!(snake("playlistURL"), "playlist_url");
        assert_eq!(field_name("type"), "r#type");
        assert_eq!(variant("continue_watching"), "ContinueWatching");
        assert_eq!(variant("1080p"), "V1080p");
    }
}
