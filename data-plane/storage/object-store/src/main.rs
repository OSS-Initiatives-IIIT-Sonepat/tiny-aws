mod block;
mod ffi;
mod server;
mod store;

use axum::{routing::put, Router};
use server::AppState;
use std::path::PathBuf;
use std::sync::Arc;

#[tokio::main]
async fn main() -> std::io::Result<()> {
    let store = store::BlockStore::new(PathBuf::from("data"))?;

    let state = AppState {
        store: Arc::new(store),
    };

    let app = Router::new()
        .route(
            "/blocks/{id}",
            put(server::put_block)
                .get(server::get_block)
                .delete(server::delete_block),
        )
        .with_state(state);

    let listener = tokio::net::TcpListener::bind("127.0.0.1:7001").await?;

    println!("object-store listening on 127.0.0.1:7001");

    axum::serve(listener, app).await.unwrap();

    Ok(())
}
