//! Renders the model as `clients/web/src/lib/generated/api.ts`: interfaces for every schema,
//! discriminated unions for the couch frames, one function per JSON operation (over the web's
//! `api()` fetch wrapper, which adds `?lang=`, the cookie and error handling) and a path builder
//! per raw operation.

use std::fmt::Write as _;

use super::model::{Field, FrameUnion, Item, Operation, Param, Response, Ty, pascal};
use crate::out;

pub fn render(
    source: &str,
    types: &[&Item],
    ops: &[Operation],
    couch: &[&Item],
    frames: &[FrameUnion],
) -> String {
    let mut s = out::header("//", source);
    s.push_str("\nimport { api, qs } from '$lib/api/client';\n");
    for item in types.iter().chain(couch) {
        write_item(&mut s, item);
    }
    for union in frames {
        write_frames(&mut s, union);
    }
    for op in ops {
        write_operation(&mut s, op);
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
            let _ = writeln!(s, "{indent} * {line}");
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
    let identifier = field
        .json
        .chars()
        .all(|c| c.is_ascii_alphanumeric() || c == '_');
    let key = if identifier {
        field.json.clone()
    } else {
        format!("'{}'", field.json)
    };
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
    op.query()
        .into_iter()
        .filter(|p| p.name != "lang")
        .collect()
}

fn write_operation(s: &mut String, op: &Operation) {
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

    let mut args: Vec<String> = op
        .path_params()
        .iter()
        .map(|p| format!("{}: {}", p.name, ty(&p.ty)))
        .collect();
    if !query.is_empty() {
        let all_optional = query.iter().all(|p| !p.required);
        args.push(if all_optional {
            format!("query: {query_type} = {{}}")
        } else {
            format!("query: {query_type}")
        });
    }
    if let Some(body) = &op.body {
        args.push(format!("body: {}", ty(body)));
    }

    let mut path = op.path.clone();
    for p in op.path_params() {
        path = path.replace(
            &format!("{{{}}}", p.name),
            &format!("${{encodeURIComponent({})}}", p.name),
        );
    }
    if !query.is_empty() {
        let fields: Vec<String> = query
            .iter()
            .map(|p| {
                let value = match (&p.ty, p.required) {
                    (Ty::List(_), true) => format!("query.{}.join(',')", p.name),
                    (Ty::List(_), false) => format!("query.{}?.join(',')", p.name),
                    (Ty::Bool, true) => format!("String(query.{})", p.name),
                    (Ty::Bool, false) => format!(
                        "query.{0} === undefined ? undefined : String(query.{0})",
                        p.name
                    ),
                    _ => format!("query.{}", p.name),
                };
                format!("{}: {value}", p.name)
            })
            .collect();
        let _ = write!(path, "${{qs({{ {} }})}}", fields.join(", "));
    }

    s.push('\n');
    let doc = match &op.summary {
        Some(summary) => format!("{summary} (`{} {}`)", op.method.to_uppercase(), op.path),
        None => format!("`{} {}`", op.method.to_uppercase(), op.path),
    };
    write_doc(s, "", Some(&doc));
    let name = out::camel(&op.id);
    let args = args.join(", ");
    match &op.response {
        Response::Raw => {
            let _ = writeln!(s, "export const {name}Path = ({args}) => `{path}`;");
        }
        response => {
            let result = match response {
                Response::Json(t) => ty(t),
                _ => "void".to_string(),
            };
            let mut opts = Vec::new();
            if op.method != "Get" {
                opts.push(format!("method: '{}'", op.method.to_uppercase()));
            }
            if op.body.is_some() {
                opts.push("body".to_string());
            }
            let opts = if opts.is_empty() {
                String::new()
            } else {
                format!(", {{ {} }}", opts.join(", "))
            };
            let _ = writeln!(
                s,
                "export const {name} = ({args}) =>\n\tapi<{result}>(`{path}`{opts});"
            );
        }
    }
}
