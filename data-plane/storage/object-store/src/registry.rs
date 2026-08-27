use serde::Serialize;
use std::time::Duration;

#[derive(Debug, Serialize)]
struct NodeRegistration {
    id: String,
    hostname: String,
    cpu_count: usize,
    role: String,
}

#[derive(Debug, Serialize)]
struct Heartbeat {
    id: String,
}

pub async fn register_and_heartbeat(registry_url: &str, node_id: &str) -> Result<(), reqwest::Error> {
    let hostname = hostname::get()
        .ok()
        .and_then(|h| h.into_string().ok())
        .unwrap_or_else(|| "storage-node".to_string());

    let registration = NodeRegistration {
        id: node_id.to_string(),
        hostname: hostname.clone(),
        cpu_count: 0,
        role: "storage".to_string(),
    };

    let client = reqwest::Client::new();

    client
        .post(format!("{}/nodes/register", registry_url))
        .json(&registration)
        .send()
        .await?
        .error_for_status()?;

    println!("storage node {} registered with control plane", node_id);

    let heartbeat = Heartbeat {
        id: node_id.to_string(),
    };

    let url = format!("{}/nodes/heartbeat", registry_url);

    tokio::spawn(async move {
        let mut ticker = tokio::time::interval(Duration::from_secs(10));

        loop {
            ticker.tick().await;

            if let Err(e) = client.post(&url).json(&heartbeat).send().await {
                eprintln!("storage heartbeat failed: {}", e);
            }
        }
    });

    Ok(())
}