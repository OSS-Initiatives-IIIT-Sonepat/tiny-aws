use crate::system::SystemInfo;
use serde::Serialize;

#[derive(Debug, Serialize, Clone)]
pub struct Node {
    pub id:     String,
    pub system: SystemInfo,
}

impl Node {
    pub fn new(system: SystemInfo) -> Self {
        let id = system.hostname.clone();
        Self { id, system }
    }
}

//derive smaller object - speciific to the network api
#[derive(Debug, Serialize)] 
pub struct NodeRegistration {
    pub id:        String, 
    pub hostname:  String,
    pub cpu_count: usize,
}

impl NodeRegistration {
    pub fn from_node (node: &Node) -> Self {
        Self {
            id:        node.id.clone(),
            hostname:  node.system.hostname.clone(),
            cpu_count: node.system.cpu_count,
        }
    }
}