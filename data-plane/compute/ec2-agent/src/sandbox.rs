// Sandbox: wraps job commands in Linux namespace isolation.
// Uses unshare(1) for PID + mount + IPC namespace isolation.
// Applies cgroup v2 resource limits, overlayfs filesystem isolation,
// pivot_root, and optionally a seccomp syscall blacklist.
//
// ponytail: no Docker, no runc — raw Linux primitives in ~250 lines.

use std::path::{Path, PathBuf};

/// Whether sandboxing is enabled. On by default on Linux; disabled by TINYAWS_SANDBOX=0.
pub fn enabled() -> bool {
    #[cfg(not(unix))]
    return false;

    #[cfg(unix)]
    {
        let v = std::env::var("TINYAWS_SANDBOX").unwrap_or_else(|_| "1".into());
        v != "0"
    }
}

/// Returns the path to the seccomp profile, if available.
fn seccomp_profile_path() -> Option<String> {
    // explicit env var takes priority
    if let Ok(p) = std::env::var("TINYAWS_SECCOMP_PROFILE") {
        if Path::new(&p).exists() {
            return Some(p);
        }
    }
    // look next to the binary
    if let Ok(exe) = std::env::current_exe() {
        if let Some(dir) = exe.parent() {
            let candidate = dir.join("seccomp-default.json");
            if candidate.exists() {
                return Some(candidate.to_string_lossy().into_owned());
            }
        }
    }
    // well-known system path
    let system = "/etc/tinyaws/seccomp-default.json";
    if Path::new(system).exists() {
        return Some(system.into());
    }
    None
}

/// Wraps a command with unshare for PID + mount + IPC namespace isolation.
/// Returns (program, args) to execute.
pub fn wrap_command(prog: &str, args: &[&str]) -> (String, Vec<String>) {
    if !enabled() {
        return (prog.into(), args.iter().map(|s| s.to_string()).collect());
    }

    let mut wrapped_args: Vec<String> = vec![
        "--pid".into(),
        "--mount".into(),
        "--ipc".into(),
        "--fork".into(),
        "--mount-proc".into(),
        "--".into(),
    ];
    wrapped_args.push(prog.into());
    for a in args {
        wrapped_args.push(a.to_string());
    }

    ("unshare".into(), wrapped_args)
}

/// Wraps a command with full filesystem isolation: overlayfs + pivot_root + unshare.
/// The process sees `image_path` as its root, with a writable overlay on top.
/// Returns (program, args) that set up the mount and exec into the command.
///
/// The returned command is a shell script that:
/// 1. Creates overlay dirs under scratch_dir
/// 2. Mounts overlayfs (lowerdir=image, upperdir=scratch/upper, workdir=scratch/work)
/// 3. Copies the app code into the merged root
/// 4. pivot_root into the merged root
/// 5. Execs the actual command
pub fn wrap_command_with_rootfs(
    prog: &str,
    args: &[&str],
    image_path: &Path,
    app_dir: &Path,
    scratch_dir: &Path,
) -> (String, Vec<String>) {
    let upper = scratch_dir.join("upper");
    let work = scratch_dir.join("work");
    let merged = scratch_dir.join("merged");
    let old_root = merged.join("old_root");

    // Build a shell script that sets up overlayfs + pivot_root, then execs the command.
    // This runs inside unshare --pid --mount --fork.
    let user_cmd = if args.is_empty() {
        prog.to_string()
    } else {
        format!("{} {}", prog, args.iter().map(|a| format!("'{}'", a)).collect::<Vec<_>>().join(" "))
    };

    let script = format!(
        r#"set -e
mkdir -p '{upper}' '{work}' '{merged}' '{old_root}'
mount -t overlay overlay -o 'lowerdir={lower},upperdir={upper},workdir={work}' '{merged}'
cp -a '{app_dir}/.' '{merged}/app/' 2>/dev/null || true
cd '{merged}'
mkdir -p '{old_root}'
pivot_root . old_root
cd /app
umount -l /old_root 2>/dev/null || true
rmdir /old_root 2>/dev/null || true
exec {cmd}"#,
        upper = upper.display(),
        work = work.display(),
        merged = merged.display(),
        old_root = old_root.display(),
        lower = image_path.display(),
        app_dir = app_dir.display(),
        cmd = user_cmd,
    );

    if !enabled() {
        return (prog.into(), args.iter().map(|s| s.to_string()).collect());
    }

    // Wrap in unshare for namespace isolation
    let wrapped_args: Vec<String> = vec![
        "--pid".into(),
        "--mount".into(),
        "--ipc".into(),
        "--fork".into(),
        "--mount-proc".into(),
        "--".into(),
        "sh".into(),
        "-c".into(),
        script,
    ];

    ("unshare".into(), wrapped_args)
}

