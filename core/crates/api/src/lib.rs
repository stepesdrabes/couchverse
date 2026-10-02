//! Typed access to the Couchverse HTTP API.
//!
//! Everything in [`types`] and [`ops`] is generated from `contract/openapi.json` by
//! `cargo xtask codegen`. An operation builder produces a [`Call`]: the [`Request`] a shell
//! performs (the core never does I/O) plus the knowledge of how to read the response. Response
//! types tolerate unknown fields and unknown enum values, so additive server changes never break
//! an older client.

mod generated;

use std::fmt::{self, Write as _};
use std::marker::PhantomData;

pub use generated::{ops, types};

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Method {
    Get,
    Post,
    Put,
    Patch,
    Delete,
}

impl Method {
    pub fn as_str(self) -> &'static str {
        match self {
            Method::Get => "GET",
            Method::Post => "POST",
            Method::Put => "PUT",
            Method::Patch => "PATCH",
            Method::Delete => "DELETE",
        }
    }
}

/// An HTTP request relative to a server's API base (`<server>/api/v1`).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Request {
    pub method: Method,
    /// Percent-encoded path below the API base, starting with `/`.
    pub path: String,
    /// Query parameters in order; absent optional parameters are omitted.
    pub query: Vec<(String, String)>,
    /// A JSON request body.
    pub body: Option<String>,
}

impl Request {
    /// The path with its percent-encoded query string.
    pub fn path_and_query(&self) -> String {
        if self.query.is_empty() {
            return self.path.clone();
        }
        let query: Vec<String> = self
            .query
            .iter()
            .map(|(k, v)| format!("{}={}", encode(k), encode(v)))
            .collect();
        format!("{}?{}", self.path, query.join("&"))
    }
}

/// A request plus how to decode its successful response into `T`.
pub struct Call<T> {
    pub request: Request,
    decode: fn(&str) -> Result<T, serde_json::Error>,
    _response: PhantomData<fn() -> T>,
}

impl<T> Call<T> {
    /// Reads a response: a 2xx is decoded as `T`, anything else becomes an [`ApiError`].
    pub fn parse(&self, status: u16, body: &str) -> Result<T, ApiError> {
        if !(200..300).contains(&status) {
            return Err(ApiError::from_response(status, body));
        }
        (self.decode)(body).map_err(|err| ApiError {
            status,
            code: ApiError::UNDECODABLE.to_string(),
            message: err.to_string(),
        })
    }
}

impl<T> fmt::Debug for Call<T> {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("Call")
            .field("request", &self.request)
            .finish_non_exhaustive()
    }
}

/// The successful response of an operation that answers 204 No Content.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct NoContent;

/// A failed call: the server's `{"error": {"code", "message"}}` envelope, or a synthetic code
/// when there was none ([`ApiError::UNDECODABLE`], `http_<status>`).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ApiError {
    pub status: u16,
    pub code: String,
    pub message: String,
}

impl ApiError {
    /// A 2xx response whose body did not match the expected type.
    pub const UNDECODABLE: &'static str = "undecodable_response";

    fn from_response(status: u16, body: &str) -> Self {
        match serde_json::from_str::<types::ApiError>(body) {
            Ok(envelope) => ApiError {
                status,
                code: envelope.error.code,
                message: envelope.error.message,
            },
            Err(_) => ApiError {
                status,
                code: format!("http_{status}"),
                message: String::new(),
            },
        }
    }
}

impl fmt::Display for ApiError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{} {}: {}", self.status, self.code, self.message)
    }
}

impl std::error::Error for ApiError {}

/// Builders used by the generated operations.
mod build {
    use super::{Call, Method, NoContent, PhantomData, Request};
    use serde::Serialize;
    use serde::de::DeserializeOwned;

    pub(crate) fn json<T: DeserializeOwned>(request: Request) -> Call<T> {
        Call {
            request,
            decode: |body| serde_json::from_str(body),
            _response: PhantomData,
        }
    }

    pub(crate) fn no_content(request: Request) -> Call<NoContent> {
        Call {
            request,
            decode: |_| Ok(NoContent),
            _response: PhantomData,
        }
    }

    pub(crate) fn request(
        method: Method,
        path: String,
        query: Vec<(String, String)>,
        body: Option<&impl Serialize>,
    ) -> Request {
        Request {
            method,
            path,
            query,
            body: body.map(|b| serde_json::to_string(b).expect("API types always serialize")),
        }
    }

    /// One path segment, percent-encoded.
    pub(crate) fn segment(value: &str) -> String {
        super::encode(value)
    }

    /// Adds `name=value` when the optional parameter is set.
    pub(crate) fn push(
        query: &mut Vec<(String, String)>,
        name: &str,
        value: Option<impl ToString>,
    ) {
        if let Some(value) = value {
            query.push((name.to_string(), value.to_string()));
        }
    }
}

/// Percent-encodes everything outside RFC 3986's unreserved set.
fn encode(value: &str) -> String {
    let mut out = String::with_capacity(value.len());
    for byte in value.bytes() {
        match byte {
            b'A'..=b'Z' | b'a'..=b'z' | b'0'..=b'9' | b'-' | b'.' | b'_' | b'~' => {
                out.push(byte as char);
            }
            _ => {
                let _ = write!(out, "%{byte:02X}");
            }
        }
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn query_values_are_percent_encoded() {
        let request = Request {
            method: Method::Get,
            path: "/search".into(),
            query: vec![("q".into(), "glass harbor & co".into())],
            body: None,
        };
        assert_eq!(
            request.path_and_query(),
            "/search?q=glass%20harbor%20%26%20co"
        );
    }

    #[test]
    fn error_envelopes_become_api_errors() {
        let call = build::no_content(build::request(
            Method::Delete,
            "/x".into(),
            vec![],
            None::<&()>,
        ));
        let err = call.parse(
            404,
            r#"{"error":{"code":"not_found","message":"resource not found"}}"#,
        );
        assert_eq!(
            err,
            Err(ApiError {
                status: 404,
                code: "not_found".into(),
                message: "resource not found".into()
            })
        );
    }

    #[test]
    fn responses_without_an_envelope_get_a_status_code() {
        let call = build::no_content(build::request(
            Method::Get,
            "/x".into(),
            vec![],
            None::<&()>,
        ));
        assert_eq!(call.parse(502, "<html>").unwrap_err().code, "http_502");
    }

    #[test]
    fn no_content_ignores_the_body() {
        let call = build::no_content(build::request(
            Method::Put,
            "/x".into(),
            vec![],
            None::<&()>,
        ));
        assert_eq!(call.parse(204, ""), Ok(NoContent));
    }
}
