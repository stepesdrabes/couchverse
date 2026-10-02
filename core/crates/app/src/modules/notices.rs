//! Transient notices (a toast on the web, a banner on TV): something the user should know that
//! has no screen of its own, like a My List change the server refused.

use serde::{Deserialize, Serialize};
use typeshare::typeshare;

use crate::effects::Ctx;
use crate::messages::{Surface, U53};

/// Notices left undismissed are dropped oldest first.
const MAX_NOTICES: usize = 5;

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Notice {
    pub id: U53,
    /// Stable and localized by the shell, like `Problem.code`.
    pub code: String,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct NoticesView {
    pub notices: Vec<Notice>,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct NoticeRef {
    pub id: U53,
}

#[derive(Default)]
pub struct Notices {
    next_id: U53,
    view: NoticesView,
}

impl Notices {
    pub fn view(&self) -> NoticesView {
        self.view.clone()
    }

    pub fn push(&mut self, ctx: &mut Ctx, code: &str) {
        self.next_id += 1;
        self.view.notices.push(Notice { id: self.next_id, code: code.to_string() });
        if self.view.notices.len() > MAX_NOTICES {
            self.view.notices.remove(0);
        }
        ctx.render(Surface::Notices);
    }

    pub fn dismiss(&mut self, ctx: &mut Ctx, id: U53) {
        let before = self.view.notices.len();
        self.view.notices.retain(|n| n.id != id);
        if self.view.notices.len() != before {
            ctx.render(Surface::Notices);
        }
    }
}
