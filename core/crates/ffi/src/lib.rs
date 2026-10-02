//! The core bridge for Swift and Kotlin: four string-in, string-out calls (plan 7.1).

use std::sync::{Mutex, MutexGuard};

use couchverse_core::{Bridge, BridgeError};

uniffi::setup_scaffolding!();

#[derive(Debug, uniffi::Error)]
pub enum CoreError {
    InvalidMessage { reason: String },
}

impl std::fmt::Display for CoreError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            CoreError::InvalidMessage { reason } => write!(f, "invalid message: {reason}"),
        }
    }
}

impl From<BridgeError> for CoreError {
    fn from(e: BridgeError) -> Self {
        match e {
            BridgeError::InvalidMessage(reason) => CoreError::InvalidMessage { reason },
        }
    }
}

/// UniFFI hands objects out as `Arc<Self>` and requires `Send + Sync`. Shells call the core from
/// one serial context, so the mutex is never contended; it turns an accidental cross-thread call
/// into a wait instead of a data race.
#[derive(uniffi::Object)]
pub struct CoreBridge {
    inner: Mutex<Bridge>,
}

#[uniffi::export]
impl CoreBridge {
    /// `config` is a `CoreConfig`.
    #[uniffi::constructor]
    pub fn new(config: &str) -> Result<Self, CoreError> {
        Ok(Self { inner: Mutex::new(Bridge::new(config)?) })
    }

    /// `message` is a `Message`; returns the `EffectRequest`s to perform.
    pub fn send(&self, message: &str) -> Result<String, CoreError> {
        Ok(self.lock().send(message)?)
    }

    /// `resolution` is a `Resolution`; returns the `EffectRequest`s to perform.
    pub fn resolve(&self, resolution: &str) -> Result<String, CoreError> {
        Ok(self.lock().resolve(resolution)?)
    }

    /// `surface` is a `Surface`; returns its view model.
    pub fn view(&self, surface: &str) -> Result<String, CoreError> {
        Ok(self.lock().view(surface)?)
    }
}

impl CoreBridge {
    fn lock(&self) -> MutexGuard<'_, Bridge> {
        // a poisoned lock means an earlier call panicked; UniFFI already reported that panic to
        // the shell, so keep serving rather than abort the app
        self.inner.lock().unwrap_or_else(std::sync::PoisonError::into_inner)
    }
}
