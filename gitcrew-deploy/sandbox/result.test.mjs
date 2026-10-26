import assert from "node:assert/strict";
import test from "node:test";

import { openclawArgs } from "./openclaw.mjs";
import { promptFor, reviewFromText } from "./result.mjs";

test("a prose reply becomes a neutral review", () => {
  const result = reviewFromText("the null check is missing", { model: "gpt-4.1" });
  assert.equal(result.conclusion, "neutral");
  assert.equal(result.metrics.model, "gpt-4.1");
});

test("json comments, labels, and switches survive", () => {
  const result = reviewFromText(
    '{"conclusion":"failure","title":"Null crash","summary":"user can be null","comments":[{"path":"src/label.ts","line":1,"body":"guard name","severity":"critical"}],"labels":["bug"],"rebase":true}',
    { inputTokens: 3 },
  );
  assert.equal(result.comments[0].severity, "critical");
  assert.deepEqual(result.labels, ["bug"]);
  assert.equal(result.rebase, true);
});

test("openclaw keeps the key out of argv and cannot deny the terminal and force env auth together", () => {
  const sameRepo = openclawArgs({
    workspace: "/workspace",
    memoryDir: "/memory",
    denyTerminal: false,
    model: { provider: "openai", model: "gpt-4.1" },
    apiKey: "sk-secret",
  });
  assert.equal(sameRepo.includes("sk-secret"), false);
  assert.equal(sameRepo.includes("--auth-env-only"), true);
  assert.equal(sameRepo.includes("--config-override"), false);

  const fork = openclawArgs({
    workspace: "/workspace",
    memoryDir: null,
    denyTerminal: true,
    model: { provider: "openai", model: "gpt-4.1" },
    apiKey: "sk-secret",
  });
  assert.equal(fork.includes("--auth-env-only"), false);
  assert.equal(fork.includes("--isolated"), true);
  assert.equal(fork.includes("--config-override"), true);
});

test("the prompt includes the diff", () => {
  const prompt = promptFor({ instructions: "Find the crash.", files: [{ path: "src/a.ts", patch: "+user.name" }] });
  assert.match(prompt.user, /src\/a\.ts/);
  assert.match(prompt.user, /user\.name/);
});
