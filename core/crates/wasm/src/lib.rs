//! The core bridge for the web (plan 7.1). wasm is single-threaded and wasm-bindgen's borrow flag
//! already rejects re-entrant calls, so the bridge needs no lock here.

use couchverse_core::Bridge;
use wasm_bindgen::prelude::*;

#[wasm_bindgen]
pub struct CoreBridge {
    inner: Bridge,
}

#[wasm_bindgen]
impl CoreBridge {
    /// `config` is a `CoreConfig`.
    #[wasm_bindgen(constructor)]
    pub fn new(config: &str) -> Result<CoreBridge, JsError> {
        Ok(Self { inner: Bridge::new(config)? })
    }

    /// `message` is a `Message`; returns the `EffectRequest`s to perform.
    pub fn send(&mut self, message: &str) -> Result<String, JsError> {
        Ok(self.inner.send(message)?)
    }

    /// `resolution` is a `Resolution`; returns the `EffectRequest`s to perform.
    pub fn resolve(&mut self, resolution: &str) -> Result<String, JsError> {
        Ok(self.inner.resolve(resolution)?)
    }

    /// `surface` is a `Surface`; returns its view model.
    pub fn view(&self, surface: &str) -> Result<String, JsError> {
        Ok(self.inner.view(surface)?)
    }
}
