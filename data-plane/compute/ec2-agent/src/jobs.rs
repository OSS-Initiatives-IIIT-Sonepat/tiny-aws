use serde::{Deserialize, Serialize};
use std::time::Duration;
use tokio::process::Command;
use tokio::time::interval;

// Shape of a job returned by scheduler GET /jobs
#[derive(Debug, Deserialize, Clone)]
struct Job {
    job_id: String,
    command: String,
}

// Body we send when PATCHing job status back to scheduler
#[derive(Debug, Serialize)]
struct JobUpdate {
    status: String,
    exit_code: Option<i32>,
    stdout: String,
    stderr: String,
}

// Starts a background task that polls scheduler every 3 seconds
// for pending jobs assigned to THIS node, runs them, reports result
pub fn start_job_worker(node_id: String, scheduler_url: String) {
    tokio::spawn(async move {
        let client = reqwest::Client::new();
        let mut ticker = interval(Duration::from_secs(3));

        // Infinite loop — agent keeps checking for work forever
        loop {
            ticker.tick().await;

            // Ask scheduler: "any pending jobs for me?"
            let url = format!(
                "{}/jobs?node_id={}&status=pending",
                scheduler_url, node_id
            );

            let jobs: Vec<Job> = match client.get(&url).send().await {
                Ok(resp) => match resp.error_for_status() {
                    Ok(resp) => resp.json().await.unwrap_or_default(),
                    Err(e) => {
                        eprintln!("job poll failed: {}", e);
                        continue; // skip this tick, try again next time
                    }
                },
                Err(e) => {
                    eprintln!("job poll failed: {}", e);
                    continue;
                }
            };

            // No work right now — wait for next tick
            let Some(job) = jobs.into_iter().next() else {
                continue;
            };

            println!("picked up job {}", job.job_id);

            // Tell scheduler we're working on it (pending → running)
            if let Err(e) = mark_running(&client, &scheduler_url, &job.job_id).await {
                eprintln!("failed to mark job running: {}", e);
                continue;
            }

            // Actually run the shell command on this machine
            let (exit_code, stdout, stderr) = run_command(&job.command).await;
            let status = if exit_code == 0 { "done" } else { "failed" };

            // Send output + final status back to scheduler
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

// PATCH /jobs/{id} with status=running so nobody else picks same job
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

// PATCH /jobs/{id} with final status, exit code, and command output
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

// Runs a shell command and returns (exit_code, stdout, stderr)
// Windows: uses cmd /C   Linux/Mac: uses sh -c
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