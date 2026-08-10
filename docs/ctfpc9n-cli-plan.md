# ctfpc9n-cli 设计与实现方案

## 1. 产品定位

`ctfpc9n-cli` 是供 AI 参赛 Agent 使用的独立命令行程序和 Skill 发布包。它只调用比赛
后端的 `/agent/v1` 参赛 API，身份、队伍隔离、题目可见性、容器生命周期、Flag 判定、
幂等和审计仍由比赛后端负责。

本项目以 `ctfplus-cli` 为实现参考：沿用其声明式命令结构、具名 session、机器 JSON
输出、错误脱敏、原子文件写入、契约生成和 `httptest` 测试模式。它是独立仓库和独立
二进制，不导入或扩展 `ctfplus-cli` 的平台运维能力。

### Agent-first

CLI 的唯一一等使用者是 Agent，而不是交互式终端用户。因此命令发现、参数校验、成功
结果和失败结果都必须是稳定 JSON；不提供 `--human`、彩色终端文本、进度条或仅靠 README
才能发现的隐式行为。每条命令的自描述信息必须可由 Agent 在不发起业务请求的情况下读取。

## 2. 边界

只导出以下参赛能力：题目、附件、动态附件、运行环境、Flag 提交和排行榜。

不得实现或调用：

- 平台账号登录、主平台 JWT、比赛创建与配置、题目管理、队伍管理或管理员接口；
- Agent Credential 的签发、轮换、吊销或状态查询；
- 数据库、MarioSDK、比赛业务配置、题目/容器/判题/排名逻辑；
- 身份输入中的 `teamId`、`userId`，或任意用户指定的请求 URL；
- 除本方案列出的 `/agent/v1` 端点外的比赛接口。

## 3. 后端契约和命令面

新仓库维护一份受控的 `contracts/runtime/` 后端 API 完整快照，并以
`contracts/agent-endpoints.json` 限定下表的 11 个参赛调用。后端修改这些接口时必须先
同步快照和 manifest，再重新生成客户端类型；生成类型不自动扩大可调用端点范围。

| CLI 命令 | Agent REST | 说明 |
|---|---|---|
| `challenge list` | `POST /agent/v1/challenges/list` | 按标签、类型列题 |
| `challenge get` | `POST /agent/v1/challenges/detail` | 读取题目详情和附件清单 |
| `attachment download` | `POST /agent/v1/attachments/download` | 下载静态附件到本地文件 |
| `attachment dynamic status` | `POST /agent/v1/attachments/dynamic/status` | 查询、等待或请求重新生成 |
| `attachment dynamic download` | `POST /agent/v1/attachments/dynamic/download` | 下载当前队伍动态附件 |
| `runtime start` | `POST /agent/v1/runtimes/start` | 请求启动运行环境 |
| `runtime inspect` | `POST /agent/v1/runtimes/inspect` | 查询平台登记端点及连通性 |
| `runtime stop` | `POST /agent/v1/runtimes/stop` | 停止运行环境 |
| `runtime extend` | `POST /agent/v1/runtimes/extend` | 延长运行环境有效期 |
| `submission flag` | `POST /agent/v1/submissions/flag` | 提交候选 Flag |
| `rank list` | `POST /agent/v1/rank` | 查询排行榜 |

`auth login` 与 `auth logout` 只管理本地 session，不新增比赛业务端点。除 `auth` 命令外，
所有命令都要求 `--session NAME`。

输入约束：

- `challenge-id` 必须为正数；
- 静态附件的 `attachment-path` 必须原样使用 `challenge get` 返回的
  `attachments[].attachment_path`；
- 动态附件和运行环境等待时间最大为 30 秒；动态附件仅在状态为 `ready` 时下载；
- `runtime start`、`runtime stop`、`runtime extend`、`submission flag` 强制
  `--request-id`，最大 64 字符，同时写入 JSON 请求体和 `X-Request-Id`；重试复用同一值；
- `submission flag` 仅支持 `--flag-stdin`，从标准输入读取一行 Flag；
- `runtime start` 成功后必须继续 `runtime inspect`；`runtime extend` 不用于修复端口
  连通性。

