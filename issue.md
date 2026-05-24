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

# 2M Code — Issue Log

> Every bug, error, and fix encountered during development.
> When an AI agent fixes a bug, it must document it here with the fix.

---

## 1. Wrong GitHub repo URLs in README & install script

**File(s):** `README.md`, `scripts/install.sh`, `agent.md`, `internal/cli/root.go`, `agent_engine/providers/openrouter_provider.py`

**Problem:** All URLs pointed to `github.com/2mcode/2mcode` but the actual remote is `github.com/ArafatAhmed-2M/2M-Code`. The quick-start curl command returned a 404.

**Fix:** Updated 29 references across 12 files to use the correct repo URL. The Go module path (`go.mod`) was left as `github.com/2mcode/2mcode` since that's an import path, not a web URL.

**Commit:** `5a830a1`

---

## 2. `2m chat` command not found ("unknown command")

**File(s):** `internal/cli/chat.go`

**Problem:** Running `2m chat <team>` returned `Error: unknown command "chat" for "2m"`. The `chat.go` file defined `chatCmd` and `runChat` but was missing the `init()` function to register itself with Cobra's `rootCmd.AddCommand(chatCmd)`. All other commands (`run`, `new-team`, `team`, `models`) had this registration.

**Fix:** Added `func init() { rootCmd.AddCommand(chatCmd) }` to `chat.go`.

**Commit:** `aad22ba`

---

## 3. `2m new-team` reviewer prompt accepts free-text instead of numbered selection

**File(s):** `internal/cli/newteam.go`

**Problem:** The reviewer prompt used `prompt()` (free-text input) while the leader prompt used `promptWithOptions()` (numbered selection). User typed "1" expecting numbered selection but it was treated as an agent name, causing `workflow reviewer '1' is not a defined agent`.

**Fix:** Changed reviewer prompt to use `promptWithOptions()` with agent names + a `(skip)` option as the first choice.

**Commit:** `4e9c5b1`

---

## 4. Provider registration incomplete — missing env vars & validation

**File(s):** `internal/team/config.go`, `internal/team/team.go`, `internal/cli/newteam.go`

**Problem:** 8 provider Python files existed (`anthropic`, `google`, `openai`, `mistral`, `cohere`, `groq`, `ollama`, `openrouter`) but the Go backend only recognized 4:
- `GetProviderAPIKey()` only mapped 4 providers' env vars — cohere, groq, openrouter missing; ollama (no key) not handled
- Team validation error message only listed 4 providers
- `new-team` wizard only offered 4 provider choices

**Fix:** Added all 8 providers to env var map, validation error, and wizard options. Special-cased ollama (no API key needed).

**Commit:** `5a830a1`

---

## 5. Install script missing permissions on binary

**File(s):** `scripts/install.sh`

**Problem:** The install script copies the binary to `/usr/local/bin/2m` but doesn't set executable permissions. Running `2m` gives `Permission denied`.

**Fix:** Added `chmod +x` after `cp` in both the direct and `sudo` paths.

**Commit:** `(pre-existing)`
**Status:** ✅ Fixed

---

## 6. Agent engine port conflict on re-run

**File(s):** `cmd/2m/main.go`

**Problem:** When `2m` is run again while a previous instance is still running, the Python agent engine fails to bind to port 8765 with `[Errno 98] address already in use`.

**Fix:** Added `killPort8765()` function in `main.go` that detects if port 8765 is in use, kills the owning process (`lsof`, `fuser`, or `taskkill` depending on OS), and waits for the port to be released before starting the new engine.

**Commit:** `(pre-existing)`
**Status:** ✅ Fixed

---

## 7. Bash tool timeout blocks agent loop

**File(s):** `agent_engine/tools/bash_tool.py`

**Problem:** Commands like `python3 -m http.server 8000` that run indefinitely (servers, watchers) hit the 30-second timeout and get killed. Even with `&`, `subprocess.run` still waited and timed out.

**Fix:** Added background command detection — if the command ends with `&`, `Popen` is used with `start_new_session=True` and returns immediately with the PID. The tool description was also updated to document this behavior.

**Commit:** `(pre-existing)`
**Status:** ✅ Fixed

---

## 8. 502 Bad Gateway — rate-limit error detail swallowed by generic message

**File(s):** `agent_engine/server.py`

