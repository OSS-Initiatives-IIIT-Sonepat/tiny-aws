use crate::instances;
use crate::node::Node;
use serde_json::to_string;
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::TcpListener;

pub async fn start(node: &Node) -> Result<(), Box<dyn std::error::Error>> {
    let addr = std::env::var("AGENT_ADDR").unwrap_or_else(|_| "0.0.0.0:8080".into());
    let listener = TcpListener::bind(&addr).await?;

    println!("ec2-agent listening on {}", addr);

    loop {
        let (mut socket, address) = listener.accept().await?;
        let node_json = to_string(node).unwrap_or_default();

        tokio::spawn(async move {
            println!("connection from {}", address);

            // read up to 64KB — enough for headers + small JSON bodies
            let mut buffer = vec![0u8; 65536];
            let bytes_read = match socket.read(&mut buffer).await {
                Ok(n) => n,
                Err(_) => return,
            };

            let raw = String::from_utf8_lossy(&buffer[..bytes_read]);

            // parse method and path from first line
            let mut lines = raw.lines();
            let first = lines.next().unwrap_or("");
            let mut parts = first.split_whitespace();
            let method = parts.next().unwrap_or("GET");
            let path = parts.next().unwrap_or("/");

            // extract body (everything after \r\n\r\n)
            let body = raw.find("\r\n\r\n")
                .map(|i| raw[i + 4..].trim())
                .unwrap_or("");

            let (status, response_body) = handle_request(method, path, body, &node_json);

            let response = format!(
                "HTTP/1.1 {}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
                status, response_body.len(), response_body
            );

            let _ = socket.write_all(response.as_bytes()).await;
        });
    }
}

// handle_request dispatches to the right handler and returns (status_line, body).
fn handle_request(method: &str, path: &str, body: &str, node_json: &str) -> (&'static str, String) {
    match (method, path) {
        ("GET", "/health") => (
            "200 OK",
            r#"{"status":"healthy","service":"ec2-agent"}"#.into(),
        ),
        ("GET", "/info") => ("200 OK", node_json.to_string()),

        // POST /instances/{id}/provision — create nspawn container
        ("POST", p) if p.starts_with("/instances/") && p.ends_with("/provision") => {
            let id = p.trim_start_matches("/instances/").trim_end_matches("/provision");
            match serde_json::from_str::<instances::InstanceSpec>(body) {
                Ok(spec) => {
                    let registry_url = std::env::var("REGISTRY_URL")
                        .unwrap_or_else(|_| "http://127.0.0.1:9000".into());
                    let inst_id = spec.id.clone();
                    std::thread::spawn(move || {
                        let status = match instances::provision(&spec) {
                            Ok(_) => "running",
                            Err(e) => {
                                eprintln!("provision {}: {}", inst_id, e);
                                "failed"
                            }
                        };
                        // PATCH registry with final status
                        let url = format!("{}/instances/{}", registry_url, inst_id);
                        let payload = format!(r#"{{"status":"{}"}}"#, status);
                        let _ = ureq::patch(&url)
                            .set("content-type", "application/json")
                            .send_string(&payload);
                    });
                    let _ = id;
                    ("202 Accepted", r#"{"provisioning":true}"#.into())
                }
                Err(e) => ("400 Bad Request", format!(r#"{{"error":"{}"}}"#, e)),
            }
        }

        // DELETE /instances/{id} — destroy nspawn container
        ("DELETE", p) if p.starts_with("/instances/") => {
            let id = p.trim_start_matches("/instances/");
            instances::destroy(id);
            ("204 No Content", String::new())
        }

        _ => ("404 Not Found", r#"{"error":"not found"}"#.into()),
    }
}
