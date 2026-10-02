//! The generated couch protocol types decode every golden frame the Go server
//! produces (contract/fixtures/couch) and encode them back unchanged.

use std::fs;
use std::path::Path;

use couchverse_api::couch::{ClientFrame, ServerFrame};
use serde_json::Value;

fn golden_frames() -> Vec<(String, Value)> {
    let dir = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../contract/fixtures/couch");
    let mut frames: Vec<(String, Value)> = fs::read_dir(&dir)
        .expect("golden frames")
        .map(|entry| {
            let path = entry.expect("dir entry").path();
            let name = path.file_stem().unwrap().to_string_lossy().into_owned();
            let value = serde_json::from_str(&fs::read_to_string(&path).unwrap()).unwrap();
            (name, value)
        })
        .collect();
    frames.sort_by(|a, b| a.0.cmp(&b.0));
    assert!(!frames.is_empty(), "no golden frames in {}", dir.display());
    frames
}

#[test]
fn every_golden_frame_round_trips() {
    for (name, value) in golden_frames() {
        let encoded = if name.starts_with("server-") {
            let frame: ServerFrame =
                serde_json::from_value(value.clone()).unwrap_or_else(|e| panic!("{name}: {e}"));
            assert_ne!(frame, ServerFrame::Unknown, "{name} decoded as unknown");
            serde_json::to_value(&frame).unwrap()
        } else {
            let frame: ClientFrame =
                serde_json::from_value(value.clone()).unwrap_or_else(|e| panic!("{name}: {e}"));
            assert_ne!(frame, ClientFrame::Unknown, "{name} decoded as unknown");
            serde_json::to_value(&frame).unwrap()
        };
        assert_eq!(
            numbers_as_f64(encoded),
            numbers_as_f64(value),
            "{name} changed in a round trip"
        );
    }
}

/// Go writes a whole float64 as `1840`, Rust as `1840.0`; both are the same JSON number.
fn numbers_as_f64(value: Value) -> Value {
    match value {
        Value::Number(n) => Value::from(n.as_f64().expect("finite")),
        Value::Array(items) => Value::Array(items.into_iter().map(numbers_as_f64).collect()),
        Value::Object(map) => {
            Value::Object(map.into_iter().map(|(k, v)| (k, numbers_as_f64(v))).collect())
        }
        other => other,
    }
}

#[test]
fn frames_from_a_newer_server_decode_as_unknown() {
    let frame: ServerFrame =
        serde_json::from_str(r#"{"type":"confetti","data":{"amount":3}}"#).unwrap();
    assert_eq!(frame, ServerFrame::Unknown);
}
