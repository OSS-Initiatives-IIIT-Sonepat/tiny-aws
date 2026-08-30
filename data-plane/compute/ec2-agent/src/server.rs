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

        println!("connection from {}", address);

        let mut buffer = [0u8; 4096];

        let bytes_read = socket.read(&mut buffer).await?;

        let request = String::from_utf8_lossy(&buffer[..bytes_read]);

        let path = request
            .lines()
            .next()
            .and_then(|line| line.split_whitespace().nth(1))
            .unwrap_or("/");

        let (status, body) = match path {
            "/health" => (
                "200 OK",
                r#"{"status":"healthy"}"#.to_string(),
            ),

            "/info" => (
                "200 OK",
                to_string(node)?,
            ),

            _ => (
                "404 Not Found",
                r#"{"error":"not found"}"#.to_string(),
            ),
        };

        let response = format!(
            "HTTP/1.1 {}\r\n\
             Content-Type: application/json\r\n\
             Content-Length: {}\r\n\
             Connection: close\r\n\
             \r\n\
             {}",
            status,
            body.len(),
            body
        );

        socket.write_all(response.as_bytes()).await?;
    }
}