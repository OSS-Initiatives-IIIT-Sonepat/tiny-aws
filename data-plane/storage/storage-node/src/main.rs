mod block;
mod store;

use std::path::PathBuf;

fn main() -> std::io::Result<()> {
    // call the blockstore constructor (initialize the store)
    let store = store::BlockStore::new(PathBuf::from("data"))?;

    let block = block::Block::new(
        "block-001".to_string(),
        b"hello from tiny-aws storage".to_vec(),
    );

    store.write_block(&block)?;

    println!("block written: {}", block.id);

    Ok(())
}