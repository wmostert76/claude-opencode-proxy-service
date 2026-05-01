<div align="center">

# Claude Go

**Use Claude Code with OpenCode Go models — all 14 of them.**

*Claude-Go is an idea by WAM-Software since 1997. Code provided by OpenCode and DeepSeek V4 Pro.*

[![Release](https://img.shields.io/github/v/release/wmostert76/claude-go?label=release)](https://github.com/wmostert76/claude-go/releases)
[![Go](https://img.shields.io/badge/go-1.23+-00ADD8)](https://go.dev)
[![Install](https://img.shields.io/badge/install-curl%20%7C%20bash-0a7)](#install)

</div>

---

## What is this?

You have an [OpenCode Go](https://opencode.ai/auth) subscription ($10/month) for 14 fast coding models. You love Claude Code's terminal workflow. But there's a problem: **only 2 of the 14 models** speak Anthropic Messages natively (MiniMax M2.5 and M2.7). The other 12 use OpenAI Chat Completions.

Claude Go is a single Go binary that translates between the two formats in real time:

```
Claude Code  ──Anthropic Messages──▶  Proxy (:8082)  ──OpenAI Chat──▶  OpenCode Go
```

| Without Claude Go | With Claude Go |
|---|---|
| 2 models usable | All 14 models usable |
| MiniMax only | DeepSeek, Kimi, GLM, MiniMax, Qwen, MiMo |

Every Claude Code tool works: file editing, shell commands, web fetching, sub-agents, tasks — across all models.

Claude Go installs Claude Code privately in its own directory. Your system's `claude` command (if any) is never touched.

---

## Install

**Linux / macOS:**
```bash
curl -fsSL https://raw.githubusercontent.com/wmostert76/claude-go/main/scripts/bootstrap.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/wmostert76/claude-go/main/scripts/bootstrap.ps1 | iex
```

Then:
```bash
claude-go install                # Downloads Claude Code locally (one time)
claude-go --api sk-xxx           # Store your OpenCode Go API key
claude-go                        # Start!
```

---

## Usage

```bash
claude-go                        # Start Claude Code
claude-go -p "Review auth.ts"    # One-shot prompt
claude-go --version              # Check version
```

Switch models anytime:
```bash
claude-go --model deepseek-v4-pro
claude-go --model kimi-k2.6
```
Or inside Claude Code: `/model deepseek-v4-pro`

---

## Commands

| Command | What it does |
|---|---|
| `claude-go` | Start Claude Code through the proxy |
| `claude-go --api <key>` | Store your OpenCode Go API key |
| `claude-go --model` | List available models |
| `claude-go --model <name>` | Set your default model |
| `claude-go --prompt <text>` | Set a persistent system prompt |
| `claude-go --prompt-clear` | Remove the system prompt |
| `claude-go setup` | Interactive setup wizard |
| `claude-go doctor` | Diagnose your setup |
| `claude-go status` | Show proxy status and config paths |
| `claude-go logs [--follow]` | View/watch proxy logs |
| `claude-go traces [--errors\|--slow\|--cost] [n]` | Show request traces |
| `claude-go trace <id>` | Full trace as JSON |
| `claude-go update` | Update to the latest version |
| `claude-go install` | Install/update private Claude Code |
| `claude-go uninstall` | Remove Claude Go |
| `claude-go models [--test]` | List or test all 14 models |
| `claude-go --completion <bash\|fish>` | Generate shell completion |
| `claude-go --version` | Print version |
| `claude-go --help` | Show help |

---

## Models

All 14 OpenCode Go models:

| Tier | Models |
|---|---|
| Best | deepseek-v4-pro |
| Strong | glm-5.1, kimi-k2.6, minimax-m2.7, qwen3.6-plus |
| Fast/Cheap | deepseek-v4-flash |
| Good | glm-5, kimi-k2.5, minimax-m2.5, qwen3.5-plus |
| Specialized | mimo-v2.5-pro, mimo-v2.5, mimo-v2-pro, mimo-v2-omni |

---

## How it works

Claude Go is a single Go binary. On start, it:

1. Starts an HTTP proxy on port 8082
2. Points `ANTHROPIC_BASE_URL` to `http://127.0.0.1:8082`
3. Launches Claude Code with the redirected endpoint

The proxy translates:
- **Messages** — Anthropic content blocks become OpenAI roles and tool messages
- **Tools** — Anthropic tool definitions become OpenAI function definitions
- **Streaming** — SSE chunks are translated between the two formats
- **Responses** — OpenAI completions become Anthropic message responses

Claude Code runs normally — it provides the tools and runs them locally. The proxy handles only the API format translation.

---

## Configuration

Everything lives in `~/.config/claude-go/config.json`:
```json
{
  "apiKey": "sk-...",
  "model": "deepseek-v4-pro",
  "proxy": { "host": "127.0.0.1", "port": 8082 }
}
```

Claude Code settings are managed automatically (`~/.claude/settings.json`):
```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "http://127.0.0.1:8082",
    "ANTHROPIC_API_KEY": "claude-go-local-proxy"
  }
}
```

Just set your key and you're done.

---

## Observability

Every request is traced:
```
~/.cache/claude-go/traces.jsonl
```

Each trace records: model requested, final model, response time, retries, failovers, token usage, errors (redacted).

Inspect with `claude-go traces`, `claude-go traces --errors`, or `claude-go trace <id>`.

The proxy retries transient failures and fails over to alternative models.

---

## Environment variables

| Variable | Default | Purpose |
|---|---|---|
| `CLAUDE_GO_CONFIG` | `~/.config/claude-go/config.json` | Config file path |
| `CLAUDE_GO_LOG` | `~/.cache/claude-go/proxy.log` | Proxy log path |
| `PROXY_PORT` | `8082` | Proxy listening port |
| `OPENCODE_GO_MODEL` | `deepseek-v4-pro` | Default model |
| `CLAUDE_GO_RETRY_ATTEMPTS` | `2` | Retries per model |
| `CLAUDE_GO_RETRY_BASE_MS` | `350` | Backoff delay |
| `CLAUDE_GO_FALLBACK_MODELS` | `glm-5.1,kimi-k2.6,minimax-m2.7,qwen3.6-plus` | Failover chain |
| `CLAUDE_GO_QUIET` | `0` | Hide startup panel |

---

## Self-updating

At every launch, Claude Go checks if a newer release is available on GitHub (cached for 6 hours). If your system has a global Claude Code installation (`npm -g @anthropic-ai/claude-code`), it syncs the latest version into Claude Go's private directory automatically.

```bash
claude-go update     # manual update
claude-go install    # reinstall Claude Code
```

---

## Troubleshooting

```bash
claude-go doctor           # Check everything
claude-go status           # Show config and proxy state
claude-go logs             # View logs
claude-go models --test    # Test all models
```

Check if the port is alive:
```bash
ss -ltnp 'sport = :8082'
```

Uninstall:
```bash
claude-go uninstall
```
This removes only the `claude-go` command. Your Claude Code and config are untouched.

---

## Requirements

- Go binary: none (static binary, no dependencies)
- Claude Code: Node.js 20+, npm (for `claude-go install` only)
- An [OpenCode Go](https://opencode.ai/auth) subscription

---

## FAQ

**Does this modify Claude Code?** No. Claude Go installs a private copy of Claude Code. Your system's `claude` command is never touched.

**Is my API key safe?** Yes. Stored locally in `~/.config/claude-go/config.json` with restricted permissions. Never committed, never sent anywhere but OpenCode Go.

**Can I still use Claude Code directly?** Yes — just use the `claude` command as normal. `claude-go` is completely independent.

**Do all Claude Code tools work?** Yes — Read, Write, Edit, Bash, WebFetch, WebSearch, Task, Agent, and all native tools work across every model.
