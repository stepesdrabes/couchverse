//! Typed access to the Couchverse HTTP API.
//!
//! Everything in [`types`] and [`ops`] is generated from `contract/openapi.json` by
//! `cargo xtask codegen`. An operation builder produces a [`Call`]: the [`Request`] a shell
//! performs (the core never does I/O) plus the knowledge of how to read the response. Response
//! types tolerate unknown fields and unknown enum values, so additive server changes never break
//! an older client.
//!
//! Uploads are calls too, but the core never holds file bytes: their request carries the marker
//! [`Body::Multipart`] or [`Body::Binary`] instead of content. The shell attaches the file (and a
//! form's other parts, listed on the operation), sends the request and hands the response to
//! [`Call::parse`] like any other. Doc comments carry what neither language types: defaults,
//! ranges and lengths of parameters and fields.

mod generated;

use std::fmt::{self, Write as _};
use std::marker::PhantomData;

pub use generated::{couch, ops, types};

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
    pub body: Body,
}

/// What a [`Request`] sends.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Body {
    /// No body.
    Empty,
    /// A JSON document, sent as `application/json`.
    Json(String),
    /// A `multipart/form-data` form the shell assembles: the file, plus any text parts the
    /// operation lists. Its HTTP stack writes the boundary into `Content-Type`.
    Multipart,
    /// Raw bytes the shell attaches (a file or a slice of one), sent as
    /// `application/octet-stream`.
    Binary,
}

impl Request {
    /// The path with its percent-encoded query string.
    pub fn path_and_query(&self) -> String {
        if self.query.is_empty() {
            return self.path.clone();
        }
        let query: Vec<String> =
            self.query.iter().map(|(k, v)| format!("{}={}", encode(k), encode(v))).collect();
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

impl<T> Clone for Call<T> {
    fn clone(&self) -> Self {
        Self { request: self.request.clone(), decode: self.decode, _response: PhantomData }
    }
}

impl<T> PartialEq for Call<T> {
    fn eq(&self, other: &Self) -> bool {
        self.request == other.request
    }
}

impl<T> fmt::Debug for Call<T> {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("Call").field("request", &self.request).finish_non_exhaustive()
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
            Ok(envelope) => {
                ApiError { status, code: envelope.error.code, message: envelope.error.message }
            }
            Err(_) => ApiError { status, code: format!("http_{status}"), message: String::new() },
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
    use super::{Body, Call, Method, NoContent, PhantomData, Request};
    use serde::Serialize;
    use serde::de::DeserializeOwned;

    pub(crate) fn json<T: DeserializeOwned>(request: Request) -> Call<T> {
        Call { request, decode: |body| serde_json::from_str(body), _response: PhantomData }
    }

    pub(crate) fn no_content(request: Request) -> Call<NoContent> {
        Call { request, decode: |_| Ok(NoContent), _response: PhantomData }
    }

    pub(crate) fn request(
        method: Method,
        path: String,
        query: Vec<(String, String)>,
        body: Body,
    ) -> Request {
        Request { method, path, query, body }
    }

    /// A JSON body, or none for an omitted optional one.
    pub(crate) fn json_body(value: Option<&impl Serialize>) -> Body {
        value.map_or(Body::Empty, |value| {
            Body::Json(serde_json::to_string(value).expect("API types always serialize"))
        })
    }

    /// One path segment, percent-encoded.
    pub(crate) fn segment(value: &str) -> String {
        super::encode(value)
    }

    /// A list query parameter as one comma-separated value.
    pub(crate) fn csv<T: ToString>(items: &[T]) -> String {
        items.iter().map(ToString::to_string).collect::<Vec<_>>().join(",")
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
            body: Body::Empty,
        };
        assert_eq!(request.path_and_query(), "/search?q=glass%20harbor%20%26%20co");
    }

    #[test]
    fn bodies_follow_the_operation() {
        let transcode = types::TranscodeRequest {
            variants: Some(vec![types::TranscodeRequestVariantsItem::V720p]),
        };
        let call = ops::admin_enqueue_transcode("f", Some(&transcode));
        assert_eq!(call.request.body, Body::Json(r#"{"variants":["720p"]}"#.into()));
        assert_eq!(ops::admin_enqueue_transcode("f", None).request.body, Body::Empty);

        let call = ops::upload_avatar();
        assert_eq!((call.request.method, &call.request.body), (Method::Post, &Body::Multipart));
        let user = r#"{"id":1,"username":"nora","displayName":"Nora","role":"member",
            "disabled":false,"avatarId":"a","bannerId":null,"bio":"","createdAt":"2026-01-01T00:00:00Z"}"#;
        assert_eq!(call.parse(200, user).unwrap().avatar_id.as_deref(), Some("a"));

        let call = ops::admin_append_upload("u", &ops::AdminAppendUploadQuery { offset: 8 });
        assert_eq!(call.request.path_and_query(), "/admin/uploads/u?offset=8");
        assert_eq!(call.request.body, Body::Binary);
    }

    #[test]
    fn error_envelopes_become_api_errors() {
        let call =
            build::no_content(build::request(Method::Delete, "/x".into(), vec![], Body::Empty));
        let err =
            call.parse(404, r#"{"error":{"code":"not_found","message":"resource not found"}}"#);
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
        let call = build::no_content(build::request(Method::Get, "/x".into(), vec![], Body::Empty));
        assert_eq!(call.parse(502, "<html>").unwrap_err().code, "http_502");
    }

    #[test]
    fn no_content_ignores_the_body() {
        let call = build::no_content(build::request(Method::Put, "/x".into(), vec![], Body::Empty));
        assert_eq!(call.parse(204, ""), Ok(NoContent));
    }
}
