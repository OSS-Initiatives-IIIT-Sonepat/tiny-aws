mod rules;

#[tokio::main]
async fn main() {
    println!("network-agent starting");
    println!("networking url: {}", std::env::var("NETWORKING_URL").unwrap_or_else(|_| "http://127.0.0.1:9005".into()));
    println!("sg_id: {}", std::env::var("SG_ID").unwrap_or_else(|_| "(none)".into()));

    // start enforcing SG rules from networking service
    rules::start_rule_enforcer().await;
}
