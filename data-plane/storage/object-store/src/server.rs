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
    body: Bytes,
) -> Result<StatusCode, StatusCode> {
    let block = crate::block::Block::new(key, body.to_vec());

    state
        .store
        .write_block(&block)
        .map_err(|_| StatusCode::INTERNAL_SERVER_ERROR)?;

    state
        .metadata
        .insert(&block.id, block.data.len() as i64)
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