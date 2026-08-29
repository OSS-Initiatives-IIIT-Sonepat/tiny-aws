use axum::{
    body::Body,
    http::{Request, StatusCode},
    middleware::Next,
    response::Response,
};
use std::env;

// Returns 401 when TINYAWS_API_KEY is set and the request lacks a matching bearer token.
// No exemptions: object-store agents use the same key as CLI.
pub async fn require_auth(req: Request<Body>, next: Next) -> Result<Response, StatusCode> {
    let key = env::var("TINYAWS_API_KEY").unwrap_or_default();
    if key.is_empty() {
        return Ok(next.run(req).await);
    }
    let auth = req
        .headers()
        .get("authorization")
        .and_then(|v| v.to_str().ok())
        .unwrap_or("");
    if auth != format!("Bearer {}", key) {
        return Err(StatusCode::UNAUTHORIZED);
    }
    Ok(next.run(req).await)
}
