import { spawn } from "node:child_process";
import { execFile } from "node:child_process";
import { readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

import { openclawArgs, openclawEnv, resultFromEnvelope } from "./openclaw.mjs";
import { promptFor, reviewFromText } from "./result.mjs";

const exec = promisify(execFile);
const here = path.dirname(fileURLToPath(import.meta.url));
const MAX_FILES = 20;
const MAX_FILE_CHARS = 100_000;

try {
await runJob();
} catch (error) {
  process.stderr.write(`${error instanceof Error ? error.message : "sandbox failed"}\n`);
  process.exit(1);
}

async function runJob() {
const raw = await readStdin();
const job = JSON.parse(raw);
const prompt = promptFor(job);
if (job.memoryDir) process.env.HERMES_HOME = job.memoryDir;
let text = "";
let metrics = { model: job.model.model };

if (job.moduleId === "hermes") {
  const python = process.env.GITCREW_PYTHON || "python3";
  const stdout = await run(python, [path.join(here, "hermes.py")], JSON.stringify({
    provider: job.model.provider === "grok" ? "xai" : job.model.provider,
    model: job.model.model,
    apiKey: job.apiKey,
    user: prompt.user,
    system: prompt.system,
    workspace: job.workspace,
    memoryDir: job.memoryDir,
    sessionId: job.sessionId,
    denyTerminal: job.denyTerminal,
  }), process.env);
  const parsed = JSON.parse(stdout);
  if (parsed.error) throw new Error(String(parsed.error));
  text = parsed.final ?? "";
  metrics = { inputTokens: parsed.inputTokens ?? 0, outputTokens: parsed.outputTokens ?? 0, model: parsed.model ?? job.model.model };
} else if (job.moduleId === "openclaw") {
  const node = process.env.GITCREW_NODE || process.execPath;
  const entry = path.join(process.env.GITCREW_OPENCLAW_ROOT || "/opt/openclaw", "openclaw.mjs");
  const stdout = await run(node, [entry, ...openclawArgs(job)], prompt.user, openclawEnv(job, process.env));
  const envelope = resultFromEnvelope(stdout, job.model.model);
  text = envelope.text;
  metrics = envelope.metrics;
} else {
  throw new Error(`Unknown loop ${job.moduleId}`);
}

const result = reviewFromText(text, metrics);
if (job.collectWrites) Object.assign(result, await changedFiles(job.workspace));
process.stdout.write(`${JSON.stringify(result)}\n`);
}

function readStdin() {
  return new Promise((resolve, reject) => {
    const chunks = [];
    process.stdin.on("data", (chunk) => chunks.push(chunk));
    process.stdin.on("error", reject);
    process.stdin.on("end", () => resolve(Buffer.concat(chunks).toString("utf8")));
  });
}

function run(command, args, stdin, env) {
  return new Promise((resolve, reject) => {
    const child = spawn(command, args, { env, stdio: ["pipe", "pipe", "pipe"] });
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
      if (code !== 0 && !stdout.trim()) reject(new Error(stderr.slice(-800) || `${command} exited ${code}`));
      else resolve(stdout);
    });
    child.stdin.end(stdin);
  });
}

async function changedFiles(workspace) {
  try {
    const diff = (await exec("git", ["diff", "HEAD"], { cwd: workspace })).stdout;
    const names = (await exec("git", ["diff", "--name-only", "HEAD"], { cwd: workspace })).stdout
      .split("\n")
      .map((line) => line.trim())
      .filter(Boolean)
      .slice(0, MAX_FILES);
    const writes = [];
    for (const file of names) {
      const content = await readFile(path.join(workspace, file), "utf8");
      if (content.length <= MAX_FILE_CHARS) writes.push({ path: file, content });
    }
    const extra = writes.length ? { writes } : {};
    if (diff.trim()) extra.patch = { diff: diff.slice(0, 200_000), message: "Apply the review fix" };
    return extra;
  } catch {
    return {};
  }
}
