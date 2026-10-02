//! The language-neutral model of the API contract: every schema becomes an [`Item`], every
//! operation an [`Operation`]. The emitters render the same model as Rust and TypeScript, so the
//! two clients share names and shapes exactly.
//!
//! The parser covers the subset of `OpenAPI` 3.1 / JSON Schema that huma emits for plain Go
//! structs: objects, arrays, string maps, `$ref`, string enums, `["T", "null"]` nullability and
//! the integer/number formats. Request bodies are JSON, a multipart form or raw
//! `application/octet-stream` bytes. Anything else is an error, so a new construct is a conscious
//! change. Validation keywords (defaults, ranges, lengths, item counts) are not types in either
//! client, so they are folded into the doc comments.

use std::collections::{BTreeMap, BTreeSet};

use serde_json::{Map, Value};

use crate::out;

#[derive(Debug, Clone, PartialEq)]
pub enum Ty {
    String,
    Bool,
    I32,
    I64,
    F64,
    /// A named [`Item`].
    Named(String),
    List(Box<Ty>),
    /// An object keyed by arbitrary strings.
    Map(Box<Ty>),
}

pub enum Item {
    Struct { name: String, doc: Option<String>, fields: Vec<Field> },
    Enum { name: String, doc: Option<String>, values: Vec<String> },
    Alias { name: String, doc: Option<String>, target: Ty },
}

impl Item {
    pub fn name(&self) -> &str {
        match self {
            Item::Struct { name, .. } | Item::Enum { name, .. } | Item::Alias { name, .. } => name,
        }
    }
}

pub struct Field {
    pub json: String,
    pub ty: Ty,
    /// Absent from the JSON object when unset.
    pub optional: bool,
    pub nullable: bool,
    pub doc: Option<String>,
}

pub struct Param {
    pub name: String,
    pub ty: Ty,
    pub required: bool,
    pub in_path: bool,
    pub doc: Option<String>,
}

pub struct Body {
    pub kind: BodyKind,
    pub required: bool,
    pub doc: Option<String>,
}

pub enum BodyKind {
    /// `application/json`.
    Json(Ty),
    /// `multipart/form-data`; the client assembles the form, so its parts are only documented.
    Multipart(Vec<FormPart>),
    /// Raw `application/octet-stream` bytes.
    Binary,
}

pub struct FormPart {
    pub name: String,
    pub required: bool,
    /// A file rather than a text value.
    pub file: bool,
    pub doc: Option<String>,
}

impl FormPart {
    /// `` `file` (file, required) ``, for the list of parts in an operation's docs.
    pub fn label(&self) -> String {
        let traits: Vec<&str> = [(self.file, "file"), (self.required, "required")]
            .into_iter()
            .filter_map(|(set, name)| set.then_some(name))
            .collect();
        if traits.is_empty() {
            format!("`{}`", self.name)
        } else {
            format!("`{}` ({})", self.name, traits.join(", "))
        }
    }
}

pub enum Response {
    Json(Ty),
    NoContent,
    /// Byte streams and sockets: only the request is typed.
    Raw,
}

pub struct Operation {
    pub id: String,
    /// `Get`, `Post`, ...
    pub method: String,
    pub path: String,
    pub summary: Option<String>,
    pub params: Vec<Param>,
    pub body: Option<Body>,
    pub response: Response,
}

impl Operation {
    pub fn query(&self) -> Vec<&Param> {
        self.params.iter().filter(|p| !p.in_path).collect()
    }

    pub fn path_params(&self) -> Vec<&Param> {
        self.params.iter().filter(|p| p.in_path).collect()
    }

    /// The body is a file or a form around one, which a client may need to send itself (to
    /// report upload progress, say).
    pub fn uploads(&self) -> bool {
        self.body.as_ref().is_some_and(|b| !matches!(b.kind, BodyKind::Json(_)))
    }
}

/// One variant of a WebSocket frame union: the envelope `type` and its payload, if any.
pub struct FrameVariant {
    pub tag: String,
    pub payload: Option<String>,
    pub doc: Option<String>,
}

pub struct FrameUnion {
    pub name: String,
    pub doc: String,
    pub variants: Vec<FrameVariant>,
}

const METHODS: [(&str, &str); 5] =
    [("get", "Get"), ("post", "Post"), ("put", "Put"), ("patch", "Patch"), ("delete", "Delete")];

/// Walks one set of named schemas into items; inline enums and objects become items named
/// after their position (`HomeRow.kind` -> `HomeRowKind`).
pub struct Schemas<'a> {
    defs: &'a Map<String, Value>,
    ref_prefix: &'static str,
    pub items: BTreeMap<String, Item>,
}

