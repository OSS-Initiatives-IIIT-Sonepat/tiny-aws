use crate::block::Block;
use std::path::PathBuf;

pub struct BlockStore {
    root: PathBuf,
}

impl BlockStore {
    pub fn new(root: PathBuf) -> std::io::Result<Self> {
        std::fs::create_dir_all(&root)?;
        Ok(Self { root })
    }

    pub fn write_block(&self, block: &Block) -> std::io::Result<()> {
        let path = self.root.join(&block.id);
        std::fs::write(path, &block.data)?;
        Ok(())
    }

    pub fn read_block(&self, id: &str) -> std::io::Result<Vec<u8>> {
        let path = self.root.join(id);
        std::fs::read(path)
    }

    pub fn delete_block(&self, id: &str) -> std::io::Result<()> {
        let path = self.root.join(id);
        std::fs::remove_file(path)?;
        Ok(())
    }
}
