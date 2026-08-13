mod block;
mod store;

use std::path::PathBuf;

fn main() -> std::io::Result<()> {
    // call the blockstore constructor
    let store = store::BlockStore::new(PathBuf::from("data"))?;

    println!("storage-node starting...");
    println!("storage directory initialized");

    Ok(())
}