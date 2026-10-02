//! The shared Couchverse client core (docs/native-clients-plan.md, section 7): a deterministic,
//! sans-I/O state machine that every client (Apple, Android, the web) runs behind the same
//! message bridge. Shells send events and effect outputs in, and receive effects to perform and
//! view models to render; the core never touches the network, a clock or storage itself.

mod api;
mod bridge;
mod core;
mod effects;
pub mod messages;
pub mod modules;
#[cfg(test)]
mod scenarios;

pub use crate::core::{AppPhase, AppView};
pub use bridge::{Bridge, BridgeError};
