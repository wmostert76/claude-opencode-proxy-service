# Claude OpenCode Proxy Service

Run Claude Code through OpenCode Go models by starting a local Anthropic-compatible proxy before every `claude` invocation.

The user-facing workflow stays the same:

```bash
claude
claude -p "Reply exactly: pong"
claude --model
claude --model deepseek-v4-pro
```

## What It Does

- Keeps `claude` as the command you type.
- Starts a local proxy on `127.0.0.1:8082`.
- Translates Claude Code's Anthropic Messages API calls to OpenCode Go's OpenAI-compatible chat-completions endpoint.
- Routes all configured models through OpenCode Go.
- Adds `claude --model` as an OpenCode Go model picker.
- Adds `claude --model <model>` to set the default model.
- Always starts Claude Code with `--allow-dangerously-skip-permissions`.

## Files

```text
bin/claude-opencode              Wrapper around Claude Code
proxy/anthropic2openai-proxy.mjs Anthropic -> OpenAI-compatible proxy
scripts/install.sh               Install wrapper as `claude`
scripts/uninstall.sh             Restore direct Claude Code symlink
```

## Requirements

- Node.js
- Python 3
- curl
- `@anthropic-ai/claude-code`
- `opencode-ai`
- OpenCode Go API key in `~/.claude/settings.json` under `env.ANTHROPIC_API_KEY`

## Install

One-click install:

```bash
curl -fsSL https://raw.githubusercontent.com/wmostert76/claude-opencode-proxy-service/main/scripts/bootstrap.sh | bash
```

This downloads the repo to:

```text
~/.local/share/claude-opencode-proxy-service
```

It also installs Claude Code with npm if it is not already installed.

Local install from a checkout:

```bash
./scripts/install.sh
```

The installer creates a `claude` wrapper in:

```text
~/.local/share/npm-global/bin/claude
```

The original Claude Code binary remains:

```text
~/.local/share/npm-global/lib/node_modules/@anthropic-ai/claude-code/bin/claude.exe
```

## Model Commands

Show OpenCode Go models, ranked:

```bash
claude --model
```

Set default:

```bash
claude --model deepseek-v4-pro
claude --model minimax-m2.7
```

Completion data:

```bash
claude --complete-models
claude --completion fish
claude --completion bash
```

Completion is not installed automatically by default.

## Notes

The proxy is intentionally always used. Direct Anthropic/Claude model routing is avoided.

DeepSeek V4 Pro is the default "best overall" model. MiniMax M2.7 is a practical fallback when Claude Code tool behavior is more important than raw reasoning quality.
