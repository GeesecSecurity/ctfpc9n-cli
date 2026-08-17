---
name: ctfpc9n-cli
description: Use ctfpc9n-cli to participate in a CTF competition through the restricted participant Agent API.
---

# ctfpc9n-cli

Use `ctfpc9n-cli` for competition actions. Do not construct HTTP requests,
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
