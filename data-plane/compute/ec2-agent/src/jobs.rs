use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::path::{Path, PathBuf};
use std::sync::{Arc, Mutex};
use std::time::Duration;
use tokio::process::Command;
use tokio::time::interval;

#[derive(Debug, Deserialize, Clone)]
struct Job {
    job_id: String,
    command: String,
    #[serde(default)]
    deploy_url: String,
    #[serde(default)]
    instance_id: String,
    #[serde(default)]
    job_type: String, // "run" (default) | "service"
    #[serde(default)]
    port: u16,
}

#[derive(Debug, Deserialize, Clone)]
struct Instance {
    id: String,
    #[allow(dead_code)]
    node_id: String,
    status: String,
}

#[derive(Debug, Serialize)]
struct JobUpdate {
    status: String,
    exit_code: Option<i32>,
    stdout: String,
    stderr: String,
}

// workspace_root returns the base dir for instance workspaces.
fn workspace_root() -> PathBuf {
    std::env::temp_dir().join("tinyaws")
}

// workspace_path returns the workspace dir for a given instance id.
fn workspace_path(instance_id: &str) -> PathBuf {
    workspace_root().join(instance_id)
}

// Polls scheduler for jobs; skips polling if all instances on this node are terminated.
pub fn start_job_worker(node_id: String, scheduler_url: String, registry_url: String) {
    // svc_id -> pid; shared between job worker (writer) and kill-poller (reader)
    // ponytail: HashMap under Mutex; fine for single-agent low-service-count use
    let service_pids: Arc<Mutex<HashMap<String, u32>>> = Arc::new(Mutex::new(HashMap::new()));

    // J4: background task that polls registry for stopped services and kills their PIDs
    {
        let pids = service_pids.clone();
        let reg = registry_url.clone();
        let nid = node_id.clone();
        tokio::spawn(async move {
            kill_stopped_services(pids, reg, nid).await;
        });
    }

    tokio::spawn(async move {
        let client = reqwest::Client::new();
        let mut ticker = interval(Duration::from_secs(3));
        // instance_id -> workspace path; maintained each poll cycle
        // ponytail: single shared map, fine for one-agent-per-node model
        let workspaces: Arc<Mutex<HashMap<String, PathBuf>>> = Arc::new(Mutex::new(HashMap::new()));

        loop {
            ticker.tick().await;

            let instances = fetch_instances(&client, &registry_url, &node_id).await;

            if !instances.is_empty() && instances.iter().all(|i| i.status != "running") {
                continue;
            }

            // B1: create workspace dirs for new running instances
            sync_workspaces(&instances, &workspaces);

            let url = format!(
                "{}/jobs?node_id={}&status=pending",
                scheduler_url, node_id
            );

            let jobs: Vec<Job> = match client.get(&url).send().await {
                Ok(resp) => match resp.error_for_status() {
                    Ok(resp) => resp.json().await.unwrap_or_default(),
                    Err(e) => {
                        eprintln!("job poll failed: {}", e);
                        continue;
                    }
                },
                Err(e) => {
                    eprintln!("job poll failed: {}", e);
                    continue;
                }
            };

            let Some(job) = jobs.into_iter().next() else {
                continue;
            };

            println!("picked up job {} type={}", job.job_id, if job.job_type == "service" { "service" } else { "run" });

            if let Err(e) = mark_running(&client, &scheduler_url, &job.job_id).await {
                eprintln!("failed to mark job running: {}", e);
                continue;
            }

            // B2/B3: resolve workspace for this job's instance
            let workspace = if !job.instance_id.is_empty() {
                workspaces.lock().ok()
                    .and_then(|m| m.get(&job.instance_id).cloned())
            } else {
                None
            };

            // Service jobs: spawn detached, register with registry, don't block.
            if job.job_type == "service" {
                let client2 = client.clone();
                let scheduler_url2 = scheduler_url.clone();
                let registry_url2 = registry_url.clone();
                let node_id2 = node_id.clone();
                let job2 = job.clone();
                let ws = workspace.clone();
                let pids2 = service_pids.clone();
                tokio::spawn(async move {
                    run_service(client2, scheduler_url2, registry_url2, node_id2, job2, ws, pids2).await;
                });
                continue;
            }

            // Run-once job: block until done, then report.
            // If instance has a real nspawn container, run inside it.
            let nspawn_prefix = if !job.instance_id.is_empty() {
                crate::instances::nspawn_exec(&job.instance_id)
            } else {
                None
            };

            let (exit_code, stdout, stderr) = if !job.deploy_url.is_empty() {
                run_deploy(&client, &job.deploy_url, workspace.as_deref()).await
            } else {
                run_command_in(&job.command, workspace.as_deref(), nspawn_prefix.as_deref()).await
            };
            let status = if exit_code == 0 { "done" } else { "failed" };

            if let Err(e) = report_result(
                &client, &scheduler_url, &job.job_id, status, exit_code, stdout, stderr,
            ).await {
                eprintln!("failed to report job result: {}", e);
            } else {
                println!("job {} finished status={}", job.job_id, status);
            }
        }
    });
}