建议调用形式：

```bash
ctfpc9n-cli auth login --session contest-a --api-base https://competition.example/api --token-stdin < /run/secrets/agent-token

ctfpc9n-cli --session contest-a challenge list --tag web --type 1
ctfpc9n-cli --session contest-a challenge get --challenge-id 1001 --page 1 --size 100
ctfpc9n-cli --session contest-a attachment download --challenge-id 1001 --attachment-path /attachments/task.zip --output ./task.zip
ctfpc9n-cli --session contest-a attachment dynamic status --challenge-id 1002 --wait-seconds 30
ctfpc9n-cli --session contest-a attachment dynamic download --challenge-id 1002 --output ./dynamic.zip
ctfpc9n-cli --session contest-a runtime start --challenge-id 1001 --request-id runtime-start-1001-a
ctfpc9n-cli --session contest-a runtime inspect --challenge-id 1001 --wait-seconds 10
ctfpc9n-cli --session contest-a submission flag --challenge-id 1001 --request-id submit-1001-a --flag-stdin
```

## 4. 认证和本地 session

认证复用 `ctfplus-cli` 的具名 session 模式，但不复用其主平台登录或运行态 Cookie 交换。
Agent Token 已能直接访问比赛后端，因此只需要安全导入和持久化。

`auth login`：

1. 接受 `--session NAME`、`--api-base URL` 和 `--token-stdin`。
2. 从 stdin 读取一行原始 Agent Token；不支持 `--token`、Token 环境变量、用户名或密码。
3. 通过最小的 `POST /agent/v1/challenges/list` 请求确认 Token 已通过 Agent 鉴权。
4. 鉴权通过后，以原子方式保存 session；鉴权失败、网络失败或不可信响应时不写文件。

session 文件位于 `~/.local/state/ctfpc9n-cli/sessions/<name>.json`。`CTFPC9N_STATE_DIR`
可以为隔离的 Agent 工作空间指定状态目录。目录权限为 `0700`，session 文件以临时文件、
原子 rename 和 `0600` 权限写入。内容仅包括规范化后的 Agent API 基址、Agent Token 和
创建时间。

`auth logout --session NAME` 删除指定 session。Token 过期或已吊销时，普通命令返回稳定的
鉴权错误，Agent 重新执行 `auth login` 覆盖 session。

所有参赛请求从 session 读取 Token 并组装 `Authorization: Bearer <token>`。Token 不得
出现在 stdout、stderr、错误对象、审计日志、文件名或 Skill 内容中。session 适合受控的
单用户 Agent 工作空间；在不可信主机或任务结束即销毁的环境中，应使用私有状态卷并在
任务结束后执行 `auth logout` 或删除整个状态目录。

全局选项：

- `--session NAME`：选择本地 session；
- `--timeout DURATION`：HTTP 超时；
- `--insecure-skip-tls-verify` 或 `-K`：仅限显式开发环境，默认严格验证 TLS 证书。

## 5. Agent-first Help、输出和文件产物

所有命令、帮助和失败默认且唯一的 stdout 格式为单行 JSON；JSON 模式不向 stderr 写
内容。成功退出码为 `0`；本地参数错误使用 `2`；认证、后端或传输失败使用 `1`。

Help 是命令契约的一部分，必须支持以下等价调用：

```bash
ctfpc9n-cli help
ctfpc9n-cli help challenge
ctfpc9n-cli help runtime start
ctfpc9n-cli runtime start --help
```

根 Help 返回完整命令目录；分组 Help 返回该分组的全部后代；每个叶子命令的 Help 返回
同一份命令元数据。`<command> --help` 必须忽略该命令原本要求的业务参数，并与
`help <command>` 输出完全一致。

参考 `ctfplus-cli` 的 `commandCatalog`、Kong `Trace` 和 `resultSchema` 机制，命令元数据
从同一份 Kong command schema、端点 manifest 和生成类型生成，至少包含：

- `path`、`summary`、`prerequisites` 和可直接执行的 `usage`；
- `flags` 与 `globalFlags`：每个参数的名称、类型、说明、必填性、required-one-of、默认值、
  enum、可重复性和敏感标记；
