# session-stream

A terminal viewer for [OpenClaw](https://github.com/openclaw/openclaw) session logs. Tails JSONL session files with color-coded roles, tool calls, and timestamps.

## Install

```bash
# Clone and build
git clone https://github.com/kayushkin/session-stream
cd session-stream
go build
ln -sf "$(pwd)/session-stream" ~/bin/session-stream
```

Requires Go 1.16 or later. No external dependencies.

## Usage

```bash
# Stream latest session (default agent: main)
session-stream

# Stream a specific agent
session-stream --agent argraphments
session-stream -a work

# List agents and session counts
session-stream --list

# List sessions for an agent
session-stream --list --agent argraphments

# Stream a specific file
session-stream ~/.openclaw/agents/main/sessions/abc123.jsonl

# Dump last 50 messages and exit
session-stream -n 50 --no-follow
```

## What it shows

- **User messages** in cyan
- **Assistant messages** in green
- **Tool calls** with ⚡ in magenta
- **Tool results** dimmed
- **System messages** in blue
- Timestamps when available

## Environment

- `OPENCLAW_STATE_DIR` — override OpenClaw state directory (default: `~/.openclaw`)