**Problem:** When OpenRouter returns 429 (rate limited), the provider raises `ConnectionError("OpenRouter API rate limit exceeded...")`. The `server.py` error handler catches it, logs the real error message, but then replaces it with a generic `"Failed to connect to ... API. Check your API key and network."` detail and always returns HTTP 502. The user sees a misleading error about API keys when the real problem is rate limiting on free-tier models.

**Fix:** The `ConnectionError` handler now uses `str(e)` from the exception as the response detail (preserving the real error like rate limit messages). It also detects rate-limit-related keywords ("rate", "quota", "credit") and returns HTTP 429 instead of 502, so the client can distinguish connection failures from rate limits.

**Commit:** `6fef01c`

---

## 9. `2m` only works from the project directory — can't find agent_engine elsewhere

**File(s):** `cmd/2m/main.go`

**Problem:** Running `2m` from outside the project directory (e.g., `cd /some/other/dir && 2m chat fullstack`) fails with `cannot find agent_engine/server.py` because `findEngineScript()` only searched relative to the binary and relative to CWD. The install script installs `agent_engine/` to `~/.2mcode/agent_engine/`, but that location was never checked.

**Fix:** Added `~/.2mcode/agent_engine/server.py` as the 2nd search path (after env var override) in `findEngineScript()`, and updated the error message to suggest reinstalling rather than vague advice.

**Commit:** `a93aece`

---

## 10. Bundled teams not found outside project directory — missing `~/.2mcode/config/teams/` in search paths

**File(s):** `internal/team/team.go`

**Problem:** `2m chat fullstack` from outside the project directory fails with `team 'fullstack' not found`. The install script copies bundled teams to `~/.2mcode/config/teams/`, but `getSearchPaths()` and `ListTeams()` only check project-local `./.2mcode/teams/`, global `~/.2mcode/teams/`, and relative-to-binary `config/teams/`. The installed location `~/.2mcode/config/teams/` was never searched.

**Fix:** Added `~/.2mcode/config/teams/<name>.yaml` to both `getSearchPaths()` and `ListTeams()` search directories (right after `~/.2mcode/teams/`). Also updated the not-found error message to list all 4 search locations.

**Commit:** `e72b8c2`

---

## 11. Circular import in providers/__init__.py

**File(s):** `agent_engine/providers/__init__.py`

**Problem:** `from providers import ...` inside the `providers` package itself causes `ImportError` on startup (circular import).

**Fix:** Changed to `from . import ...` (relative import).

**Commit:** `e72b8c2`

---

## 12. Duplicate import in anthropic_provider.py

**File(s):** `agent_engine/providers/anthropic_provider.py`

**Problem:** `import anthropic` called twice in `_get_client()` — first on line 26 (unused), then on line 34 (actual).

**Fix:** Removed the first unused import.

**Commit:** `e72b8c2`

---

## 13. Cost calculation uses wrong model for token pricing

**File(s):** `internal/orchestrator/orchestrator.go`, `internal/orchestrator/cost.go`

**Problem:** `RunTask` called `EstimateCost(t.Agents[0].Model, ...)` using the first agent's model for ALL agents' total tokens. With different models having different pricing (e.g., Claude Opus is 60× more expensive than Haiku), this gave wildly wrong cost estimates for mixed-provider teams. Also `cost.go` had an unused loop variable `i` suppressed with `_ = i`.

**Fix:** Added per-agent token tracking in `RunTask`, stored in a `map[agentName]struct{Input, Output}`, then calls `TotalCost()` which iterates per-agent using its own model pricing. Removed the `_ = i` hack by using `for _, agent := range`.

**Commit:** `e72b8c2`

---

## 14. Custom tool InputSchema default lost on range-copy

**File(s):** `internal/team/team.go`

**Problem:** The `Validate()` method used `for _, ct := range t.CustomTools` which creates a copy — so `ct.InputSchema = {...}` default assignment was lost immediately.

**Fix:** Changed to `for i := range t.CustomTools` with `ct := &t.CustomTools[i]` (pointer to actual element).

**Commit:** `e72b8c2`

---

## 15. Team/task name swap on not-found fallback in `2m run`

**File(s):** `internal/cli/run.go`

**Problem:** When a team wasn't found, the fallback set `teamName = args[len-1]` (last arg = task) and `task = args[:len-1]` (everything else). This swapped them, producing confusing error messages like `team 'Build a REST API' not found`.

