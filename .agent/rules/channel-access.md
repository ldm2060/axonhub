---
alwaysApply: false
globs: "internal/scopes/rule_channel_usage*.go, internal/server/orchestrator/channel_access.go, internal/server/orchestrator/candidates.go, internal/server/biz/model.go, internal/server/middleware/auth.go, internal/contexts/container.go, internal/contexts/context.go, internal/server/biz/user.go, internal/server/gql/axonhub.graphql, internal/server/gql/publish_request.graphql, frontend/src/features/channels/data/shared.ts, frontend/src/features/channels/components/channels-share-dialog.tsx, frontend/src/gql/sharing.ts"
---

# 渠道共享与渠道可用性（Fork 规则）

设计全文见 [.agent/summary/2026-09-20-channel-sharing-and-access-design.md](../summary/2026-09-20-channel-sharing-and-access-design.md)。上游没有这套东西，合并时**不要**整段覆盖。

## 两条独立的规则

1. **可见性**（既有）：`visibility` + `shared_with` 决定用户在「我的渠道」里看到什么；`read_channels` 持有者绕过 privacy 过滤，能在管理页看到全部渠道。
2. **可用性**（本 fork 新增，`scopes.CanRouteThroughChannel`）：决定请求能路由到哪些渠道。行为主体 = 会话用户，或 API key 的属主用户；`service_account` / `noauth` / 无主体走系统路径不设限。放行条件：自己的渠道 / `published` / `shared` 且在 `shared_with` 里 / 系统 Owner 或 `write_channels` / `owner_id` 为空的无主渠道。

**可见 ≠ 可用。** 不要用 `read_channels` 当可用性门禁（自注册用户默认就有它）。

## 硬约束

1. 候选过滤必须在 `DefaultSelector.Select` **返回之前**做（`filterCandidatesByChannelAccess`），两条 return 路径都要过。不要挪进 `resolveAssociations`、渠道缓存或关联缓存：那些缓存按「渠道数 + 更新时间」做 key 并被所有请求共享，用户维度的过滤会把缓存毒化。
2. `middleware/auth.go` 三处 API key 认证都必须调用 `withAPIKeyPrincipalUser`，漏一处那条入口就退化成"无主体 = 不设限"。
3. `ListEnabledModels` 里的过滤必须早于 `queryConfiguredModelFacades`，否则配置型模型会把不可用渠道的模型 ID 泄漏到 `/v1/models`、`/v1/models/{id}`、Anthropic 与 Gemini 的模型列表。
4. 共享链路必须走 `mySharedChannels`（服务端按 `shared_with` 判定）；用 `where: {visibility: 'shared'}` 的列表查询会在 `read_channels` 账号上把别人的共享渠道全拉下来，其他账号上则为空。
5. `shared_with` 存的是**裸数字用户 ID**，UI 里流转的是 GUID（`gid://axonhub/User/2`）。两侧比较一律用 `extractNumberID` / `extractNumberIDAsNumber`。
6. 无主渠道（`owner_id = 0/NULL`）保持对所有人开放，否则升级用户的历史渠道会被锁死。

## 相关改动记录

- `2ac6fe17` 共享链路（可见性）修复
- `94203204` 可用性门禁