// Spawns a service process detached, registers it with the registry, monitors it.
async fn run_service(
    client: reqwest::Client,
    scheduler_url: String,
    registry_url: String,
    node_id: String,
    job: Job,
    workspace: Option<PathBuf>,
    service_pids: Arc<Mutex<HashMap<String, u32>>>,
) {
    // determine the deploy dir — download and extract zip if deploy_url set
    let run_dir = if !job.deploy_url.is_empty() {
        let dir = workspace.clone()
            .unwrap_or_else(|| std::env::temp_dir().join(format!("tinyaws-svc-{}", job.job_id)));
        if let Err(e) = download_and_extract(&client, &job.deploy_url, &dir).await {
            eprintln!("service {}: extract failed: {}", job.job_id, e);
            let _ = report_result(&client, &scheduler_url, &job.job_id, "failed", -1, String::new(), e).await;
            return;
        }
        dir
    } else {
        workspace.clone().unwrap_or_else(std::env::temp_dir)
    };

    let log_path = run_dir.join("service.log");

    // build the command
    #[cfg(windows)]
    let start_script = run_dir.join("start.ps1");
    #[cfg(not(windows))]
    let start_script = run_dir.join("start.sh");

    let (prog, args): (&str, Vec<&str>) = if !job.command.is_empty() {
        #[cfg(windows)]
        { ("cmd", vec!["/C", &job.command]) }
        #[cfg(not(windows))]
        { ("sh", vec!["-c", &job.command]) }
    } else if start_script.exists() {
        #[cfg(windows)]
        { ("powershell", vec!["-NoProfile", "-File", start_script.to_str().unwrap_or("")]) }
        #[cfg(not(windows))]
        { ("sh", vec![start_script.to_str().unwrap_or("")]) }
    } else {
        eprintln!("service {}: no command and no start script", job.job_id);
        let _ = report_result(&client, &scheduler_url, &job.job_id, "failed", -1, String::new(), "no start script".into()).await;
        return;
    };

    // open log file for stdout/stderr
    let log_file = std::fs::OpenOptions::new()
        .create(true).append(true).open(&log_path)
        .unwrap_or_else(|_| std::fs::File::create(&log_path).unwrap());
    let log_clone = log_file.try_clone().unwrap_or_else(|_| std::fs::File::create(&log_path).unwrap());

    // K1: on Linux, wrap with unshare for pid+mount namespace isolation when TINYAWS_ISOLATE=1
    // ponytail: requires root (or user namespaces enabled); off by default; upgrade to rootless with newuidmap if needed
    #[cfg(unix)]
    let (prog, args) = if std::env::var("TINYAWS_ISOLATE").as_deref() == Ok("1") {
        let mut wrapped = vec!["--pid", "--mount", "--fork", "--"];
        wrapped.push(prog);
        wrapped.extend_from_slice(&args);
        ("unshare", wrapped)
    } else {
        (prog, args)
    };

    let mut cmd = std::process::Command::new(prog);
    cmd.args(&args)
        .current_dir(&run_dir)
        .stdout(log_file)
        .stderr(log_clone);

    // Unix: new process group so we can kill the whole tree
    #[cfg(unix)]
    {
        use std::os::unix::process::CommandExt;
        unsafe { cmd.pre_exec(|| { libc::setpgid(0, 0); Ok(()) }); }
    }

    let child = match cmd.spawn() {
        Ok(c) => c,
        Err(e) => {
            eprintln!("service {}: spawn failed: {}", job.job_id, e);
            let _ = report_result(&client, &scheduler_url, &job.job_id, "failed", -1, String::new(), e.to_string()).await;
            return;
        }
    };

    let pid = child.id() as i32;
    println!("service {} spawned pid={} port={} log={}", job.job_id, pid, job.port, log_path.display());

    // register with registry so the LB and CLI can find it
    let svc_id = register_service(&client, &registry_url, &node_id, &job, pid).await;

    // J4: track svc_id -> pid so kill-poller can send SIGTERM
    if let Some(ref id) = svc_id {
        let _ = service_pids.lock().map(|mut m| m.insert(id.clone(), child.id()));
    }

    // J5: background log upload to object store
    let log_done = Arc::new(std::sync::atomic::AtomicBool::new(false));
    if let Some(ref id) = svc_id {
        start_log_upload(id.clone(), log_path.clone(), log_done.clone());
    }

    // monitor: wait for process to exit, then update registry
    // ponytail: blocking wait in spawn_blocking; upgrade to tokio::process if needed
    let mut child = child;

    let exit_status = tokio::task::spawn_blocking(move || child.wait()).await;

    let exit_code = match exit_status {
        Ok(Ok(status)) => status.code().unwrap_or(-1),
        _ => -1,
    };

    // signal log uploader to stop and do one final upload
    log_done.store(true, std::sync::atomic::Ordering::Relaxed);

    let final_status = if exit_code == 0 { "done" } else { "crashed" };
    println!("service {} exited exit_code={} status={}", job.job_id, exit_code, final_status);

    // remove from kill-poller map — already exited
    if let Some(ref id) = svc_id {
        let _ = service_pids.lock().map(|mut m| m.remove(id));
    }

    // update registry service record
    if let Some(ref id) = svc_id {
        let url = format!("{}/services/{}", registry_url, id);
        let _ = client.patch(&url)
            .json(&serde_json::json!({"status": final_status}))
            .send().await;
    }

    // report final job status to scheduler
    let _ = report_result(
        &client, &scheduler_url, &job.job_id,
        if exit_code == 0 { "done" } else { "failed" },
        exit_code, String::new(), String::new(),
    ).await;
}

