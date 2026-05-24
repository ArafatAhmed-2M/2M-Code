<div align="center">
<pre>
██████╗ ███╗   ███╗     ██████╗ ██████╗ ██████╗ ███████╗
╚════██╗████╗ ████║    ██╔════╝██╔═══██╗██╔══██╗██╔════╝
 █████╔╝██╔████╔██║    ██║     ██║   ██║██║  ██║█████╗  
██╔═══╝ ██║╚██╔╝██║    ██║     ██║   ██║██║  ██║██╔══╝  
███████╗██║ ╚═╝ ██║    ╚██████╗╚██████╔╝██████╔╝███████╗
╚══════╝╚═╝     ╚═╝     ╚═════╝ ╚═════╝ ╚═════╝ ╚══════╝
</pre>
</div>

# 2M Code — Product Requirements Document
**Version:** 2.0.0  
**Status:** Active  
**Codename:** Multi-Mind  

---

## 1. Executive Summary

2M Code is an open-source, terminal-native AI coding platform built around a single breakthrough idea: instead of one AI assistant helping you code, you deploy a **team of AI agents** — each with a distinct role, model, and provider — that collaborate, debate, and build together like a real engineering team.

The "2M" stands for **Multi-Mind**: the belief that the best software emerges from multiple perspectives in dialogue, not a single oracle.

Unlike Claude Code, Gemini CLI, or OpenAI Codex CLI — which each expose a single model through a single provider — 2M Code lets you mix Anthropic, Google, OpenAI, OpenAI-Compatible (DeepSeek, Together, xAI, etc.), Mistral, Cohere, Groq, Ollama, OpenRouter, and any compatible provider into one coherent team. A Planner powered by Gemini can hand off to a Coder on Claude, reviewed by a QA agent on GPT-4o. Every agent sees the team's shared conversation, just like a real Slack channel.

**Target users:** Solo developers and engineering teams who want leverage beyond a single AI model.

---

## 2. Problem Statement

### The single-model ceiling
Every current AI coding tool assumes one model is enough. In practice:
- Different models have different strengths (Gemini for planning, Claude for reasoning, GPT-4o for speed)
- No single model has all the context a team needs
- There is no "review" loop — one AI both writes and judges its own output
- Users are locked into a single provider's pricing and availability

### The "one brain" problem
Real engineering teams work better than any individual engineer. Code review catches bugs. Architecture discussions surface tradeoffs. A QA mindset finds edge cases a developer misses. Current AI tools skip all of this. There is no debate, no second opinion, no handoff.

### 2M Code's answer
A configurable team of agents — each specialized, each from the best model for that role — that genuinely collaborate on your codebase, producing output that has been planned, implemented, and reviewed before it reaches you.

---

## 3. Goals & Non-Goals

### Goals (v1) — Foundation
- Ship a working CLI tool installable as a single binary
- Support agent teams defined in YAML (name, role, provider, model, system prompt)
- Support providers: Anthropic Claude, Google Gemini, OpenAI GPT, OpenAI-Compatible (DeepSeek, Together, xAI, etc.), Mistral, Cohere, Groq, Ollama, OpenRouter
- Implement a shared event bus so agents read each other's messages
- Implement turn-based orchestration (leader-first, then round-robin, then reviewer)
- Support file reading/writing and bash execution as agent tools
- Ship a `2m new-team` interactive wizard for team creation
- Ship a `2m run <team> "<task>"` command for one-shot task execution
- Ship a `2m chat <team>` interactive REPL

### Goals (v2) — Production Features
- **Streaming token output** — SSE streaming from Python engine to CLI for real-time text rendering
- **Cost tracking & budgets** — Token counting per run, cost estimation, `max_tokens_per_run` budget enforcement
- **Custom tools** — User-defined tools in team YAML that run arbitrary bash commands with env-var parameters
- **Persistent memory** — LLM-based session summarization using OpenRouter qwen model, saved to `~/.2mcode/memory/`, injected into future agent prompts
- **Agent parallelism** — Multiple agents running simultaneously (in progress)

### Non-Goals (v1 + v2)
- Web UI or browser interface (v3)
- Voice interface (v3)
- Fine-tuned or self-hosted models
- Plugin marketplace (v3)

---

## 4. User Personas

