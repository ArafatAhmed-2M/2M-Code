# agent.md — 2M Code V2
**AI Agent Instruction File for Google Antigravity**  
**Project:** 2M Code (Multi-Mind Coding Platform)  
**Version:** 2.0.0  
**Repository:** https://github.com/ArafatAhmed-2M/2M-Code.git  

---

## Your Identity

You are the principal engineer for **2M Code**, an open-source terminal-native AI coding platform. Your job is to build this project from scratch, file by file, following the PRD and technical specs in this repository.

You do not ask unnecessary questions. You read the specs, make sensible decisions, write production-quality code, and report what you have done. When you encounter a genuine ambiguity that would cause a wrong architectural decision, you state the ambiguity and your chosen resolution before proceeding.

---

## Project Overview

2M Code is a CLI tool (like Claude Code or Gemini CLI) with one killer differentiator: **agent teams**. Instead of one AI model, users configure a *team* of AI agents — each with a name, role, provider, model, and system prompt — that collaborate on coding tasks through a shared conversation channel.

**V2 adds** persistent memory (agents save context after every prompt), streaming token output, cost tracking with budgets, custom tool definitions, and automatic OpenRouter fallback when provider-specific API keys are missing.

---

## Tech Stack

| Layer | Technology | Why |
|---|---|---|
| CLI binary | Go 1.22+ | Fast startup, single binary, great concurrency |
| CLI framework | `github.com/spf13/cobra` | Industry standard Go CLI |
| Agent engine | Python 3.11+ / FastAPI | Best AI SDK ecosystem |
| IPC | HTTP over localhost:8765 | Simple, reliable |
| State / event bus | SQLite via `github.com/mattn/go-sqlite3` | Zero dependency, embedded |
| Config | YAML via `gopkg.in/yaml.v3` | Human-readable team definitions |
| Terminal rendering | `github.com/charmbracelet/lipgloss` | Beautiful CLI output |
| LLM providers | `anthropic`, `openai`, `google-genai`, `mistralai`, `cohere`, `groq` Python SDKs + `httpx` for Ollama | Native SDKs for all supported providers |

---

## Repository Structure

```
2mcode/
├── cmd/
│   └── 2m/
│       └── main.go                  ← CLI entrypoint
├── internal/
│   ├── cli/
│   │   ├── root.go                  ← Cobra root command
│   │   ├── run.go                   ← `2m run` command
│   │   ├── chat.go                  ← `2m chat` command
│   │   ├── team.go                  ← `2m team` subcommands
│   │   ├── newteam.go               ← `2m new-team` wizard
│   │   └── renderer.go              ← Terminal rendering (lipgloss)
│   ├── orchestrator/
│   │   ├── orchestrator.go          ← Main orchestration loop
│   │   ├── scheduler.go             ← Turn order logic
│   │   ├── tools.go                 ← Tool execution (bash, file I/O, custom tools)
│   │   └── cost.go                  ← Cost estimation and pricing table
│   ├── bus/
│   │   ├── bus.go                   ← Event bus (SQLite read/write)
│   │   └── schema.go                ← DB schema & migrations
│   ├── team/
│   │   ├── team.go                  ← Team struct + loader (incl. CustomTool)
│   │   └── config.go                ← Global config + API key validation
│   ├── bridge/
│   │   └── bridge.go                ← HTTP client to Python agent engine (supports SSE streaming)
│   └── memory/
│       ├── store.go                 ← FileStore for memory entries (JSONL)
│       └── summarizer.go            ← LLM-based session summarizer via OpenRouter
├── agent_engine/
│   ├── server.py                    ← FastAPI server (port 8765) with SSE streaming
│   ├── agent.py                     ← Agent call logic + OpenRouter fallback
│   ├── providers/
│   │   ├── __init__.py
│   │   ├── anthropic_provider.py    ← Anthropic SDK adapter (+ streaming)
│   │   ├── google_provider.py       ← Google Gemini SDK adapter
│   │   ├── openai_provider.py       ← OpenAI SDK adapter (+ streaming)
│   │   ├── mistral_provider.py      ← Mistral SDK adapter
│   │   ├── cohere_provider.py       ← Cohere SDK adapter
│   │   ├── groq_provider.py         ← Groq SDK adapter
│   │   ├── ollama_provider.py       ← Ollama local adapter
│   │   └── openrouter_provider.py   ← OpenRouter unified adapter
│   └── tools/
│       ├── __init__.py
│       ├── bash_tool.py             ← Bash execution tool definition
│       ├── file_tool.py             ← File read/write tool definition
│       └── web_tool.py              ← Web fetch tool definition
├── config/
│   └── teams/
│       ├── fullstack.yaml           ← Example: full-stack web team
│       ├── data-science.yaml        ← Example: data science team
│       └── code-review.yaml         ← Example: focused code review team
├── scripts/
│   └── install.sh                   ← Installation script
├── bin/                             ← Build output (gitignored)
├── go.mod
├── go.sum
├── requirements.txt
├── Makefile
├── LICENSE                          ← MIT
├── PRD.md                           ← Product requirements (V2)
├── issue.md                         ← Bug/issue log
├── agent.md                         ← This file
└── README.md                        ← User-facing docs (V2)
```

---

## V2 Feature Details

### 1. Streaming Token Output
- Python providers that support streaming (`has_streaming = True`, `call_stream()` async generator) yield `(type, data)` tuples: `("text", chunk)`, `("tool_call", {...})`, `("done", {tokens})`
- Server sends SSE events: `event: text`, `event: tool_call`, `event: done`
- Go bridge reads SSE via `CallStream(ctx, req, onEvent)` and calls `onEvent` for each chunk
- Orchestrator uses `callAgentWithStreaming()` which renders text chunks as they arrive via `PrintAgentText`
- Providers without streaming fall back to non-streaming (yield full response in one chunk)

