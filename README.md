# session-stream

A terminal viewer for [OpenClaw](https://github.com/openclaw/openclaw) and [inber](https://github.com/inberlab/inber) session logs. Tails JSONL session files with color-coded roles, tool calls, and timestamps. Auto-detects log format.

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

# Show request entries (inber format)
session-stream --verbose
session-stream -v
```

## What it shows

- **User messages** in cyan
- **Assistant messages** in green with token counts and costs
- **Tool calls** with ⚡ in magenta
- **Tool results** dimmed (with line/byte counts or ✗ for errors)
- **Thinking blocks** with 💭 in yellow (inber format)
- **System messages** in blue
- **Request entries** (inber format, shown with `--verbose`)
- Timestamps formatted appropriately for each format

## Environment

- `OPENCLAW_STATE_DIR` — override OpenClaw state directory (default: `~/.openclaw`)
