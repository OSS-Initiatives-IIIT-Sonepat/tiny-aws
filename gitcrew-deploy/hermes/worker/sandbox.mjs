import { spawn } from "node:child_process";

/** Run the Hermes sandbox entry in-process. No Docker. */
export function runSandbox({ entry, workspace, memoryDir, job }) {
  const node = process.env.GITCREW_NODE || process.execPath;
  const env = { ...process.env };
  if (memoryDir) env.HERMES_HOME = memoryDir;
  const payload = {
    ...job,
    workspace,
    memoryDir: memoryDir || null,
  };
  return new Promise((resolve, reject) => {
    const child = spawn(node, [entry], { env, stdio: ["pipe", "pipe", "pipe"] });
    let stdout = "";
    let stderr = "";
    child.stdout.setEncoding("utf8");
    child.stderr.setEncoding("utf8");
    child.stdout.on("data", (chunk) => {
      stdout += chunk;
    });
    child.stderr.on("data", (chunk) => {
      stderr += chunk;
    });
    child.on("error", reject);
    child.on("close", (code) => {
      if (code !== 0 && !stdout.trim()) {
        reject(new Error(stderr.slice(-800) || `sandbox exited ${code}`));
      } else {
        resolve(stdout);
      }
    });
    child.stdin.end(JSON.stringify(payload));
  });
}
