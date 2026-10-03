//! The bridge's JSON wire format, exactly as the Swift, Kotlin and TypeScript shells see it.

use couchverse_core::{Bridge, BridgeError};
use serde_json::{Value, json};

fn bridge() -> Bridge {
    let config = json!({
        "platform": "tvos",
        "authMode": "bearer",
        "deviceName": "Living Room",
        "locale": "en-GB",
    });
    Bridge::new(&config.to_string()).expect("config decodes")
}

fn effects(json: &str) -> Vec<Value> {
    serde_json::from_str(json).expect("effects are a JSON array")
}

#[test]
fn messages_and_effects_are_adjacently_tagged_json() {
    let mut bridge = bridge();
    let out = bridge.send(r#"{"nowMs": 1000, "event": {"type": "appStarted"}}"#).unwrap();
    assert_eq!(
        effects(&out),
        [
            json!({"id": 1, "effect": {"type": "store", "content": {"key": "servers", "op": {"type": "read"}}}}),
            json!({"id": 2, "effect": {"type": "store", "content": {"key": "accounts", "op": {"type": "read"}}}}),
            json!({"id": 3, "effect": {"type": "store", "content": {"key": "player.prefs", "op": {"type": "read"}}}}),
        ]
    );

    let empty = json!({"type": "stored", "content": {}});
    bridge.resolve(&json!({"nowMs": 1010, "id": 1, "output": empty}).to_string()).unwrap();
    let out = bridge.resolve(&json!({"nowMs": 1012, "id": 2, "output": empty}).to_string());
    let render = effects(&out.unwrap()).pop().expect("a render");
    assert_eq!(render["effect"]["type"], "render");
    assert!(
        render["effect"]["content"]["surfaces"]
            .as_array()
            .unwrap()
            .contains(&json!({"type": "app"}))
    );

    let app: Value = serde_json::from_str(&bridge.view(r#"{"type": "app"}"#).unwrap()).unwrap();
    assert_eq!(app, json!({"phase": "welcome"}));
}

#[test]
fn http_effects_carry_everything_a_shell_needs() {
    let mut bridge = bridge();
    let event = json!({
        "nowMs": 5,
        "event": {"type": "serverAddressSubmitted", "content": {"address": "https://tv.home"}},
    });
    let out = effects(&bridge.send(&event.to_string()).unwrap());
    assert_eq!(
        out[0],
        json!({
            "id": 1,
            "effect": {"type": "http", "content": {
                "method": "GET",
                "url": "https://tv.home/api/v1/server",
                "headers": [{"name": "Accept", "value": "application/json"}],
            }},
        })
    );

    let response = json!({
        "type": "http",
        "content": {"status": 200, "body": json!({
            "id": "srv", "name": "Home", "version": "1.0.0", "apiLevel": 1, "accent": "#e50914",
        }).to_string()},
    });
    bridge.resolve(&json!({"nowMs": 9, "id": 1, "output": response}).to_string()).unwrap();
    let servers: Value =
        serde_json::from_str(&bridge.view(r#"{"type": "servers"}"#).unwrap()).unwrap();
    assert_eq!(servers["servers"][0]["url"], "https://tv.home");
    assert_eq!(
        servers["add"],
        json!({"status": "loaded", "address": "https://tv.home", "added": "srv"})
    );
}

#[test]
fn markdown_is_a_view_of_its_source() {
    let bridge = bridge();
    let surface = json!({"type": "markdown", "content": "Hi **there** <b>x</b>"});
    let doc: Value = serde_json::from_str(&bridge.view(&surface.to_string()).unwrap()).unwrap();
    assert_eq!(
        doc,
        json!({"blocks": [{"type": "paragraph", "content": [
            {"type": "text", "content": "Hi "},
            {"type": "strong", "content": [{"type": "text", "content": "there"}]},
            {"type": "text", "content": " <b>x</b>"},
        ]}]})
    );
}

#[test]
fn malformed_messages_are_rejected_without_touching_the_core() {
    let mut bridge = bridge();
    let err = bridge.send(r#"{"nowMs": 1, "event": {"type": "launchMissiles"}}"#).unwrap_err();
    assert!(matches!(err, BridgeError::InvalidMessage(_)));
    assert!(bridge.view("not json").is_err());
    assert!(Bridge::new("{}").is_err());
    // a resolution for an effect nobody waits for is ignored
    let out = bridge.resolve(r#"{"nowMs": 1, "id": 99, "output": {"type": "timerFired"}}"#);
    assert_eq!(out.unwrap(), "[]");
}
