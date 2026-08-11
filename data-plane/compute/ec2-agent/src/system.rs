use serde::Serialize;
use sysinfo::System;

#[derive(Debug, Serialize, Clone)]
pub struct SystemInfo {
    pub hostname: String,
    pub cpu_count: usize,
    pub memory_total_mb: u64,
    pub memory_available_mb: u64,
    pub operating_system: String,
    pub architecture: String,
}

pub fn get_system_info() -> SystemInfo {
    let mut system = System::new_all();
    system.refresh_all();

    let hostname = hostname::get()
        .map(|name| name.to_string_lossy().to_string())
        .unwrap_or_else(|_| "unknown".to_string());

    SystemInfo {
        hostname,
        cpu_count: num_cpus::get(),
        memory_total_mb: system.total_memory() / 1024 / 1024,
        memory_available_mb: system.available_memory() / 1024 / 1024,
        operating_system: System::long_os_version()
            .unwrap_or_else(|| "unknown".to_string()),
        architecture: System::cpu_arch(),
    }
}