use crate::system::SystemInfo;

pub struct Node {
    pub id: String,
    pub system: SystemInfo,
}

impl Node {
    pub fn new(system: SystemInfo) -> Self {
        let id = system.hostname.clone();

        Self { id, system }
    }
}