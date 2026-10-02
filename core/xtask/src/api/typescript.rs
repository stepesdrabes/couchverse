//! Renders the model as `clients/web/src/lib/generated/api.ts`: interfaces for every schema,
//! discriminated unions for the couch frames and one function per typed operation over the web's
//! `api()` fetch wrapper (which adds `?lang=`, the cookie and error handling, and sends a JSON
//! body, a `FormData` or bytes as such). Each function takes a trailing `CallOptions` passed
//! through to `api()`. Raw operations (byte streams, sockets) and uploads also get a path builder
//! returning the full `<base>/...` path, for an `<img src>`, a media element or a request made
//! without `api()`, such as one reporting upload progress.

use std::fmt::Write as _;

use super::model::{
    BodyKind, Field, FrameUnion, Item, Operation, Param, Response, Ty, bullet, pascal,
};
use crate::out;

pub fn render(
    source: &str,
    base: &str,
    types: &[&Item],
    ops: &[Operation],
    couch: &[&Item],
    frames: &[FrameUnion],
) -> String {
    let mut s = out::header("//", source);
    s.push_str("\nimport { api, qs, type CallOptions } from '$lib/api/client';\n");
    for item in types.iter().chain(couch) {
        write_item(&mut s, item);
    }
    for union in frames {
        write_frames(&mut s, union);
    }
    for op in ops {
        write_operation(&mut s, op, base);
    }
    s
}

fn ty(t: &Ty) -> String {
    match t {
        Ty::String => "string".to_string(),
        Ty::Bool => "boolean".to_string(),
        Ty::I32 | Ty::I64 | Ty::F64 => "number".to_string(),
        Ty::Named(name) => name.clone(),
        Ty::List(item) => match item.as_ref() {
            Ty::Named(_) | Ty::String | Ty::Bool | Ty::I32 | Ty::I64 | Ty::F64 => {
                format!("{}[]", ty(item))
            }
            other => format!("Array<{}>", ty(other)),
        },
        Ty::Map(value) => format!("Record<string, {}>", ty(value)),
    }
}

fn write_doc(s: &mut String, indent: &str, doc: Option<&str>) {
    let Some(doc) = doc else { return };
    let lines: Vec<&str> = doc.lines().collect();
    if let [line] = lines.as_slice() {
        let _ = writeln!(s, "{indent}/** {line} */");
    } else {
        let _ = writeln!(s, "{indent}/**");
        for line in lines {
            let _ = writeln!(s, "{}", format!("{indent} * {line}").trim_end());
        }
        let _ = writeln!(s, "{indent} */");
    }
}

fn write_item(s: &mut String, item: &Item) {
    s.push('\n');
    match item {
        Item::Struct { name, doc, fields } => {
            write_doc(s, "", doc.as_deref());
            let _ = writeln!(s, "export interface {name} {{");
            for field in fields {
                write_field(s, field);
            }
            s.push_str("}\n");
        }
        Item::Enum { name, doc, values } => {
            write_doc(s, "", doc.as_deref());
            let union: Vec<String> = values.iter().map(|v| format!("'{v}'")).collect();
            let _ = writeln!(s, "export type {name} = {};", union.join(" | "));
        }
        Item::Alias { name, doc, target } => {
            write_doc(s, "", doc.as_deref());
            let _ = writeln!(s, "export type {name} = {};", ty(target));
        }
    }
}

fn write_field(s: &mut String, field: &Field) {
    write_doc(s, "\t", field.doc.as_deref());
    let optional = if field.optional { "?" } else { "" };
    let null = if field.nullable { " | null" } else { "" };
    let identifier = field.json.chars().all(|c| c.is_ascii_alphanumeric() || c == '_');
    let key = if identifier { field.json.clone() } else { format!("'{}'", field.json) };
    let _ = writeln!(s, "\t{key}{optional}: {}{null};", ty(&field.ty));
}

fn write_frames(s: &mut String, union: &FrameUnion) {
    let _ = writeln!(s, "\n/** {} */", union.doc);
    let _ = writeln!(s, "export type {} =", union.name);
    let variants: Vec<String> = union
        .variants
        .iter()
        .map(|v| match &v.payload {
            Some(payload) => format!("\t| {{ type: '{}'; data: {payload} }}", v.tag),
            None => format!("\t| {{ type: '{}' }}", v.tag),
        })
        .collect();
    let _ = writeln!(s, "{};", variants.join("\n"));
}

/// The web client appends `?lang=` to every request itself.
fn query_params(op: &Operation) -> Vec<&Param> {
    op.query().into_iter().filter(|p| p.name != "lang").collect()
}

/// The argument carrying the body, its type and its `@param` text.
fn body_arg(op: &Operation) -> Option<(&'static str, String, Vec<String>)> {
    let body = op.body.as_ref()?;
    let optional = if body.required { "" } else { "?" };
    let mut doc: Vec<String> = body.doc.iter().flat_map(|d| lines(d)).collect();
    let (name, t) = match &body.kind {
        BodyKind::Json(t) => {
            if !body.required {
                doc.push("May be omitted, which sends no body.".to_string());
            }
            ("body", ty(t))
        }
        BodyKind::Multipart(parts) => {
            doc.push("Sent as `multipart/form-data`, with the parts:".to_string());
            for part in parts {
                doc.extend(bullet(&part.label(), part.doc.as_deref()));
            }
            ("form", "FormData".to_string())
        }
        BodyKind::Binary => {
            doc.push("Sent as `application/octet-stream`.".to_string());
            ("body", "Blob | ArrayBuffer | Uint8Array<ArrayBuffer>".to_string())
        }
    };
    Some((name, format!("{name}{optional}: {t}"), doc))
}

