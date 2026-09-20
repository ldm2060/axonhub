# 渠道共享与渠道可用性控制设计

**日期：** 2026-09-20
**状态：** 已实现
**提交：** `2ac6fe17`（共享链路修复）、`94203204`（按归属/共享/发布控制可用性）

## 背景

本 fork 把 AxonHub 改造为"用户级管理"：每个用户拥有自己的渠道（`channel.owner_id`）、模型，并用 `visibility`（`private` / `shared` / `published`）+ `shared_with`（用户 ID 数组）表达共享。

两个问题：

1. **共享 UI 完全不可用。** 四处 bug，根因都是"`shared_with` 里是裸数字 ID，而 UI 里流转的用户 ID 是 GUID（`gid://axonhub/User/2`）"：
   - `frontend/src/features/channels/data/shared.ts` 用 `Number(currentUser.id)` 与 `sharedWith` 比对 → `NaN` → 接收方的"共享"标签页恒为空；
   - 共享弹窗用 `users` 列表（需要 `read_users`，且只有前 100 个）匹配 `edge.node.id === String(userId)` → 永远匹配不上 → 列表回退显示裸编号；没有 `read_users` 的用户调 `users` 只返回自己 → 下拉框为空，根本无法共享；
   - 取消共享传裸数字 `"2"`，而 `ID` 标量要求 `gid://axonhub/` 前缀 → 报 `guid must start with gid://axonhub/`；
   - 共享成功后弹窗仍显示打开时那一行（陈旧数据）。
2. **共享不构成任何约束。** `ChannelService.reloadEnabledChannels` 把**所有 enabled 渠道**无差别缓存，`orchestrator.DefaultSelector` 不做任何用户过滤 → 任何 API key 都能路由到任何渠道，包括别人的 private 渠道。"共享"只影响看得见什么。

## 数据模型

本次**没有 schema 变更**：`owner_id`、`visibility`、`shared_with` 都是既有字段，新增的 `Channel.sharedUsers` 与 `shareableUsers` 是读时计算的 GraphQL 字段。**不需要迁移脚本。**

## 规则一：可见性（沿用既有设计，本次未改）

- privacy 规则 `scopes.ChannelVisibilityQueryRule`：持有 `read_channels` 的用户**绕过**可见性过滤（能查到全部渠道）；其余用户只能查到 `published` 或自己的。
- 「我的渠道」三标签：公开 = `published`；共享 = `mySharedChannels`（服务端按 `shared_with` 判定当前用户）；我的 = `owner_id = 自己`。
- 因此：**可见 ≠ 可用**。管理页看到别人的 private 渠道，不代表能调用。

## 规则二：可用性（本次新增，`scopes.CanRouteThroughChannel`）

请求的**行为主体**（acting user）：

| 请求来源 | 行为主体 |
|---|---|
| 网页会话（JWT） | 登录用户 |
| API key（type `user` / `personal`，且 `user_id` 非空） | 该 key 的属主用户 |
| API key（`service_account` / `noauth`）或无主体 | 无 → 走系统路径，不设限 |

主体可用的渠道条件（**任一**成立即可）：

| 条件 | 说明 |
|---|---|
| `user.IsOwner` 或持有 `write_channels` | 管理员/系统所有者；渠道测试功能依赖此项 |
| `channel.owner_id = user.ID` | 自己的渠道 |
| `visibility = published` | 发布给所有人 |
| `visibility = shared` 且 `user.ID ∈ shared_with` | 共享给该用户 |
| `channel.owner_id = 0/NULL` | 无主渠道（早于归属特性的历史数据）= 系统渠道，对所有人开放，避免升级后把老数据锁死 |

其余情况一律拒绝。注意 `read_channels` **刻意不作为放行条件**：自注册用户的默认 scopes（`biz.DefaultUserScopes`）就包含它，用它当门禁等于没有门禁。

### 边界与已知取舍

- `allow_no_auth: true` 时，未携带任何凭证的请求会退化成 noauth key → 无主体 → 不设限。这是既有设计（开放访问），不是本规则的漏洞。
- service account key 属于项目而非用户，不参与用户级门禁，保持系统路径。
- 上游（upstream）若引入自己的渠道/项目隔离机制，需要判断与本规则是否冲突，见下文合并注意点。

## 落点与实现要点

| 位置 | 作用 |
|---|---|
| `internal/scopes/rule_channel_usage.go` | 规则唯一来源 `CanRouteThroughChannel` |
| `internal/server/orchestrator/channel_access.go` | 主体解析 + 候选过滤 |
| `internal/server/orchestrator/candidates.go` → `DefaultSelector.Select` | 在**选完候选之后**过滤（两条 return 路径都要过） |
| `internal/server/biz/model.go` → `ListEnabledModels` | 模型列表按同一规则裁剪 |
| `internal/server/middleware/auth.go` → `withAPIKeyPrincipalUser` | 把 API key 属主写进 context |
| `internal/contexts/{container,context}.go` | `WithPrincipalUser` / `GetPrincipalUser` / `GetActingUser` |
| `internal/server/biz/user.go` | `SharedUsersByIDs` / `SearchShareableUsers`（供共享 UI 显示名字、选人） |
| `internal/server/gql/axonhub.graphql`、`publish_request.graphql` | `Channel.sharedUsers`、`Query.shareableUsers` |

