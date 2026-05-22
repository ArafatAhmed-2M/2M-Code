# 2M Code — The AI coding platform that thinks in teams

> **Multi-Mind:** Instead of one AI assistant, deploy a team of AI agents that plan, implement, and review code together — each from the best model for the job.

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev)
[![Python Version](https://img.shields.io/badge/Python-3.11+-3776AB?logo=python)](https://python.org)

---

## What is 2M Code?

Every current AI coding tool gives you one model, one perspective, one brain. But real engineering teams work differently — they plan, implement, review, and iterate. A tech lead breaks down the problem. A senior engineer builds the solution. A QA engineer catches bugs before they ship. **2M Code brings this dynamic to AI.**

With 2M Code, you define a **team** of AI agents in a simple YAML file. Each agent has a name, a role, a provider (Anthropic, Google, OpenAI, Mistral), and a system prompt that defines their personality and expertise. When you give the team a task, they collaborate through a shared conversation channel — each agent sees what the others have said, builds on their work, and contributes their unique perspective. The result? Code that has been planned, implemented, *and* reviewed before it reaches you.

---

## Installation

### Quick Install (macOS/Linux)

```bash
curl -sSL https://raw.githubusercontent.com/2mcode/2mcode/main/scripts/install.sh | bash
```

### Manual Install

```bash
# 1. Clone the repository
git clone https://github.com/2mcode/2mcode.git
cd 2mcode

# 2. Install Python dependencies
pip install -r requirements.txt

# 3. Build the Go binary
go build -o bin/2m ./cmd/2m

# 4. Add to PATH
export PATH="$PATH:$(pwd)/bin"

# 5. Set up API keys (at least one provider)
export ANTHROPIC_API_KEY="your-key"
export GOOGLE_API_KEY="your-key"
export OPENAI_API_KEY="your-key"
```

### Requirements

- **Go 1.22+** — [Install Go](https://go.dev/dl/)
- **Python 3.11+** — [Install Python](https://python.org/downloads/)
- **API key** for at least one provider (Anthropic, Google, OpenAI, or Mistral)

---

## Quick Start

### 1. Create a team

```bash
2m new-team
```

This launches an interactive wizard that walks you through creating a team — naming agents, assigning roles, choosing providers, and setting the workflow.

### 2. Run a task

```bash
2m run fullstack "Build a REST API for user authentication with JWT"
```

Watch your team collaborate in real time:

```
╭─ Aria · Tech Lead ────────────────────────
│ I'll break this into three subtasks:
│ 1. Database schema for users table
│ 2. Auth endpoints (register, login, refresh)
│ 3. JWT middleware for protected routes
╰──────────────────────────────────────────

╭─ Dev · Senior Engineer ───────────────────
│ Starting with the database schema...
│ ⚙ running bash: mkdir -p internal/auth
│ └ [created directory]
│ ...
╰──────────────────────────────────────────

╭─ Quinn · QA Engineer ────────────────────
│ Code Review Results:
│ ✓ Auth flow is solid
│ ⚠ Warning: Add rate limiting to login endpoint
│ ⚠ Warning: JWT secret should use env var, not hardcoded
╰──────────────────────────────────────────

✓ Team completed task in 4 turns · 3,241 tokens · 12.3s
```

### 3. Interactive chat

```bash
2m chat fullstack
```

Opens a REPL where you can have an ongoing conversation with your team.

---

## Team Configuration

Teams are defined in YAML. Here's a complete example:

```yaml
name: fullstack
description: "A full-stack web development team"
version: "1.0"

agents:
  - name: Aria
    role: Tech Lead
    provider: anthropic          # Uses Claude
    model: claude-opus-4-5
    color: cyan
    max_context: 20              # Last 20 messages as context
    tools: [bash, read_file, write_file]
    system_prompt: |
      You are Aria, the Tech Lead. Break down tasks, set architecture
      direction, and coordinate the team...

  - name: Dev
    role: Senior Engineer
    provider: google             # Uses Gemini
    model: gemini-1.5-pro
    color: green
    tools: [bash, read_file, write_file]
    system_prompt: |
      You are Dev, the Senior Engineer. Implement features based on
      the tech lead's plan...

  - name: Quinn
    role: QA Engineer
    provider: openai             # Uses GPT-4o
    model: gpt-4o
    color: yellow
    tools: [bash, read_file]     # QA doesn't write files
    system_prompt: |
      You are Quinn, the QA Engineer. Review all code for bugs,
      security issues, and quality...

workflow:
  orchestration: leader_first    # Aria speaks first
  turns_per_task: 1              # One round of turns per task
  leader: Aria                   # Leader agent
  reviewer: Quinn                # Reviewer speaks last
  max_tokens_per_turn: 4096
```

Teams can be stored in:
- **Project-local:** `./.2mcode/teams/` — shared via version control
- **Global:** `~/.2mcode/teams/` — personal teams
- **Bundled:** `config/teams/` — example teams included with 2M Code

---

## Supported Providers

| Provider | Available Models (Examples) | Required Env Var | Notes |
|---|---|---|---|
| **Anthropic** | `claude-3.5-sonnet`, `claude-3-opus` | `ANTHROPIC_API_KEY` | Best for complex reasoning and lead roles. |
| **Google** | `gemini-1.5-pro`, `gemini-2.0-flash` | `GOOGLE_API_KEY` | Massive context window (up to 2M tokens). |
| **OpenAI** | `gpt-4o`, `o1-preview` | `OPENAI_API_KEY` | Strong all-rounder. |
| **Mistral** | `mistral-large`, `codestral` | `MISTRAL_API_KEY` | Excellent code-specific models. |
| **Cohere** | `command-r-plus`, `command-r` | `COHERE_API_KEY` | Strong tool-use and RAG capabilities. |
| **Groq** | `llama3-70b-8192`, `mixtral-8x7b-32768` | `GROQ_API_KEY` | Ultra-fast LPU inference (500+ tokens/sec). |
| **OpenRouter**| `anthropic/claude-3.5-sonnet`, etc. | `OPENROUTER_API_KEY` | Unified API for 200+ models. |
| **Ollama** | `llama3`, `mistral`, `codellama` | *None* | Runs locally and privately. Connects to `localhost:11434` |

---

## How It Works

1. **Shared Team Channel.** All agent messages are stored in a SQLite database. Each agent reads the last N messages as context before responding — so they genuinely see each other's work, like a shared Slack channel.

2. **Turn-Based Orchestration.** Agents take turns in a defined order. In `leader_first` mode, the leader speaks first (usually the planner), then workers implement, then the reviewer gives final feedback.

3. **Tool Access.** Agents can run bash commands, read files, and write files — just like you do when coding. The orchestrator handles tool execution and feeds results back to the agent.

---

## CLI Commands

```
2m new-team              Create a new team interactively
2m team list             List all configured teams
2m team show <name>      Show team config details
2m run <team> "<task>"   Run a one-shot task with a team
2m chat <team>           Start an interactive REPL with a team
2m history <team>        Show last session's team channel log
2m config set <key>      Set global config values
```

---

## Roadmap

### v1 (Current)
- ✅ Multi-provider agent teams (Anthropic, Google, OpenAI, Mistral)
- ✅ YAML team configuration
- ✅ Shared team channel (SQLite event bus)
- ✅ Leader-first and round-robin orchestration
- ✅ Tool support: bash, file read/write, web fetch
- ✅ Interactive chat REPL
- ✅ Team creation wizard

### v2 (Planned)
- 🔲 Agent parallelism (simultaneous turns)
- 🔲 Persistent memory across sessions
- 🔲 Streaming token output
- 🔲 Cost tracking and budgets per team run
- 🔲 Custom tool definitions in team YAML

### v3 (Future)
- 🔲 Web dashboard for team monitoring
- 🔲 Plugin/extension system
- 🔲 Agent self-improvement via feedback loops
- 🔲 Integration with GitHub PRs and CI/CD

---

## Contributing

We welcome contributions! Here's how to get started:

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Make your changes and add tests
4. Run the test suite: `make test`
5. Submit a pull request

### Development Setup

```bash
git clone https://github.com/2mcode/2mcode.git
cd 2mcode
pip install -r requirements.txt
make build
make run-dev ARGS="team list"
```

### Code Standards

- **Go:** `go vet ./...` must pass with no warnings
- **Python:** PEP 8 compliant (use `black` formatter)
- All public functions must have documentation comments
- No hardcoded API keys anywhere in source code
- Error messages must be actionable (what went wrong + what to do)

---

## License

MIT License — see [LICENSE](LICENSE) for details.

---

<p align="center">
  <strong>2M Code</strong> — Because the best code comes from multiple minds.<br>
  Built with ❤️ by the 2M Code team.
</p>
