pub struct SystemInfo {
    pub hostname: String,
    pub cpu_count: usize,
}

pub fn get_system_info() -> Result<SystemInfo, Box<dyn std::error::Error>> {
    let hostname = hostname::get()?
        .to_string_lossy()
        .to_string();

    let cpu_count = num_cpus::get();

    Ok(SystemInfo {
        hostname,
        cpu_count,
    })
}