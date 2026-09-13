use serde::Deserialize;
use std::path::PathBuf;

#[derive(Debug, Deserialize)]
pub struct InstanceSpec {
    pub id: String,
    pub cpu_limit: String,
    pub mem_limit_mb: u64,
    #[allow(dead_code)]
    pub instance_type: String,   // stored for logging; not used in provision logic
    #[serde(default)]
    pub base_image: String,
    #[serde(default)]
    pub volumes: Vec<String>,    // bind mounts in "host:container" format
}

// rootfs_base returns the base rootfs path all instances clone from.
// ponytail: single shared base image; upgrade to per-instance overlayfs when storage matters
pub fn rootfs_base() -> PathBuf {
    let base = std::env::var("TINYAWS_ROOTFS_BASE")
        .unwrap_or_else(|_| "/var/lib/tinyaws/base".into());
    PathBuf::from(base)
}

// rootfs_path returns the rootfs directory for a given instance.
pub fn rootfs_path(instance_id: &str) -> PathBuf {
    let root = std::env::var("TINYAWS_INSTANCES_DIR")
        .unwrap_or_else(|_| "/var/lib/tinyaws/instances".into());
    PathBuf::from(root).join(instance_id)
}

// provision creates a rootfs for instance_id by copying the base image,
// then boots it as a systemd-nspawn container with the given resource limits.
// ponytail: cp -a is simpler than overlayfs; use overlayfs when disk space matters
pub fn provision(spec: &InstanceSpec) -> Result<(), String> {
    // use spec.base_image if set, otherwise fall back to env/default
    let base = if spec.base_image.is_empty() {
        rootfs_base()
    } else {
        PathBuf::from(&spec.base_image)
    };
    let dest = rootfs_path(&spec.id);

    if !base.exists() {
        return Err(format!(
            "base rootfs not found at {}. Run scripts/bootstrap-rootfs.sh first.",
            base.display()
        ));
    }
    if dest.exists() {
        return Ok(()); // already provisioned
    }

    // copy base rootfs
    let status = std::process::Command::new("cp")
        .args(["-a", base.to_str().unwrap(), dest.to_str().unwrap()])
        .status()
        .map_err(|e| format!("cp failed: {}", e))?;
    if !status.success() {
        return Err(format!("cp base rootfs failed: {:?}", status.code()));
    }

    // boot the container with resource limits via systemd-nspawn
    // --boot: run /sbin/init inside the container
    // -M: machine name = instance_id (used by machinectl)
    let mem_bytes = spec.mem_limit_mb * 1024 * 1024;

    let mut nspawn_args = vec![
        "--unit".to_string(), format!("tinyaws-{}", spec.id),
        format!("--property=CPUQuota={}", spec.cpu_limit),
        format!("--property=MemoryMax={}M", spec.mem_limit_mb),
        "--".to_string(),
        "systemd-nspawn".to_string(),
        "--boot".to_string(),
        format!("--machine={}", spec.id),
        format!("--directory={}", dest.display()),
        "--network-veth".to_string(),
        "--resolv-conf=copy-host".to_string(),
    ];

    // add bind mounts for persistent volumes (--volume "host:container")
    for vol in &spec.volumes {
        nspawn_args.push(format!("--bind={}", vol));
    }

    let status = std::process::Command::new("systemd-run")
        .args(&nspawn_args)
        .status()
        .map_err(|e| format!("systemd-run failed: {}", e))?;

    if !status.success() {
        return Err(format!("nspawn boot failed: {:?}", status.code()));
    }

    let _ = mem_bytes; // used in comment only
    println!("instance {} provisioned (cpu={} mem={}MB)", spec.id, spec.cpu_limit, spec.mem_limit_mb);

    // set up veth networking for this instance (best-effort)
    // ponytail: derive seq from instance id suffix; collisions possible but unlikely at tiny scale
    let seq: u16 = spec.id.trim_start_matches(|c: char| !c.is_ascii_digit())
        .parse().unwrap_or(2);
    let seq = if seq < 2 { 2 } else { seq }; // 0 and 1 reserved for bridge
    if let Err(e) = crate::networking::setup_instance_network(&spec.id, seq) {
        eprintln!("instance {} networking failed (non-fatal): {}", spec.id, e);
    }

    Ok(())
}