- `request`：传输类型、HTTP Method、固定路径、headers 来源、body JSON Schema 和 stdin
  参数。`Authorization` 只标记来源为 session 且为 sensitive，不能出现实际 Token；
- `resultSchema`：JSON Schema Draft 2020-12，完整描述下方 result envelope 与该命令
  `data` 的后端响应类型；
- 成功响应 `data` 按后端类型原样返回；Token、Flag、Cookie、Password 和 Secret 只在错误
  输出前统一脱敏。

例如，`help runtime start` 的 JSON data 应包含以下结构，字段 schema 由契约生成而非手写：

```json
{
  "path": "runtime start",
  "usage": "ctfpc9n-cli --session NAME runtime start --challenge-id ID --request-id ID",
  "flags": [{"name": "--challenge-id", "type": "uint64", "required": true}],
  "request": {
    "transport": "http",
    "method": "POST",
    "path": "/agent/v1/runtimes/start",
    "headers": [
      {"name": "Authorization", "source": "session", "sensitive": true},
      {"name": "X-Request-Id", "source": "--request-id"}
    ],
    "bodySchema": {"$ref": "#/$defs/agentapi.AgentRuntimeRequest"}
  },
  "resultSchema": {"$schema": "https://json-schema.org/draft/2020-12/schema"}
}
```

`auth login` 的 request 元数据声明 `--token-stdin` 为敏感 stdin 输入和其鉴权请求；
`auth logout` 声明本地 session 删除操作，不伪造 HTTP 调用。

```json
{"schemaVersion":"v1","ok":true,"command":"runtime inspect","data":{"status":"ready"}}
```

```json
{"schemaVersion":"v1","ok":false,"command":"runtime start","error":{"code":"UNAVAILABLE","message":"...","retryable":true}}
```

沿用 `ctfplus-cli` 的错误分类和敏感字段脱敏策略。Token、Flag、Cookie、Password 和
Secret 必须在错误输出前被统一脱敏。CLI 不输出调试日志，也不维护本地操作审计日志。

附件命令要求显式 `--output PATH`。下载内容流式写入临时文件，设置 `0600` 权限后原子
rename 到目标路径；失败不能覆盖已有目标文件。成功后仅输出文件 manifest：

```json
{"schemaVersion":"v1","ok":true,"command":"attachment download","data":{"output":"/abs/task.zip","fileName":"task.zip","contentType":"application/zip","bytes":12345}}
```

## 6. Skill 发布包

发布一个 `ctfpc9n-cli` Skill：

```text
skills/
└── ctfpc9n-cli/
    └── SKILL.md
```

Skill 只指导 Agent 调用 `ctfpc9n-cli`，不封装 HTTP 或复制业务逻辑。它必须声明：

1. 身份只来自受保护的本地 session，不能构造身份字段或操作网页；
2. 先列题、后读题，附件路径只取自题目详情；
3. 动态附件先查询状态，运行环境先启动再检查；
4. 写操作必须生成、保留并复用 `request-id`；
5. 得到完整 Flag 后立即使用 `submission flag` 提交；
6. 不得回显或持久化受保护 session 之外的 Token、Flag 和敏感响应。

Skill 使用标准 `SKILL.md` 格式。具体安装路径由 Agent 宿主决定，二进制中不写死任何
宿主目录。

## 7. 仓库结构和实现约定

```text
.
├── cmd/ctfpc9n-cli/
│   └── main.go                 # 进程入口和退出码
├── contracts/
│   ├── runtime/                # 后端 API 完整快照
│   └── agent-endpoints.json     # 11 个允许生成和调用的端点
├── internal/
│   ├── cli/                    # Kong 命令 schema、分发和 JSON 输出
│   ├── generated/agentapi/     # 从快照和 endpoint manifest 生成的类型和端点定义
│   ├── agentapi/               # Bearer REST 调用、输入校验、附件流下载
│   └── session/                # session 的安全读写、加载和删除
├── skills/
│   └── ctfpc9n-cli/
│       └── SKILL.md
├── tools/                      # 固定版本的契约生成工具与脚本
├── docs/
│   └── ctfpc9n-cli-plan.md
├── README.md
├── go.mod
└── go.sum
```

