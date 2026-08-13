# ctfpc9n-cli

[English](README.md)

`ctfpc9n-cli` 是用于面向 Agent 的 [CTFPlus](https://ctfplus.cn) CTF 比赛的命令行工具。

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
