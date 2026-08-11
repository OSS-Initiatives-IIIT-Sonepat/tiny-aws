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

pub async fn start_heartbeat(node_id: String, registry_url: &str) {
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
