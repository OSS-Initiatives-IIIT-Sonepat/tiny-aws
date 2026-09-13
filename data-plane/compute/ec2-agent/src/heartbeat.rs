use serde::Serialize;
use std::time::Duration;
use tokio::time::interval;

#[derive(Debug, Serialize)]
pub struct Heartbeat {
    pub id: String,
}

impl Heartbeat {
    pub fn new(id: String) -> Self {
        Self { id }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn heartbeat_new_sets_id() {
        let hb = Heartbeat::new("node-42".into());
        assert_eq!(hb.id, "node-42");
    }

    #[test]
    fn heartbeat_serializes_to_json() {
        let hb = Heartbeat::new("n1".into());
        let json = serde_json::to_string(&hb).unwrap();
        assert!(json.contains(r#""id":"n1""#));
    }

    #[test]
    fn heartbeat_empty_id() {
        let hb = Heartbeat::new(String::new());
        assert_eq!(hb.id, "");
        let json = serde_json::to_string(&hb).unwrap();
        assert!(json.contains(r#""id":"""#));
    }
}

// Spawns a background task that sends heartbeats every 10 seconds.
pub fn start_heartbeat(node_id: String, registry_url: &str) {
    let client = reqwest::Client::new();
    let heartbeat = Heartbeat::new(node_id);
    let registry_url = registry_url.to_string();
    
    tokio::spawn(async move {
        let mut ticker = interval(Duration::from_secs(10));

        loop {
            ticker.tick().await;

            let url = format!("{}/nodes/heartbeat", registry_url);
            
            if let Err(e) = client
                .post(&url)
                .json(&heartbeat)
                .send()
                .await
            {
                eprintln!("heartbeat failed: {}", e);
            }
        }
    });
}
