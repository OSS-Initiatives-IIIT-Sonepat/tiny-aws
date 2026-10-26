import { spawn } from "node:child_process";

/** One throwaway container. The model key rides on stdin. The GitHub token never does. */
export function dockerArgs({ image, workspace, memoryDir }) {
  const args = ["run", "--rm", "-i", "-v", `${workspace}:/workspace`, "-w", "/workspace"];
  if (memoryDir) {
    args.push("-v", `${memoryDir}:/memory`, "-e", "HERMES_HOME=/memory");
  }
  args.push(image);
  return args;
}

export function runContainer({ image, workspace, memoryDir, job }) {
  const entry = process.env.GITCREW_SANDBOX_ENTRY;
  if (entry) return runSandbox(entry, workspace, memoryDir, job);
  const args = dockerArgs({ image, workspace, memoryDir });
  return new Promise((resolve, reject) => {
    const child = spawn("docker", args, { stdio: ["pipe", "pipe", "pipe"] });
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
      if (code !== 0 && !stdout.trim()) reject(new Error(stderr.slice(-800) || `docker exited ${code}`));
      else resolve(stdout);
    });
    child.stdin.end(JSON.stringify(job));
  });
}

/** Same stdin contract as the container, without Docker. Tiny AWS already has the loops in the rootfs. */
function runSandbox(entry, workspace, memoryDir, job) {
  const node = process.env.GITCREW_NODE || process.execPath;
  const env = { ...process.env };
  if (memoryDir) env.HERMES_HOME = memoryDir;
  const payload = { ...job, workspace, memoryDir: memoryDir || null };
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
      if (code !== 0 && !stdout.trim()) reject(new Error(stderr.slice(-800) || `sandbox exited ${code}`));
      else resolve(stdout);
    });
    child.stdin.end(JSON.stringify(payload));
  });
}
