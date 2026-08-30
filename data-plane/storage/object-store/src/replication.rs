use std::sync::{Arc, RwLock};
use std::time::Duration;

// ReplicationPolicy holds the list of healthy peer object-store URLs.
// ponytail: in-memory peer list, no persistent config — peers are re-discovered from registry on restart
#[derive(Clone)]
pub struct ReplicationPolicy {
    pub peers: Arc<RwLock<Vec<String>>>,
    pub factor: usize,
}

impl ReplicationPolicy {
    // Builds policy from REPLICATION_FACTOR env (default 1 = no replication).
    pub fn new() -> Self {
        let factor = std::env::var("REPLICATION_FACTOR")
            .ok()
            .and_then(|v| v.parse().ok())
            .unwrap_or(1)
            .max(1);
        Self {
            peers: Arc::new(RwLock::new(vec![])),
            factor,
        }
    }

    // Returns the first `factor-1` healthy peers to write to.
    pub fn write_peers(&self) -> Vec<String> {
        let peers = self.peers.read().unwrap();
        let n = (self.factor - 1).min(peers.len());
        peers[..n].to_vec()
    }

    // Returns all known peers for read fallback.
    pub fn read_peers(&self) -> Vec<String> {
        self.peers.read().unwrap().clone()
    }
}

// Spawns a background task that refreshes healthy peer list from registry every 15s.
pub fn start_peer_discovery(
    policy: ReplicationPolicy,
    registry_url: String,
    own_node_id: String,
) {
    tokio::spawn(async move {
        let client = reqwest::Client::new();
        let mut ticker = tokio::time::interval(Duration::from_secs(15));

        loop {
            ticker.tick().await;
            let url = format!("{}/nodes?role=storage", registry_url);
            let nodes: serde_json::Value = match client.get(&url).send().await {
                Ok(r) => r.json().await.unwrap_or_default(),
                Err(e) => {
                    eprintln!("peer discovery failed: {}", e);
                    continue;
                }
            };

            // Registry returns a map of id -> node; collect healthy peers that are not us.
            let mut peers = vec![];
            if let Some(map) = nodes.as_object() {
                for (id, node) in map {
                    if id == &own_node_id {
                        continue;
                    }
                    if node.get("status").and_then(|s| s.as_str()) != Some("healthy") {
                        continue;
                    }
                    // Derive object-store URL from node hostname and default port 7001.
                    // ponytail: assumes all storage nodes run on :7001; add per-node addr field if needed
                    if let Some(hostname) = node.get("hostname").and_then(|h| h.as_str()) {
                        peers.push(format!("http://{}:7001", hostname));
                    }
                }
            }

            *policy.peers.write().unwrap() = peers.clone();
            if !peers.is_empty() {
                println!("replication peers updated: {:?}", peers);
            }
        }
    });
}

// Replicates raw bytes to all write peers. Logs errors but does not fail the primary write.
pub async fn replicate_put(peers: Vec<String>, key: &str, data: bytes::Bytes, content_type: &str) {
    if peers.is_empty() {
        return;
    }
    let client = reqwest::Client::new();
    for peer in peers {
        let url = format!("{}/objects/{}", peer, key);
        if let Err(e) = client
            .put(&url)
            .header("content-type", content_type)
            .body(data.clone())
            .send()
            .await
        {
            eprintln!("replicate PUT {} to {}: {}", key, peer, e);
        }
    }
}

// Tries to fetch key from each peer; returns first success.
pub async fn peer_get(peers: Vec<String>, key: &str) -> Option<bytes::Bytes> {
    if peers.is_empty() {
        return None;
    }
    let client = reqwest::Client::new();
    for peer in peers {
        let url = format!("{}/objects/{}", peer, key);
        if let Ok(resp) = client.get(&url).send().await {
            if resp.status().is_success() {
                if let Ok(b) = resp.bytes().await {
                    return Some(b);
                }
            }
        }
    }
    None
}

// Fans out DELETE to all write peers.
pub async fn replicate_delete(peers: Vec<String>, key: &str) {
    if peers.is_empty() {
        return;
    }
    let client = reqwest::Client::new();
    for peer in peers {
        let url = format!("{}/objects/{}", peer, key);
        if let Err(e) = client.delete(&url).send().await {
            eprintln!("replicate DELETE {} to {}: {}", key, peer, e);
        }
    }
}
