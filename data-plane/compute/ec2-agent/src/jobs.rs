use serde::{Deserialize, Serialize};
use std::time::Duration;
use tokio::process::Command;
use tokio::time::interval;

#[derive(Debug, Deserialize, Clone)]
struct Job {
    job_id: String,
    command: String,
}

#[derive(Debug, Deserialize)]
struct Instance {
    #[allow(dead_code)]
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

// Polls scheduler for jobs; skips polling if all instances on this node are terminated.
pub fn start_job_worker(node_id: String, scheduler_url: String, registry_url: String) {
    tokio::spawn(async move {
        let client = reqwest::Client::new();
        let mut ticker = interval(Duration::from_secs(3));

        loop {
            ticker.tick().await;

            if !can_accept_jobs(&client, &registry_url, &node_id).await {
                continue;
            }

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

            let (exit_code, stdout, stderr) = run_command(&job.command).await;
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

// True if node has no instances or at least one running instance.
async fn can_accept_jobs(client: &reqwest::Client, registry_url: &str, node_id: &str) -> bool {
    let url = format!("{}/instances?node_id={}", registry_url, node_id);

    let instances: Vec<Instance> = match client.get(&url).send().await {
        Ok(resp) => match resp.error_for_status() {
            Ok(resp) => resp.json().await.unwrap_or_default(),
            Err(_) => return true,
        },
        Err(_) => return true,
    };

    if instances.is_empty() {
        return true;
    }

    instances.iter().any(|i| i.status == "running")
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

async fn run_command(command: &str) -> (i32, String, String) {
    #[cfg(windows)]
    let output = Command::new("cmd").args(["/C", command]).output().await;

    #[cfg(not(windows))]
    let output = Command::new("sh").args(["-c", command]).output().await;

    match output {
        Ok(output) => {
            let code = output.status.code().unwrap_or(-1);
            let stdout = String::from_utf8_lossy(&output.stdout).to_string();
            let stderr = String::from_utf8_lossy(&output.stderr).to_string();
            (code, stdout, stderr)
        }
        Err(e) => (-1, String::new(), e.to_string()),
    }
}
