use axum::http::HeaderMap;
use sha2::{Digest, Sha256};
use crate::metadata::{MetadataStore, ObjectMeta};
use crate::store::BlockStore;
use axum::{
    body::Bytes,
    extract::{Path, State},
    http::StatusCode,
    Json,
};
use std::sync::Arc;

#[derive(Clone)]
pub struct AppState {
    pub store: Arc<BlockStore>,
    pub metadata: Arc<MetadataStore>,
}

pub async fn put_object(
    State(state): State<AppState>,
    Path(key): Path<String>,
    headers: HeaderMap,
    body: Bytes,
) -> Result<StatusCode, StatusCode> {
    let data = body.to_vec();
    let block = crate::block::Block::new(key.clone(), data.clone());

    state
        .store
        .write_block(&block)
        .map_err(|_| StatusCode::INTERNAL_SERVER_ERROR)?;

    let content_type = headers
        .get("content-type")
        .and_then(|v| v.to_str().ok())
        .unwrap_or("application/octet-stream")
        .to_string();

    let hash = Sha256::digest(&data);
    let etag = hex::encode(hash);

    state
        .metadata
        .upsert(&key, data.len() as i64, &content_type, &etag)
        .map_err(|_| StatusCode::INTERNAL_SERVER_ERROR)?;

    Ok(StatusCode::CREATED)
}

pub async fn get_object(
    State(state): State<AppState>,
    Path(key): Path<String>,
) -> Result<Bytes, StatusCode> {
    let data = state
        .store
        .read_block(&key)
        .map_err(|_| StatusCode::NOT_FOUND)?;

    Ok(Bytes::from(data))
}

pub async fn delete_object(
    State(state): State<AppState>,
    Path(key): Path<String>,
) -> Result<StatusCode, StatusCode> {
    state
        .store
        .delete_block(&key)
        .map_err(|_| StatusCode::NOT_FOUND)?;

    state.metadata.remove(&key).ok();

    Ok(StatusCode::NO_CONTENT)
}

pub async fn list_objects(
    State(state): State<AppState>,
) -> Result<Json<Vec<ObjectMeta>>, StatusCode> {
    state
        .metadata
        .list()
        .map(Json)
        .map_err(|_| StatusCode::INTERNAL_SERVER_ERROR)
}

pub async fn get_object_meta(
    State(state): State<AppState>,
    Path(key): Path<String>,
) -> Result<Json<ObjectMeta>, StatusCode> {
    match state
        .metadata
        .get(&key)
        .map_err(|_| StatusCode::INTERNAL_SERVER_ERROR)?
    {
        Some(meta) => Ok(Json(meta)),
        None => Err(StatusCode::NOT_FOUND),
    }
}