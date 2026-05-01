<div align="center">

# Claude OpenCode Proxy Service

**Use Claude Code like normal, but run it through OpenCode Go models.**

[![Release](https://img.shields.io/github/v/release/wmostert76/claude-opencode-proxy-service?include_prereleases&label=release)](https://github.com/wmostert76/claude-opencode-proxy-service/releases)
[![Shell](https://img.shields.io/badge/shell-bash%20%2B%20fish-4eaa25)](#shell-completion)
[![Node](https://img.shields.io/badge/node-%3E%3D20-339933)](#requirements)
[![Install](https://img.shields.io/badge/install-curl%20%7C%20bash-0a7)](#one-line-install)

</div>

---

## Why

**The problem:** Claude Code speaks Anthropic Messages format. OpenCode Go has 14 models, but only **2 speak Anthropic Messages natively** (MiniMax M2.5/M2.7). The other **12 speak OpenAI Chat Completions**, making them unusable with Claude Code without translation.

This proxy does exactly that — it translates Anthropic Messages ⟷ OpenAI Chat Completions in real time, so all 14 Go models work with Claude Code.

```
Claude Code  ──Anthropic Messages──▶  Proxy (:8082)  ──OpenAI Chat──▶  OpenCode Go
```

| Without proxy | With proxy |
|---|---|
| 2/14 Go models usable via Claude Code | 14/14 Go models usable via Claude Code |
| Only MiniMax M2.5, M2.7 | Adds DeepSeek, Kimi, GLM, Qwen, Mimo |

All Claude Code native tools work: Bash, Read, Write, Edit, WebFetch, WebSearch, Task, NotebookEdit, Agent, and more.

---

## One-Line Install

```bash
curl -fsSL https://raw.githubusercontent.com/wmostert76/claude-opencode-proxy-service/main/scripts/bootstrap.sh | bash
claude setup
```

The installer:

- downloads this repo to `~/.local/share/claude-opencode-proxy-service`
- installs Claude Code with npm if missing
- installs the `claude` wrapper into `~/.local/share/npm-global/bin/claude`
- keeps your OpenCode Go API key in `~/.config/claude-opencode-proxy/config.json`
- points Claude Code at the local proxy through `~/.claude/settings.json`
- does not commit, print, or upload secrets

Use the setup wizard once:

```bash
claude setup
```

Or store your OpenCode Go API key directly:

```bash
claude --api sk-your-opencode-go-key
```

---

## What Works

All Claude Code native tools function across all 14 OpenCode Go models:

| Category | Tools |
|----------|-------|
| Files | Read, Write, Edit, NotebookEdit |
| Shell | Bash |
| Web | WebFetch, WebSearch |
| Agents | Agent (sub-agents) |
| Planning | EnterPlanMode, ExitPlanMode |
| Tasks | TaskCreate, TaskUpdate, TaskList, TaskGet, TaskOutput, TaskStop |
| Worktrees | EnterWorktree, ExitWorktree |
| Scheduling | CronCreate, CronDelete, CronList, ScheduleWakeup |
| Interaction | AskUserQuestion |

**Models tested (14/14):** deepseek-v4-pro, glm-5.1, kimi-k2.6, minimax-m2.7, qwen3.6-plus, deepseek-v4-flash, glm-5, kimi-k2.5, minimax-m2.5, qwen3.5-plus, mimo-v2.5-pro, mimo-v2.5, mimo-v2-pro, mimo-v2-omni.

Model selection is handled natively by Claude Code via `/v1/models` discovery (Claude Code v2.1.126+). Use `/model` inside Claude Code to switch models. The wrapper also keeps `claude --model <name>` as a convenience shortcut.

---

## Startup Preview

Every normal `claude` launch starts the proxy and shows a compact status panel:

```text
Claude OpenCode Proxy Service
─────────────────────────────
  Release    v0.8.0
  State      ready
  Proxy      http://127.0.0.1:8082
  Provider   OpenCode Go
  Model      auto (via /v1/models discovery)
  Tools      all Claude Code native tools
  Config     /home/you/.config/claude-opencode-proxy/config.json
  Log        /home/you/.cache/claude-opencode-proxy/proxy.log
  Mode       Claude Code passthrough + local adapter
─────────────────────────────
```

Set `CLAUDE_OPENCODE_QUIET=1` if you want to hide the panel.

---

## Daily Usage

Use Claude Code as usual:

```bash
claude
claude -p "Reply exactly: pong"
claude --version
```

Model switching inside Claude Code:

```text
/model                  # shows auto-discovered OpenCode Go models
/model deepseek-v4-pro  # switch model for this session
```

Management commands:

```bash
claude setup
claude doctor
claude status
claude logs
claude logs --follow
claude traces
claude traces --errors
claude traces --slow
claude traces --cost
claude trace <trace-id>
claude update
claude models --test
claude --api sk-your-opencode-go-key
claude --model
claude --model deepseek-v4-pro
```

The API key and preferred model are stored in:

```text
~/.config/claude-opencode-proxy/config.json
```

---

## Configuration

Proxy config:

```json
{
  "apiKey": "sk-...",
  "model": "deepseek-v4-pro",
  "proxy": {
    "host": "127.0.0.1",
    "port": 8082
  }
}
```

Claude settings are kept generic and point Claude Code at the local adapter:

```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "http://127.0.0.1:8082",
    "ANTHROPIC_API_KEY": "claude-opencode-local-proxy"
  }
}
```

---

## Project Layout

```text
bin/claude-opencode              Claude wrapper and management commands
proxy/anthropic2openai-proxy.mjs Anthropic Messages ↔ OpenAI Chat Completions adapter
scripts/bootstrap.sh             curl | bash installer
scripts/install.sh               local installer
scripts/uninstall.sh             restore direct Claude Code symlink
VERSION                          base release version
```

---

## CLI Commands

| Command | Purpose |
| --- | --- |
| `claude setup` | Interactive API key setup wizard |
| `claude doctor` | Checks dependencies, config, key, and proxy state |
| `claude status` | Shows release, proxy, config, log and running PIDs |
| `claude logs` | Shows the latest proxy logs |
| `claude logs --follow` | Follows proxy logs live |
| `claude traces` | Shows recent request traces |
| `claude traces --errors` | Shows recent failed traces |
| `claude traces --slow` | Shows slowest traces |
| `claude traces --cost` | Shows highest token/cost traces |
| `claude trace <trace-id>` | Shows one trace as JSON |
| `claude update` | Re-runs the GitHub one-line installer in-place |
| `claude models --test` | Pings every OpenCode Go model with a small request |
| `claude --model` | Shows available OpenCode Go models |
| `claude --model <model>` | Stores the preferred OpenCode Go model |
| `claude --api <key>` | Stores the OpenCode Go API key |

Normal `claude` launches enable all Claude Code native tools with `--dangerously-skip-permissions` for a smooth experience.

---

## How It Works

The proxy translates between two API formats transparently:

**Anthropic Messages → OpenAI Chat Completions** (outgoing):
- Messages with content blocks (text, tool_use, tool_result) are translated to OpenAI roles
- Tool definitions are mapped to OpenAI function format
- System prompts are passed through

**OpenAI Chat Completions → Anthropic Messages** (incoming):
- Text responses and tool_calls are translated back to Anthropic content blocks
- SSE streaming chunks are converted between the two event formats
- Stop reasons and usage data are remapped

The proxy is tool-agnostic — it translates format, not logic. Claude Code provides the tool definitions and executes tools locally. The model decides which tool to call.

---

## Observability

Every proxy call writes a JSONL trace with:

- trace id
- requested and final model
- latency
- retry count
- failover path
- upstream status
- token usage when the provider returns it
- redacted error text

Trace log:

```text
~/.cache/claude-opencode-proxy/traces.jsonl
```

Inspect traces:

```bash
claude traces
claude traces 50
claude traces --errors
claude traces --slow 10
claude traces --cost 10
claude trace <trace-id>
```

The proxy retries transient upstream failures and can fail over from the primary model to:

```text
glm-5.1, kimi-k2.6, minimax-m2.7, qwen3.6-plus
```

Override fallback order:

```bash
CLAUDE_OPENCODE_FALLBACK_MODELS=glm-5.1,kimi-k2.6
```

---

## Environment Variables

| Variable | Default | Purpose |
| --- | --- | --- |
| `PROXY_PORT` | `8082` | Port the proxy listens on |
| `ANTHROPIC_API_KEY` | (required) | OpenCode Go API key |
| `OPENCODE_GO_MODEL` | `deepseek-v4-pro` | Default model |
| `CLAUDE_OPENCODE_RETRY_ATTEMPTS` | `2` | Max retry attempts per model |
| `CLAUDE_OPENCODE_RETRY_BASE_MS` | `350` | Base backoff delay (ms) |
| `CLAUDE_OPENCODE_FALLBACK_MODELS` | `glm-5.1,kimi-k2.6,minimax-m2.7,qwen3.6-plus` | Failover chain |
| `CLAUDE_OPENCODE_QUIET` | `0` | Suppress startup status panel |
| `CLAUDE_OPENCODE_LOG` | `~/.cache/claude-opencode-proxy/proxy.log` | Proxy log path |
| `CLAUDE_OPENCODE_TRACE_LOG` | `~/.cache/claude-opencode-proxy/traces.jsonl` | Trace log path |
| `CLAUDE_OPENCODE_CONFIG` | `~/.config/claude-opencode-proxy/config.json` | Config file path |

---

## Releases

Every push to `main` creates a GitHub release.

Release tags use:

```text
v<VERSION>-<short-commit-sha>
```

Example:

```text
v0.8.0-46bb486
```

The base version lives in:

```text
VERSION
```

---

## Troubleshooting

Check the wrapper:

```bash
type -a claude
claude --version
claude doctor
claude status
```

Check models:

```bash
claude --api sk-your-opencode-go-key
claude --model
```

Check proxy logs:

```bash
claude logs
claude logs --follow
tail -f ~/.cache/claude-opencode-proxy/proxy.log
```

Check port cleanup:

```bash
ss -ltnp 'sport = :8082'
```

Restore the direct Claude Code symlink:

```bash
~/.local/share/claude-opencode-proxy-service/scripts/uninstall.sh
```

---

## Security Notes

- API keys are stored in `~/.config/claude-opencode-proxy/config.json`.
- `~/.claude/settings.json` only points Claude Code at the local proxy.
- API keys are not stored in this repository.
- API keys are not printed by the installer.
- The proxy listens only on `127.0.0.1`.