impl<'a> Schemas<'a> {
    pub fn parse(defs: &'a Map<String, Value>, ref_prefix: &'static str) -> Result<Self, String> {
        let mut schemas = Self { defs, ref_prefix, items: BTreeMap::new() };
        for (name, schema) in schemas.defs {
            let rust = type_name(name);
            if schema.get("enum").is_some() {
                schemas.enum_item(&rust, schema)?;
            } else if schema.get("properties").is_some() {
                schemas.struct_item(&rust, schema)?;
            } else {
                let target = schemas.type_of(schema, &rust)?;
                let doc = description(schema);
                schemas.items.insert(rust.clone(), Item::Alias { name: rust, doc, target });
            }
        }
        Ok(schemas)
    }

    fn struct_item(&mut self, name: &str, schema: &Value) -> Result<(), String> {
        let required: BTreeSet<&str> =
            schema["required"].as_array().into_iter().flatten().filter_map(Value::as_str).collect();
        let mut fields = Vec::new();
        for (json, prop) in schema["properties"].as_object().into_iter().flatten() {
            let ty = self.type_of(prop, &format!("{name}{}", pascal(json)))?;
            fields.push(Field {
                json: json.clone(),
                ty,
                optional: !required.contains(json.as_str()),
                nullable: nullable(prop),
                doc: documented(description(prop), prop),
            });
        }
        let doc = description(schema);
        self.items.insert(name.to_string(), Item::Struct { name: name.to_string(), doc, fields });
        Ok(())
    }

    fn enum_item(&mut self, name: &str, schema: &Value) -> Result<(), String> {
        let values = schema["enum"]
            .as_array()
            .into_iter()
            .flatten()
            .map(|v| v.as_str().map(str::to_string).ok_or("only string enums are supported"))
            .collect::<Result<Vec<_>, _>>()
            .map_err(|e| format!("{name}: {e}"))?;
        let doc = description(schema);
        self.items.insert(name.to_string(), Item::Enum { name: name.to_string(), doc, values });
        Ok(())
    }

    /// The type of a schema; `hint` names inline enums and objects.
    pub fn type_of(&mut self, schema: &Value, hint: &str) -> Result<Ty, String> {
        if let Some(reference) = schema.get("$ref").and_then(Value::as_str) {
            let name = reference
                .strip_prefix(self.ref_prefix)
                .ok_or_else(|| format!("{hint}: unsupported $ref {reference}"))?;
            return Ok(Ty::Named(type_name(name)));
        }
        if schema.get("enum").is_some() {
            self.enum_item(hint, schema)?;
            return Ok(Ty::Named(hint.to_string()));
        }
        Ok(match base_type(schema) {
            Some("string") => Ty::String,
            Some("boolean") => Ty::Bool,
            Some("integer") if schema["format"] == "int32" => Ty::I32,
            Some("integer") => Ty::I64,
            Some("number") => Ty::F64,
            Some("array") => {
                Ty::List(Box::new(self.type_of(&schema["items"], &format!("{hint}Item"))?))
            }
            Some("object") if schema.get("properties").is_some() => {
                self.struct_item(hint, schema)?;
                Ty::Named(hint.to_string())
            }
            Some("object") => match schema.get("additionalProperties") {
                Some(value @ Value::Object(_)) => {
                    Ty::Map(Box::new(self.type_of(value, &format!("{hint}Value"))?))
                }
                _ => return Err(format!("{hint}: free-form objects are not supported")),
            },
            other => return Err(format!("{hint}: unsupported schema type {other:?}")),
        })
    }

