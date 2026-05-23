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

**Commit:** `(pending — next push)`
**Status:** ✅ Fixed

---

## 6. Agent engine port conflict on re-run

**File(s):** `cmd/2m/main.go`

**Problem:** When `2m` is run again while a previous instance is still running, the Python agent engine fails to bind to port 8765 with `[Errno 98] address already in use`.

**Fix:** Added `killPort8765()` function in `main.go` that detects if port 8765 is in use, kills the owning process (`lsof`, `fuser`, or `taskkill` depending on OS), and waits for the port to be released before starting the new engine.

**Commit:** `(pending — next push)`
**Status:** ✅ Fixed

---

## 7. Bash tool timeout blocks agent loop

**File(s):** `agent_engine/tools/bash_tool.py`

**Problem:** Commands like `python3 -m http.server 8000` that run indefinitely (servers, watchers) hit the 30-second timeout and get killed. Even with `&`, `subprocess.run` still waited and timed out.

**Fix:** Added background command detection — if the command ends with `&`, `Popen` is used with `start_new_session=True` and returns immediately with the PID. The tool description was also updated to document this behavior.

**Commit:** `(pending — next push)`
**Status:** ✅ Fixed

---

## 8. 502 Bad Gateway — rate-limit error detail swallowed by generic message

**File(s):** `agent_engine/server.py`

**Problem:** When OpenRouter returns 429 (rate limited), the provider raises `ConnectionError("OpenRouter API rate limit exceeded...")`. The `server.py` error handler catches it, logs the real error message, but then replaces it with a generic `"Failed to connect to ... API. Check your API key and network."` detail and always returns HTTP 502. The user sees a misleading error about API keys when the real problem is rate limiting on free-tier models.

**Fix:** The `ConnectionError` handler now uses `str(e)` from the exception as the response detail (preserving the real error like rate limit messages). It also detects rate-limit-related keywords ("rate", "quota", "credit") and returns HTTP 429 instead of 502, so the client can distinguish connection failures from rate limits.

**Commit:** `(pending)`
