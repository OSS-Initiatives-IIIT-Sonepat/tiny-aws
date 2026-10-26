import { spawn } from "node:child_process";
import { mkdir, rm, writeFile } from "node:fs/promises";
import path from "node:path";

/** Clone env. The token is an http header, not part of the git argv or the image. */
export function cloneEnv(token) {
  const basic = Buffer.from(`x-access-token:${token}`).toString("base64");
  return {
    GIT_CONFIG_COUNT: "1",
    GIT_CONFIG_KEY_0: "http.extraheader",
    GIT_CONFIG_VALUE_0: `AUTHORIZATION: basic ${basic}`,
  };
}

export async function prepareCheckout({ root, stepId, fullName, headSha, token, isFork, diff }) {
  const directory = path.join(root, "checkouts", stepId);
  await mkdir(directory, { recursive: true });
  if (isFork || !token) {
    await writeFile(path.join(directory, "PULL.diff"), diff || "");
    return directory;
  }
  await runGit(["init", directory], {});
  await runGit(["-C", directory, "remote", "add", "origin", `https://github.com/${fullName}.git`], {});
  await runGit(["-C", directory, "fetch", "--depth", "1", "origin", headSha], cloneEnv(token));
  await runGit(["-C", directory, "checkout", "FETCH_HEAD"], {});
  return directory;
}

export async function removeCheckout(directory) {
  await rm(directory, { recursive: true, force: true });
}

function runGit(args, env) {
  return new Promise((resolve, reject) => {
    const child = spawn("git", args, { env: { ...process.env, ...env }, stdio: ["ignore", "pipe", "pipe"] });
    let stderr = "";
    child.stderr.setEncoding("utf8");
    child.stderr.on("data", (chunk) => {
      stderr += chunk;
    });
    child.on("error", reject);
    child.on("close", (code) => {
      if (code !== 0) reject(new Error(stderr.slice(-400) || `git exited ${code}`));
      else resolve();
    });
  });
}
