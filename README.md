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
- Creates a `claude-go` command that starts the proxy automatically

Then set your API key:

```bash
claude-go --api sk-your-opencode-go-key
```

Done. Use `claude-go` just like Claude Code.

---

## Usage

```bash
claude-go                       # Start Claude Code
claude-go -p "Review auth.ts"   # One-shot prompt
claude-go --version             # Check version
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
| `claude-go logs` | View recent proxy logs |
| `claude-go logs --follow` | Watch logs in real time |
| `claude-go traces` | Show recent request traces |
| `claude-go traces --errors` | Show failed requests |
| `claude-go traces --slow` | Show slowest requests |
| `claude-go traces --cost` | Show most expensive requests |
| `claude-go trace <id>` | Full trace for a specific request |
| `claude-go update` | Update to the latest version |
| `claude-go models --test` | Test connectivity to all 14 models |

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
1. The `claude-go` wrapper starts the proxy on port 8082
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

Inspect with `claude-go traces`, `claude-go traces --errors`, or `claude-go trace <id>`.

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
claude-go doctor           # Check everything
claude-go status           # Show config and proxy state
claude-go logs             # View logs
claude-go models --test    # Test all models
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
claude-go update
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
scripts/uninstall.sh               Remove claude-go symlink
VERSION                          Current version
```

---

## FAQ

**Does this modify Claude Code?** No. Claude Go installs a private copy of Claude Code inside its own directory. Your system's `claude` command (if installed) is never touched.

**Can I still use Claude Code directly?** Yes. Claude Go uses its own `claude-go` command. Any existing `claude` command continues to work directly with Anthropic's API.

**How do I uninstall?** Run `~/.local/share/claude-go/scripts/uninstall.sh`. This removes only the `claude-go` symlink. Your system `claude` command and Claude Code are untouched.

**Do all Claude Code tools work?** Yes — Read, Write, Edit, Bash, WebFetch, WebSearch, Task, Agent, and all native tools work across every model.
