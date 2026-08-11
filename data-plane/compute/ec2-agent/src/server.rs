use crate::node::Node;
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::TcpListener;

pub async fn start(node: &Node) -> Result<(), Box<dyn std::error::Error>> {
    // pub: The server now receives the node it is serving.
    let listener = TcpListener::bind("127.0.0.1:8080").await?;

    println!("ec2-agent listening on 127.0.0.1:8080");

    loop {
        let (mut socket, address) = listener.accept().await?;

        println!("connection from {}", address);

        let mut buffer = [0u8; 1024];

        let bytes_read = socket.read(&mut buffer).await?;

        let request = String::from_utf8_lossy(&buffer[..bytes_read]);

        println!("request:\n{}", request);

        let body = format!(
            "node_id={}\nhostname={}\ncpu_count={}\n",
            node.id,
            node.system.hostname,
            node.system.cpu_count
        );

        let response = format!(
            "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: {}\r\n\r\n{}",
            body.len(),
            body
        );

        socket.write_all(response.as_bytes()).await?;
    }
}