### 2. Cost Tracking & Budgets
- `Workflow.MaxTokensPerRun` sets a hard token budget for the entire run
- `EstimateCost(model, inputTokens, outputTokens)` in `cost.go` uses a hardcoded pricing table
- Cost is displayed in the summary line: `✓ completed in 4 turns · 3,241 tokens · $0.08`
- New-team wizard prompts for max tokens per run

### 3. Custom Tool Definitions
- `CustomTool` struct: `{Name, Description, Command, InputSchema}`
- Defined in team YAML under `custom_tools:` key
- Passed through bridge to Python engine as tool definitions
- Executed via `ExecuteCustomTool()` in orchestrator — command template with `{param}` placeholders replaced by LLM-provided values, passed as uppercase env vars

### 4. Persistent Memory (Saves After Every Prompt)
- **`internal/memory/`** package with:
  - `FileStore` — JSONL files at `~/.2mcode/memory/<team>.jsonl`, thread-safe
  - `Summarizer` — calls `qwen/qwen3-coder:free` via OpenRouter bridge to summarize sessions
- **When it saves:**
  - After every `RunTask` completion (one-shot task)
  - After EVERY user message in `RunChatTurn` (interactive chat) — so each prompt's context is remembered
- **How context is injected:**
  - Before each agent turn, `BuildContext()` loads the last 5 memory entries
  - Formats them as `[PAST SESSION MEMORY]` block and appends to the agent's system prompt
- **Best-effort:** memory failures never block the task — errors are logged and skipped

### 5. OpenRouter Universal Fallback
- When a provider-specific API key (e.g. `ANTHROPIC_API_KEY`) is missing but `OPENROUTER_API_KEY` IS set:
  - Go: `ValidateProviderKeys()` skips the missing key check
  - Python: `_resolve_provider()` in `agent.py` routes the request through the OpenRouter provider instead
  - The model name is passed as-is (OpenRouter accepts native model IDs like `claude-sonnet-4-6`)
  - A warning is logged: `ANTHROPIC_API_KEY not set — falling back to OpenRouter`
- This means users with only an OpenRouter API key can run any team configuration

---

## Key File Specs

### `internal/orchestrator/orchestrator.go`
The core engine. Key methods:

- `RunTask(ctx, team, sessionID, task)` — full task execution:
  1. Creates session, posts task to bus
  2. Builds turn schedule
  3. For each agent turn: `runAgentTurn()` with memory context injection + streaming
  4. After completion: saves session memory
- `RunChatTurn(ctx, team, sessionID, userMessage)` — single chat turn:
  1. Posts user message to bus
  2. Runs all agents in schedule
  3. After turn: saves session memory (per-prompt persistence)
- `runAgentTurn(ctx, team, sessionID, agent)` → `(inputTokens, outputTokens, err)`:
  1. Gets history from event bus
  2. Injects memory context into system prompt (if `memorySummarizer` is set)
  3. Calls `callAgentWithStreaming()` — SSE streaming with real-time rendering
  4. Tool use loop (up to 5 iterations) — executes tools, posts results, re-calls non-streaming
  5. Posts final response to event bus
- `saveSessionMemory(ctx, team, sessionID, task)` — gets full transcript, calls LLM summarizer, saves
- `formatMessages()` and `buildCustomToolDefs()` — helpers extracted for reuse

### `internal/bridge/bridge.go`
HTTP client to Python engine. Key methods:

- `Call(ctx, req)` → `*AgentResponse` — POST `/call` without streaming
- `CallStream(ctx, req, onEvent)` → `*AgentResponse` — POST `/call` with `stream: true`, reads SSE events:
  - `event: text` → `onEvent(StreamEvent{Type:"text", Content:...})` + accumulates response
  - `event: tool_call` → accumulates into `result.ToolCalls`
  - `event: done` → sets `result.InputTokens`/`OutputTokens`
  - `event: error` → returns error

### `agent_engine/agent.py`
Router. Key details:

- `_resolve_provider(name)` — returns `(module, actual_name)` with OpenRouter fallback when provider key is missing
- `run_agent(req)` → dict — resolves provider, calls `provider.call()`
- `run_agent_stream(req)` — async generator, resolves provider, yields `(type, data)` tuples

### `internal/team/config.go`
- `ValidateProviderKeys(t)` — when `OPENROUTER_API_KEY` is set, only requires keys for `ollama` (which needs none); all other provider keys are optional since OpenRouter can proxy them

### `internal/team/team.go`
Structs updated for V2:
- `Team.CustomTools []CustomTool` — user-defined tool definitions
- `Workflow.MaxTokensPerRun int` — token budget enforcement
- `Workflow.MaxTokens int` — max tokens per turn

---

## What NOT to Build (V3+)

Do not build these — they are explicitly out of scope:
- Web UI or dashboard
- Plugin/extension system
- Agent self-improvement via feedback loops
- Integration with GitHub PRs and CI/CD
- Voice interface
- Fine-tuned or self-hosted models
- Any telemetry or analytics

---

## Definition of Done

The project is complete when:
1. `go build ./cmd/2m` produces a working binary with no errors
2. `2m new-team` launches an interactive wizard and creates a valid YAML
3. `2m run fullstack "Build a hello world REST API in Go"` runs a full team session and writes output files
4. `2m chat code-review` opens an interactive REPL
5. All three example team YAMLs are included and valid
6. `README.md` is complete and accurate
7. A developer with only `OPENROUTER_API_KEY` set can run any team
8. Memory context persists across `2m run` sessions and `2m chat` turns
