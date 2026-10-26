/** Argv for `openclaw agent exec`. The model key stays in the environment, never in argv. */

export function openclawArgs(job) {
  const args = [
    "agent",
    "exec",
    "--json",
    "--cwd",
    job.workspace,
    "--model",
    `${job.model.provider}/${job.model.model}`,
    "--message-file",
    "-",
  ];
  if (job.memoryDir) args.push("--state-dir", job.memoryDir);
  else args.push("--isolated");
  if (job.denyTerminal) {
    args.push("--config-override", 'tools.deny=["exec","bash","process"]');
  } else {
    args.push("--auth-env-only");
  }
  return args;
}

export function openclawEnv(job, base) {
  const env = { ...base };
  const name = { openai: "OPENAI_API_KEY", xai: "XAI_API_KEY", anthropic: "ANTHROPIC_API_KEY" }[job.model.provider];
  if (name) env[name] = job.apiKey;
  return env;
}

export function resultFromEnvelope(stdout, model) {
  const envelope = JSON.parse(stdout);
  if (!envelope.ok && envelope.error?.message) throw new Error(envelope.error.message);
  return {
    text: envelope.final ?? "",
    metrics: {
      inputTokens: envelope.usage?.input ?? 0,
      outputTokens: envelope.usage?.output ?? 0,
      model: envelope.model ?? model,
    },
  };
}
