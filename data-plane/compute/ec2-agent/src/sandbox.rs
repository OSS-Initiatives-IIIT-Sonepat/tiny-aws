// Sandbox: wraps job commands in Linux namespace isolation.
// Uses unshare(1) for PID + mount + IPC namespace isolation.
// Applies cgroup v2 resource limits and optionally a seccomp syscall blacklist.
//
// ponytail: no Docker, no runc — raw Linux primitives in ~180 lines.
// Upgrade to pivot_root + overlayfs when full filesystem isolation is needed.

use std::path::Path;

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

/// Wraps a command for nspawn-based instances, applying the seccomp profile if available.
/// Used by the instance provisioner when running with systemd-nspawn.
pub fn nspawn_seccomp_args() -> Vec<String> {
    if let Some(profile) = seccomp_profile_path() {
        // systemd-nspawn supports --seccomp-policy but it's version-dependent.
        // For broad compat, we pass the profile path for the caller to handle.
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