// Registers a service with the registry and returns its service ID.
async fn register_service(
    client: &reqwest::Client,
    registry_url: &str,
    node_id: &str,
    job: &Job,
    pid: i32,
) -> Option<String> {
    let payload = serde_json::json!({
        "node_id": node_id,
        "instance_id": job.instance_id,
        "port": job.port,
        "pid": pid,
        "deploy_url": job.deploy_url,
    });
    match client.post(format!("{}/services", registry_url))
        .json(&payload).send().await
    {
        Ok(resp) => {
            let svc: serde_json::Value = resp.json().await.unwrap_or_default();
            svc["id"].as_str().map(|s| s.to_string())
        }
        Err(e) => {
            eprintln!("service registration failed: {}", e);
            None
        }
    }
}

// J5: tails log_path to object store every 30s until done_flag is set.
// ponytail: full-read-and-PUT every 30s; upgrade to append-only streaming if log files get large
fn start_log_upload(svc_id: String, log_path: std::path::PathBuf, done_flag: Arc<std::sync::atomic::AtomicBool>) {
    let store_url = crate::config::object_store_url();
    let api_key = std::env::var("TINYAWS_API_KEY").unwrap_or_default();
    tokio::spawn(async move {
        let client = reqwest::Client::new();
        let mut ticker = interval(Duration::from_secs(30));
        let key = format!("logs/{}/service.log", svc_id);
        let url = format!("{}/objects/{}", store_url, key);
        loop {
            ticker.tick().await;
            if let Ok(data) = std::fs::read(&log_path) {
                let mut req = client.put(&url)
                    .header("content-type", "text/plain")
                    .body(data);
                if !api_key.is_empty() {
                    req = req.header("authorization", format!("Bearer {}", api_key));
                }
                let _ = req.send().await;
            }
            if done_flag.load(std::sync::atomic::Ordering::Relaxed) {
                break;
            }
        }
    });
}

