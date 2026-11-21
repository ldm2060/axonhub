# Kimi Code OAuth 最小身份对齐设计规格

**日期：** 2026-07-18
**状态：** 已批准

## Context

AxonHub 作为多 AI 提供商的统一 API 网关，通过 Kimi Code OAuth 通道支持 Kimi Code CLI 客户端。该通道实现了 RFC 8628 设备授权流、token 轮换与持久化、并发刷新保护、401 后强制刷新、以及与官方 CLI 一致的 HTTP 身份头。

本次工作源于对 Kimi Code OAuth 全链路的整体复核。审计对照了：

- 官方 `@moonshot-ai/kimi-code` npm 稳定版 0.27.0 的打包源码；
- 仓库内 Codex OAuth 的成熟结构；
- Kimi Code OAuth 初始实现及最近四次相关修复提交（`bc2857ee`、`e876cbc6`、`b2d5437e`、`ff1691c4`）。

## 调查证据

审计确认以下组件与上游协议一致或属于有意的本地增强：

| 组件 | 结论 | 依据 |
|------|------|------|
| 设备授权流 (RFC 8628) | 一致 | endpoint、grant type、polling 间隔与官方一致 |
| Token 轮换与保留 | 一致 | refresh token 回退、access token 保留策略正确 |
| 并发刷新保护 | 本地增强 | `pendingPersistence` + `ForceRefresh(failedAccessToken)` 是必要增强 |
| 401 后强制刷新 | 一致 | 触发条件与重试语义正确 |
| 身份头格式 | 存在偏差 | `CLIVersion` 固定为 0.26.0，官方当前为 0.27.0 |
| 平台/架构映射 | 一致 | `amd64 -> x64`、`arm64 -> arm64` 与官方 `deviceModel()` 一致 |
| Client ID | 一致 | `17e5f671-d194-4dfb-9706-5516cb48c098` 与官方匹配 |
| PKCE/state | 不适用 | Kimi 使用 device flow，不使用 authorization-code flow 的 PKCE |
| 前端轮询 | 一致 | `useDeviceFlow` 行为正确 |
| `refreshAndReplace` 显式 Authorization 赋值 | 无害冗余 | 代码冗余而非功能缺陷 |

## 根因

`llm/transformer/kimicode/constants.go:10` 中 `CLIVersion` 常量值为 `"0.26.0"`，而官方 `@moonshot-ai/kimi-code@0.27.0` bundle 中 `X-Msh-Version` 和 `User-Agent` 使用的版本为 `0.27.0`。此偏差导致 AxonHub 发出的身份头与当前官方 CLI 不一致：

- `X-Msh-Version: 0.26.0`（应为 `0.27.0`）
- `User-Agent: kimi-code-cli/0.26.0`（应为 `kimi-code-cli/0.27.0`）

## 批准范围

仅修正确认的上游身份偏差 `CLIVersion 0.26.0 -> 0.27.0`，并先更新现有官方身份测试做 TDD 红灯。具体：

1. 修改 `llm/transformer/kimicode/identity_test.go` 中 `TestBuildIdentityHeadersMatchesOfficialCLI` 的预期值：
   - `User-Agent` 预期从 `"kimi-code-cli/0.26.0"` 改为 `"kimi-code-cli/0.27.0"`
   - `X-Msh-Version` 预期从 `"0.26.0"` 改为 `"0.27.0"`
   - 其他 `X-Msh-*` 头预期不变，证明本次不改变 client ID、平台、设备 ID、设备模型或架构映射
2. 运行该测试，确认因生产常量仍为 0.26.0 而失败（TDD 红灯）
3. 修改 `llm/transformer/kimicode/constants.go:10` 的 `CLIVersion` 从 `"0.26.0"` 改为 `"0.27.0"`
4. 重跑测试确认转绿

## 关键文件

| 文件 | 行 | 角色 |
|------|----|------|
| `llm/transformer/kimicode/constants.go` | 10 | `CLIVersion` 常量，唯一生产代码修改点 |
| `llm/transformer/kimicode/identity_test.go` | 21, 23 | `TestBuildIdentityHeadersMatchesOfficialCLI` 中 `User-Agent` 和 `X-Msh-Version` 预期值 |

## TDD 流程

### 红灯

1. 修改测试预期值为 0.27.0
2. 运行 `cd llm && go test ./transformer/kimicode -run TestBuildIdentityHeadersMatchesOfficialCLI`
3. 预期失败，差异明确显示 `0.26.0`（实际）与 `0.27.0`（预期）

### 绿灯

1. 修改 `CLIVersion` 为 `"0.27.0"`
2. 重跑同一测试命令
3. 预期通过

### 回归

1. 运行 `cd llm && go test ./transformer/kimicode/...`
2. 所有测试通过

## 验证

按 AGENTS.md 要求，提交前执行完整验证：

### Go 格式

- `gofmt -l` 对改动文件无输出
- 清理 golangci-lint 缓存，`--new-from-rev` 检查避免缓存假阳性

### 根模块

- `go build ./...`
- `golangci-lint run --timeout 10m --max-same-issues 50 ./...`
- `go test ./...`

### llm 模块

- `cd llm && go build ./...`
- `cd llm && golangci-lint run --timeout 10m --max-same-issues 50 ./...`
- `cd llm && go test ./...`

### 运行时验收

因修改了后端 Go 代码，按项目规则须：

1. `go build -o axonhub.exe ./cmd/axonhub/`
2. 停止旧进程，启动新二进制，确认 8090 可用
3. 浏览器打开 Kimi Code 通道 OAuth 界面，确认页面可用且无控制台错误
4. 通过可观察网络证据确认身份头为 `X-Msh-Version: 0.27.0` 和 `User-Agent: kimi-code-cli/0.27.0`

真实 Kimi 账户的设备授权需要用户交互；若现有登录态不可用，应明确报告该边界，不能声称完成真实授权。

## 非目标

以下明确不在本次范围内：

1. 不清理 `retryingExecutor.refreshAndReplace` 中无害的显式 Authorization 赋值
2. 不扩张 API handler 或前端测试矩阵
3. 不修改 Kimi 自定义 `TokenProvider`（`pendingPersistence`、`ForceRefresh`、刷新 token 回退是必要增强）
4. 不修改设备流中的 PKCE/state（Kimi 使用 RFC 8628 device flow，不适用 authorization-code flow 的 PKCE）
5. 不修改 `internal/server/biz/kimi_code_identity_platform.go` 的 Go -> Node 架构映射
6. 不修改 OAuth endpoint、client ID、grant type、重试状态/退避、过期窗口、持久化格式或数据库结构
7. 不引入动态版本探测或运行时网络查询；身份仍由固定常量统一供 OAuth、模型和消息请求使用
8. 不修改前端 `useDeviceFlow` 或相关组件
