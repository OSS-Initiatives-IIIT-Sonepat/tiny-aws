use crate::metadata::MetadataStore;
use crate::store::BlockStore;
use axum::{
    body::Bytes,
    extract::{Path, State},
    http::StatusCode,
};
use std::sync::Arc;

#[derive(Clone)]
pub struct AppState {
    pub store: Arc<BlockStore>,
    pub metadata: Arc<MetadataStore>,
}

pub async fn put_block(
    State(state): State<AppState>,
    Path(id): Path<String>,
    body: Bytes,
) -> Result<StatusCode, StatusCode> {
    let block = crate::block::Block::new(id, body.to_vec());

    state
        .store
        .write_block(&block)
        .map_err(|_| StatusCode::INTERNAL_SERVER_ERROR)?;

    Ok(StatusCode::CREATED)
}

pub async fn get_block(
    State(state): State<AppState>,
    Path(id): Path<String>,
) -> Result<Bytes, StatusCode> {
    let data = state
        .store
        .read_block(&id)
        .map_err(|_| StatusCode::NOT_FOUND)?;

    Ok(Bytes::from(data))
}

pub async fn delete_block(
    State(state): State<AppState>,
    Path(id): Path<String>,
) -> Result<StatusCode, StatusCode> {
    state
        .store
        .delete_block(&id)
        .map_err(|_| StatusCode::NOT_FOUND)?;

    Ok(StatusCode::NO_CONTENT)
}
