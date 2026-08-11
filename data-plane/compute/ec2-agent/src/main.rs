use tokio::net::TcpListener;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let listener = TcpListener::bind("127.0.0.1:8080").await?;

    println!("ec2-agent listening on 127.0.0.1:8080");

    loop {
        let (socket, address) = listener.accept().await?;

        println!("connection from {}", address);

        drop(socket);
    }
}