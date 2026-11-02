---
name: ctfpc9n-cli
description: Use ctfpc9n-cli for restricted competition participation or authorized AI evaluations with separate sessions.
---

# ctfpc9n-cli

Use `ctfpc9n-cli` for authorized competition and AI evaluation actions. Do not construct HTTP requests,
operate a competition webpage, or supply team, user, or credential identity
fields. The selected local session is the only source of identity.

Before an unfamiliar operation, query the CLI contract:

```bash
ctfpc9n-cli help
ctfpc9n-cli help <group>
ctfpc9n-cli help <group> <command>
```

All stdout is JSON. Inspect `ok` before using `data`. On `ok: false`, act on
the stable error `code`; only retry errors whose `retryable` field is true.
For example, use `jq` to inspect that JSON and gate access to `data`:

```bash
ctfpc9n-cli help runtime start | jq '.data'

ctfpc9n-cli --session <name> challenge list | jq -e \
  'if .ok then .data else error(.error.code + ": " + .error.message) end'
```

Do not parse JSON with shell text tools or use `data` when `ok` is false.

## Choose the correct session profile

Main-platform AI evaluations require v1.3.0 or newer. Run
`ctfpc9n-cli help evaluation` first. If missing, replace the old executable
with the matching verified release asset; do not fall back to competition
commands. Use the platform-provided API base, run ID and AI subidentity.

For AI evaluation operations use the `evaluation` group exclusively:

```bash
ctfpc9n-cli --session ai-agent --api-base <platform-api-base> evaluation auth login --token-stdin
ctfpc9n-cli --session ai-agent evaluation runs
ctfpc9n-cli --session ai-agent evaluation challenge list --run-id <run-id> --page 1 --page-size 50
ctfpc9n-cli --session ai-agent evaluation report --run-id <run-id> --page 1 --page-size 50
ctfpc9n-cli --session ai-agent evaluation auth logout
```

Only start a created run. Resume an already-running run without restarting it;
completed or timed_out runs are read-only by default. Resume a completed or
timed_out run only when the user explicitly requests it; the resume period
opens only new questions and questions that require retesting. Query leaf help
before runtime, attachment, Flag, patch and finish operations. Read all pages
according to total. Always submit Flags and tokens through stdin; patches are
uploaded from local files. Preserve request IDs on retries and never change the
supplied run ID or scope.

Evaluation challenge safety rules:

- In `challenge list` or `challenge get`, skip any item with `locked: true`; do
  not submit a Flag, start or extend its runtime, or submit a patch.
- If Flag submission returns `alreadySolved: true`, the challenge is already
  passed and its score will not change; do not submit that challenge again.
- If an operation returns `CONFLICT` with the message
  `续测时段只开放新增或需重测的题目`, skip that challenge without retrying;
  handle other `CONFLICT` errors according to the existing rules.
- A challenge with `kind: "unavailable"` is not evaluable; skip it. This is
  different from `locked`.
The server automatically updates rankings for explicitly public-ranked runs;
do not try to publish, create new runs, or change credentials.

Competition and evaluation sessions cannot be mixed or overwrite the opposite
kind. Use separate names. The remaining competition workflow below applies only
to competition sessions, not evaluations.

## Session Handling

Use an existing named session for normal operations. To import a new Agent
token, prefer sending one token line through protected stdin:

```bash
ctfpc9n-cli --session <name> auth login --api-base <https-url> --token-stdin
```

`auth login` also accepts `--token <value>` or the `CTFPC9N_TOKEN` environment
variable. An explicit `--token` or `--token-stdin` overrides the environment;
the two flags cannot be combined. Prefer stdin because command arguments may
be visible in process listings or shell history and environment variables may
be inherited by child processes. Do not echo, log, save, or put tokens or
Flags in filenames. End a temporary session with:

```bash
ctfpc9n-cli --session <name> auth logout
```

## Required Workflow

1. Run `stage progress` before reading challenges. When stage mode is enabled,
   use its server-provided unlock state and locked reasons; do not infer access
   from the competition mode or dependency graph locally.
2. List participant-visible challenges, then read an unlocked challenge.
3. Use a static `attachment_path` only when it appears in `challenge get`
   output. Supply a new explicit `--output` path for each download.
4. For dynamic attachments, query `attachment dynamic status`; download only
   after the returned status permits it. Do not assume regeneration is
   idempotent.
5. For a runtime, call `runtime start`, preserve its request ID, then call
   `runtime inspect`. Treat runtime state in successful JSON as business state,
   not a CLI error.
6. For every runtime write and Flag submission, generate one stable request ID
   and retain it in the current task state. Reuse the exact ID on a retry.
7. Once the complete Flag is known, submit it immediately through stdin with
   `submission flag --flag-stdin`.

Use leaf Help immediately before an operation to obtain its current request,
response, and argument contract.
