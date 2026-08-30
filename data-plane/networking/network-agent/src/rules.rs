use serde::Deserialize;
use std::time::Duration;
use tokio::process::Command;
use tokio::time::interval;

#[derive(Debug, Deserialize)]
struct SGRule {
    id: String,
    #[serde(default)]
    direction: String,
    action: String,
    protocol: String,
    port: u16,
    cidr: String,
}

// networking_url returns NETWORKING_URL env or default.
fn networking_url() -> String {
    std::env::var("NETWORKING_URL").unwrap_or_else(|_| "http://127.0.0.1:9005".into())
}

// sg_id returns the security group to enforce, from SG_ID env.
fn sg_id() -> String {
    std::env::var("SG_ID").unwrap_or_default()
}

// Polls networking service for SG rules and applies them every 30s.
pub async fn start_rule_enforcer() {
    let mut ticker = interval(Duration::from_secs(30));
    let client = reqwest::Client::new();

    loop {
        ticker.tick().await;

        let sg = sg_id();
        if sg.is_empty() {
            continue;
        }

        let url = format!("{}/security-groups/{}/rules", networking_url(), sg);
        let rules: Vec<SGRule> = match client.get(&url).send().await {
            Ok(r) => r.json().await.unwrap_or_default(),
            Err(e) => {
                eprintln!("rule fetch error: {}", e);
                continue;
            }
        };

        for rule in &rules {
            apply_rule(rule).await;
        }
    }
}

// F6/F7: applies one SG rule via platform firewall.
async fn apply_rule(rule: &SGRule) {
    #[cfg(windows)]
    apply_rule_windows(rule).await;

    #[cfg(not(windows))]
    apply_rule_linux(rule).await;
}

// F6: Windows — netsh advfirewall (best-effort)
#[cfg(windows)]
async fn apply_rule_windows(rule: &SGRule) {
    let action = if rule.action == "allow" { "allow" } else { "block" };
    let dir = if rule.direction == "inbound" { "in" } else { "out" };
    let proto = if rule.protocol == "*" { "any" } else { &rule.protocol };

    let mut args = vec![
        "advfirewall".to_string(), "firewall".to_string(), "add".to_string(), "rule".to_string(),
        format!("name=tinyaws-{}", rule.id),
        format!("dir={}", dir),
        format!("action={}", action),
        format!("protocol={}", proto),
        format!("remoteip={}", rule.cidr),
    ];
    if rule.port > 0 {
        args.push(format!("localport={}", rule.port));
    }

    let status = Command::new("netsh").args(&args).status().await;
    if let Ok(s) = status {
        if !s.success() {
            eprintln!("netsh rule {} failed", rule.id);
        } else {
            println!("applied rule {} ({})", rule.id, rule.action);
        }
    }
}

// F7: Linux — iptables (best-effort)
#[cfg(not(windows))]
async fn apply_rule_linux(rule: &SGRule) {
    let chain = if rule.direction == "inbound" { "INPUT" } else { "OUTPUT" };
    let target = if rule.action == "allow" { "ACCEPT" } else { "DROP" };
    let proto = if rule.protocol == "*" { "all" } else { &rule.protocol };

    let mut args = vec![
        "-A".to_string(), chain.to_string(),
        "-p".to_string(), proto.to_string(),
        "-s".to_string(), rule.cidr.clone(),
        "-j".to_string(), target.to_string(),
    ];
    if rule.port > 0 {
        args.push("--dport".to_string());
        args.push(rule.port.to_string());
    }

    let status = Command::new("iptables").args(&args).status().await;
    if let Ok(s) = status {
        if !s.success() {
            eprintln!("iptables rule {} failed", rule.id);
        } else {
            println!("applied rule {} ({})", rule.id, rule.action);
        }
    }
}
