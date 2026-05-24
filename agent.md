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

# agent.md — 2M Code V3
**AI Agent Instruction File**  
**Project:** 2M Code (Multi-Mind Coding Platform)  
**Version:** 3.0.0  
**Repository:** https://github.com/ArafatAhmed-2M/2M-Code.git  

---

## Your Identity

You are the principal engineer for **2M Code**, an open-source terminal-native AI coding platform. Your job is to build this project from scratch, file by file, following the PRD and technical specs in this repository.

You do not ask unnecessary questions. You read the specs, make sensible decisions, write production-quality code, and report what you have done. When you encounter a genuine ambiguity that would cause a wrong architectural decision, you state the ambiguity and your chosen resolution before proceeding.

---

## Project Overview

2M Code is a CLI tool (like Claude Code or Gemini CLI) with one killer differentiator: **agent teams**. Instead of one AI model, users configure a *team* of AI agents — each with a name, role, provider, model, and system prompt — that collaborate on coding tasks through a shared conversation channel.

**V2 added** persistent memory (agents save context after every prompt), streaming token output, cost tracking with budgets, custom tool definitions, automatic OpenRouter fallback, and a generic OpenAI-Compatible provider adapter.

**V3 adds** a Python-based plugin/extension system (lifecycle hooks for custom tools, agent behaviors, and CLI commands), GitHub PR and CI/CD integration, agent self-improvement via feedback loops, a basic web dashboard for session monitoring, and closes remaining V2 gaps (tests, `2m history`, `web_fetch` tool fix, streaming renderer fix, chat budget enforcement).

---

## Tech Stack

| Layer | Technology | Why |
|---|---|---|
| CLI binary | Go 1.22+ | Fast startup, single binary, great concurrency |
| CLI framework | `github.com/spf13/cobra` | Industry standard Go CLI |
| Agent engine | Python 3.11+ / FastAPI | Best AI SDK ecosystem |
| IPC | HTTP over localhost:8765 | Simple, reliable |
| State / event bus | SQLite via `modernc.org/sqlite` | Zero dependency, embedded, pure-Go (no CGO) |
| Config | YAML via `gopkg.in/yaml.v3` | Human-readable team definitions |
| Terminal rendering | `github.com/charmbracelet/lipgloss` | Beautiful CLI output |
| LLM providers | `anthropic`, `openai`, `google-genai`, `mistralai`, `cohere`, `groq` Python SDKs + `httpx` for Ollama + generic `openai` SDK for OpenAI-compatible | Native SDKs for all supported providers + universal adapter |

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
│   │   ├── openai_compatible_provider.py ← Generic OpenAI-compatible adapter
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
│       ├── code-review.yaml         ← Example: focused code review team
│       └── test-openrouter.yaml     ← Example: OpenRouter free models test team
├── scripts/
│   └── install.sh                   ← Installation script
├── bin/
│   ├── 2m.exe                       ← Build output (gitignored)
│   └── 2mcode.cmd                   ← Windows wrapper: `2mcode` runs `2m` from any terminal
├── go.mod
├── go.sum
├── requirements.txt
├── Makefile
├── LICENSE                          ← Non-Commercial / Attribution Required
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
- `Agent.BaseURL string` — API base URL (openai_compatible only); overrides `OPENAI_COMPATIBLE_BASE_URL` env var

---

## V3 Features — Implementation Order

These are listed in priority order. Build them in this sequence:

| Priority | Feature | What It Does | Complexity |
|----------|---------|--------------|------------|
| P0 | Plugin/extension system | ✅ **Done.** Python-based plugins with lifecycle hooks: `on_agent_turn_start`, `on_agent_turn_end`, `on_tool_exec`, `on_startup`, `on_shutdown`. Scans `~/.2mcode/plugins/` and `.2mcode/plugins/`. Users write a single `.py` file that subclasses a base class. `2m plugin list` CLI command. | Medium |
| P1 | GitHub PR & CI/CD integration | `2m github review <pr-url>` — fetches PR diff, runs the configured team, posts review as a comment. Optional webhook server to auto-review on push. | High |
| P2 | Agent self-improvement loops | After each task, agent B reviews agent A's output, provides structured feedback. Feedback is injected into agent A's next turn. Agents improve across a session. | Medium |
| P3 | Web dashboard (read-only) | Simple FastAPI-based web UI showing live sessions, agent messages, token usage, cost. Built with Jinja2 templates + HTMX — no JS framework. | High |
| P4 | V2 gap closure | Tests, `2m history`, `web_fetch` tool fix, streaming renderer fix, chat budget enforcement. | Low |

### What NOT to Build (V4+)

Do not build these during V3:
- Multi-user session sharing (V4)
- Team management UI with roles (V4)
- Audit logging (V4)
- Agent personas (V4)
- Autonomous agent mode (V5)
- Cross-project memory (V5)
- Natural language workflow builder (V5)
- Self-hosted model fine-tuning (V5)
- Real-time collaboration (V5)
- Voice interface (deferred indefinitely)
- Telemetry or analytics (deferred indefinitely)

---

## Definition of Done

### V3 Done

