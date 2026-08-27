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

#[derive(Debug, Serialize)]
pub struct NodeRegistration {
    pub id:        String,
    pub hostname:  String,
    pub cpu_count: usize,
    pub role:      String,
}

impl NodeRegistration {
    pub fn from_node(node: &Node) -> Self {
        Self {
            id:        node.id.clone(),
            hostname:  node.system.hostname.clone(),
            cpu_count: node.system.cpu_count,
            role:      "compute".to_string(),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_node_creation() {
        let system_info = SystemInfo {
            hostname: "test-node".to_string(),
            cpu_count: 4,
            memory_total_mb: 8192,
            memory_available_mb: 4096,
            operating_system: "Linux".to_string(),
            architecture: "x86_64".to_string(),
        };

        let node = Node::new(system_info);

        assert_eq!(node.id, "test-node");
        assert_eq!(node.system.cpu_count, 4);
    }

    #[test]
    fn test_node_registration_serialization() {
        let system_info = SystemInfo {
            hostname: "test-node".to_string(),
            cpu_count: 8,
            memory_total_mb: 16384,
            memory_available_mb: 8192,
            operating_system: "Windows".to_string(),
            architecture: "x86_64".to_string(),
        };

        let node = Node::new(system_info);
        let registration = NodeRegistration::from_node(&node);

        assert_eq!(registration.id, "test-node");
        assert_eq!(registration.hostname, "test-node");
        assert_eq!(registration.cpu_count, 8);
        assert_eq!(registration.role, "compute");

        let json = serde_json::to_string(&registration).expect("serialization failed");
        assert!(json.contains("test-node"));
    }
}
