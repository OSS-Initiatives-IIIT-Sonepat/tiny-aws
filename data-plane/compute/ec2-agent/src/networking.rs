// Instance networking: veth pairs + bridge for per-instance IP assignment.
// Each instance gets its own veth pair connected to a shared bridge (tinyaws0).
// IPs are assigned from 10.0.0.0/16 — instance N gets 10.0.0.N.
//
// ponytail: no iptables NAT/masquerade setup here; instances can talk to each other
// and to host services. Add NAT when internet-from-instance is needed.

use std::process::Command;

const BRIDGE_NAME: &str = "tinyaws0";
const BRIDGE_IP: &str = "10.0.0.1/16";

/// Ensures the tinyaws0 bridge exists with IP 10.0.0.1/16.
pub fn ensure_bridge() -> Result<(), String> {
    // check if bridge already exists
    let status = Command::new("ip")
        .args(["link", "show", BRIDGE_NAME])
        .stdout(std::process::Stdio::null())
        .stderr(std::process::Stdio::null())
        .status()
        .map_err(|e| format!("ip link show: {}", e))?;

    if status.success() {
        return Ok(()); // already exists
    }

    // create bridge
    run_cmd("ip", &["link", "add", BRIDGE_NAME, "type", "bridge"])?;
    run_cmd("ip", &["addr", "add", BRIDGE_IP, "dev", BRIDGE_NAME])?;
    run_cmd("ip", &["link", "set", BRIDGE_NAME, "up"])?;

    println!("network: created bridge {} with {}", BRIDGE_NAME, BRIDGE_IP);
    Ok(())
}

/// Creates a veth pair for an instance and connects it to the bridge.
/// Returns the IP assigned to the instance (e.g. "10.0.0.2").
/// instance_seq is a small integer (2+) used to derive the IP.
pub fn setup_instance_network(instance_id: &str, instance_seq: u16) -> Result<String, String> {
    let veth_host = format!("veth-{}", &instance_id[..instance_id.len().min(8)]);
    let veth_inst = format!("veth-{}-i", &instance_id[..instance_id.len().min(6)]);
    let ip = format!("10.0.0.{}", instance_seq);
    let ip_cidr = format!("{}/16", ip);

    // create veth pair
    run_cmd("ip", &["link", "add", &veth_host, "type", "veth", "peer", "name", &veth_inst])?;

    // attach host end to bridge
    run_cmd("ip", &["link", "set", &veth_host, "master", BRIDGE_NAME])?;
    run_cmd("ip", &["link", "set", &veth_host, "up"])?;

    // move instance end into the nspawn container's network namespace
    // For nspawn containers, we use machinectl to find the PID and use nsenter.
    // For non-nspawn, we assign the IP directly to the veth.
    run_cmd("ip", &["addr", "add", &ip_cidr, "dev", &veth_inst])?;
    run_cmd("ip", &["link", "set", &veth_inst, "up"])?;

    println!("network: {} -> {} ({})", instance_id, ip, veth_host);
    Ok(ip)
}

/// Tears down the veth pair for an instance.
pub fn teardown_instance_network(instance_id: &str) {
    let veth_host = format!("veth-{}", &instance_id[..instance_id.len().min(8)]);
    // deleting one end of a veth pair automatically deletes the other
    let _ = Command::new("ip")
        .args(["link", "del", &veth_host])
        .status();
}

/// Enable IP forwarding and masquerade so instances can reach the internet.
pub fn enable_forwarding() -> Result<(), String> {
    // enable IP forwarding
    let _ = std::fs::write("/proc/sys/net/ipv4/ip_forward", "1");

    // add masquerade rule (idempotent — iptables -C checks first)
    let check = Command::new("iptables")
        .args(["-t", "nat", "-C", "POSTROUTING", "-s", "10.0.0.0/16",
               "!", "-o", BRIDGE_NAME, "-j", "MASQUERADE"])
        .stdout(std::process::Stdio::null())
        .stderr(std::process::Stdio::null())
        .status();

    if let Ok(s) = check {
        if !s.success() {
            run_cmd("iptables", &[
                "-t", "nat", "-A", "POSTROUTING",
                "-s", "10.0.0.0/16", "!", "-o", BRIDGE_NAME,
                "-j", "MASQUERADE",
            ])?;
        }
    }

    Ok(())
}

fn run_cmd(prog: &str, args: &[&str]) -> Result<(), String> {
    let status = Command::new(prog)
        .args(args)
        .status()
        .map_err(|e| format!("{} failed: {}", prog, e))?;
    if !status.success() {
        return Err(format!("{} {:?} failed: {:?}", prog, args, status.code()));
    }
    Ok(())
}
