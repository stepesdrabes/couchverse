//! Editing your own profile: display name and bio, password, avatar and banner. Image bytes never
//! pass through the core: when the user picks a file the shell keeps it and hands the core a
//! handle, and the core asks the shell to upload it with an `Upload` effect.

use couchverse_api::types::{PasswordChange, ProfileUpdate, User};
use couchverse_api::{Call, NoContent, ops};
use serde::{Deserialize, Serialize};
use typeshare::typeshare;

use crate::api::{Endpoint, Failure, decode};
use crate::core::Pending;
use crate::effects::Ctx;
use crate::messages::{EffectOutput, LoadStatus, Problem, Surface};

/// The form part every image upload carries its file in.
const FILE_FIELD: &str = "file";

#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum ImageSlot {
    Avatar,
    Banner,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ProfileEdit {
    pub display_name: String,
    /// Markdown, at most 2000 characters.
    pub bio: String,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PasswordForm {
    pub current: String,
    /// At least 8 characters.
    pub new: String,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ImageChoice {
    pub slot: ImageSlot,
    /// The shell's handle for the picked file; the upload effect hands it back.
    pub file: String,
}

#[typeshare]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ImageSlotRef {
    pub slot: ImageSlot,
}

/// Where one save stands: `loading` while it runs, `loaded` once it succeeded.
#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SaveState {
    pub status: LoadStatus,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub problem: Option<Problem>,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ProfileEditorView {
    pub details: SaveState,
    pub password: SaveState,
    pub avatar: SaveState,
    pub banner: SaveState,
}

#[derive(Debug, Clone, PartialEq)]
pub enum ProfilePending {
    Details(Call<User>),
    Password(Call<NoContent>),
    Image { slot: ImageSlot, call: Call<User> },
}

#[derive(Debug, PartialEq)]
pub enum ProfileChange {
    None,
    Unauthorized,
    /// The profile changed; the rest of the core shows the new one.
    Updated(User),
}

#[derive(Default)]
pub struct Profile {
    view: ProfileEditorView,
}

impl Profile {
    pub fn view(&self) -> ProfileEditorView {
        self.view.clone()
    }

    pub fn reset(&mut self, ctx: &mut Ctx) {
        self.view = ProfileEditorView::default();
        ctx.render(Surface::ProfileEditor);
    }

    pub fn save_details(&mut self, ctx: &mut Ctx, endpoint: &Endpoint, edit: &ProfileEdit) {
        let body = ProfileUpdate {
            display_name: edit.display_name.trim().to_string(),
            bio: Some(edit.bio.clone()),
        };
        let call = ops::update_profile(&body);
        self.view.details = loading();
        ctx.http(endpoint.request(&call.request), Pending::Profile(ProfilePending::Details(call)));
        ctx.render(Surface::ProfileEditor);
    }

    pub fn change_password(&mut self, ctx: &mut Ctx, endpoint: &Endpoint, form: &PasswordForm) {
        let body = PasswordChange {
            current_password: form.current.clone(),
            new_password: form.new.clone(),
        };
        let call = ops::change_password(&body);
        self.view.password = loading();
        ctx.http(endpoint.request(&call.request), Pending::Profile(ProfilePending::Password(call)));
        ctx.render(Surface::ProfileEditor);
    }

    pub fn upload(&mut self, ctx: &mut Ctx, endpoint: &Endpoint, choice: &ImageChoice) {
        let call = match choice.slot {
            ImageSlot::Avatar => ops::upload_avatar(),
            ImageSlot::Banner => ops::upload_banner(),
        };
        *self.state(choice.slot) = loading();
        let request = endpoint.request(&call.request);
        let pending = Pending::Profile(ProfilePending::Image { slot: choice.slot, call });
        ctx.upload(request, &choice.file, FILE_FIELD, pending);
        ctx.render(Surface::ProfileEditor);
    }

    pub fn remove(&mut self, ctx: &mut Ctx, endpoint: &Endpoint, slot: ImageSlot) {
        let call = match slot {
            ImageSlot::Avatar => ops::delete_avatar(),
            ImageSlot::Banner => ops::delete_banner(),
        };
        *self.state(slot) = loading();
        let request = endpoint.request(&call.request);
        ctx.http(request, Pending::Profile(ProfilePending::Image { slot, call }));
        ctx.render(Surface::ProfileEditor);
    }

    pub fn resolve(
        &mut self,
        ctx: &mut Ctx,
        pending: ProfilePending,
        output: EffectOutput,
    ) -> ProfileChange {
        ctx.render(Surface::ProfileEditor);
        match pending {
            ProfilePending::Details(call) => settle(&mut self.view.details, decode(&call, output)),
            ProfilePending::Image { slot, call } => settle(self.state(slot), decode(&call, output)),
            ProfilePending::Password(call) => match decode(&call, output) {
                Ok(NoContent) => {
                    self.view.password = SaveState { status: LoadStatus::Loaded, problem: None };
                    ProfileChange::None
                }
                Err(failure) => failed(&mut self.view.password, &failure),
            },
        }
    }

    fn state(&mut self, slot: ImageSlot) -> &mut SaveState {
        match slot {
            ImageSlot::Avatar => &mut self.view.avatar,
            ImageSlot::Banner => &mut self.view.banner,
        }
    }
}

fn loading() -> SaveState {
    SaveState { status: LoadStatus::Loading, problem: None }
}

fn settle(state: &mut SaveState, result: Result<User, Failure>) -> ProfileChange {
    match result {
        Ok(user) => {
            *state = SaveState { status: LoadStatus::Loaded, problem: None };
            ProfileChange::Updated(user)
        }
        Err(failure) => failed(state, &failure),
    }
}

fn failed(state: &mut SaveState, failure: &Failure) -> ProfileChange {
    *state = SaveState { status: LoadStatus::Failed, problem: Some(failure.problem()) };
    if failure.unauthorized() { ProfileChange::Unauthorized } else { ProfileChange::None }
}