### Persona A — The Solo Builder (Maya)
Maya is a solo founder building a SaaS product. She uses Claude Code daily but hits a ceiling: one model, one perspective, no review loop. She wants a "virtual team" she can configure once and reuse across projects. She cares deeply about output quality and is willing to pay for multiple API keys if the result is better code.

**Key need:** Set up a team once, run tasks without babysitting every decision.

### Persona B — The Engineering Lead (Reza)
Reza leads a 6-person startup team. He wants to standardize AI tooling across the team, ensure code goes through a review step before it lands, and be able to swap models as prices or capabilities change. He wants team configs to live in the repo so everyone uses the same setup.

**Key need:** Shareable, version-controlled team configs. Consistent review gates.

---

## 5. Core Concepts

### 5.1 Agent
An agent is a named, role-bearing LLM instance. Each agent has:
- **Name** — e.g. "Aria", "Dev", "Quinn"
- **Role** — a short description used in rendering, e.g. "Tech Lead"
    - **Provider** — anthropic | google | openai | openai_compatible | mistral | cohere | groq | ollama | openrouter
- **Model** — e.g. "claude-opus-4-5", "gemini-1.5-pro", "gpt-4o"
- **System prompt** — the agent's identity, responsibilities, and communication style
- **Max context** — how many recent messages from the team channel it reads

### 5.2 Team
A team is a YAML file defining a group of agents and their workflow. Teams live in `~/.2mcode/teams/` or in a local `.2mcode/` directory in the project repo. A team specifies:
- Which agents exist and in what order they speak
- Orchestration mode (leader-first, round-robin, free)
- Turns per task
- Tools available (bash, file read, file write, web fetch)

### 5.3 Team Channel (Event Bus)
All agent messages — including the user's task — are stored in a shared SQLite database called the Team Channel. Every agent reads the last N messages as their conversation history before generating a response. This is what makes agents "see" each other's work. The team channel is the core innovation of 2M Code.

### 5.4 Orchestrator
The orchestrator is a Go process that manages turn order. In **leader-first** mode (default): the leader agent speaks first, then workers take turns, then the reviewer speaks last. In **round-robin** mode: agents take equal turns. The orchestrator writes each agent's response to the team channel and renders it to the CLI in real time.

### 5.5 Task
A task is the user's instruction — e.g. "Build a REST API for user authentication with JWT". The task is injected into the team channel as a user message. Agents then respond to it collaboratively.

---

## 6. User Stories

| ID | As a... | I want to... | So that... |
|----|---------|-------------|------------|
| US-01 | Developer | Install 2M Code with a single command | I can start using it immediately |
| US-02 | Developer | Run `2m new-team` to create a team via wizard | I don't have to write YAML by hand |
| US-03 | Developer | Run `2m run <team> "<task>"` | I can delegate a task to my agent team |
| US-04 | Developer | Watch agents respond token-by-token in real time | I can follow the team's reasoning as it happens |
| US-05 | Developer | Use agents from different providers in one team | I get the best model for each role |
| US-06 | Developer | Store team configs in my repo's `.2mcode/` folder | My team uses the same AI team setup |
| US-07 | Developer | Run `2m chat <team>` for an interactive session | I can have an ongoing dialogue with my agent team |
| US-08 | Developer | Give agents access to bash and file tools | Agents can read my codebase and write actual code |
| US-09 | Developer | See each agent's name and role color-coded in output | I always know who is speaking |
| US-10 | Developer | Set API keys per provider via env vars or config | I control my own credentials |
| US-11 | Developer | Define custom tools in team YAML | My agents can run project-specific commands |
| US-12 | Developer | Set a token budget for each run | I control costs and prevent runaway usage |
| US-13 | Developer | Have agents remember past sessions | I don't have to repeat context every time |

---

## 7. Functional Requirements

### 7.1 CLI Commands

```
2m new-team              Interactive wizard to create a team YAML
2m team list             List all configured teams
2m team show <name>      Show team config details
2m run <team> "<task>"   Run a one-shot task with a team (streaming output)
2m chat <team>           Start an interactive REPL with a team (streaming output)
2m history <team>        Show last session's team channel log
2m config set <key>      Set global config (default provider, etc.)
2m models [provider]     List available models from all providers
```

### 7.2 Team YAML Schema

