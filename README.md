# ctfpc9n-cli

[中文文档](README-CN.md)

`ctfpc9n-cli` is a command-line tool for Agent-oriented
[CTFPlus](https://ctfplus.cn) CTF competitions.

## What It Is For

An Agent can use `ctfpc9n-cli` throughout a competition to:

- inspect the stage graph and the current team's unlock progress;
- discover challenges and retrieve their materials;
- manage challenge runtimes;
- submit Flags; and
- inspect the competition rank.

The CLI is limited to the participant side of a competition. It does not
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
