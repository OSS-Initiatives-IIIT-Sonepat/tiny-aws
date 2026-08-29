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

            println!("picked up job {}", job.job_id);

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

            let (exit_code, stdout, stderr) = if !job.deploy_url.is_empty() {
                run_deploy(&client, &job.deploy_url, workspace.as_deref()).await
            } else {
                run_command_in(&job.command, workspace.as_deref()).await
            };
            let status = if exit_code == 0 { "done" } else { "failed" };

            if let Err(e) = report_result(
                &client,
                &scheduler_url,
                &job.job_id,
                status,
                exit_code,
                stdout,
                stderr,
            )
            .await
            {
                eprintln!("failed to report job result: {}", e);
            } else {
                println!("job {} finished with status={}", job.job_id, status);
            }
        }
    });
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
    // remove terminated instances from map (workspace stays on disk until controller cleans it)
    map.retain(|id, _| instances.iter().any(|i| i.id == *id && i.status == "running"));
}

// Downloads zip from deploy_url, extracts to workspace (or temp dir), runs start script.
async fn run_deploy(
    client: &reqwest::Client,
    deploy_url: &str,
    workspace: Option<&Path>,
) -> (i32, String, String) {
    let bytes = match client.get(deploy_url).send().await {
        Ok(resp) => match resp.bytes().await {
            Ok(b) => b,
            Err(e) => return (-1, String::new(), format!("download read error: {}", e)),
        },
        Err(e) => return (-1, String::new(), format!("download error: {}", e)),
    };

    let deploy_dir = workspace
        .map(|p| p.to_path_buf())
        .unwrap_or_else(|| std::env::temp_dir().join("tinyaws-deploy"));
    let zip_path = deploy_dir.join("app.zip");

    if let Err(e) = std::fs::create_dir_all(&deploy_dir) {
        return (-1, String::new(), format!("mkdir error: {}", e));
    }
    if let Err(e) = std::fs::write(&zip_path, &bytes) {
        return (-1, String::new(), format!("write zip error: {}", e));
    }

    let zip_str = zip_path.to_string_lossy();
    let dir_str = deploy_dir.to_string_lossy();

    // ponytail: shell-out extraction; add zip crate if cross-platform extraction without shell matters
    #[cfg(windows)]
    let extract_cmd = format!(
        "powershell -NoProfile -Command \"Expand-Archive -Force '{}' '{}'\"",
        zip_str, dir_str
    );
    #[cfg(not(windows))]
    let extract_cmd = format!("unzip -o '{}' -d '{}'", zip_str, dir_str);

    let (code, out, err) = run_command_in(&extract_cmd, None).await;
    if code != 0 {
        return (code, out, format!("extract failed: {}", err));
    }

    #[cfg(windows)]
    let start_script = deploy_dir.join("start.ps1");
    #[cfg(not(windows))]
    let start_script = deploy_dir.join("start.sh");

    if !start_script.exists() {
        return (-1, String::new(), "no start script found in deploy archive".into());
    }

    #[cfg(windows)]
    let run_cmd = format!(
        "powershell -NoProfile -File \"{}\"",
        start_script.to_string_lossy()
    );
    #[cfg(not(windows))]
    let run_cmd = format!("sh '{}'", start_script.to_string_lossy());

    run_command_in(&run_cmd, Some(&deploy_dir)).await
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
// B4: on Unix spawns in a new process group for isolation; on Windows uses a Job Object.
async fn run_command_in(command: &str, workdir: Option<&Path>) -> (i32, String, String) {
    #[cfg(windows)]
    let mut cmd = {
        let mut c = Command::new("cmd");
        c.args(["/C", command]);
        c
    };
    #[cfg(not(windows))]
    let mut cmd = {
        let mut c = Command::new("sh");
        c.args(["-c", command]);
        c
    };

    if let Some(dir) = workdir {
        cmd.current_dir(dir);
    }

    // B4: Unix — new process group so kill(-pgid) cleans up children
    #[cfg(unix)]
    {
        use std::os::unix::process::CommandExt;
        unsafe { cmd.pre_exec(|| { libc::setpgid(0, 0); Ok(()) }); }
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
