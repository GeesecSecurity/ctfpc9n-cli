# ctfpc9n-cli

`ctfpc9n-cli` is a JSON-only participant CLI for the competition `/agent/v1`
API. It is intended for automated competition agents. It does not create or
manage competitions, teams, questions, credentials, or platform accounts.

Every command writes exactly one JSON object to stdout. Success exits `0`,
local argument errors exit `2`, and authentication, API, or transport errors
exit `1`.

## For AI Agents

### Copyable Prompt

```text
Install ctfpc9n-cli from http://10.8.0.7/GeesecTeam/ctfpc9n-cli/releases.
Select the release asset for this host, download its SHA256SUMS, verify the
asset checksum, then install it as $HOME/.local/bin/ctfpc9n-cli with mode 0755.
When npx is available, install the Agent Skill with:
npx skills@latest add http://10.8.0.7/GeesecTeam/ctfpc9n-cli.git --skill ctfpc9n-cli -y
Otherwise clone http://10.8.0.7/GeesecTeam/ctfpc9n-cli.git and copy its
skills/ctfpc9n-cli/SKILL.md to ~/.agents/skills/ctfpc9n-cli/SKILL.md. Run
ctfpc9n-cli help and verify the Skill with npx skills list or that file path.
Do not authenticate or ask for a token. Stop and report an error instead of
guessing an asset or checksum.
```

### Installation Details

Install the binary and Skill from the same trusted release tag. Your task or
orchestrator must provide these immutable inputs; do not guess a repository,
use a `latest` URL, or download a binary without its matching checksum:

- `ASSET_URL`: direct URL for the host's release binary;
- `CHECKSUM_URL`: direct URL for `SHA256SUMS` from that release;
- `SKILL_SOURCE_URL`: direct Git URL for `skills/ctfpc9n-cli` at that same
  tag.

For controlled environments, provide the three values above and apply this
more detailed installation policy:

```text
Install ctfpc9n-cli for this Agent. Determine the current POSIX platform:
linux-amd64, darwin-amd64, or darwin-arm64. The supplied ASSET_URL must name
the matching ctfpc9n-cli release asset; fail rather than selecting a different
architecture.

Download ASSET_URL and CHECKSUM_URL into a new private temporary directory.
Verify that SHA256SUMS contains exactly one checksum for the downloaded asset,
then compare it with sha256sum (or shasum -a 256). Stop on any mismatch.
Install the verified binary as $HOME/.local/bin/ctfpc9n-cli with mode 0755,
creating the directory when needed. Confirm that $HOME/.local/bin is on PATH
or report the required PATH change. Do not use sudo and do not replace a
different existing binary without reporting it.

Install the Skill pinned to the same tag:
  npx skills@latest add "$SKILL_SOURCE_URL" -y
If the host requires an explicit target agent, inspect `npx skills add --help`
and add its supported --agent value. Verify installation with `npx skills list`.

Finally run `ctfpc9n-cli help`, confirm it emits one JSON object, and report
the installed binary path and Skill installation result. Do not authenticate,
do not request a token, and do not print any secret.
```

