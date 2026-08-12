use crate::block::Block;
use std::path::PathBuf;

// responsible for storing and retrieving blocks on disk
pub struct BlockStore {
    root: PathBuf,
}

// defines what block store can do
impl BlockStore {
    pub fn new(root: PathBug) -> self {
        std::fs::create_dir_all(&root).unwrap();

        Ok(Self {root})
    }
}