**Fix:** Changed fallback to `teamName = args[0]`, `task = strings.Join(args[1:], " ")`.

**Commit:** `e72b8c2`

---

## 16. `top_p` used as context_length fallback in OpenRouter provider

**File(s):** `agent_engine/providers/openrouter_provider.py`

**Problem:** The `list_models()` fallback used `getattr(m, "top_p", 0)` as the default for `context_length` — `top_p` is a sampling parameter (0-1), unrelated to context length. Always returned 0 or 1 instead of actual context size.

**Fix:** Changed to `getattr(m, "context_length", 0)` (just 0 as the final fallback).

**Commit:** `e72b8c2`

---

## 17. Custom tool `{param}` placeholders not substituted in command

**File(s):** `internal/orchestrator/tools.go`

**Problem:** The `executeCustomTool()` function passed `ct.Command` directly to `exec.Command` without replacing `{param}` placeholders with actual LLM-provided values. A command like `npm run lint -- {paths}` would literally run with `{paths}` unsubstituted.

**Fix:** Added `strings.ReplaceAll` substitution loop before execution that replaces each `{key}` with the corresponding input value.

**Commit:** `e72b8c2`

---

## 18. Added generic OpenAI-Compatible provider

**File(s):** `agent_engine/providers/openai_compatible_provider.py` (new), `agent_engine/providers/__init__.py`, `agent_engine/agent.py`, `internal/team/team.go`, `internal/team/config.go`, `README.md`, `PRD.md`, `SETUP.md`, `agent.md`

**Problem:** Users needed native provider support for DeepSeek, Together AI, xAI Grok, Perplexity, Fireworks, and other OpenAI-compatible APIs without going through OpenRouter.

**Fix:** Added a new `openai_compatible` provider adapter that reads `OPENAI_COMPATIBLE_API_KEY` and `OPENAI_COMPATIBLE_BASE_URL` (default: `https://api.openai.com/v1`). Supports streaming, tool calling, and model listing via the generic OpenAI SDK. Registered in Go validation, Python router, env var mappings, and all documentation.

**Commit:** `f0f3a1c`

---

## 19. Streaming renderer — per-chunk output produces fragmented display

**File(s):** `internal/cli/renderer.go`

**Problem:** `PrintAgentText()` printed every SSE chunk immediately on a new line. With multi-word chunks arriving from the streaming provider, each fragment appeared on its own line — e.g., `"Hello"` then `" world"` rendered as two lines instead of one. Empty responses (e.g., tool-only calls) produced a stray `│` character with no text.

**Fix:** Added a text buffer per agent in `PrintAgentText` that accumulates chunks. The buffer is flushed (printed) only when a newline is encountered in the accumulated text or when `FlushAgentText()` is called at agent end. Whitespace-only output is suppressed entirely.

**Commit:** `(pending)`

---

## 20. `base_url` config for `openai_compatible` provider

**File(s):** `internal/team/team.go`, `internal/bridge/bridge.go`, `internal/orchestrator/orchestrator.go`, `internal/cli/newteam.go`, `agent_engine/server.py`, `agent_engine/agent.py`, `agent_engine/providers/openai_compatible_provider.py`, `agent_engine/providers/*.py` (all 8 others — added `**kwargs`)

**Problem:** The `openai_compatible` provider only supported `base_url` via the `OPENAI_COMPATIBLE_BASE_URL` environment variable. This required users to set a global env var even when different teams used different endpoints (e.g., DeepSeek in one team, Together in another).

**Fix:** Added `Agent.BaseURL` to the Go `Agent` struct (team YAML) and threaded it through the bridge → Python server → agent router → provider. The `openai_compatible_provider._get_client()` accepts an optional `base_url` parameter that takes precedence over the env var. All 8 other providers got `**kwargs` on their `call`/`call_stream` signatures so future provider-specific configs pass through without breaking them.

**Usage in team YAML:**
```yaml
agents:
  - name: Codex
    provider: openai_compatible
    model: deepseek-chat
    base_url: https://api.deepseek.com
```

**Resolution order:** YAML `base_url` → `OPENAI_COMPATIBLE_BASE_URL` env var → `https://api.openai.com/v1`

**Commit:** `(pending)`
