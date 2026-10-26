/** Turn a loop's stdout into the AgentResult GitCrew already stores. */

const SEVERITIES = new Set(["critical", "high", "medium", "low"]);

export function reviewFromText(text, metrics) {
  const body = String(text ?? "").trim();
  if (!body) throw new Error("The review loop returned an empty reply");
  try {
    return { ...parseReview(body), metrics };
  } catch {
    return {
      conclusion: "neutral",
      title: "Review",
      summary: body.slice(0, 60_000),
      comments: [],
      metrics,
    };
  }
}

function parseReview(text) {
  const fenced = text.match(/```(?:json)?\s*([\s\S]*?)```/);
  const body = (fenced?.[1] ?? text).trim();
  const start = body.indexOf("{");
  const end = body.lastIndexOf("}");
  if (start === -1 || end <= start) throw new Error("The review reply was not JSON");
  const raw = JSON.parse(body.slice(start, end + 1));
  if (!["success", "failure", "neutral"].includes(raw.conclusion)) throw new Error("Bad conclusion");
  if (typeof raw.title !== "string" || !raw.title.trim()) throw new Error("Bad title");
  const comments = Array.isArray(raw.comments) ? raw.comments : [];
  const result = {
    conclusion: raw.conclusion,
    title: raw.title.slice(0, 200),
    summary: typeof raw.summary === "string" ? raw.summary.slice(0, 60_000) : "",
    comments: comments
      .filter((comment) => comment && typeof comment.path === "string" && typeof comment.body === "string")
      .slice(0, 500)
      .map((comment) => ({
        path: comment.path,
        line: Number.isInteger(comment.line) && comment.line > 0 ? comment.line : 1,
        body: String(comment.body).slice(0, 10_000),
        ...(SEVERITIES.has(comment.severity) ? { severity: comment.severity } : {}),
      })),
  };
  if (Array.isArray(raw.labels)) {
    result.labels = raw.labels.filter((label) => typeof label === "string" && label.trim()).slice(0, 10);
  }
  if (typeof raw.rebase === "boolean") result.rebase = raw.rebase;
  if (typeof raw.rerunCi === "boolean") result.rerunCi = raw.rerunCi;
  return result;
}

export function promptFor(job) {
  const diff = (job.files ?? [])
    .map((file) => `### ${file.path}\n${file.patch ?? "(no patch)"}`)
    .join("\n\n");
  const system =
    job.system ||
    `You review a pull request by reading the checkout and running the tests that belong to the change.
Reply with one JSON object and nothing else:
{"conclusion":"success"|"failure"|"neutral","title":"one line","summary":"markdown","comments":[{"path":"file","line":1,"body":"what to change","severity":"critical"|"high"|"medium"|"low"}],"labels":[],"rebase":false,"rerunCi":false}
Only comment on lines in the diff. Edit the checkout when you were asked for a fix. Do not push.`;
  return { system, user: `${String(job.instructions ?? "").trim()}\n\n${diff || "(no changed files)"}` };
}
