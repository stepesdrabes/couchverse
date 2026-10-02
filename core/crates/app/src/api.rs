//! Turning the generated API calls into HTTP effects and their outputs back into results.

use couchverse_api::{ApiError, Call, Request};

use crate::messages::{
    EffectOutput, HttpFailure, HttpFailureKind, HttpHeader, HttpRequest, Problem,
};

/// Where API requests go and how they authenticate.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Endpoint {
    /// The server's base URL (`https://host:port`); empty for the web, whose requests are
    /// origin-relative and carry the browser's cookie.
    pub base: String,
    /// A device token sent as a bearer; `None` for anonymous calls and for the web.
    pub token: Option<String>,
}

impl Endpoint {
    pub fn anonymous(base: &str) -> Self {
        Self { base: base.to_string(), token: None }
    }

    pub fn request(&self, request: &Request) -> HttpRequest {
        let mut headers =
            vec![HttpHeader { name: "Accept".into(), value: "application/json".into() }];
        if request.body.is_some() {
            headers
                .push(HttpHeader { name: "Content-Type".into(), value: "application/json".into() });
        }
        if let Some(token) = &self.token {
            headers.push(HttpHeader {
                name: "Authorization".into(),
                value: format!("Bearer {token}"),
            });
        }
        HttpRequest {
            method: request.method.as_str().to_string(),
            url: format!("{}/api/v1{}", self.base, request.path_and_query()),
            headers,
            body: request.body.clone(),
        }
    }
}

/// Why an API call did not produce its result.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Failure {
    /// No HTTP response at all.
    Network(HttpFailure),
    /// The server answered with an error, or with something that was not the expected JSON.
    Api(ApiError),
}

impl Failure {
    pub fn unauthorized(&self) -> bool {
        matches!(self, Failure::Api(e) if e.status == 401)
    }

    pub fn problem(&self) -> Problem {
        match self {
            Failure::Network(f) => {
                let code = match f.kind {
                    HttpFailureKind::Offline => "offline",
                    HttpFailureKind::Timeout => "timeout",
                    HttpFailureKind::Tls => "tls",
                    HttpFailureKind::Other => "network",
                };
                Problem::new(code, &f.message)
            }
            Failure::Api(e) => Problem::new(&e.code, &e.message),
        }
    }
}

/// Reads an HTTP effect's output as the call's typed result.
pub fn decode<T>(call: &Call<T>, output: EffectOutput) -> Result<T, Failure> {
    match output {
        EffectOutput::Http(response) => {
            call.parse(response.status, &response.body).map_err(Failure::Api)
        }
        EffectOutput::HttpFailed(failure) => Err(Failure::Network(failure)),
        other => Err(Failure::Network(HttpFailure {
            kind: HttpFailureKind::Other,
            message: format!("expected an HTTP output, got {other:?}"),
        })),
    }
}