/// Holds the paths for an active overlay mount, used for cleanup.
pub struct OverlayMount {
    pub merged: PathBuf,
    pub scratch_dir: PathBuf,
}

/// Creates scratch dirs for an overlay mount. Returns the scratch base path.
/// Actual mounting happens inside the unshare'd process (wrap_command_with_rootfs).
pub fn prepare_overlay(job_id: &str) -> PathBuf {
    let scratch = PathBuf::from(format!("/tmp/tinyaws-overlay/{}", job_id));
    let _ = std::fs::create_dir_all(scratch.join("upper"));
    let _ = std::fs::create_dir_all(scratch.join("work"));
    let _ = std::fs::create_dir_all(scratch.join("merged"));
    scratch
}

/// Cleans up overlay dirs after job completion.
pub fn cleanup_overlay(job_id: &str) {
    let scratch = format!("/tmp/tinyaws-overlay/{}", job_id);
    // try to unmount merged first (may already be unmounted if process exited cleanly)
    #[cfg(unix)]
    {
        let merged = format!("{}/merged", scratch);
        let _ = std::process::Command::new("umount")
            .args(["-l", &merged])
            .status();
    }
    let _ = std::fs::remove_dir_all(&scratch);
}

/// Wraps a command for nspawn-based instances, applying the seccomp profile if available.
pub fn nspawn_seccomp_args() -> Vec<String> {
    if let Some(profile) = seccomp_profile_path() {
        vec![format!("--settings=override"), format!("--seccomp-profile={}", profile)]
    } else {
        vec![]
    }
}

/// Sets up cgroup v2 resource limits for a job.
/// Creates /sys/fs/cgroup/tinyaws/<job_id>/ and writes memory.max + cpu.max.
/// Returns the cgroup path if successful, None otherwise.
pub fn setup_cgroup(job_id: &str, mem_limit_mb: u64, cpu_percent: u64) -> Option<String> {
    #[cfg(not(unix))]
    {
        let _ = (job_id, mem_limit_mb, cpu_percent);
        return None;
    }

    #[cfg(unix)]
    {
        let cgroup_base = "/sys/fs/cgroup/tinyaws";
        let cgroup_path = format!("{}/{}", cgroup_base, job_id);

        // ensure parent exists
        let _ = std::fs::create_dir_all(cgroup_base);

        // create cgroup dir for this job
        if std::fs::create_dir_all(&cgroup_path).is_err() {
            eprintln!("sandbox: failed to create cgroup {}", cgroup_path);
            return None;
        }

        // memory.max in bytes
        if mem_limit_mb > 0 {
            let mem_bytes = mem_limit_mb * 1024 * 1024;
            if std::fs::write(
                format!("{}/memory.max", cgroup_path),
                mem_bytes.to_string(),
            ).is_err() {
                eprintln!("sandbox: failed to set memory.max for {}", job_id);
            }
        }

        // cpu.max: <quota> <period>  — e.g. "50000 100000" for 50% of one core
        if cpu_percent > 0 {
            let quota = cpu_percent * 1000; // microseconds per 100ms period
            let period = 100_000u64;
            if std::fs::write(
                format!("{}/cpu.max", cgroup_path),
                format!("{} {}", quota, period),
            ).is_err() {
                eprintln!("sandbox: failed to set cpu.max for {}", job_id);
            }
        }

        println!("sandbox: cgroup {} (mem={}MB cpu={}%)", job_id, mem_limit_mb, cpu_percent);
        Some(cgroup_path)
    }
}

/// Cleans up a cgroup directory after job completion.
pub fn cleanup_cgroup(job_id: &str) {
    #[cfg(unix)]
    {
        let cgroup_path = format!("/sys/fs/cgroup/tinyaws/{}", job_id);
        // rmdir only works when cgroup has no processes
        let _ = std::fs::remove_dir(&cgroup_path);
    }
    #[cfg(not(unix))]
    { let _ = job_id; }
}
