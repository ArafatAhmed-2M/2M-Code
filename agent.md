# agent.md — 2M Code
**AI Agent Instruction File for Google Antigravity**  
**Project:** 2M Code (Multi-Mind Coding Platform)  
**Version:** 1.0.0  

---

## Your Identity

You are the principal engineer for **2M Code**, an open-source terminal-native AI coding platform. Your job is to build this project from scratch, file by file, following the PRD and technical specs in this repository.

You do not ask unnecessary questions. You read the specs, make sensible decisions, write production-quality code, and report what you have done. When you encounter a genuine ambiguity that would cause a wrong architectural decision, you state the ambiguity and your chosen resolution before proceeding.

---

## Project Overview

2M Code is a CLI tool (like Claude Code or Gemini CLI) with one killer differentiator: **agent teams**. Instead of one AI model, users configure a *team* of AI agents — each with a name, role, provider (Anthropic/Google/OpenAI/Mistral), model, and system prompt — that collaborate on coding tasks through a shared conversation channel.

The name "2M" stands for **Multi-Mind**.

**What makes it different from all other CLI coding tools:**
1. Multiple agents, each from any provider, work as a team
2. Agents share a "team channel" — every agent sees every other agent's messages
3. Users define teams in YAML — shareable, version-controllable
4. The reviewer agent always sees the full implementation before giving feedback
5. Mix providers: Planner on Gemini, Coder on Claude, Reviewer on GPT-4o

---

## Tech Stack

| Layer | Technology | Why |
|---|---|---|
| CLI binary | Go 1.22+ | Fast startup, single binary, great concurrency |
| CLI framework | `github.com/spf13/cobra` | Industry standard Go CLI |
| Agent engine | Python 3.11+ / FastAPI | Best AI SDK ecosystem |
| IPC | HTTP over Unix socket (localhost:8765) | Simple, reliable |
| State / event bus | SQLite via `github.com/mattn/go-sqlite3` | Zero dependency, embedded |
| Config | YAML via `gopkg.in/yaml.v3` | Human-readable team definitions |
| Terminal rendering | `github.com/charmbracelet/lipgloss` | Beautiful CLI output |
| LLM providers | `anthropic`, `openai`, `google-generativeai` Python SDKs | Native SDKs |

---

## Repository Structure

Build the project with exactly this structure:

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
│   │   └── tools.go                 ← Tool execution (bash, file I/O)
│   ├── bus/
│   │   ├── bus.go                   ← Event bus (SQLite read/write)
│   │   └── schema.go                ← DB schema & migrations
│   ├── team/
│   │   ├── team.go                  ← Team struct + loader
│   │   └── config.go                ← Global config (~/.2mcode/config.yaml)
│   └── bridge/
│       └── bridge.go                ← HTTP client to Python agent engine
├── agent_engine/
│   ├── server.py                    ← FastAPI server (port 8765)
│   ├── agent.py                     ← Agent call logic + tool handling
│   ├── providers/
│   │   ├── __init__.py
│   │   ├── anthropic_provider.py    ← Anthropic SDK adapter
│   │   ├── google_provider.py       ← Google Gemini SDK adapter
│   │   ├── openai_provider.py       ← OpenAI SDK adapter
│   │   └── mistral_provider.py      ← Mistral SDK adapter
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
├── go.mod
├── go.sum
├── requirements.txt
├── Makefile
└── README.md
```

---

## File-by-File Implementation Specs

### `go.mod`
```
module github.com/2mcode/2mcode

go 1.22

require (
    github.com/spf13/cobra v1.8.0
    github.com/mattn/go-sqlite3 v1.14.22
    gopkg.in/yaml.v3 v3.0.1
    github.com/charmbracelet/lipgloss v0.10.0
)
```

---

### `cmd/2m/main.go`
Entry point. Starts the Python agent engine subprocess, waits for it to be ready (health check on `/health`), then hands off to the Cobra CLI. On exit (signal or command completion), kills the Python subprocess.

```go
func main() {
    // 1. Find Python and agent_engine/server.py
    // 2. Start: python agent_engine/server.py &
    // 3. Poll GET http://localhost:8765/health until 200 (timeout 10s)
    // 4. Defer: kill python process
    // 5. cli.Execute()
}
```

---

### `internal/bus/schema.go`
Defines and migrates the SQLite schema on first run.

```sql
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    team_name TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    agent_name TEXT NOT NULL,
    role TEXT NOT NULL CHECK(role IN ('user','assistant','system')),
    content TEXT NOT NULL,
    tool_calls TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);

CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id, created_at);
```

---

### `internal/bus/bus.go`
Provides `Post(sessionID, agentName, role, content string)` and `GetHistory(sessionID string, limit int) []Message`.

The history returned by `GetHistory` is formatted as OpenAI-compatible message objects:
```go
type Message struct {
    Role    string // "user" or "assistant"
    Content string
    Name    string // agent name (used to distinguish speakers)
}
```

When formatting for the LLM API, prepend the agent name to the content so the model knows who said what:
`"[Aria · Tech Lead]: I will break this into three subtasks..."`

---

### `internal/team/team.go`
Loads and validates team YAML files. Searches in this order:
1. `./.2mcode/teams/<name>.yaml` (project-local)
2. `~/.2mcode/teams/<name>.yaml` (global)
3. Built-in `config/teams/<name>.yaml` (bundled examples)

Structs:

```go
type Team struct {
    Name        string    `yaml:"name"`
    Description string    `yaml:"description"`
    Agents      []Agent   `yaml:"agents"`
    Workflow    Workflow  `yaml:"workflow"`
}

type Agent struct {
    Name        string   `yaml:"name"`
    Role        string   `yaml:"role"`
    Provider    string   `yaml:"provider"`
    Model       string   `yaml:"model"`
    SystemPrompt string  `yaml:"system_prompt"`
    MaxContext  int      `yaml:"max_context"`
    Color       string   `yaml:"color"`
    Tools       []string `yaml:"tools"`
}

type Workflow struct {
    Orchestration  string `yaml:"orchestration"` // leader_first | round_robin
    TurnsPerTask   int    `yaml:"turns_per_task"`
    Leader         string `yaml:"leader"`
    Reviewer       string `yaml:"reviewer"`
    MaxTokens      int    `yaml:"max_tokens_per_turn"`
}
```

---

### `internal/orchestrator/orchestrator.go`
The core engine. Method: `RunTask(team Team, sessionID string, task string, renderer Renderer)`.

**Leader-first orchestration:**
```
1. Post task as user message to bus
2. Find leader agent → runAgentTurn(leader)
3. For round in 1..workflow.TurnsPerTask:
     For each non-leader, non-reviewer agent:
         runAgentTurn(agent)
4. If reviewer defined → runAgentTurn(reviewer)
5. Print completion summary
```

**runAgentTurn(agent):**
```
1. history = bus.GetHistory(sessionID, agent.MaxContext)
2. request = AgentRequest{
       Provider: agent.Provider,
       Model:    agent.Model,
       System:   agent.SystemPrompt,
       Messages: history,
       Tools:    agent.Tools,
   }
3. Call bridge.Call(request) → streaming response
4. If response contains tool_use blocks:
     Execute tools via tools.Run(toolName, toolInput)
     Post tool result back, call bridge.Call again with result
5. Post final text response to bus
6. renderer.PrintAgentTurn(agent, response)
```

---

### `internal/bridge/bridge.go`
HTTP client that POSTs to `http://localhost:8765/call`.

Request body (JSON):
```json
{
  "provider": "anthropic",
  "model": "claude-opus-4-5",
  "system": "You are Aria, the tech lead...",
  "messages": [{"role": "user", "content": "[user]: Build a REST API..."}],
  "tools": ["bash", "read_file", "write_file"],
  "max_tokens": 4096,
  "stream": false
}
```

Response body (JSON):
```json
{
  "content": "I'll break this into three subtasks...",
  "tool_calls": [],
  "input_tokens": 312,
  "output_tokens": 841
}
```

---

### `agent_engine/server.py`
FastAPI server. Single endpoint: `POST /call`. Also `GET /health`.

```python
from fastapi import FastAPI
from pydantic import BaseModel
from agent import run_agent

app = FastAPI()

class AgentRequest(BaseModel):
    provider: str
    model: str
    system: str
    messages: list[dict]
    tools: list[str] = []
    max_tokens: int = 4096

@app.get("/health")
def health(): return {"status": "ok"}

@app.post("/call")
async def call(req: AgentRequest):
    return await run_agent(req)
```

---

### `agent_engine/agent.py`
Routes requests to the correct provider. Handles tool definitions.

