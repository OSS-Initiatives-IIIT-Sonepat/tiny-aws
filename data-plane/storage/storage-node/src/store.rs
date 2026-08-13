use crate::block::Block;
use std::path::PathBuf;

// responsible for storing and retrieving blocks on disk
pub struct BlockStore {
    root: PathBuf,
}

// defines what block store can do
impl BlockStore {
    pub fn new(root: PathBuf) -> std::io::Result<Self> {
        //create directory in the root of the file
        std::fs::create_dir_all(&root)?;

        Ok(Self {root})
    }

    pub fn write_block(&self, block: &Block) -> std::io::Result<()> {
        let path = self.root.join(&block.id);
    
        std::fs::write(path, &block.data)?;
    
        Ok(())
    }

    pub fn delete_block(&self, id: &str) -> std::io::Result<()> {
        let path = self.root.join(id);
        std::fs::remove_file(path)?;
        Ok(())
    }

    pub fn read_block(&self, id: &str) -> std::io::Result<Vec<u8>> {
        let path = self.root.join(id);
        let data = std::fs::read(path)?;
        Ok(data)
    }
}