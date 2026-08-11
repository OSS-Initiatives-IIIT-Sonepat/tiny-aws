use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::TcpListener;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let listener = TcpListener::bind("127.0.0.1:8080").await?;

    println!("ec2-agent listening on 127.0.0.1:8080");

    loop {
        let (mut socket, address) = listener.accept().await?;

        println!("connection from {}", address);

        let mut buffer = [0u8; 1024];

        let bytes_read = socket.read(&mut buffer).await?;

        println!("received {} bytes", bytes_read);

        let request = String::from_utf8_lossy(&buffer[..bytes_read]);

        println!("request: {}", request);

        let response = "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\n\r\nhello from tiny-aws";

        socket.write_all(response.as_bytes()).await?;
    }
}
