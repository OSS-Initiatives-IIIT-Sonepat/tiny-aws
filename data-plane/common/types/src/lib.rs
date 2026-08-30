use serde::{Deserialize, Serialize};

/// Node registration payload shared across ec2-agent and object-store.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NodeRegistration {
    pub id: String,
    pub hostname: String,
    pub cpu_count: usize,
    pub role: String,
}

/// Heartbeat payload.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Heartbeat {
    pub id: String,
}

/// Job as returned by the scheduler.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Job {
    pub job_id: String,
    pub node_id: String,
    #[serde(default)]
    pub instance_id: String,
    pub command: String,
    #[serde(default)]
    pub deploy_url: String,
    pub status: String,
}

/// Job status update sent by agent to scheduler.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct JobUpdate {
    pub status: String,
    pub exit_code: Option<i32>,
    pub stdout: String,
    pub stderr: String,
}
