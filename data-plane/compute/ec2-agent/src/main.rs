mod heartbeat;
mod node;
mod server;
mod system;

use node::NodeRegistration;
use tokio::signal;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let system_info = system::get_system_info();

    let node = node::Node::new(system_info);

    let registration = NodeRegistration::from_node(&node);

    println!("node: {}", node.id);
    println!("cpus: {}", node.system.cpu_count);
    println!(
        "memory: {} MB / {} MB",
        node.system.memory_available_mb,
        node.system.memory_total_mb
    );

    let client = reqwest::Client::new();

    client
        .post("http://127.0.0.1:9000/nodes/register")
        .json(&registration)
        .send()
        .await?
        .error_for_status()?;

    println!("node registered with control plane");

    heartbeat::start_heartbeat(node.id.clone(), "http://127.0.0.1:9000").await;

    // Start server (blocks until Ctrl+C)
    let ctrl_c = signal::ctrl_c();
    
    tokio::select! {
        _ = server::start(&node) => {
            println!("server stopped");
        }
        _ = ctrl_c => {
            println!("\nshutting down gracefully...");
        }
    }

    println!("node shut down");

    Ok(())
}