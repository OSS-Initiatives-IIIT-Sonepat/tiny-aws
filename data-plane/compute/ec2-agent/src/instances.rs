use serde::Deserialize;
use std::path::PathBuf;

#[derive(Debug, Deserialize)]
pub struct InstanceSpec {
    pub id: String,
    pub cpu_limit: String,
    pub mem_limit_mb: u64,
    pub instance_type: String,
    #[serde(default)]
    pub base_image: String, // path to rootfs base; empty = use TINYAWS_ROOTFS_BASE default
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
    let status = std::process::Command::new("systemd-run")
        .args([
            "--unit", &format!("tinyaws-{}", spec.id),
            &format!("--property=CPUQuota={}", spec.cpu_limit),
            &format!("--property=MemoryMax={}M", spec.mem_limit_mb),
            "--",
            "systemd-nspawn",
            "--boot",
            &format!("--machine={}", spec.id),
            &format!("--directory={}", dest.display()),
            "--network-veth",
            "--resolv-conf=copy-host",
        ])
        .status()
        .map_err(|e| format!("systemd-run failed: {}", e))?;

    if !status.success() {
        return Err(format!("nspawn boot failed: {:?}", status.code()));
    }

    let _ = mem_bytes; // used in comment only
    println!("instance {} provisioned (cpu={} mem={}MB)", spec.id, spec.cpu_limit, spec.mem_limit_mb);
    Ok(())
}

// destroy stops the nspawn container and removes its rootfs.
pub fn destroy(instance_id: &str) {
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
