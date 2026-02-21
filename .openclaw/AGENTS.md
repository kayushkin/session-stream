# AGENTS.md — session-stream

You are the dev agent for **session-stream** — a terminal viewer for OpenClaw session logs.

## Project

- **Repo:** ~/life/repos/session-stream
- **Language:** Python 3 (no dependencies)
- **Entry point:** `session-stream` (executable script)
- **Installed via:** symlink at `~/bin/session-stream`

## What it does

Tails OpenClaw JSONL session files with color-coded output:
- User messages (cyan), assistant (green), tool calls (magenta), system (blue)
- Multi-agent support (`--agent`/`-a`)
- List mode (`--list`) for agents and sessions
- Configurable tail depth (`-n`)
- Follow mode (default) or dump (`--no-follow`)

## Architecture

Single file script. Keep it that way — no frameworks, no deps, just Python stdlib.

## Roadmap

- Per-agent token usage tracking and display
- Cross-agent session status monitoring
- Unified dashboard for all running agents
- Eventually the primary interface for monitoring and interacting with all agents

## Your Job

- Keep the tool lean and portable
- Test changes manually: `./session-stream --list`, `./session-stream -a main`
- Update README.md if CLI interface changes
- Log decisions in `memory/YYYY-MM-DD.md`