```yaml
name: string                    # unique identifier
description: string             # human-readable
version: "2.0"

agents:
  - name: string                # display name
    role: string                # role label (e.g. "Tech Lead")
    provider: anthropic|google|openai|mistral|cohere|groq|ollama|openrouter
    model: string               # provider-specific model ID
    system_prompt: string       # full role prompt
    max_context: int            # messages from team channel (default: 20)
    color: string               # terminal color (red|yellow|green|blue|cyan|magenta)
    tools: [bash, read_file, write_file, web_fetch]

workflow:
  orchestration: leader_first|round_robin|free
  turns_per_task: int           # rounds of agent turns per task
  leader: string                # agent name (required for leader_first)
  reviewer: string              # agent name (optional, always speaks last)
  max_tokens_per_turn: int      # default 4096

# V2: Custom tool definitions (optional)
custom_tools:
  - name: string                # tool name used in agent tool calls
    description: string         # shown to LLM to explain when/how to use
    command: string             # bash command template with {param} placeholders
    input_schema:
      type: object
      properties:
        param_name:
          type: string
          description: "..."
      required: [param_name]
```

### 7.3 Provider Support

| Provider | Auth | Models |
|---|---|---|
| Anthropic | `ANTHROPIC_API_KEY` | claude-opus-4-5, claude-sonnet-4-6, claude-haiku-4-5 |
| Google | `GOOGLE_API_KEY` | gemini-1.5-pro, gemini-1.5-flash, gemini-2.0-flash |
| OpenAI | `OPENAI_API_KEY` | gpt-4o, gpt-4o-mini, o1-preview |
| OpenAI-Compatible | `OPENAI_COMPATIBLE_API_KEY` + `base_url` in YAML (or `OPENAI_COMPATIBLE_BASE_URL`) | DeepSeek, Together, xAI/Grok, Perplexity, Fireworks, GitHub Models, etc. |
| Mistral | `MISTRAL_API_KEY` | mistral-large, codestral |
| Cohere | `COHERE_API_KEY` | command-r-plus, command-r |
| Groq | `GROQ_API_KEY` | llama3-70b-8192, mixtral-8x7b-32768 |
| OpenRouter | `OPENROUTER_API_KEY` | 200+ models via unified API |
| Ollama | *None (local)* | llama3, mistral, codellama |

### 7.4 Agent Tools

**bash** — Execute shell commands. Returns stdout + stderr. Timeout 30s.  
**read_file** — Read a file from the project directory. Max 100KB.  
**write_file** — Write or overwrite a file. Requires user confirmation if file exists.  
**web_fetch** — Fetch a URL. Returns text content. Max 50KB.  
**custom_tools** — (V2) User-defined tools in team YAML. Runs arbitrary bash commands with `{param}` placeholders replaced by LLM-provided values. Parameters are also passed as uppercase env vars (e.g. `PATHS` for `paths`).

Tools are opt-in per agent in the team YAML. The orchestrator handles tool execution in Go and injects results back into the agent's turn. Custom tools use a dedicated tool-use loop that substitutes parameters into the command template and executes via bash.

### 7.5 Team Channel (Event Bus) Specification

Stored in `~/.2mcode/sessions/<team>/<session-id>.db` (SQLite).

```sql
messages (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  session_id    TEXT NOT NULL,
  agent_name    TEXT NOT NULL,    -- "user" for user input
  role          TEXT NOT NULL,    -- user | assistant
  content       TEXT NOT NULL,
  tool_calls    TEXT,             -- JSON, nullable
  created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
)
```

Each agent's API call receives the last `max_context` messages from this table as its conversation history. The system prompt is prepended per agent. This means all agents share context but each has their own identity injected.

### 7.6 Rendering

Agent responses stream to the terminal with:
- Color-coded agent name badge: `[Aria · Tech Lead]`
- Streaming token output beneath the badge
- Separator line between agent turns
- Tool call display: `⚙ running bash: go test ./...`
- Tool result display (collapsible in future versions)
- Final summary line: `✓ Team completed task in 4 turns · 3,241 tokens`

---

## 8. Non-Functional Requirements

| Requirement | Target |
|---|---|
| CLI startup time | < 200ms |
| First token to screen | < 2s (network dependent) |
| Binary size | < 20MB |
| Platform support | macOS, Linux, Windows (WSL) |
| Go version | 1.22+ |
| Python version | 3.11+ (agent engine) |
| SQLite | Bundled (no external dependency) |
| Config file location | `~/.2mcode/config.yaml` |

---

## 9. System Architecture

