mod config;
mod heartbeat;
mod jobs;
mod node;
mod registry;
mod server;
mod system;

use node::NodeRegistration;
use tokio::signal;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let registry_url = config::registry_url();
    let scheduler_url = config::scheduler_url();

    // Gather CPU/RAM info and build a node identity
    let system_info = system::get_system_info();
    let node = node::Node::new(system_info);
    let registration = NodeRegistration::from_node(&node);

    println!("cpus: {}", node.system.cpu_count);
    println!(
        "memory: {} MB / {} MB",
        node.system.memory_available_mb,
        node.system.memory_total_mb
    );
    println!("node: {}", node.id);
    println!("registry url: {}", registry_url);
    println!("scheduler url: {}", scheduler_url);

    // Register this machine with control-plane registry (retries if registry is down)
    registry::register_with_retry(&registry_url, &registration).await?;
    println!("node registered with control plane");

    // Background task: ping registry every 10s so we stay "healthy"
    heartbeat::start_heartbeat(node.id.clone(), &registry_url);

    // Background task: poll scheduler for jobs and run them
    jobs::start_job_worker(node.id.clone(), scheduler_url, registry_url.clone());

    let ctrl_c = signal::ctrl_c();

    // Run HTTP server (/health, /info) until Ctrl+C
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