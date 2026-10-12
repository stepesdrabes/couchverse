//! The API contract (`contract/openapi.json` and the WebSocket protocols,
//! `contract/couch-protocol.schema.json` and `contract/couchtv-protocol.schema.json`) as typed
//! clients: `core/crates/api/src/generated.rs` for the core and
//! `clients/web/src/lib/generated/api.ts` for the web.

mod model;
mod rust;
mod typescript;

use std::path::{Path, PathBuf};

use serde_json::Map;

use crate::out;

const OPENAPI: &str = "contract/openapi.json";
const RUST_TARGET: &str = "core/crates/api/src/generated.rs";
const TS_TARGET: &str = "clients/web/src/lib/generated/api.ts";

/// A WebSocket protocol: its schema, the Rust module its types go in, and the prefix its frame
/// unions take in TypeScript, where every type shares one namespace.
struct Source {
    schema: &'static str,
    module: &'static str,
    ts_prefix: &'static str,
    what: &'static str,
}

const PROTOCOLS: [Source; 2] = [
    Source {
        schema: "contract/couch-protocol.schema.json",
        module: "couch",
        ts_prefix: "",
        what: "the couch WebSocket",
    },
    Source {
        schema: "contract/couchtv-protocol.schema.json",
        module: "couchtv",
        ts_prefix: "CouchTv",
        what: "the Couch TV WebSocket",
    },
];

/// A protocol read from its schema: the payload types and the two frame unions.
pub struct Protocol<'a> {
    pub module: &'static str,
    pub ts_prefix: &'static str,
    pub items: Vec<&'a model::Item>,
    /// Payloads the HTTP API also has (`CouchSession` embeds the host state): they come from
    /// the same Go types, so the protocol's module reuses those definitions.
    pub shared: Vec<&'a str>,
    pub frames: &'a [model::FrameUnion; 2],
}

pub fn generate(root: &Path, written: &mut Vec<PathBuf>) -> Result<(), String> {
    let spec = out::read_json(&root.join(OPENAPI))?;
    let schemas = spec["components"]["schemas"]
        .as_object()
        .ok_or_else(|| format!("{OPENAPI}: no components.schemas"))?;
    let mut api = model::Schemas::parse(schemas, "#/components/schemas/")?;
    let ops = api.operations(&spec)?;
    let base = spec["servers"][0]["url"].as_str().ok_or_else(|| format!("{OPENAPI}: no server"))?;

    // each protocol's payload schemas, and its two frame unions
    let mut payloads = Vec::new();
    let mut frames = Vec::new();
    for source in &PROTOCOLS {
        let doc = out::read_json(&root.join(source.schema))?;
        let defs =
            doc["$defs"].as_object().ok_or_else(|| format!("{}: no $defs", source.schema))?;
        payloads.push(
            defs.iter()
                .filter(|(name, _)| !name.ends_with("Frame"))
                .map(|(k, v)| (k.clone(), v.clone()))
                .collect::<Map<_, _>>(),
        );
        frames.push([
            model::frame_union(
                "ServerFrame",
                &format!("Frames the server sends on {}.", source.what),
                &defs["ServerFrame"],
            )?,
            model::frame_union(
                "ClientFrame",
                &format!("Frames clients send on {}.", source.what),
                &defs["ClientFrame"],
            )?,
        ]);
    }
    let parsed = payloads
        .iter()
        .map(|p| model::Schemas::parse(p, "#/$defs/"))
        .collect::<Result<Vec<_>, _>>()?;

    let types: Vec<_> = api.items.values().collect();
    let protocols: Vec<Protocol> = PROTOCOLS
        .iter()
        .zip(&parsed)
        .zip(&frames)
        .map(|((source, schemas), frames)| {
            let (shared, items): (Vec<_>, Vec<_>) =
                schemas.items.values().partition(|item| api.items.contains_key(item.name()));
            Protocol {
                module: source.module,
                ts_prefix: source.ts_prefix,
                items,
                shared: shared.iter().map(|item| item.name()).collect(),
                frames,
            }
        })
        .collect();
    let schemas: Vec<&str> = PROTOCOLS.iter().map(|s| s.schema).collect();
    let source = format!("{OPENAPI}, {}", schemas.join(" and "));
    let rust = rust::render(&source, &types, &ops, &protocols)?;
    out::write(&root.join(RUST_TARGET), &rust, written)?;
    let ts = typescript::render(&source, base, &types, &ops, &protocols);
    out::write(&root.join(TS_TARGET), &ts, written)
}
