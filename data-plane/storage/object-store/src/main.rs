mod block;
mod config;
mod ffi;
mod metadata;
mod registry;
mod server;
mod store;

use axum::{routing::get, routing::put, Router};
use metadata::MetadataStore;
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

    let state = AppState {
        store: Arc::new(store),
        metadata: Arc::new(metadata),
    };

    let app = Router::new()
        .route("/blocks", get(server::list_blocks))
        .route(
            "/blocks/{id}",
            put(server::put_block)
                .get(server::get_block)
                .delete(server::delete_block),
        )
        .route("/blocks/{id}/meta", get(server::get_block_meta))
        .with_state(state);

    let listener = tokio::net::TcpListener::bind(&listen_addr).await?;

    println!("object-store listening on {}", listen_addr);

    axum::serve(listener, app).await.unwrap();

    Ok(())
}
