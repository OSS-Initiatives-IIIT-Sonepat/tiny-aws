import { createServer } from "node:http";
import { mkdir } from "node:fs/promises";
import path from "node:path";

import { prepareCheckout, removeCheckout } from "./clone.mjs";
import { runSandbox } from "./sandbox.mjs";
import { signBody, signaturesMatch } from "./sign.mjs";

const port = Number(process.env.GITCREW_WORKER_PORT || 8787);
const secret = process.env.GITCREW_WORKER_SECRET || "";
const moduleId = process.env.GITCREW_MODULE_ID || "hermes";
const entry = process.env.GITCREW_SANDBOX_ENTRY || "/app/sandbox/main.mjs";
const stateRoot = process.env.GITCREW_LOOP_STATE || path.join(process.cwd(), ".gitcrew", "loops");

if (!secret) {
  process.stderr.write("Set GITCREW_WORKER_SECRET\n");
  process.exit(1);
}

const server = createServer(async (request, response) => {
  if (request.method === "GET" && request.url === "/health") {
    response
      .writeHead(200, { "content-type": "application/json" })
      .end(JSON.stringify({ ok: true, loop: moduleId, docker: false }));
    return;
  }
  if (request.method !== "POST" || request.url !== "/v1/jobs") {
    response.writeHead(404).end();
    return;
  }
  const body = await readBody(request);
  if (!signaturesMatch(secret, body, request.headers["x-gitcrew-signature"])) {
    response.writeHead(401).end("bad signature");
    return;
  }
  let job;
  try {
    job = JSON.parse(body);
    if (!job.apiKey && process.env.OPENAI_API_KEY) job.apiKey = process.env.OPENAI_API_KEY;
    if (!job?.request?.callbackUrl || !job.moduleId || !job.model || !job.apiKey) {
      throw new Error("incomplete job");
    }
    if (job.moduleId !== moduleId) {
      response.writeHead(400).end(`this worker only runs ${moduleId}`);
      return;
    }
  } catch {
    response.writeHead(400).end("bad job");
    return;
  }
  response.writeHead(202, { "content-type": "application/json" }).end('{"accepted":true}');
  runJob(job).catch((error) => {
    process.stderr.write(`${error instanceof Error ? error.message : "job failed"}\n`);
  });
});

server.listen(port, () => {
  process.stdout.write(`gitcrew-${moduleId} listening on ${port} (no docker)\n`);
});

async function runJob(job) {
  const stepId = job.request.stepId;
  const isFork = Boolean(job.request.pullRequest?.isFork);
  const memoryDir =
    !isFork && job.memory
      ? path.join(stateRoot, "memory", job.moduleId, String(job.installationId), String(job.request.repository.id))
      : null;
  if (memoryDir) await mkdir(memoryDir, { recursive: true });
  const files = job.request.changedFiles ?? [];
  const diff = files.map((file) => `### ${file.path}\n${file.patch ?? ""}`).join("\n\n");
  const workspace = await prepareCheckout({
    root: stateRoot,
    stepId,
    fullName: job.request.repository.fullName,
    headSha: job.request.pullRequest.headSha,
    token: isFork ? null : job.cloneToken,
    isFork,
    diff,
  });
  try {
    const stdout = await runSandbox({
      entry,
      workspace,
      memoryDir,
      job: {
        moduleId: job.moduleId,
        instructions: job.instructions,
        system: job.system,
        model: job.model,
        apiKey: job.apiKey,
        denyTerminal: isFork,
        workspace,
        memoryDir,
        sessionId: stepId,
        collectWrites: Boolean(job.collectWrites) && !isFork,
        files,
      },
    });
    const result = JSON.parse(stdout);
    await postResult(job.request.callbackUrl, result);
  } catch (error) {
    await postResult(job.request.callbackUrl, {
      conclusion: "failure",
      title: "Review failed",
      summary: error instanceof Error ? error.message : "The sandbox failed",
      comments: [],
    }).catch(() => {});
  } finally {
    await removeCheckout(workspace);
  }
}

async function postResult(url, result) {
  const body = JSON.stringify(result);
  const response = await fetch(url, {
    method: "POST",
    headers: { "content-type": "application/json", "x-gitcrew-signature": signBody(secret, body) },
    body,
  });
  if (!response.ok) throw new Error(`callback answered ${response.status}`);
}

function readBody(request) {
  return new Promise((resolve, reject) => {
    let body = "";
    request.setEncoding("utf8");
    request.on("data", (chunk) => {
      body += chunk;
      if (body.length > 2_000_000) reject(new Error("job too large"));
    });
    request.on("end", () => resolve(body));
    request.on("error", reject);
  });
}