// Downloads zip from deploy_url and extracts to dir.
async fn download_and_extract(client: &reqwest::Client, deploy_url: &str, dir: &PathBuf) -> Result<(), String> {
    let bytes = client.get(deploy_url).send().await
        .map_err(|e| format!("download error: {}", e))?
        .bytes().await
        .map_err(|e| format!("download read error: {}", e))?;

    std::fs::create_dir_all(dir).map_err(|e| format!("mkdir error: {}", e))?;
    let zip_path = dir.join("app.zip");
    std::fs::write(&zip_path, &bytes).map_err(|e| format!("write zip error: {}", e))?;

    #[cfg(windows)]
    let extract_cmd = format!("powershell -NoProfile -Command \"Expand-Archive -Force '{}' '{}'\"",
        zip_path.to_string_lossy(), dir.to_string_lossy());
    #[cfg(not(windows))]
    let extract_cmd = format!("unzip -o '{}' -d '{}'", zip_path.to_string_lossy(), dir.to_string_lossy());

    let (code, _, err) = run_command_in(&extract_cmd, None, None).await;
    if code != 0 {
        return Err(format!("extract failed: {}", err));
    }
    Ok(())
}

// Downloads zip from deploy_url, extracts to workspace (or temp dir), runs start script.
async fn run_deploy(
    client: &reqwest::Client,
    deploy_url: &str,
    workspace: Option<&Path>,
) -> (i32, String, String) {
    let deploy_dir = workspace
        .map(|p| p.to_path_buf())
        .unwrap_or_else(|| std::env::temp_dir().join("tinyaws-deploy"));

    if let Err(e) = download_and_extract(client, deploy_url, &deploy_dir).await {
        return (-1, String::new(), e);
    }

    #[cfg(windows)]
    let start_script = deploy_dir.join("start.ps1");
    #[cfg(not(windows))]
    let start_script = deploy_dir.join("start.sh");

    if !start_script.exists() {
        return (-1, String::new(), "no start script found in deploy archive".into());
    }

    #[cfg(windows)]
    let run_cmd = format!("powershell -NoProfile -File \"{}\"", start_script.to_string_lossy());
    #[cfg(not(windows))]
    let run_cmd = format!("sh '{}'", start_script.to_string_lossy());

    run_command_in(&run_cmd, Some(&deploy_dir), None).await
}

// J4: polls registry every 10s for stopped services on this node and SIGTERMs them.
async fn kill_stopped_services(
    service_pids: Arc<Mutex<HashMap<String, u32>>>,
    registry_url: String,
    node_id: String,
) {
    let client = reqwest::Client::new();
    let mut ticker = interval(Duration::from_secs(10));
    loop {
        ticker.tick().await;
        let url = format!("{}/services?node_id={}&status=stopped", registry_url, node_id);
        let svcs: Vec<serde_json::Value> = match client.get(&url).send().await {
            Ok(r) => r.json().await.unwrap_or_default(),
            Err(_) => continue,
        };
        for svc in svcs {
            let id = svc["id"].as_str().unwrap_or("").to_string();
            if id.is_empty() { continue; }
            let pid = match service_pids.lock().ok().and_then(|m| m.get(&id).copied()) {
                Some(p) => p,
                None => continue,
            };
            #[cfg(unix)]
            {
                // SIGTERM to the process group
                unsafe { libc::kill(-(pid as i32), libc::SIGTERM); }
                println!("kill_stopped_services: sent SIGTERM to pgid {} (svc {})", pid, id);
            }
            #[cfg(windows)]
            {
                // best-effort on Windows — taskkill the PID tree
                let _ = std::process::Command::new("taskkill")
                    .args(["/PID", &pid.to_string(), "/T", "/F"])
                    .status();
            }
            if let Ok(mut map) = service_pids.lock() { map.remove(&id); }
        }
    }
}