// destroy stops the nspawn container and removes its rootfs.
pub fn destroy(instance_id: &str) {
    // tear down veth networking
    crate::networking::teardown_instance_network(instance_id);

    // stop the systemd unit
    let _ = std::process::Command::new("machinectl")
        .args(["poweroff", instance_id])
        .status();
    // give it 3s to stop gracefully, then terminate
    std::thread::sleep(std::time::Duration::from_secs(3));
    let _ = std::process::Command::new("machinectl")
        .args(["terminate", instance_id])
        .status();
    // remove rootfs
    let path = rootfs_path(instance_id);
    if path.exists() {
        let _ = std::fs::remove_dir_all(&path);
        println!("instance {} rootfs removed", instance_id);
    }
}

// nspawn_exec returns the command prefix to run a command inside the instance container.
// Used by the job runner to execute jobs inside the right container.
pub fn nspawn_exec(instance_id: &str) -> Option<Vec<String>> {
    let dest = rootfs_path(instance_id);
    if !dest.exists() {
        return None;
    }
    Some(vec![
        "systemd-nspawn".into(),
        format!("--machine={}", instance_id),
        "--quiet".into(),
        "--".into(),
    ])
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::Mutex;

    static ENV_LOCK: Mutex<()> = Mutex::new(());

    #[test]
    fn rootfs_base_default() {
        let _g = ENV_LOCK.lock().unwrap();
        unsafe { std::env::remove_var("TINYAWS_ROOTFS_BASE") };
        assert_eq!(rootfs_base(), PathBuf::from("/var/lib/tinyaws/base"));
    }

    #[test]
    fn rootfs_base_from_env() {
        let _g = ENV_LOCK.lock().unwrap();
        unsafe { std::env::set_var("TINYAWS_ROOTFS_BASE", "/custom/base") };
        assert_eq!(rootfs_base(), PathBuf::from("/custom/base"));
        unsafe { std::env::remove_var("TINYAWS_ROOTFS_BASE") };
    }

    #[test]
    fn rootfs_path_default() {
        let _g = ENV_LOCK.lock().unwrap();
        unsafe { std::env::remove_var("TINYAWS_INSTANCES_DIR") };
        assert_eq!(
            rootfs_path("i-abc123"),
            PathBuf::from("/var/lib/tinyaws/instances/i-abc123")
        );
    }

    #[test]
    fn rootfs_path_from_env() {
        let _g = ENV_LOCK.lock().unwrap();
        unsafe { std::env::set_var("TINYAWS_INSTANCES_DIR", "/tmp/inst") };
        assert_eq!(rootfs_path("i-1"), PathBuf::from("/tmp/inst/i-1"));
        unsafe { std::env::remove_var("TINYAWS_INSTANCES_DIR") };
    }

    #[test]
    fn instance_spec_deserialize() {
        let json = r#"{
            "id": "i-test",
            "cpu_limit": "200%",
            "mem_limit_mb": 512,
            "instance_type": "t2.micro"
        }"#;
        let spec: InstanceSpec = serde_json::from_str(json).unwrap();
        assert_eq!(spec.id, "i-test");
        assert_eq!(spec.cpu_limit, "200%");
        assert_eq!(spec.mem_limit_mb, 512);
        assert_eq!(spec.instance_type, "t2.micro");
        assert_eq!(spec.base_image, "");
        assert!(spec.volumes.is_empty());
    }

    #[test]
    fn instance_spec_with_volumes() {
        let json = r#"{
            "id": "i-vol",
            "cpu_limit": "100%",
            "mem_limit_mb": 256,
            "instance_type": "t2.small",
            "base_image": "/custom/rootfs",
            "volumes": ["/data:/data", "/logs:/var/log"]
        }"#;
        let spec: InstanceSpec = serde_json::from_str(json).unwrap();
        assert_eq!(spec.base_image, "/custom/rootfs");
        assert_eq!(spec.volumes.len(), 2);
        assert_eq!(spec.volumes[0], "/data:/data");
    }

    #[test]
    fn nspawn_exec_nonexistent_returns_none() {
        let _g = ENV_LOCK.lock().unwrap();
        unsafe { std::env::set_var("TINYAWS_INSTANCES_DIR", "/tmp/tinyaws-test-nonexistent") };
        assert!(nspawn_exec("no-such-instance").is_none());
        unsafe { std::env::remove_var("TINYAWS_INSTANCES_DIR") };
    }
}
