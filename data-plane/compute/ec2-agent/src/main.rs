mod node;
mod server;
mod system;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let system_info = system::get_system_info()?;

    let node = node::Node::new(system_info);

    println!("node: {}", node.id);
    println!("cpus: {}", node.system.cpu_count);

    server::start(&node).await?;

    Ok(())
}