The project is V3-complete when:
1. All V2 Definition of Done items still pass
2. Plugin system works: user creates `~/.2mcode/plugins/my_plugin.py` with a plugin class, and it hooks into agent turns / tool execution
3. `2m github review <pr-url>` fetches a PR diff and runs a team review
4. `2m history <session-id>` shows formatted session transcript
5. `2m run` and `2m chat` enforce token budget consistently
6. `web_fetch` tool actually fetches URLs (not stub)
7. Streaming renderer outputs cleanly (no fragment-per-chunk)
8. At least basic test files exist for Go and Python
9. `context-for-ai.md` exists with current session state for AI resumability
10. All docs (PRD.md, README.md, SETUP.md, agent.md, issue.md) are updated for V3

### V2 Done (legacy — still applies)

The project was V2-complete when:
1. `go build ./cmd/2m` produces a working binary with no errors
2. `2m new-team` launches an interactive wizard and creates a valid YAML
3. `2m run fullstack "Build a hello world REST API in Go"` runs a full team session and writes output files
4. `2m chat code-review` opens an interactive REPL
5. All example team YAMLs are included and valid
6. `README.md` is complete and accurate
7. A developer with only `OPENROUTER_API_KEY` set can run any team
8. Memory context persists across `2m run` sessions and `2m chat` turns
9. All 9 providers work: anthropic, google, openai, openai_compatible, mistral, cohere, groq, ollama, openrouter
10. `2mcode` command works from any terminal (via `bin/2mcode.cmd` wrapper + user PATH)

---

## Bugs Fixed (Session: 2026-05-24)

The following bugs were found and fixed. All future agents should verify these are not reintroduced.

| # | File | Bug | Fix |
|---|------|-----|-----|
| 1 | `agent_engine/providers/__init__.py:22` | `from providers import …` causes circular ImportError on startup | Changed to `from . import …` (relative import) |
| 2 | `agent_engine/providers/anthropic_provider.py:26,34` | Duplicate `import anthropic` — first one unused | Removed the first import |
| 3 | `internal/orchestrator/cost.go:80-84` | Unused loop var `i` with hacky `_ = i` suppression | Changed to `for _, agent := range` |
| 4 | `internal/orchestrator/orchestrator.go:136` | Cost estimated using only `t.Agents[0].Model` for all agents' tokens (wrong when agents use different models) | Now tracks per-agent tokens and calls `TotalCost()` for accurate per-model aggregation |
| 5 | `internal/team/team.go:242-244` | `ct.InputSchema` default set on range-copy (no effect on actual struct) | Changed to `for i := range` with pointer `&t.CustomTools[i]` |
| 6 | `internal/cli/run.go:77-78` | Fallback on team-not-found swaps team name and task (confusing error) | Changed `args[len-1]` → `args[0]`, `args[:len-1]` → `args[1:]` |
| 7 | `agent_engine/providers/openrouter_provider.py:70` | `top_p` used as fallback for `context_length` (completely wrong attribute) | Changed to just `0` |
| 8 | `internal/orchestrator/tools.go:50-108` | Custom tool `{param}` placeholders never substituted in command template | Added `strings.ReplaceAll` substitution loop before execution |
| 9 | `internal/cli/renderer.go` | Streaming renderer printed every SSE chunk on a new line; empty responses produced stray `│` | `PrintAgentText` now buffers chunks and flushes on newlines |
| 10 | `cmd/2m/main.go` | Engine startup blocks `--help`, `--version`, and bare `2mcode` invocation | Added `needsEngine()` check; only starts engine for `run`, `chat`, `history`, `models`, `plugin` |
| 11 | `internal/bus/schema.go` | `go-sqlite3` requires CGO; Windows has no GCC | Migrated to `modernc.org/sqlite` (pure Go, no build tools) |

### Features Added
| # | Feature | Details |
|---|---------|---------|
| 1 | **OpenAI-Compatible provider** | Full provider adapter with streaming, tool calling, model listing |
| 2 | **`base_url` in team YAML** | `Agent.BaseURL` overrides `OPENAI_COMPATIBLE_BASE_URL` env var; per-agent endpoint config |
| 3 | **`2mcode` Windows launcher** | `bin/2mcode.cmd` — type `2mcode` from any terminal |
| 4 | **Instant CLI** | No engine startup delay for help, version, new-team, team, config, completion |
| 5 | **Streaming buffer** | Text chunks accumulated and flushed on newlines — smooth output |
| 6 | **Pure-Go SQLite** | `modernc.org/sqlite` replaces `go-sqlite3` — no CGO needed |

## What's Still Needed (see context-for-ai.md for live state)

The canonical state of what's been done and what's next lives in `context-for-ai.md` at the repo root. It is regenerated each session so the next AI can pick up without losing context.

### V2 Gaps (P4 priority for V3)
- **Tests:** No test files exist yet in either Go or Python.
- **`2m history` command:** Only a stub exists (`team.go:173-186`).
- **`web_fetch` tool:** Go-side `ExecuteTool` returns a stub string instead of actually fetching a URL.
- **Streaming renderer:** ~~`PrintAgentText` prints every SSE chunk on a new line~~ ✅ FIXED — now buffers and flushes on newlines
- **Chat token budget:** `RunTask` enforces `MaxTokensPerRun` but `RunChatTurn` does not.

### V3 Features (P0-P3 priority)
See the table in the V3 Features section above. Start with P0 (plugin system), then P1, P2, P3, then P4 (V2 gaps).
