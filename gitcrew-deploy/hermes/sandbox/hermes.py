"""One review through Hermes's own agent loop.

The loop is installed in the image at GITCREW_HERMES_ROOT. This file only
hands it the prompt, the checkout, and the memory directory.
"""

import json
import os
import sys

root = os.environ.get("GITCREW_HERMES_ROOT", "")
if not root or not os.path.isfile(os.path.join(root, "run_agent.py")):
    print(json.dumps({"error": "Hermes loop is not installed at GITCREW_HERMES_ROOT"}))
    sys.exit(1)

sys.path.insert(0, root)

from run_agent import AIAgent  # noqa: E402


def main() -> None:
    job = json.load(sys.stdin)
    provider = job["provider"]
    env_name = {"openai": "OPENAI_API_KEY", "xai": "XAI_API_KEY", "anthropic": "ANTHROPIC_API_KEY"}.get(provider)
    if env_name:
        os.environ[env_name] = job["apiKey"]
    base_url = {"openai": "https://api.openai.com/v1", "xai": "https://api.x.ai/v1"}.get(provider)
    agent = AIAgent(
        model=job["model"],
        provider=provider,
        api_key=job["apiKey"],
        base_url=base_url,
        quiet_mode=True,
        skip_memory=not job.get("memoryDir"),
        session_id=job.get("sessionId"),
        cwd=job["workspace"],
        enabled_toolsets=["file"] if job.get("denyTerminal") else ["file", "terminal"],
        max_iterations=8,
        save_trajectories=False,
    )
    result = agent.run_conversation(user_message=job["user"], system_message=job.get("system") or None)
    print(
        json.dumps(
            {
                "final": result.get("final_response") or "",
                "inputTokens": int(getattr(agent, "session_input_tokens", 0) or 0),
                "outputTokens": int(getattr(agent, "session_output_tokens", 0) or 0),
                "model": job["model"],
                "error": result.get("error"),
            }
        )
    )


if __name__ == "__main__":
    main()