```python
from providers import anthropic_provider, google_provider, openai_provider, mistral_provider
from tools import TOOL_DEFINITIONS, execute_tool

PROVIDERS = {
    "anthropic": anthropic_provider,
    "google": google_provider,
    "openai": openai_provider,
    "mistral": mistral_provider,
}

async def run_agent(req):
    provider = PROVIDERS[req.provider]
    tools = [TOOL_DEFINITIONS[t] for t in req.tools if t in TOOL_DEFINITIONS]
    return await provider.call(req.model, req.system, req.messages, tools, req.max_tokens)
```

---

### `agent_engine/providers/anthropic_provider.py`
```python
import anthropic, os

client = anthropic.Anthropic(api_key=os.environ.get("ANTHROPIC_API_KEY"))

async def call(model, system, messages, tools, max_tokens):
    resp = client.messages.create(
        model=model,
        max_tokens=max_tokens,
        system=system,
        messages=messages,
        tools=tools if tools else [],
    )
    text_content = next((b.text for b in resp.content if b.type == "text"), "")
    tool_calls = [
        {"name": b.name, "input": b.input, "id": b.id}
        for b in resp.content if b.type == "tool_use"
    ]
    return {
        "content": text_content,
        "tool_calls": tool_calls,
        "input_tokens": resp.usage.input_tokens,
        "output_tokens": resp.usage.output_tokens,
    }
```

Follow the same pattern for `google_provider.py` (using `google.generativeai`), `openai_provider.py` (using `openai.OpenAI()`), and `mistral_provider.py` (using `mistral.Mistral()`). Each adapter normalizes responses to the same dict shape.

---

### `agent_engine/tools/__init__.py`
```python
import subprocess, os

TOOL_DEFINITIONS = {
    "bash": {
        "name": "bash",
        "description": "Execute a bash command. Returns stdout and stderr.",
        "input_schema": {
            "type": "object",
            "properties": {
                "command": {"type": "string", "description": "The bash command to run"}
            },
            "required": ["command"]
        }
    },
    "read_file": {
        "name": "read_file",
        "description": "Read the contents of a file.",
        "input_schema": {
            "type": "object",
            "properties": {
                "path": {"type": "string", "description": "File path to read"}
            },
            "required": ["path"]
        }
    },
    "write_file": {
        "name": "write_file",
        "description": "Write content to a file.",
        "input_schema": {
            "type": "object",
            "properties": {
                "path": {"type": "string", "description": "File path to write"},
                "content": {"type": "string", "description": "Content to write"}
            },
            "required": ["path", "content"]
        }
    }
}

def execute_tool(name: str, input: dict) -> str:
    if name == "bash":
        result = subprocess.run(input["command"], shell=True, capture_output=True, text=True, timeout=30)
        return result.stdout + result.stderr
    elif name == "read_file":
        with open(input["path"], "r") as f:
            return f.read(102400)  # max 100KB
    elif name == "write_file":
        with open(input["path"], "w") as f:
            f.write(input["content"])
        return f"Written: {input['path']}"
    return f"Unknown tool: {name}"
```

---

### `internal/cli/renderer.go`
Uses `charmbracelet/lipgloss` to render agent output.

Color scheme:
- User input: dim white
- Agent name badge: agent's configured color, bold
- Agent response: default terminal color
- Tool call line: cyan, prefixed with `⚙`
- Tool result line: dim, prefixed with `└`
- Completion summary: green, prefixed with `✓`
- Error: red, prefixed with `✗`

Render format per agent turn:
```
╭─ Aria · Tech Lead ────────────────────────
│ I'll break this task into three subtasks:
│ 1. Set up the database schema
│ 2. Implement the API endpoints
│ 3. Add authentication middleware
╰──────────────────────────────────────────
```

---

### Example Team YAMLs

**`config/teams/fullstack.yaml`** — A full-stack web development team:
- **Aria** (Tech Lead) — Anthropic claude-opus-4-5 — Plans, delegates, synthesizes
- **Dev** (Senior Engineer) — Google gemini-1.5-pro — Implements features
- **Quinn** (QA Engineer) — OpenAI gpt-4o — Reviews code and plans

**`config/teams/code-review.yaml`** — A focused code review team:
- **Alex** (Security Reviewer) — Anthropic claude-sonnet-4-6 — Security and vulnerability review
- **Sam** (Performance Reviewer) — Google gemini-1.5-flash — Performance and optimization
- **Jordan** (Style Reviewer) — OpenAI gpt-4o-mini — Code style and maintainability

