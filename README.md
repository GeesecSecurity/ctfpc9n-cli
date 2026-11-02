# ctfpc9n-cli

[中文文档](README-CN.md)

`ctfpc9n-cli` is a command-line tool for Agent-oriented
[CTFPlus](https://ctfplus.cn) CTF competitions and main-platform AI evaluations.

## What It Is For

An Agent can use `ctfpc9n-cli` throughout a competition to:

- inspect the stage graph and the current team's unlock progress;
- discover challenges and retrieve their materials;
- manage challenge runtimes;
- submit Flags; and
- inspect the competition rank.

The CLI is limited to participant competition operations and authorized AI evaluation runs. It does not
create or manage competitions, teams, challenges, credentials, or platform
accounts.

## For AI Agents

### Copyable Prompt

```text
Install ctfpc9n-cli from https://github.com/GeesecSecurity/ctfpc9n-cli/releases.
Select the release asset for this host, download its SHA256SUMS, verify the
asset checksum, then install it as $HOME/.local/bin/ctfpc9n-cli with mode 0755.
When npx is available, install the Agent Skill with:
npx skills@latest add -g https://github.com/GeesecSecurity/ctfpc9n-cli.git --skill ctfpc9n-cli -y
Otherwise download https://raw.githubusercontent.com/GeesecSecurity/ctfpc9n-cli/refs/heads/main/skills/ctfpc9n-cli/SKILL.md,
create ~/.agents/skills/ctfpc9n-cli/, and save it as SKILL.md. Run
ctfpc9n-cli help and verify the Skill with npx skills list or that file path.
Do not authenticate or ask for a token. Stop and report an error instead of
guessing an asset or checksum.
```

## For Humans

This tool is for CTFPlus participants who use AI Agents in a competition, and
for the people who set up or oversee those Agents.

1. Download the binary for the Agent host from
   [GitHub Releases](https://github.com/GeesecSecurity/ctfpc9n-cli/releases).
2. Place the binary on `PATH` with executable permissions.
3. Copy [`skills/ctfpc9n-cli/SKILL.md`](skills/ctfpc9n-cli/SKILL.md) to
   `~/.agents/skills/ctfpc9n-cli/SKILL.md`.


## AI evaluation (v1.3.1+)

Create an AI subidentity and an evaluation on the platform, then copy its supplied API base and run ID. Evaluation sessions are separate from competition sessions. The CLI does not create accounts, benchmarks or runs, and exposes no administrator or publication commands.

Replace the old binary with the matching v1.3.1+ release asset after verifying `SHA256SUMS`. Check the executable selected by `command -v ctfpc9n-cli`, then run `ctfpc9n-cli help evaluation` before connecting.

Provide one token/Flag line on protected stdin; do not put secrets in command arguments, logs or shell history. The API base and IDs below are supplied by the platform handoff:

```sh
ctfpc9n-cli --session ai-agent --api-base https://www.ctfplus.cn/api evaluation auth login --token-stdin
ctfpc9n-cli --session ai-agent evaluation runs
ctfpc9n-cli --session ai-agent evaluation run start --run-id RUN_ID --request-id start-request-001
ctfpc9n-cli --session ai-agent evaluation challenge list --run-id RUN_ID --page 1 --page-size 50
ctfpc9n-cli --session ai-agent evaluation submission flag --run-id RUN_ID --challenge-id CHALLENGE_ID --request-id flag-request-001 --flag-stdin
ctfpc9n-cli --session ai-agent evaluation run finish --run-id RUN_ID --request-id finish-request-001
ctfpc9n-cli --session ai-agent evaluation report --run-id RUN_ID --page 1 --page-size 50
ctfpc9n-cli --session ai-agent evaluation auth logout
```

Read all pages using `total`; unattempted questions still count toward the benchmark denominator. Query leaf help for attachments, runtime lifecycle and patch upload/submit/status. Patches come from local files. Use one stable request ID per write operation and preserve it with identical parameters on retry.

Completed or `timed_out` evaluations are read-only by default. Only start a completed or timed-out evaluation again when the user explicitly requests a resume; the new period opens only new questions and questions that require retesting. In `challenge list` and `challenge get`, skip items with `locked: true`: do not submit a Flag, start or extend a runtime, or submit a patch. If Flag submission returns `alreadySolved: true`, the challenge is already passed and its score will not change, so do not submit it again. A `CONFLICT` with message `续测时段只开放新增或需重测的题目` means the challenge is locked for this resume period; skip it without retrying, while other `CONFLICT` errors follow the normal retry rules. A challenge with `kind: "unavailable"` is not evaluable and must be skipped; it is distinct from `locked`.

Legacy sessions without a kind remain competition sessions. Neither session type may overwrite or operate as the other; choose separate names. Public-ranked evaluations update platform progress and final rankings automatically, without a CLI publish command. Model identity and token use remain unverified declarations.
