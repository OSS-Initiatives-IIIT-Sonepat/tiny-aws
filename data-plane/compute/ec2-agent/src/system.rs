use serde::Serialize;
use sysinfo::System;

#[derive(Debug, Serialize, Clone)]
pub struct SystemInfo {
    pub hostname:            String,
    pub cpu_count:           usize,
    pub memory_total_mb:     u64,
    pub memory_available_mb: u64,
    pub operating_system:    String,
    pub architecture:        String,
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

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_system_info_structure() {
        let system_info = SystemInfo {
            hostname: "test-host".to_string(),
            cpu_count: 8,
            memory_total_mb: 16384,
            memory_available_mb: 8192,
            operating_system: "Linux".to_string(),
            architecture: "x86_64".to_string(),
        };

        assert_eq!(system_info.hostname, "test-host");
        assert_eq!(system_info.cpu_count, 8);
        assert!(system_info.memory_total_mb > 0);
        assert!(system_info.memory_available_mb <= system_info.memory_total_mb);
    }

    #[test]
    fn test_system_info_serialization() {
        let system_info = SystemInfo {
            hostname: "test-host".to_string(),
            cpu_count: 4,
            memory_total_mb: 8192,
            memory_available_mb: 4096,
            operating_system: "Windows".to_string(),
            architecture: "arm64".to_string(),
        };

        // Verify it can be serialized to JSON
        let json = serde_json::to_string(&system_info).expect("serialization failed");
        assert!(json.contains("test-host"));
        assert!(json.contains("\"cpu_count\":4"));
    }
}