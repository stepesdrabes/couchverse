//! The API contract (`contract/openapi.json` and `contract/couch-protocol.schema.json`) as typed
//! clients: `core/crates/api/src/generated.rs` for the core and
//! `clients/web/src/lib/generated/api.ts` for the web.

mod model;
mod rust;
mod typescript;

use std::path::{Path, PathBuf};

use serde_json::Map;

use crate::out;

const OPENAPI: &str = "contract/openapi.json";
const COUCH: &str = "contract/couch-protocol.schema.json";
const RUST_TARGET: &str = "core/crates/api/src/generated.rs";
const TS_TARGET: &str = "clients/web/src/lib/generated/api.ts";

pub fn generate(root: &Path, written: &mut Vec<PathBuf>) -> Result<(), String> {
    let spec = out::read_json(&root.join(OPENAPI))?;
    let schemas = spec["components"]["schemas"]
        .as_object()
        .ok_or_else(|| format!("{OPENAPI}: no components.schemas"))?;
    let mut api = model::Schemas::parse(schemas, "#/components/schemas/")?;
    let ops = api.operations(&spec)?;
    let base = spec["servers"][0]["url"].as_str().ok_or_else(|| format!("{OPENAPI}: no server"))?;

    let couch_doc = out::read_json(&root.join(COUCH))?;
    let defs = couch_doc["$defs"].as_object().ok_or_else(|| format!("{COUCH}: no $defs"))?;
    let payloads: Map<_, _> = defs
        .iter()
        .filter(|(name, _)| !name.ends_with("Frame"))
        .map(|(k, v)| (k.clone(), v.clone()))
        .collect();
    let couch = model::Schemas::parse(&payloads, "#/$defs/")?;
    let frames = [
        model::frame_union(
            "ServerFrame",
            "Frames the server sends on the couch WebSocket.",
            &defs["ServerFrame"],
        )?,
        model::frame_union(
            "ClientFrame",
            "Frames clients send on the couch WebSocket.",
            &defs["ClientFrame"],
        )?,
    ];

    let types: Vec<_> = api.items.values().collect();
    // payloads the HTTP API also returns (CouchSession embeds the host state) come from the
    // same Go types, so the couch module reuses those definitions
    let (shared, couch_types): (Vec<_>, Vec<_>) =
        couch.items.values().partition(|item| api.items.contains_key(item.name()));
    let shared: Vec<&str> = shared.iter().map(|item| item.name()).collect();
    let source = format!("{OPENAPI} and {COUCH}");
    let rust = rust::render(&source, &types, &ops, &couch_types, &shared, &frames)?;
    out::write(&root.join(RUST_TARGET), &rust, written)?;
    let ts = typescript::render(&source, base, &types, &ops, &couch_types, &frames);
    out::write(&root.join(TS_TARGET), &ts, written)
}
