mod block;
mod store;
mod server;

use axum::{routing::put, Router};
use std::sync::Arc;
use std::path::PathBuf;

#[tokio::main]
async fn main() -> std::io::Result<()> {
    // call the blockstore constructor (initialize the store)
    let store = store::BlockStore::new(PathBuf::from("data"))?;

    let block = block::Block::new(
        "block-001".to_string(),
        b"hello from tiny-aws storage".to_vec(),
    );

    store.write_block(&block)?;

    let loaded = store.read_block("block-001")?;

    println!("read: {}", String::from_utf8_lossy(&loaded));

    // store.delete_block("block-001")?;

    println!("block not deleted");
    println!("block written: {}", block.id);


    let state = server::AppState {
        store: Arc::new(store),
    };
    
    use axum::{routing::{delete, get, put}, Router};

    let app = Router::new()
    .route(
        "/blocks/{id}",
        put(server::put_block)
            .get(server::get_block)
            .delete(server::delete_block),
    )
    .with_state(state);
    
    let listener = tokio::net::TcpListener::bind("127.0.0.1:7001").await?;
    
    println!("storage-node listening on 127.0.0.1:7001");
    
    axum::serve(listener, app)
        .await
        .unwrap();
    
    Ok(())
}