/// `@param name doc` with the doc's further lines under it.
fn param_tag(name: &str, doc: &[String]) -> Vec<String> {
    let mut lines = doc.iter();
    let mut out = vec![match lines.next() {
        Some(first) => format!("@param {name} {first}"),
        None => format!("@param {name}"),
    }];
    out.extend(lines.cloned());
    out
}

fn lines(doc: &str) -> Vec<String> {
    doc.lines().map(str::to_string).collect()
}

/// The path below the API base as a template literal's content, query string included.
fn template_path(op: &Operation, query: &[&Param]) -> String {
    let mut path = op.path.clone();
    for p in op.path_params() {
        path = path
            .replace(&format!("{{{}}}", p.name), &format!("${{encodeURIComponent({})}}", p.name));
    }
    if !query.is_empty() {
        let fields: Vec<String> = query
            .iter()
            .map(|p| {
                let value = match (&p.ty, p.required) {
                    (Ty::List(_), true) => format!("query.{}.join(',')", p.name),
                    (Ty::List(_), false) => format!("query.{}?.join(',')", p.name),
                    (Ty::Bool, true) => format!("String(query.{})", p.name),
                    (Ty::Bool, false) => {
                        format!("query.{0} === undefined ? undefined : String(query.{0})", p.name)
                    }
                    _ => format!("query.{}", p.name),
                };
                format!("{}: {value}", p.name)
            })
            .collect();
        let _ = write!(path, "${{qs({{ {} }})}}", fields.join(", "));
    }
    path
}

/// The summary with method and path, then `@param` tags for documented path parameters and the
/// body.
fn operation_doc(op: &Operation, body: Option<&(&str, String, Vec<String>)>) -> String {
    let mut doc = vec![match &op.summary {
        Some(summary) => format!("{summary} (`{} {}`)", op.method.to_uppercase(), op.path),
        None => format!("`{} {}`", op.method.to_uppercase(), op.path),
    }];
    let mut tags: Vec<String> = op
        .path_params()
        .iter()
        .filter_map(|p| p.doc.as_deref().map(|d| param_tag(&p.name, &lines(d))))
        .flatten()
        .collect();
    if let Some((name, _, body_doc)) = body {
        tags.extend(param_tag(name, body_doc));
    }
    if !tags.is_empty() {
        doc.push(String::new());
        doc.extend(tags);
    }
    doc.join("\n")
}

fn write_operation(s: &mut String, op: &Operation, base: &str) {
    let query = query_params(op);
    let query_type = format!("{}Query", pascal(&op.id));
    if !query.is_empty() {
        let _ = writeln!(s, "\nexport interface {query_type} {{");
        for p in &query {
            write_doc(s, "\t", p.doc.as_deref());
            let optional = if p.required { "" } else { "?" };
            let _ = writeln!(s, "\t{}{optional}: {};", p.name, ty(&p.ty));
        }
        s.push_str("}\n");
    }

    let mut args: Vec<String> =
        op.path_params().iter().map(|p| format!("{}: {}", p.name, ty(&p.ty))).collect();
    if !query.is_empty() {
        let all_optional = query.iter().all(|p| !p.required);
        args.push(if all_optional {
            format!("query: {query_type} = {{}}")
        } else {
            format!("query: {query_type}")
        });
    }
    let path = template_path(op, &query);
    let name = out::camel(&op.id);
    let path_builder =
        format!("export const {name}Path = ({}) => `{base}{path}`;", args.join(", "));
    let body = body_arg(op);

    s.push('\n');
    write_doc(s, "", Some(&operation_doc(op, body.as_ref())));
    let result = match &op.response {
        Response::Raw => {
            let _ = writeln!(s, "{path_builder}");
            return;
        }
        Response::Json(t) => ty(t),
        Response::NoContent => "void".to_string(),
    };
    let mut fields = Vec::new();
    if op.method != "Get" {
        fields.push(format!("method: '{}'", op.method.to_uppercase()));
    }
    if let Some((arg, decl, _)) = body {
        args.push(decl);
        fields.push(if arg == "body" { "body".to_string() } else { format!("body: {arg}") });
    }
    args.push("opts?: CallOptions".to_string());
    let opts = if fields.is_empty() {
        "opts".to_string()
    } else {
        format!("{{ ...opts, {} }}", fields.join(", "))
    };
    let _ = writeln!(
        s,
        "export const {name} = ({}) =>\n\tapi<{result}>(`{path}`, {opts});",
        args.join(", ")
    );
    if op.uploads() {
        let _ = writeln!(
            s,
            "\n/** The full path of {{@link {name}}}, to send its body without `api()`. */\n{path_builder}"
        );
    }
}
