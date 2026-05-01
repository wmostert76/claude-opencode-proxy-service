<div align="center">

# Claude Go

**Use Claude Code with OpenCode Go models — all 14 of them.**

[![Shell](https://img.shields.io/badge/shell-bash%20%2B%20fish-4eaa25)](#shell-completion)
[![Node](https://img.shields.io/badge/node-%3E%3D20-339933)](#requirements)
[![Install](https://img.shields.io/badge/install-curl%20%7C%20bash-0a7)](#install)

</div>

---

## What is this?

You have an [OpenCode Go](https://opencode.ai/auth) subscription ($10/month) for 14 fast coding models. You love Claude Code's terminal workflow. But there's a problem: **only 2 of the 14 models** speak Anthropic Messages natively (MiniMax M2.5 and M2.7). The other 12 use OpenAI Chat Completions.

Claude Go fixes this with a local proxy that translates between the two formats in real time:

```
Claude Code  ──Anthropic Messages──▶  Proxy (:8082)  ──OpenAI Chat──▶  OpenCode Go
```

| Without Claude Go | With Claude Go |
|---|---|
| 2 models usable | All 14 models usable |
| MiniMax only | DeepSeek, Kimi, GLM, MiniMax, Qwen, MiMo |

Every Claude Code tool works: file editing, shell commands, web fetching, sub-agents, tasks — across all models.

---

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/wmostert76/claude-go/main/scripts/bootstrap.sh | bash
```

This single command:
- Downloads Claude Go to `~/.local/share/claude-go`
- Installs Claude Code if you don't have it yet
- Creates a `claude` wrapper that starts the proxy automatically

Then set your API key:

```bash
claude --api sk-your-opencode-go-key
```

Done. Use Claude Code as normal.

---

## Usage

```bash
claude                         # Start Claude Code
claude -p "Review auth.ts"     # One-shot prompt
claude --version               # Check version
```

Switch models anytime:

```bash
claude --model deepseek-v4-pro
claude --model kimi-k2.6
```

Or inside Claude Code: `/model deepseek-v4-pro`

---

## Commands

| Command | What it does |
|---|---|
| `claude` | Start Claude Code through the proxy |
| `claude --api <key>` | Store your OpenCode Go API key |
| `claude --model` | List available models |
| `claude --model <name>` | Set your default model |
| `claude --prompt <text>` | Set a persistent system prompt |
| `claude --prompt-clear` | Remove the system prompt |
| `claude setup` | Interactive setup wizard |
| `claude doctor` | Diagnose your setup |
| `claude status` | Show proxy status and config paths |
| `claude logs` | View recent proxy logs |
| `claude logs --follow` | Watch logs in real time |
| `claude traces` | Show recent request traces |
| `claude traces --errors` | Show failed requests |
| `claude traces --slow` | Show slowest requests |
| `claude traces --cost` | Show most expensive requests |
| `claude trace <id>` | Full trace for a specific request |
| `claude update` | Update to the latest version |
| `claude models --test` | Test connectivity to all 14 models |

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

Claude Code sends Anthropic-formatted messages. The proxy translates:

- **Messages** — Anthropic content blocks become OpenAI roles and tool messages
- **Tools** — Anthropic tool definitions become OpenAI function definitions
- **Streaming** — SSE chunks are translated between the two formats
- **Responses** — OpenAI completions become Anthropic message responses

The proxy handles format translation only. Claude Code provides the tools and runs them locally. The model decides which tool to use.

Under the hood:
1. The `claude` wrapper starts the proxy on port 8082
2. `ANTHROPIC_BASE_URL` is set to the proxy
3. Claude Code talks to the proxy as if it were the Anthropic API
4. The proxy translates and forwards to OpenCode Go

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

```text
~/.cache/claude-go/traces.jsonl
```

Each trace records:
- Requested model and final model used
- Response time
- Retry count and failover path
- Token usage
- Error details (redacted)

Inspect with `claude traces`, `claude traces --errors`, or `claude trace <id>`.

The proxy retries transient failures and can fail over to alternative models.

---

## Environment variables

| Variable | Default | Purpose |
|---|---|---|
| `CLAUDE_GO_CONFIG` | `~/.config/claude-go/config.json` | Config file path |
| `CLAUDE_GO_LOG` | `~/.cache/claude-go/proxy.log` | Proxy log path |
| `PROXY_PORT` | `8082` | Proxy listening port |
| `OPENCODE_GO_MODEL` | `deepseek-v4-pro` | Default model |
| `CLAUDE_GO_RETRY_ATTEMPTS` | `2` | Retries per model |
| `CLAUDE_GO_RETRY_BASE_MS` | `350` | Backoff delay in ms |
| `CLAUDE_GO_FALLBACK_MODELS` | `glm-5.1,kimi-k2.6,minimax-m2.7,qwen3.6-plus` | Failover chain |
| `CLAUDE_GO_QUIET` | `0` | Hide startup panel |
| `CLAUDE_GO_DEBUG` | `0` | Verbose proxy output |

---

## Troubleshooting

```bash
claude doctor           # Check everything
claude status           # Show config and proxy state
claude logs             # View logs
claude models --test    # Test all models
```

Check if the port is alive:

```bash
ss -ltnp 'sport = :8082'
```

Restore direct Claude Code (without the proxy):

```bash
~/.local/share/claude-go/scripts/uninstall.sh
```

---

## Updating

```bash
claude update
```

---

## Requirements

- Node.js 20+
- npm
- Python 3
- curl
- An [OpenCode Go](https://opencode.ai/auth) subscription

---

## Project structure

```
bin/claude-opencode              Wrapper and management commands
proxy/anthropic2openai-proxy.mjs Protocol translator
scripts/bootstrap.sh             One-line installer
scripts/install.sh               Local setup
scripts/uninstall.sh             Restore direct Claude Code
VERSION                          Current version
```

---

## FAQ

**Does this modify Claude Code?** No. Claude Code is installed normally. The wrapper redirects its traffic through the proxy.

**Is my API key safe?** Yes. Stored locally in `~/.config/claude-go/config.json` with restricted permissions. Never committed or sent anywhere but OpenCode Go.

**Can I still use Claude Code directly?** Yes. Run `scripts/uninstall.sh` to restore the direct symlink.

**Do all Claude Code tools work?** Yes — Read, Write, Edit, Bash, WebFetch, WebSearch, Task, Agent, and all native tools work across every model.
