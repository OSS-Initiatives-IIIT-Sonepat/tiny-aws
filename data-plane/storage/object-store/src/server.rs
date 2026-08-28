use axum::http::HeaderMap;
use sha2::{Digest, Sha256};
use crate::metadata::{BucketMeta, MetadataStore, ObjectMeta};
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

// Writes bytes to engine + metadata for a given object id.
fn write_object(
    state: &AppState,
    key: &str,
    headers: &HeaderMap,
    data: Vec<u8>,
) -> Result<(), StatusCode> {
    let block = crate::block::Block::new(key.to_string(), data.clone());
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
        .upsert(key, data.len() as i64, &content_type, &etag)
        .map_err(|_| StatusCode::INTERNAL_SERVER_ERROR)?;

    Ok(())
}

pub async fn put_object(
    State(state): State<AppState>,
    Path(key): Path<String>,
    headers: HeaderMap,
    body: Bytes,
) -> Result<StatusCode, StatusCode> {
    write_object(&state, &key, &headers, body.to_vec())?;
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

// POST /buckets/{bucket} — create bucket.
pub async fn create_bucket(
    State(state): State<AppState>,
    Path(bucket): Path<String>,
) -> Result<StatusCode, StatusCode> {
    if state
        .metadata
        .bucket_exists(&bucket)
        .map_err(|_| StatusCode::INTERNAL_SERVER_ERROR)?
    {
        return Err(StatusCode::CONFLICT);
    }
    state
        .metadata
        .create_bucket(&bucket)
        .map_err(|_| StatusCode::INTERNAL_SERVER_ERROR)?;
    Ok(StatusCode::CREATED)
}

// GET /buckets — list buckets.
pub async fn list_buckets(
    State(state): State<AppState>,
) -> Result<Json<Vec<BucketMeta>>, StatusCode> {
    state
        .metadata
        .list_buckets()
        .map(Json)
        .map_err(|_| StatusCode::INTERNAL_SERVER_ERROR)
}

// PUT /buckets/{bucket}/objects/{key}.
pub async fn put_bucket_object(
    State(state): State<AppState>,
    Path((bucket, key)): Path<(String, String)>,
    headers: HeaderMap,
    body: Bytes,
) -> Result<StatusCode, StatusCode> {
    if !state
        .metadata
        .bucket_exists(&bucket)
        .map_err(|_| StatusCode::INTERNAL_SERVER_ERROR)?
    {
        return Err(StatusCode::NOT_FOUND);
    }
    let object_id = MetadataStore::bucket_object_id(&bucket, &key);
    write_object(&state, &object_id, &headers, body.to_vec())?;
    Ok(StatusCode::CREATED)
}

// GET /buckets/{bucket}/objects/{key}.
pub async fn get_bucket_object(
    State(state): State<AppState>,
    Path((bucket, key)): Path<(String, String)>,
) -> Result<Bytes, StatusCode> {
    let object_id = MetadataStore::bucket_object_id(&bucket, &key);
    get_object(State(state), Path(object_id)).await
}

// DELETE /buckets/{bucket}/objects/{key}.
pub async fn delete_bucket_object(
    State(state): State<AppState>,
    Path((bucket, key)): Path<(String, String)>,
) -> Result<StatusCode, StatusCode> {
    let object_id = MetadataStore::bucket_object_id(&bucket, &key);
    delete_object(State(state), Path(object_id)).await
}

// GET /buckets/{bucket}/objects — list objects in bucket.
pub async fn list_bucket_objects(
    State(state): State<AppState>,
    Path(bucket): Path<String>,
) -> Result<Json<Vec<ObjectMeta>>, StatusCode> {
    state
        .metadata
        .list_by_bucket(&bucket)
        .map(Json)
        .map_err(|_| StatusCode::INTERNAL_SERVER_ERROR)
}
