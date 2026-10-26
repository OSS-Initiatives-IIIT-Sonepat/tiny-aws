import assert from "node:assert/strict";
import test from "node:test";

import { cloneEnv } from "./clone.mjs";
import { dockerArgs } from "./docker.mjs";

test("the clone header carries the token and the git command does not", () => {
  const env = cloneEnv("ghs_secret_token");
  assert.equal(JSON.stringify(["fetch", "--depth", "1", "origin", "abc"]).includes("ghs_secret_token"), false);
  assert.equal(env.GIT_CONFIG_KEY_0, "http.extraheader");
  assert.match(env.GIT_CONFIG_VALUE_0, /^AUTHORIZATION: basic /);
  assert.equal(env.GIT_CONFIG_VALUE_0.includes("ghs_secret_token"), false);
});

test("the container is removed and does not receive a GitHub token", () => {
  const args = dockerArgs({ image: "gitcrew-sandbox:local", workspace: "/tmp/checkout", memoryDir: "/tmp/memory" });
  assert.equal(args.includes("--rm"), true);
  assert.equal(args.includes("ghs_secret_token"), false);
  assert.equal(args.includes("-e"), true);
  const fork = dockerArgs({ image: "gitcrew-sandbox:local", workspace: "/tmp/checkout", memoryDir: null });
  assert.equal(fork.includes("/memory"), false);
});
