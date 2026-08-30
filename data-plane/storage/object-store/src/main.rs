mod auth;
mod block;
mod config;
mod ffi;
mod metadata;
mod registry;
mod replication;
mod server;
mod store;

use axum::{middleware, routing::get, routing::put, Router};
use metadata::MetadataStore;
use replication::ReplicationPolicy;
use server::AppState;
use std::sync::Arc;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let registry_url = config::registry_url();
    let listen_addr = config::listen_addr();
    let storage_root = config::storage_root();
    let metadata_db = config::metadata_db();

    let node_id = format!("storage-{}", hostname::get()?.to_string_lossy());
    println!("node_id: {}", node_id);

    registry::register_and_heartbeat(&registry_url, &node_id).await?;

    let store = store::BlockStore::new(storage_root)?;
    let metadata = MetadataStore::new(metadata_db)?;
    let repl = ReplicationPolicy::new();

    // C5: background peer discovery from registry
    replication::start_peer_discovery(repl.clone(), registry_url.clone(), node_id.clone());

    let state = AppState {
        store: Arc::new(store),
        metadata: Arc::new(metadata),
        replication: repl,
    };

    let app = Router::new()
        .route("/objects", get(server::list_objects))
        .route(
            "/objects/{id}",
            put(server::put_object)
                .get(server::get_object)
                .delete(server::delete_object),
        )
        .route("/objects/{id}/meta", get(server::get_object_meta))
        .route("/buckets", get(server::list_buckets))
        .route("/buckets/{bucket}", put(server::create_bucket))
        .route("/buckets/{bucket}/objects", get(server::list_bucket_objects))
        .route(
            "/buckets/{bucket}/objects/{key}",
            put(server::put_bucket_object)
                .get(server::get_bucket_object)
                .delete(server::delete_bucket_object),
        )
        .layer(middleware::from_fn(auth::require_auth))
        .with_state(state);

    let listener = tokio::net::TcpListener::bind(&listen_addr).await?;

    println!("object-store listening on {}", listen_addr);

    axum::serve(listener, app).await.unwrap();

    Ok(())
}
