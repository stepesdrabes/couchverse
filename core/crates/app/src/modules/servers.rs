//! The servers this install knows: adding one by address (checked against its `/server`
//! identity, so a random web page never passes as a server), the API-level verdict, and the
//! persisted list.

use couchverse_api::{Call, ops, types::ServerInfo};
use serde::{Deserialize, Serialize};
use typeshare::typeshare;

use crate::api::{Endpoint, Failure, decode};
use crate::core::Pending;
use crate::effects::Ctx;
use crate::messages::{EffectOutput, LoadStatus, Problem, Surface};

/// The oldest server API level this client works with.
pub const MIN_API_LEVEL: i64 = 1;

const STORE_KEY: &str = "servers";

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Server {
    pub id: String,
    /// Base URL without a trailing slash, e.g. `https://media.example.com`.
    pub url: String,
    pub name: String,
    pub version: String,
    pub api_level: u32,
    pub accent: String,
    /// Plain http: shells mark the server as not encrypted.
    pub insecure: bool,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ServerAddress {
    /// What the user typed: a URL, or a host with an optional port.
    pub address: String,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ServerRef {
    pub server_id: String,
}

#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ServersView {
    pub servers: Vec<Server>,
    pub add: AddServerView,
}

/// The add-server flow. `added` names the server once the address checked out, so the shell
/// moves on to signing in.
#[typeshare]
#[derive(Debug, Clone, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AddServerView {
    pub status: LoadStatus,
    pub address: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub added: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub problem: Option<Problem>,
}

#[derive(Debug, Clone, PartialEq)]
pub enum ServersPending {
    Load,
    /// Checking the candidate URL at `index`; the next one is tried when it cannot be reached.
    Identify {
        candidates: Vec<String>,
        index: usize,
        call: Call<ServerInfo>,
    },
}

#[derive(Default)]
pub struct Servers {
    servers: Vec<Server>,
    add: AddServerView,
}

impl Servers {
    pub fn get(&self, id: &str) -> Option<&Server> {
        self.servers.iter().find(|s| s.id == id)
    }

    pub fn view(&self) -> ServersView {
        ServersView { servers: self.servers.clone(), add: self.add.clone() }
    }

    pub fn load(&mut self, ctx: &mut Ctx) {
        ctx.store_read(STORE_KEY, Pending::Servers(ServersPending::Load));
    }

    pub fn submit_address(&mut self, ctx: &mut Ctx, address: &str) {
        let candidates = candidate_urls(address);
        self.add = AddServerView { address: address.to_string(), ..AddServerView::default() };
        if candidates.is_empty() {
            self.add.status = LoadStatus::Failed;
            self.add.problem = Some(Problem::new("invalid_address", "nothing to connect to"));
        } else {
            self.add.status = LoadStatus::Loading;
            identify(ctx, candidates, 0);
        }
        ctx.render(Surface::Servers);
    }

    pub fn remove(&mut self, ctx: &mut Ctx, id: &str) {
        self.servers.retain(|s| s.id != id);
        self.persist(ctx);
        ctx.render(Surface::Servers);
    }

    /// Handles an effect output; returns the server that was just added or updated.
    pub fn resolve(
        &mut self,
        ctx: &mut Ctx,
        pending: ServersPending,
        output: EffectOutput,
    ) -> Option<Server> {
        match pending {
            ServersPending::Load => {
                if let EffectOutput::Stored(stored) = output {
                    self.servers = stored
                        .value
                        .and_then(|json| serde_json::from_str(&json).ok())
                        .unwrap_or_default();
                }
                ctx.render(Surface::Servers);
                None
            }
            ServersPending::Identify { candidates, index, call } => {
                let result = decode(&call, output);
                self.identified(ctx, &candidates, index, result)
            }
        }
    }