采用 `ctfplus-cli` 已使用的 `kong` 命令 schema 和固定版本 `goctl` 契约生成流程。命令
schema 是唯一的对外 CLI 清单；生成类型不能自动变成可调用命令。HTTP 传输、JSON 编解码
和文件写入使用 Go 标准库。

实现按以下 `ctfplus-cli` 模式对齐：

| 参考实现 | ctfpc9n-cli 落地 |
|---|---|
| `command_schema.go` | 只声明 `auth`、`challenge`、`attachment`、`runtime`、`submission`、`rank` 六个命令域 |
| `commandCatalog` | 封闭的命令目录，同时驱动根 Help、分组 Help、叶子 Help 和命令完整性测试 |
| Kong `Trace` | 从实际解析规则生成 usage、flags、required-one-of、enum 和默认值，避免另写一份 Help 参数表 |
| `result_schema.go` | 为每个叶子命令生成 model-backed JSON Schema，覆盖 result envelope 与 `data` 响应 |
| generated endpoint definitions | 生成每个命令的 Method、路径、body schema、response schema；Help 的 `request` 直接引用这些定义 |
| `output.go` | 使用版本化 JSON envelope、稳定错误码、退出码映射与敏感字段声明 |
| `session.go` | 使用具名 session、`0700` 状态目录、`0600` 原子文件和 logout |
| agent contract/parse tests | 逐条验证 JSON Help、inline Help 等价性、request/response schema 和默认 JSON 错误 |

不新增自定义认证框架、插件系统、配置层或抽象 HTTP SDK。`agentapi` 只处理本方案中
未由契约表达的行为：Bearer 注入、`X-Request-Id`、错误标准化、敏感脱敏和二进制附件
流写入。

## 8. 实施顺序

1. 建立新仓库、Go module、固定版本的 `goctl` 生成工具、完整契约快照、endpoint manifest
   和空的 Kong command schema。
2. 先实现 command catalog、JSON Help 和 result schema；为每个叶子命令写测试，验证
   `help <command>` 与 `<command> --help` 等价，且包含完整 request/response 元数据。
3. 以 `httptest` 写 `auth login/logout` 的失败测试：Token 仅从 stdin 读取、鉴权失败不落盘、
   session 目录 `0700`、session 文件 `0600`、logout 后不能加载。
4. 实现 session、统一 JSON result、错误脱敏和 Agent REST 客户端。
5. 按“只读、运行环境写操作、Flag 提交、附件流下载”的顺序实现命令；每个命令先补精确
   的 Method、路径、Bearer、请求体和 `X-Request-Id` 测试。
6. 编写 `ctfpc9n-cli` Skill 和命令参考；发布各平台静态二进制。

## 9. 验收标准

- `ctfpc9n-cli help` 只列出 `auth login/logout` 和第 3 节定义的参赛命令；
- 每个叶子命令同时支持 `help <command>` 和 `<command> --help`，两者输出完全相同的 JSON；
- 每条命令 Help 都含有 flags、global flags、request 元数据和 model-backed `resultSchema`；
- 正常结果、Help、参数错误、鉴权错误和后端错误均为版本化 JSON，且 JSON 模式不写 stderr；
- 只生成和调用第 3 节列出的 11 个 Agent REST 调用；
- 所有参赛命令强制 `--session`，不存在 Token 环境变量或 `--token`；
- `auth login` 仅从 stdin 接收 Token，鉴权成功后原子保存 `0700`/`0600` session；
- `auth logout` 完整删除 session，Token/Flag 不会出现在输出或本地日志；
- 所有写操作发送稳定的 body `requestId` 和 `X-Request-Id`，重试不会静默改写该值；
- 附件只写入显式目标路径，失败不覆盖既有文件；
- `go test -mod=readonly ./...` 和 `go build ./cmd/ctfpc9n-cli` 通过；
- `ctfplus-cli` 不发生代码、依赖、命令或发布物变更。