**`config/teams/data-science.yaml`** — A data science team:
- **Nova** (Data Lead) — Anthropic claude-opus-4-5 — Problem framing and approach
- **Sage** (ML Engineer) — Google gemini-1.5-pro — Model implementation
- **River** (Data Engineer) — OpenAI gpt-4o — Data pipeline and tooling

Write complete, realistic system prompts for each agent. System prompts should be 150–300 words, specific to the role, and instruct the agent to communicate as if speaking to their teammates.

---

### `README.md`

Write a full README with:
1. **Headline:** "2M Code — The AI coding platform that thinks in teams"
2. **What is 2M Code?** (2 paragraphs)
3. **Installation** (`curl -sSL ... | sh` and manual steps)
4. **Quick start** (three commands: `2m new-team`, `2m run`, `2m chat`)
5. **Team config example** (full YAML with explanations)
6. **Supported providers** (table)
7. **How it works** (team channel concept, 3 bullet points)
8. **Roadmap** (v1, v2, v3 items)
9. **Contributing**
10. **License: MIT**

---

### `scripts/install.sh`
```bash
#!/usr/bin/env bash
# 2M Code installer
# Installs Go binary and Python agent engine

set -e
REPO="https://github.com/2mcode/2mcode"
INSTALL_DIR="/usr/local/bin"

echo "Installing 2M Code..."
# Download latest release binary for OS/arch
# Install Python dependencies via pip
# Create ~/.2mcode/config.yaml template
# Print success message with next steps
```

---

### `Makefile`
```makefile
build:
    go build -o bin/2m ./cmd/2m

install: build
    cp bin/2m /usr/local/bin/2m

test:
    go test ./...
    cd agent_engine && python -m pytest

run-dev:
    go run ./cmd/2m $(ARGS)

clean:
    rm -rf bin/
```

---

### `requirements.txt`
```
anthropic>=0.25.0
openai>=1.30.0
google-generativeai>=0.5.0
mistralai>=0.4.0
fastapi>=0.111.0
uvicorn>=0.30.0
pydantic>=2.7.0
```

---

## Build Order

Build files in this exact order to ensure each piece is testable before the next depends on it:

1. `requirements.txt` + `go.mod`
2. `agent_engine/server.py` + `agent_engine/providers/anthropic_provider.py` — test: single Anthropic call works
3. All other provider adapters (`google`, `openai`, `mistral`)
4. `agent_engine/tools/__init__.py` — test: bash tool executes correctly
5. `agent_engine/agent.py` — full agent routing
6. `internal/bus/schema.go` + `internal/bus/bus.go` — test: post and read messages
7. `internal/team/team.go` + `internal/team/config.go`
8. `internal/bridge/bridge.go`
9. `internal/orchestrator/orchestrator.go` + `internal/orchestrator/scheduler.go`
10. `internal/orchestrator/tools.go`
11. `internal/cli/renderer.go`
12. `internal/cli/run.go` + `internal/cli/chat.go`
13. `internal/cli/team.go` + `internal/cli/newteam.go`
14. `cmd/2m/main.go` — wire it all together
15. `config/teams/*.yaml` — all three example teams
16. `README.md`
17. `scripts/install.sh`
18. `Makefile`

---

## Quality Standards

- All Go code must pass `go vet ./...` with no warnings
- All Python code must follow PEP 8 (use `black` formatter)
- Every public function/method must have a docstring or Go doc comment
- No hardcoded API keys anywhere in source
- All file paths use `os.path.join` / `filepath.Join` (never string concatenation)
- Error messages must be actionable: tell the user what went wrong AND what to do next
- Every tool execution is logged (not printed unless `--verbose`) with timestamp and duration

---

## Code Conventions

### Go
- Use `fmt.Errorf("context: %w", err)` for error wrapping
- Prefer explicit returns over named returns
- HTTP calls get a 30-second timeout
- Use `context.Context` for all long-running operations

### Python
- Use `async/await` throughout the agent engine
- All provider adapters must return the same dict shape (content, tool_calls, input_tokens, output_tokens)
- Never catch bare `except:` — always catch specific exceptions

---

## What NOT to Build in v1

Do not build these — they are explicitly out of scope for v1:
- Web UI or dashboard
- Agent parallelism (simultaneous turns)
- Persistent cross-session memory
- Plugin/extension system
- Fine-tuned models
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
7. A developer with API keys for one provider can get from zero to first team run in under 5 minutes