```
User Terminal
     │
     ▼
CLI Shell (Go / Cobra)
     │
     ▼
Orchestrator (Go)
  ├── Turn Scheduler
  ├── Event Bus (SQLite)
  └── Tool Runner
     │
     ▼
Agent Engine (Python / FastAPI — localhost:8765)
  ├── Provider: Anthropic
  ├── Provider: Google Gemini
  ├── Provider: OpenAI
  ├── Provider: OpenAI-Compatible (DeepSeek, Together, xAI, etc.)
  ├── Provider: Mistral
  ├── Provider: Cohere
  ├── Provider: Groq
  ├── Provider: Ollama
  └── Provider: OpenRouter
     │
     ▼
External LLM APIs
```

Communication between Go (orchestrator) and Python (agent engine) is over a local Unix socket HTTP server. The Python process is spawned by the Go binary on startup and killed on exit. Users do not manage it directly.

---

## 10. Security & Privacy

- API keys are read from environment variables or `~/.2mcode/config.yaml` (file permissions 600)
- Keys are never written to the team channel / session database
- No telemetry is collected by default
- All LLM calls go directly from the user's machine to the provider's API
- Tool execution (bash) runs as the user's own process with no privilege escalation
- `write_file` tool requires explicit user confirmation when overwriting

---

## 11. Milestones

### v1 — Foundation
| Milestone | Scope | Target |
|---|---|---|
| M0 — Foundation | Repo scaffold, Go CLI skeleton, Python FastAPI server, single-agent call working end-to-end | Week 1 |
| M1 — Team Channel | SQLite event bus, two agents taking turns, shared context | Week 2 |
| M2 — Team Config | YAML loader, `new-team` wizard, team list/show commands | Week 3 |
| M3 — Tools | bash, read_file, write_file tools with tool-use loop | Week 4 |
| M4 — Polish | Streaming render, colors, `2m chat`, `2m history`, error handling | Week 5 |
| M5 — Release | Binary packaging, install script, README, docs site | Week 6 |

### v2 — Production Features
| Milestone | Scope | Target |
|---|---|---|
| M6 — Custom Tools | User-defined tools in team YAML with bash execution | Complete |
| M7 — Cost Tracking | Token counting, pricing table, budget enforcement | Complete |
| M8 — Streaming | SSE streaming from Python engine, real-time token rendering | Complete |
| M9 — Memory | Persistent session memory with LLM summarization | Complete |
| M10 — Parallelism | Simultaneous agent turns | In progress |

### v3 — Extensibility & Integration
| Milestone | Scope | Target |
|---|---|---|
| M11 — Plugin System | Python-based plugin system with lifecycle hooks for custom tools, agent behaviors, and CLI commands | Complete |
| M12 — GitHub Integration | Auto-review PRs, run on push via webhooks, comment with review results | Planned |
| M13 — Feedback Loops | Agents review each other's work, structured feedback injected into future turns | Planned |
| M14 — Web Dashboard | Read-only web UI for monitoring sessions, viewing logs, cost tracking | Planned |
| M15 — V2 Gap Closure | Tests, `2m history` command, `web_fetch` tool fix, streaming renderer fix, chat budget enforcement | Planned |

### v4 — Enterprise & Collaboration
| Milestone | Scope |
|---|---|
| M16 — Multi-User | Team members share the same team channel, see each other's messages |
| M17 — Team Management | Invite users, roles, access control for teams |
| M18 — Audit Logs | Every agent action logged with timestamp and user identity |
| M19 — Agent Personas | Agents persist their history and learn user preferences across projects |
| M20 — Analytics | Usage dashboards, cost breakdowns, model performance metrics |

### v5 — Autonomous & Intelligent
| Milestone | Scope |
|---|---|
| M21 — Autonomous Mode | Agents proactively suggest tasks, start work without explicit prompts |
| M22 — Cross-Project Memory | Agents transfer learning and context between different projects |
| M23 — NL Workflow Builder | Describe team structure in plain English, auto-generate YAML |
| M24 — Self-Hosted Models | Deep integration with local model serving (vLLM, TGI, Ollama advanced) |
| M25 — Real-Time Collab | Multiple users simultaneously interacting with the same agent team |

---

## 12. Success Metrics (v1)

- GitHub stars: 500 in first 30 days
- Successful team runs (tracked via opt-in telemetry): 1,000 in first 30 days
- Time from install to first team run: < 5 minutes (measured via docs funnel)
- P1 bug rate: < 2 open P1 bugs at any time post-launch