    fn identified(
        &mut self,
        ctx: &mut Ctx,
        candidates: &[String],
        index: usize,
        result: Result<ServerInfo, Failure>,
    ) -> Option<Server> {
        let url = &candidates[index];
        let info = match result {
            Ok(info) => info,
            Err(Failure::Network(_)) if index + 1 < candidates.len() => {
                identify(ctx, candidates.to_vec(), index + 1);
                return None;
            }
            Err(failure) => {
                // any HTTP answer that is not a server identity: a web page, a proxy error
                let problem = match &failure {
                    Failure::Api(_) => Problem::new("not_a_server", failure.problem().detail),
                    Failure::Network(_) => failure.problem(),
                };
                self.fail(ctx, problem);
                return None;
            }
        };
        if info.api_level < MIN_API_LEVEL {
            self.fail(
                ctx,
                Problem::new(
                    "server_outdated",
                    format!("{url} is at API level {}", info.api_level),
                ),
            );
            return None;
        }
        let server = Server {
            id: info.id,
            url: url.clone(),
            name: info.name,
            version: info.version,
            api_level: u32::try_from(info.api_level).unwrap_or(u32::MAX),
            accent: info.accent,
            insecure: url.starts_with("http://"),
        };
        // the id is the identity: a server that moved address replaces its old entry
        match self.servers.iter_mut().find(|s| s.id == server.id) {
            Some(existing) => *existing = server.clone(),
            None => self.servers.push(server.clone()),
        }
        self.persist(ctx);
        self.add.status = LoadStatus::Loaded;
        self.add.added = Some(server.id.clone());
        ctx.render(Surface::Servers);
        Some(server)
    }

    fn fail(&mut self, ctx: &mut Ctx, problem: Problem) {
        self.add.status = LoadStatus::Failed;
        self.add.problem = Some(problem);
        ctx.render(Surface::Servers);
    }

    fn persist(&self, ctx: &mut Ctx) {
        let json = serde_json::to_string(&self.servers).expect("servers serialize");
        ctx.store_write(STORE_KEY, json);
    }
}

fn identify(ctx: &mut Ctx, candidates: Vec<String>, index: usize) {
    let call = ops::get_server();
    let request = Endpoint::anonymous(&candidates[index]).request(&call.request);
    ctx.http(request, Pending::Servers(ServersPending::Identify { candidates, index, call }));
}

/// The base URLs to try for what the user typed. Without a scheme https comes first, then http
/// (plain http is allowed, D14); a path or trailing slash is dropped, since `/api/v1` is fixed.
pub fn candidate_urls(address: &str) -> Vec<String> {
    let trimmed = address.trim();
    if trimmed.is_empty() || trimmed.contains(char::is_whitespace) {
        return vec![];
    }
    let (schemes, rest): (&[&str], &str) = if let Some(rest) = trimmed.strip_prefix("https://") {
        (&["https"], rest)
    } else if let Some(rest) = trimmed.strip_prefix("http://") {
        (&["http"], rest)
    } else if trimmed.contains("://") {
        return vec![];
    } else {
        (&["https", "http"], trimmed)
    };
    let authority = rest.split(['/', '?', '#']).next().unwrap_or_default();
    if authority.is_empty() {
        return vec![];
    }
    schemes.iter().map(|scheme| format!("{scheme}://{}", authority.to_ascii_lowercase())).collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn bare_hosts_try_https_then_http() {
        assert_eq!(
            candidate_urls(" Media.Example.com:8080/web/ "),
            ["https://media.example.com:8080", "http://media.example.com:8080"]
        );
    }

    #[test]
    fn an_explicit_scheme_is_kept() {
        assert_eq!(candidate_urls("http://192.168.1.5:8080"), ["http://192.168.1.5:8080"]);
        assert_eq!(candidate_urls("https://tv.home/"), ["https://tv.home"]);
    }

    #[test]
    fn nonsense_has_no_candidates() {
        assert!(candidate_urls("").is_empty());
        assert!(candidate_urls("my server").is_empty());
        assert!(candidate_urls("ftp://files.example.com").is_empty());
        assert!(candidate_urls("https://").is_empty());
    }
}