// B1: for each running instance ensure workspace dir exists; drops terminated ones.
fn sync_workspaces(instances: &[Instance], workspaces: &Arc<Mutex<HashMap<String, PathBuf>>>) {
    let Ok(mut map) = workspaces.lock() else { return };
    for inst in instances {
        if inst.status == "running" {
            let path = workspace_path(&inst.id);
            if !path.exists() {
                if let Err(e) = std::fs::create_dir_all(&path) {
                    eprintln!("failed to create workspace {}: {}", path.display(), e);
                } else {
                    println!("workspace created: {}", path.display());
                }
            }
            map.insert(inst.id.clone(), path);
        }
    }
    map.retain(|id, _| instances.iter().any(|i| i.id == *id && i.status == "running"));
}

// Fetches all instances on this node from registry.
async fn fetch_instances(
    client: &reqwest::Client,
    registry_url: &str,
    node_id: &str,
) -> Vec<Instance> {
    let url = format!("{}/instances?node_id={}", registry_url, node_id);
    match client.get(&url).send().await {
        Ok(resp) => match resp.error_for_status() {
            Ok(resp) => resp.json().await.unwrap_or_default(),
            Err(_) => vec![],
        },
        Err(_) => vec![],
    }
}

async fn mark_running(
    client: &reqwest::Client,
    scheduler_url: &str,
    job_id: &str,
) -> Result<(), reqwest::Error> {
    let url = format!("{}/jobs/{}", scheduler_url, job_id);
    let update = JobUpdate {
        status: "running".to_string(),
        exit_code: None,
        stdout: String::new(),
        stderr: String::new(),
    };
    client.patch(&url).json(&update).send().await?.error_for_status()?;
    Ok(())
}

async fn report_result(
    client: &reqwest::Client,
    scheduler_url: &str,
    job_id: &str,
    status: &str,
    exit_code: i32,
    stdout: String,
    stderr: String,
) -> Result<(), reqwest::Error> {
    let url = format!("{}/jobs/{}", scheduler_url, job_id);
    let update = JobUpdate {
        status: status.to_string(),
        exit_code: Some(exit_code),
        stdout,
        stderr,
    };
    client.patch(&url).json(&update).send().await?.error_for_status()?;
    Ok(())
}

// B3: runs command in the given working directory (or current dir if None).
// B4: on Unix spawns in a new process group for isolation.
// nspawn_prefix: when Some, prepends systemd-nspawn args to run inside a container (Linux only).
#[allow(unused_variables)]
async fn run_command_in(command: &str, workdir: Option<&Path>, nspawn_prefix: Option<&[String]>) -> (i32, String, String) {
    #[cfg(windows)]
    let mut cmd = {
        let mut c = Command::new("cmd");
        c.args(["/C", command]);
        c
    };
    #[cfg(not(windows))]
    let mut cmd = if let Some(prefix) = nspawn_prefix {
        // run inside nspawn container: systemd-nspawn --machine=i-N --quiet -- sh -c <cmd>
        let mut c = Command::new(&prefix[0]);
        for arg in &prefix[1..] { c.arg(arg); }
        c.args(["sh", "-c", command]);
        c
    } else {
        let mut c = Command::new("sh");
        c.args(["-c", command]);
        c
    };

    if let Some(dir) = workdir {
        cmd.current_dir(dir);
    }

    // B4: Unix — new process group so kill(-pgid) cleans up children
    #[cfg(unix)]
    // pre_exec is from std::os::unix::process::CommandExt, auto-imported in edition 2024
    unsafe {
        cmd.pre_exec(|| { libc::setpgid(0, 0); Ok(()) });
    }

    match cmd.output().await {
        Ok(output) => {
            let code = output.status.code().unwrap_or(-1);
            let stdout = String::from_utf8_lossy(&output.stdout).to_string();
            let stderr = String::from_utf8_lossy(&output.stderr).to_string();
            (code, stdout, stderr)
        }
        Err(e) => (-1, String::new(), e.to_string()),
    }
}
