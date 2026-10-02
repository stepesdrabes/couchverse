//! Issuing effects. Modules never see effect ids being resolved: they register what they are
//! waiting for as a [`Pending`] value, and the core routes the output back to them.

use std::collections::{HashMap, HashSet};

use crate::core::Pending;
use crate::messages::{
    Effect, EffectRef, EffectRequest, HttpRequest, RenderRequest, StoreOp, StoreRequest, Surface,
    TimerRequest, U53,
};

/// Effects issued so far in this call and the continuations still waiting for outputs.
#[derive(Default)]
pub struct Registry {
    next_id: U53,
    pending: HashMap<U53, Pending>,
    /// Streaming effects (repeating timers) stay registered after an output.
    streaming: HashSet<U53>,
    issued: Vec<EffectRequest>,
    render: Vec<Surface>,
}

impl Registry {
    /// The continuation for an output, removed unless the effect streams.
    pub fn take(&mut self, id: U53) -> Option<Pending> {
        if self.streaming.contains(&id) {
            self.pending.get(&id).cloned()
        } else {
            self.pending.remove(&id)
        }
    }

    /// Everything issued during this call, with renders folded into one `Render` effect.
    pub fn drain(&mut self) -> Vec<EffectRequest> {
        let mut issued = std::mem::take(&mut self.issued);
        let surfaces = std::mem::take(&mut self.render);
        if !surfaces.is_empty() {
            self.next_id += 1;
            issued.push(EffectRequest {
                id: self.next_id,
                effect: Effect::Render(RenderRequest { surfaces }),
            });
        }
        issued
    }

    fn issue(&mut self, effect: Effect, pending: Option<Pending>, streaming: bool) -> U53 {
        self.next_id += 1;
        let id = self.next_id;
        if let Some(pending) = pending {
            self.pending.insert(id, pending);
            if streaming {
                self.streaming.insert(id);
            }
        }
        self.issued.push(EffectRequest { id, effect });
        id
    }

    fn forget(&mut self, id: U53) {
        self.pending.remove(&id);
        self.streaming.remove(&id);
    }

    #[cfg(test)]
    pub fn pending_count(&self) -> usize {
        self.pending.len()
    }
}

/// The context a module runs in for one message: the shell's clock and a way to ask for effects.
pub struct Ctx<'a> {
    pub now: U53,
    registry: &'a mut Registry,
}

impl<'a> Ctx<'a> {
    pub fn new(now: U53, registry: &'a mut Registry) -> Self {
        Self { now, registry }
    }

    pub fn http(&mut self, request: HttpRequest, pending: Pending) -> U53 {
        self.registry.issue(Effect::Http(request), Some(pending), false)
    }

    pub fn after(&mut self, ms: U53, pending: Pending) -> U53 {
        let timer = TimerRequest { after_ms: ms, repeat: false };
        self.registry.issue(Effect::Timer(timer), Some(pending), false)
    }

    pub fn every(&mut self, ms: U53, pending: Pending) -> U53 {
        let timer = TimerRequest { after_ms: ms, repeat: true };
        self.registry.issue(Effect::Timer(timer), Some(pending), true)
    }

    /// Stops a timer; its outputs still in flight are dropped.
    pub fn cancel_timer(&mut self, id: U53) {
        self.registry.forget(id);
        self.registry.issue(Effect::CancelTimer(EffectRef { id }), None, false);
    }

    pub fn store_read(&mut self, key: &str, pending: Pending) {
        self.store(false, key, StoreOp::Read, Some(pending));
    }

    pub fn store_write(&mut self, key: &str, value: String) {
        self.store(false, key, StoreOp::Write(value), None);
    }

    pub fn store_delete(&mut self, key: &str) {
        self.store(false, key, StoreOp::Delete, None);
    }

    pub fn secure_read(&mut self, key: &str, pending: Pending) {
        self.store(true, key, StoreOp::Read, Some(pending));
    }

    pub fn secure_write(&mut self, key: &str, value: String) {
        self.store(true, key, StoreOp::Write(value), None);
    }

    pub fn secure_delete(&mut self, key: &str) {
        self.store(true, key, StoreOp::Delete, None);
    }

    fn store(&mut self, secure: bool, key: &str, op: StoreOp, pending: Option<Pending>) {
        let request = StoreRequest { key: key.to_string(), op };
        let effect = if secure { Effect::SecureStore(request) } else { Effect::Store(request) };
        self.registry.issue(effect, pending, false);
    }

    /// Marks a view model as changed; the shell re-reads it once this call returns.
    pub fn render(&mut self, surface: Surface) {
        if !self.registry.render.contains(&surface) {
            self.registry.render.push(surface);
        }
    }
}