**为什么必须在 `Select` 之后过滤，而不是过滤渠道缓存：** enabled-channel 缓存与关联（association）缓存按「渠道数 + 最新更新时间」做 key，且被所有请求共享。一旦把用户维度的过滤放进缓存层，缓存就会被第一个访问它的用户"毒化"，其他用户拿到错误的候选集。放在选择之后，缓存依旧与用户无关。

**粘性路由**（trace/thread sticky）通过 `hasCandidate(channelID)` 在候选集里查，因此被过滤掉的渠道不会被固定选中。

## 各 API 的暴露面（实测）

| 入口 | 行为 |
|---|---|
| `GET /v1/models` | 只列可用渠道的模型 |
| `GET /v1/models/{model}` | 不可用的模型返回 404 `model_not_found` |
| `GET /anthropic/v1/models` | 同上（走同一个 `ListEnabledModels`） |
| `GET /gemini/v1beta/models` | 同上 |
| `POST /v1/graphql`（API key 可调用） | 只暴露 API key 自身管理，无渠道/模型查询 |

## 合并上游时的注意点（重要）

上游 `looplj/axonhub` 与这些位置高度重叠，合并时逐条核对：

1. **`biz.ModelService.ListEnabledModels`**：上游会持续改写里面的 profile 过滤逻辑。我们在函数开头插入的 `contexts.GetActingUser` + `lo.Filter(channels, ... CanRouteThroughChannel)` 必须保留（它必须在 `queryConfiguredModelFacades(ctx, allowedModelIDs, channels)` 之前生效，否则配置型模型会泄漏不可用渠道的模型 ID）。
2. **`orchestrator` 选择链路**：上游可能重构 `selectModelCandidates` / `resolveAssociations` / 装饰器。无论怎么改，`DefaultSelector.Select` 的两条返回路径都必须经过 `filterCandidatesByChannelAccess`；不要把它挪进 `resolveAssociations` 或候选缓存。
3. **`middleware/auth.go`**：三处 API key 认证（`WithAPIKeyConfig`、service-account 专用分支、`WithGeminiKeyAuth`）都要调用 `withAPIKeyPrincipalUser`，否则那条入口上会退化成"无主体 = 不设限"。
4. **`contexts` container**：`PrincipalUser` 字段不能丢；上游若重写 container，需要一并带上 `WithPrincipalUser` / `GetPrincipalUser` / `GetActingUser`。
5. **GraphQL**：`Channel.sharedUsers` 与 `Query.shareableUsers` 定义在 `axonhub.graphql` / `publish_request.graphql`，`gqlgen generate` 后要确认 `SharedUser` 类型与 resolver 仍在。
6. **共享链路前端**：`data/shared.ts` 必须走 `mySharedChannels`；若改回 `where: {visibility: 'shared'}` 的列表查询，会在有 `read_channels` 的账号上把**所有用户**的共享渠道拉到前端，在其余账号上则返回空。
7. **不要**把 `CanRouteThroughChannel` 简化成 `read_channels` 判断，也不要给无主渠道加限制（会锁死升级用户的历史渠道）。

## 验证记录（2026-09-20，本机）

- API：`testuser@test.com` 的 key 调 `glm-5.3`（仅存在于 admin 的 private channel 11）→ `model not found`；改前为成功返回。
- API：同一 key 调 `xopglm51`（channel 4 共享给该用户）→ 路由到 channel 4（被上游凭据拒绝，属预期）。
- API：Owner 的 key 调 `glm-5.3` → 正常完成。
- `/v1/models`：testuser 只看到 `xopglm51`；Owner 两个都看到；`/v1/models/glm-5.3` 对 testuser 返回 404。
- 浏览器：Owner 打开共享弹窗 →「已共享给」显示 `Test User / testuser@test.com`（改前显示 `2`）；点 + 后列表与「已共享」徽章即时更新；点 X 取消共享成功（改前报 GUID 错误）；切到 `testuser@test.com` 后「我的渠道 → 共享」看到被共享的渠道（改前恒为空）。
- 单测：`internal/scopes` 表驱动 10 例、`internal/server/orchestrator` 选择器 3 例（含"无主体仍走系统路径"）、`internal/server/biz` 共享用户查询 3 例；`go test ./...`（root + llm）全过；`golangci-lint` 0 issues。
