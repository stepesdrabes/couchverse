//! JSON in, JSON out: the whole surface the shells call. The ffi (Swift, Kotlin) and wasm (web)
//! crates are thin wrappers around this type, so all three behave identically.

use std::fmt;

use crate::core::Core;
use crate::messages::{CoreConfig, EffectRequest, Message, Resolution, Surface};

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum BridgeError {
    /// The shell sent JSON that does not decode into the expected message.
    InvalidMessage(String),
}

impl fmt::Display for BridgeError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            BridgeError::InvalidMessage(reason) => write!(f, "invalid message: {reason}"),
        }
    }
}

impl std::error::Error for BridgeError {}

fn decode<'a, T: serde::Deserialize<'a>>(json: &'a str) -> Result<T, BridgeError> {
    serde_json::from_str(json).map_err(|e| BridgeError::InvalidMessage(e.to_string()))
}

fn encode(effects: &[EffectRequest]) -> String {
    serde_json::to_string(effects).expect("effects always serialize")
}

pub struct Bridge {
    core: Core,
}

impl Bridge {
    /// `config` is a `CoreConfig`.
    pub fn new(config: &str) -> Result<Self, BridgeError> {
        Ok(Self { core: Core::new(decode::<CoreConfig>(config)?) })
    }

    /// `message` is a `Message`; returns the `EffectRequest`s to perform.
    pub fn send(&mut self, message: &str) -> Result<String, BridgeError> {
        Ok(encode(&self.core.send(decode::<Message>(message)?)))
    }

    /// `resolution` is a `Resolution`; returns the `EffectRequest`s to perform.
    pub fn resolve(&mut self, resolution: &str) -> Result<String, BridgeError> {
        Ok(encode(&self.core.resolve(decode::<Resolution>(resolution)?)))
    }

    /// `surface` is a `Surface`; returns its view model.
    pub fn view(&self, surface: &str) -> Result<String, BridgeError> {
        let surface = decode::<Surface>(surface)?;
        Ok(self.core.view(&surface).to_string())
    }
}
