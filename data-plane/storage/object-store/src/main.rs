#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let client = reqwest::Client::new();

    let response = client
        .get("http://172.25.48.67:9870/webhdfs/v1/tiny-aws?op=LISTSTATUS")
        .send()
        .await?;

    println!("status: {}", response.status());
    println!("body: {}", response.text().await?);

    Ok(())
}