    /// Every operation of an `OpenAPI` document, sorted by id.
    pub fn operations(&mut self, spec: &Value) -> Result<Vec<Operation>, String> {
        let mut ops = Vec::new();
        for (path, item) in spec["paths"].as_object().ok_or("no paths")? {
            for (method, variant) in METHODS {
                if let Some(op) = item.get(method) {
                    ops.push(self.operation(path, variant, op)?);
                }
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
            let name = param["name"].as_str().ok_or("parameter without a name")?.to_string();
            let location = param["in"].as_str().unwrap_or_default();
            if location != "path" && location != "query" {
                continue;
            }
            let ty = self.type_of(&param["schema"], &format!("{op_name}{}", pascal(&name)))?;
            params.push(Param {
                required: location == "path" || param["required"] == true,
                in_path: location == "path",
                doc: documented(description(param), &param["schema"]),
                name,
                ty,
            });
        }
        let mut undeclared = path.split('{').skip(1).filter_map(|s| s.split_once('}'));
        if let Some((name, _)) =
            undeclared.find(|(name, _)| !params.iter().any(|p| p.in_path && p.name == *name))
        {
            return Err(format!("{id}: path parameter {{{name}}} is not declared"));
        }

        let body = self.request_body(&op_name, &op["requestBody"])?;
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

    fn request_body(&mut self, op_name: &str, body: &Value) -> Result<Option<Body>, String> {
        let Some(content) = body["content"].as_object() else { return Ok(None) };
        let mut media = content.iter();
        let (Some((media_type, spec)), None) = (media.next(), media.next()) else {
            return Err(format!("{op_name}: a request body needs exactly one media type"));
        };
        let schema = &spec["schema"];
        let kind = match media_type.as_str() {
            "application/json" => BodyKind::Json(self.type_of(schema, &format!("{op_name}Body"))?),
            "multipart/form-data" => BodyKind::Multipart(form_parts(op_name, schema)?),
            "application/octet-stream" => BodyKind::Binary,
            other => return Err(format!("{op_name}: unsupported request body {other}")),
        };
        Ok(Some(Body { kind, required: body["required"] == true, doc: description(body) }))
    }
}

fn form_parts(op_name: &str, schema: &Value) -> Result<Vec<FormPart>, String> {
    let required: BTreeSet<&str> =
        schema["required"].as_array().into_iter().flatten().filter_map(Value::as_str).collect();
    let mut parts = Vec::new();
    for (name, prop) in schema["properties"].as_object().into_iter().flatten() {
        if !matches!(base_type(prop), Some("string" | "integer" | "number" | "boolean")) {
            return Err(format!("{op_name}: form part {name} is not a scalar"));
        }
        // no type is generated for a part, so its closed set is spelled out
        let values: Vec<String> = prop["enum"]
            .as_array()
            .into_iter()
            .flatten()
            .filter_map(Value::as_str)
            .map(|v| format!("`{v}`"))
            .collect();
        let one_of = (!values.is_empty()).then(|| format!("One of {}.", values.join(", ")));
        let lines: Vec<String> =
            [description(prop), one_of, constraints(prop)].into_iter().flatten().collect();
        parts.push(FormPart {
            name: name.clone(),
            required: required.contains(name.as_str()),
            file: prop["format"] == "binary",
            doc: (!lines.is_empty()).then(|| lines.join("\n")),
        });
    }
    Ok(parts)
}

/// A `oneOf` of `{"type": {"const": tag}, "data": {"$ref": ...}}` objects.
pub fn frame_union(name: &str, doc: &str, union: &Value) -> Result<FrameUnion, String> {
    let mut variants = Vec::new();
    for variant in union["oneOf"].as_array().into_iter().flatten() {
        let props = &variant["properties"];
        let tag = props["type"]["const"]
            .as_str()
            .ok_or_else(|| format!("{name}: a variant has no const type"))?;
        variants.push(FrameVariant {
            tag: tag.to_string(),
            payload: props["data"]["$ref"]
                .as_str()
                .map(|r| type_name(r.trim_start_matches("#/$defs/"))),
            doc: description(variant),
        });
    }
    Ok(FrameUnion { name: name.to_string(), doc: doc.to_string(), variants })
}

fn base_type(schema: &Value) -> Option<&str> {
    match &schema["type"] {
        Value::String(t) => Some(t),
        Value::Array(types) => types.iter().filter_map(Value::as_str).find(|t| *t != "null"),
        _ => None,
    }
}

fn nullable(schema: &Value) -> bool {
    schema["type"].as_array().is_some_and(|types| types.iter().any(|t| t == "null"))
}

fn description(schema: &Value) -> Option<String> {
    schema["description"].as_str().map(str::to_string)
}

/// A description followed by the schema's validation keywords on a line of their own.
fn documented(description: Option<String>, schema: &Value) -> Option<String> {
    match (description, constraints(schema)) {
        (Some(text), Some(rules)) => Some(format!("{text}\n{rules}")),
        (text, rules) => text.or(rules),
    }
}

/// The default and the bounds on a value, its length or its item count as a sentence, such as
/// "From 1 to 60 characters.".
fn constraints(schema: &Value) -> Option<String> {
    let mut parts = Vec::new();
    if let Some(default) = schema.get("default") {
        parts.push(format!("default `{}`", literal(default)));
    }
    let bounds = [
        ("minimum", "maximum", ("", "")),
        ("minLength", "maxLength", (" character", " characters")),
        ("minItems", "maxItems", (" item", " items")),
    ];
    for (min, max, (one, many)) in bounds {
        let (min, max) = (schema.get(min).map(literal), schema.get(max).map(literal));
        let unit = |n: &str| if n == "1" { one } else { many };
        parts.push(match (min, max) {
            (Some(min), Some(max)) if min == max => format!("exactly {min}{}", unit(&min)),
            (Some(min), Some(max)) => format!("from {min} to {max}{many}"),
            (Some(min), None) => format!("at least {min}{}", unit(&min)),
            (None, Some(max)) => format!("at most {max}{}", unit(&max)),
            (None, None) => continue,
        });
    }
    let sentence = parts.join(", ");
    let mut chars = sentence.chars();
    let first = chars.next()?;
    Some(format!("{}{}.", first.to_uppercase(), chars.as_str()))
}

fn literal(value: &Value) -> String {
    value.as_str().map_or_else(|| value.to_string(), str::to_string)
}

/// A markdown list item `- label: doc`, the doc's further lines indented under it.
pub fn bullet(label: &str, doc: Option<&str>) -> Vec<String> {
    let mut lines = doc.into_iter().flat_map(str::lines);
    let mut out = vec![match lines.next() {
        Some(first) => format!("- {label}: {first}"),
        None => format!("- {label}"),
    }];
    out.extend(lines.map(|line| format!("  {line}")));
    out
}

/// Schema names are type names already, except for acronym runs: `APIError` -> `ApiError`.
pub fn type_name(name: &str) -> String {
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
pub fn pascal(name: &str) -> String {
    let camel = out::camel(name);
    let mut chars = camel.chars();
    chars.next().map(|c| c.to_uppercase().chain(chars).collect()).unwrap_or_default()
}

/// `getTitle` -> `get_title`, `playlistURL` -> `playlist_url`, `surface-2` -> `surface_2`.
pub fn snake(name: &str) -> String {
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

/// `continue_watching` -> `ContinueWatching`, `1080p` -> `V1080p`, and the empty string ->
/// `Empty`.
pub fn variant(value: &str) -> String {
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
    fn names_follow_conventions() {
        assert_eq!(type_name("APIError"), "ApiError");
        assert_eq!(type_name("HomeRow"), "HomeRow");
        assert_eq!(snake("adminGetTitleStorage"), "admin_get_title_storage");
        assert_eq!(snake("playlistURL"), "playlist_url");
        assert_eq!(variant("continue_watching"), "ContinueWatching");
        assert_eq!(variant("1080p"), "V1080p");
        assert_eq!(variant(""), "Empty");
    }

    #[test]
    fn constraints_read_as_a_sentence() {
        let rules = |schema: Value| constraints(&schema);
        assert_eq!(
            rules(serde_json::json!({"default": 50, "minimum": 1, "maximum": 200})).as_deref(),
            Some("Default `50`, from 1 to 200.")
        );
        assert_eq!(rules(serde_json::json!({"default": "all"})).as_deref(), Some("Default `all`."));
        assert_eq!(
            rules(serde_json::json!({"minLength": 1, "maxLength": 60})).as_deref(),
            Some("From 1 to 60 characters.")
        );
        assert_eq!(
            rules(serde_json::json!({"minLength": 1})).as_deref(),
            Some("At least 1 character.")
        );
        assert_eq!(
            rules(serde_json::json!({"minItems": 10, "maxItems": 10})).as_deref(),
            Some("Exactly 10 items.")
        );
        assert_eq!(rules(serde_json::json!({"type": "string"})), None);
    }

    #[test]
    fn request_bodies_keep_their_kind() {
        let defs = Map::new();
        let mut schemas = Schemas::parse(&defs, "#/components/schemas/").unwrap();
        let form = serde_json::json!({
            "required": true,
            "content": {"multipart/form-data": {"schema": {
                "type": "object",
                "required": ["file"],
                "properties": {
                    "file": {"type": "string", "format": "binary", "description": "An image."},
                    "kind": {"type": "string", "enum": ["poster", "backdrop"]}
                }
            }}}
        });
        let body = schemas.request_body("Upload", &form).unwrap().unwrap();
        let BodyKind::Multipart(parts) = body.kind else { panic!("not a form") };
        assert!(body.required);
        assert_eq!(parts.len(), 2);
        assert!(parts[0].file && parts[0].required);
        assert_eq!(parts[1].doc.as_deref(), Some("One of `poster`, `backdrop`."));

        let bytes = serde_json::json!({"content": {"application/octet-stream": {
            "schema": {"type": "string", "format": "binary"}
        }}});
        let body = schemas.request_body("Append", &bytes).unwrap().unwrap();
        assert!(matches!(body.kind, BodyKind::Binary) && !body.required);

        let xml = serde_json::json!({"content": {"application/xml": {"schema": {}}}});
        assert!(schemas.request_body("Xml", &xml).is_err());
        assert!(schemas.request_body("None", &Value::Null).unwrap().is_none());
    }
}
