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

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn node_registration_roundtrip() {
        let reg = NodeRegistration {
            id: "n-1".into(),
            hostname: "host-a".into(),
            cpu_count: 4,
            role: "compute".into(),
        };
        let json = serde_json::to_string(&reg).unwrap();
        let got: NodeRegistration = serde_json::from_str(&json).unwrap();
        assert_eq!(got.id, "n-1");
        assert_eq!(got.cpu_count, 4);
        assert_eq!(got.role, "compute");
    }

    #[test]
    fn heartbeat_roundtrip() {
        let hb = Heartbeat { id: "n-1".into() };
        let json = serde_json::to_string(&hb).unwrap();
        let got: Heartbeat = serde_json::from_str(&json).unwrap();
        assert_eq!(got.id, "n-1");
    }

    #[test]
    fn job_defaults() {
        // instance_id and deploy_url have serde(default), so missing fields should work.
        let json = r#"{
            "job_id": "job-1",
            "node_id": "n-1",
            "command": "echo hi",
            "status": "pending"
        }"#;
        let job: Job = serde_json::from_str(json).unwrap();
        assert_eq!(job.job_id, "job-1");
        assert_eq!(job.instance_id, "");
        assert_eq!(job.deploy_url, "");
    }

    #[test]
    fn job_update_with_exit_code() {
        let upd = JobUpdate {
            status: "done".into(),
            exit_code: Some(0),
            stdout: "ok\n".into(),
            stderr: String::new(),
        };
        let json = serde_json::to_string(&upd).unwrap();
        let got: JobUpdate = serde_json::from_str(&json).unwrap();
        assert_eq!(got.exit_code, Some(0));
        assert_eq!(got.stdout, "ok\n");
    }

    #[test]
    fn job_update_no_exit_code() {
        let json = r#"{"status":"running","exit_code":null,"stdout":"","stderr":""}"#;
        let got: JobUpdate = serde_json::from_str(json).unwrap();
        assert_eq!(got.exit_code, None);
    }
}