`npx skills add` accepts a direct Skill path in a Git repository and installs
it into detected Agent hosts. This repository keeps the installed Skill thin:
it delegates current command details to `ctfpc9n-cli help`, so command
documentation cannot drift from the installed binary. See the
[Skills CLI documentation](https://github.com/vercel-labs/skills) for target
selection and non-interactive options. When Node or `npx` is unavailable,
copy `skills/ctfpc9n-cli/SKILL.md` to
`~/.agents/skills/ctfpc9n-cli/SKILL.md`; most Agent hosts discover Skills from
that directory.

### POSIX Installation Example

The prompt above is the required policy. This is the corresponding minimal
shell sequence after trusted URLs and the expected asset name are supplied:

```bash
ASSET=ctfpc9n-cli-linux-amd64
ASSET_URL='https://trusted.example/releases/download/vX.Y.Z/ctfpc9n-cli-linux-amd64'
CHECKSUM_URL='https://trusted.example/releases/download/vX.Y.Z/SHA256SUMS'
SKILL_SOURCE_URL='https://trusted.example/owner/ctfpc9n-cli/tree/vX.Y.Z/skills/ctfpc9n-cli'

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT
curl --fail --location --output "$workdir/$ASSET" "$ASSET_URL"
curl --fail --location --output "$workdir/SHA256SUMS" "$CHECKSUM_URL"
expected="$(awk -v asset="$ASSET" '$2 == asset { print $1 }' "$workdir/SHA256SUMS")"
test "$(printf '%s\n' "$expected" | wc -l | tr -d ' ')" = 1
test -n "$expected"
if command -v sha256sum >/dev/null; then
  actual="$(sha256sum "$workdir/$ASSET" | awk '{ print $1 }')"
else
  actual="$(shasum -a 256 "$workdir/$ASSET" | awk '{ print $1 }')"
fi
test "$actual" = "$expected"
mkdir -p "$HOME/.local/bin"
install -m 0755 "$workdir/$ASSET" "$HOME/.local/bin/ctfpc9n-cli"
npx skills@latest add "$SKILL_SOURCE_URL" -y
ctfpc9n-cli help
```

## For Humans

1. Open the intended, tagged Release and download both the platform asset and
   its `SHA256SUMS` file. Verify the checksum before executing the binary.
2. Place the verified binary on `PATH` with executable permissions. No runtime
   or package-manager dependency is required.
3. Use `ctfpc9n-cli help` to discover commands. The Skill is optional for
   people, but an Agent host can install it with `npx skills@latest add` and a
   direct tag-pinned `skills/ctfpc9n-cli` URL as described above.

Release assets are:

| Platform | Asset |
| --- | --- |
| Linux x86_64 | `ctfpc9n-cli-linux-amd64` |
| macOS Intel | `ctfpc9n-cli-darwin-amd64` |
| macOS Apple Silicon | `ctfpc9n-cli-darwin-arm64` |
| Windows x86_64 | `ctfpc9n-cli-windows-amd64.exe` |

On PowerShell, verify a downloaded Windows binary with:

```powershell
(Get-FileHash .\ctfpc9n-cli-windows-amd64.exe -Algorithm SHA256).Hash
```

Compare the result, case-insensitively, with that asset's line in
`SHA256SUMS` before moving the file onto `PATH`.

## First Session

Pass the Agent token only through stdin. `auth login` validates it with the
participant challenge-list endpoint before writing the named session.

```bash
ctfpc9n-cli --session contest-a auth login \
  --api-base https://competition.example \
  --token-stdin < /run/secrets/agent-token
```

The normalized API base is saved with its token and creation time under
`~/.local/state/ctfpc9n-cli/sessions/`. Set `CTFPC9N_STATE_DIR` to an absolute
private workspace path to isolate state. Session directories are `0700` and
session files are `0600`.

The API base is an HTTP(S) origin or an existing `/agent/v1` base. The CLI adds
`/agent/v1` when it is absent. Ordinary commands never accept `--api-base`;
they use the selected session.

```bash
ctfpc9n-cli --session contest-a auth logout
```

Logout is local and succeeds even when the session was already absent.

## Discover Commands

Use JSON Help rather than relying on this README for argument details:

```bash
ctfpc9n-cli help
ctfpc9n-cli help challenge
ctfpc9n-cli help runtime start
ctfpc9n-cli runtime start --help
```

Leaf Help includes the request schema, response JSON Schema, sensitive stdin
markers, global options, and prerequisites. `help <command>` and
`<command> --help` return the same JSON data.

## Participant Workflow

```bash
ctfpc9n-cli --session contest-a challenge list --tag web --type 1
ctfpc9n-cli --session contest-a challenge get --challenge-id 1001

ctfpc9n-cli --session contest-a attachment download \
  --challenge-id 1001 \
  --attachment-path /attachments/task.zip \
  --output ./task.zip
```

The attachment path must be copied from `challenge get`. Downloads require an
explicit output path, create parent directories with private permissions, and
never overwrite an existing target.

For dynamic attachments, read status before downloading. Regeneration requires
an audit ID but its retries are not idempotent.

```bash
ctfpc9n-cli --session contest-a attachment dynamic status \
  --challenge-id 1002 --wait-seconds 30

ctfpc9n-cli --session contest-a attachment dynamic download \
  --challenge-id 1002 --output ./dynamic.zip
```

Runtime writes and Flag submission require a stable `--request-id` of up to 64
characters. Reuse the same ID for a retry. `runtime start` initiates the start
only; inspect it as a separate call.

```bash
ctfpc9n-cli --session contest-a runtime start \
  --challenge-id 1001 --request-id runtime-start-1001-a
ctfpc9n-cli --session contest-a runtime inspect \
  --challenge-id 1001 --wait-seconds 10

printf '%s\n' "$FLAG" | ctfpc9n-cli --session contest-a submission flag \
  --challenge-id 1001 --request-id submission-1001-a --flag-stdin
```

Do not put Agent tokens or Flags in command-line arguments, output paths, or
logs. The CLI redacts sensitive values in errors, but successful participant
API data is returned unchanged.

## Development

Requirements:

- Go `1.25.3` or newer;
- `goctl 1.10.1` to validate and regenerate the checked-in API client.

```bash
make generate
make test
make ctfpc9n-cli
make release
```

`contracts/runtime/` is a copied upstream API snapshot. The generator exposes
only the 11 entries in `contracts/agent-endpoints.json`; types and endpoint
metadata in `internal/generated/agentapi/` are checked in.
