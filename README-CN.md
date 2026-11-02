# ctfpc9n-cli

[English](README.md)

`ctfpc9n-cli` 是用于面向 Agent 的 [CTFPlus](https://ctfplus.cn) CTF 比赛与主平台 AI 评测的命令行工具。

## 用途

Agent 可以在比赛过程中使用 `ctfpc9n-cli`：

- 查看闯关图和当前队伍的解锁进度；
- 查询题目并获取题目材料；
- 管理题目运行环境；
- 提交 Flag；
- 查看比赛排行榜。

## 面向 AI Agent

### 可直接使用的提示词

```text
从 https://github.com/GeesecSecurity/ctfpc9n-cli/releases 安装 ctfpc9n-cli。
选择当前主机对应的发布产物，下载 SHA256SUMS，校验产物的校验和，再以 0755 权限安装到
$HOME/.local/bin/ctfpc9n-cli。npx 可用时，使用以下命令安装 Agent Skill：
npx skills@latest add -g https://github.com/GeesecSecurity/ctfpc9n-cli.git --skill ctfpc9n-cli -y
否则，下载 https://raw.githubusercontent.com/GeesecSecurity/ctfpc9n-cli/refs/heads/main/skills/ctfpc9n-cli/SKILL.md，
创建 ~/.agents/skills/ctfpc9n-cli/，并将其保存为 SKILL.md。执行 ctfpc9n-cli help，并通过
npx skills list 或检查该文件路径来验证 Skill。不要认证，
不要索要 Token。无法确定发布产物或校验和时停止并报告错误，不要猜测。
```

## 面向使用者

该工具面向在 CTFPlus 比赛中使用 AI Agent 的参赛者，以及负责部署或监管这些 Agent 的人员。

1. 从 [GitHub Releases](https://github.com/GeesecSecurity/ctfpc9n-cli/releases)
   下载运行 Agent 的主机对应的二进制文件。
2. 以可执行权限将二进制文件放入 `PATH`。
3. 将 [`skills/ctfpc9n-cli/SKILL.md`](skills/ctfpc9n-cli/SKILL.md) 复制到
   `~/.agents/skills/ctfpc9n-cli/SKILL.md`。


## AI 评测（v1.3.1 起）

AI 评测使用主平台创建的专用 AI 子身份，不使用比赛会话。先在平台创建身份和评测，复制接入说明中的主平台 API 地址、评测 ID。CLI 不创建身份/题集/评测、不管理管理员功能，也不会替用户发布成绩。

升级时重新下载 v1.3.1 或更新版本的系统/架构对应产物，校验 `SHA256SUMS`，替换 `command -v ctfpc9n-cli` 指向的旧程序，然后验证：

```sh
ctfpc9n-cli help evaluation
ctfpc9n-cli help evaluation auth login
```

以下示例的 API 地址和 run ID 由平台接入说明提供；Token/Flag 通过受保护的 stdin 输入一行，不放到命令参数、日志或 shell 历史：

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

- `challenge list` 与 `report` 均分页，按 `total` 读取完整范围；未尝试题目仍计入评测分母。
- 附件下载、容器 start/inspect/stop/extend、补丁 upload/submit/status 的参数以对应 `help evaluation ...` 为准。补丁从本地文件上传，Flag 与 Token 仅从 stdin 输入。
- 每个不同写操作使用独立 request ID；重试同一操作必须保留原 ID 和参数。
- `completed` 或 `timed_out` 的评测默认只读。只有用户明确要求续测时，才再次执行 `run start`；新的时段只开放新增题和需要重测的题。
- 在 `challenge list` 或 `challenge get` 中看到 `locked: true` 的题，直接跳过：不要提交 Flag、启动或续期环境，也不要提交补丁。Flag 提交返回 `alreadySolved: true` 时，表示题目已经通过、成绩不会变化，不要再次提交。
- 收到 `CONFLICT` 且 message 为「续测时段只开放新增或需重测的题目」时，跳过该题且不要重试；其他 `CONFLICT` 仍按原有规则处理。`kind: "unavailable"` 的题不可评测，直接跳过；它与 `locked` 是两回事。
- 旧的无 kind 会话仍按比赛会话读取；比赛与评测不能混用或覆盖同名的另一类型会话。为评测使用独立会话名。
- 勾选公开打榜的运行由平台判题和结算自动更新排行榜；CLI 无需也不能手动发布成绩。模型身份和 Token 用量不因使用此 CLI 而得到平台验证。
