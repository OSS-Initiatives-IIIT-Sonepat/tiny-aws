#[derive(Debug, Clone)]
pub struct Block {
    pub id: String,
    pub data: Vec<u8>,
}

impl Block {
    pub fn new(id: String, data: Vec<u8>) -> Self {
        Self { id, data }
    }
}
