use crate::node::NodeRegistration;
use std::time::Duration;

pub async fn register_with_retry(
    registry_url: &str,
    registration: &NodeRegistration,
) -> Result<(), reqwest::Error> {
    let client = reqwest::Client::new();
    let url = format!("{}/nodes/register", registry_url);

    for attempt in 1..=10 {
        match client.post(&url).json(registration).send().await {
            Ok(response) => match response.error_for_status() {
                Ok(_) => return Ok(()),
                Err(e) => eprintln!("register attempt {} failed: {}", attempt, e),
            },
            Err(e) => eprintln!("register attempt {} failed: {}", attempt, e),
        }

        tokio::time::sleep(Duration::from_secs(2)).await;
    }

    client
        .post(&url)
        .json(registration)
        .send()
        .await?
        .error_for_status()?;

    Ok(())
}
