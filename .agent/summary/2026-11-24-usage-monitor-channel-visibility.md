# 使用监视器的按用户可见性（电池面板）

日期：2026-11-24 · 分支：`unstable` · 关联规则：[.agents/rules/quota-header-monitors.md](../../.agents/rules/quota-header-monitors.md)、[.agents/rules/channel-access.md](../../.agents/rules/channel-access.md)

## 需求

一个用户是否能在右上角电池面板中看到某个配额监控，取决于该监控对应的渠道对此用户是否可用（公开、分享或拼车）。

## 改动前

`usageMonitorChannelsList`（`UsageMonitorService.ListChannels`）返回全部未软删除的监控，
没有任何用户维度过滤：任何登录用户都能在电池面板、`/admin/usage-monitor`、
渠道拼车设置和监控绑定选择器里看到所有人的监控，包括别人的账号额度数据。
（`UsageMonitorChannel` 没有 ent Policy，所以这条列表本来就是全量返回的。）

## 改动后

新增 `internal/server/biz/usage_monitor_visibility.go`，在 `ListChannels` 里按请求主体过滤：

- 取 `contexts.GetActingUser(ctx)`；无主体（service account / noauth / 后台任务）不过滤。
- 系统 Owner、`IsAdmin` 直接返回全量：他们本来就能路由所有渠道，
  不该因为"不是自己建的监控"而从面板里丢数据。
- 其余用户：监控可见 ⟺ 存在一条它对应的渠道能被 `scopes.CanRouteThroughChannel` 放行，
  或者该监控是此人自己添加的。
  - 创建者永远看到自己的监控：否则"别人的私有渠道恰好是同 providerType"
    会让自己加的模板监控从自己面板里消失。
  - 渠道查询走 `authz.RunWithSystemBypassVoid`：请求主体自己的渠道查询会被可见性
    规则过滤掉（`ChannelVisibilityQueryRule` 只放行 published + 自己的），
    共享渠道不在结果里，可用性判断就无从谈起。
- "监控对应的渠道"与周期额度估算的归属规则保持一致（`monitorCostChannelIDs`）：
  1. 绑定渠道（`channel_id`）优先，且只认这一条——同 providerType 的其它渠道不会
     把已经描述某个账号的监控放宽；
  2. 未绑定且有 `provider_type`：所有启用渠道里 `channelProviderType` 相同的；
  3. 额外并入 `settings.pooling.monitorId` 指向该监控的渠道（拼车成员由此看到监控，
     不管监控绑定在哪个渠道上）。

## 影响面

- 电池面板（`quota-badges.tsx`）：共享/拼车成员看到对应监控，其他人看不到；
  面板里没有任何监控时电池图标整体隐藏（原有 `hasContent` 逻辑）。
- `/admin/usage-monitor`、拼车设置的监控选择器、渠道的监控绑定选择器：
  同样只列出可用的监控。非 Admin 的渠道运维者如果对某渠道没有可用性，
  会在这些选择器里看不到绑定在该渠道上的监控——这是"可见 ≠ 可用"规则的必然结果。
- 前端零改动：面板渲染服务端返回的列表。

## 验证

- `GOEXPERIMENT=jsonv2 go test ./...`（root）与 `cd llm && go test ./...` 通过。
- `golangci-lint run`（root 与 llm）0 issues。
- 新增 `TestUsageMonitorService_ListChannelsFiltersMonitorsByChannelAccess`：
  共享渠道绑定 / 私有渠道绑定 / 公开渠道绑定 / 同 providerType 模板 /
  拼车监控 / 自定义监控 / 作者自有但候选渠道不可用 / 系统 Owner / 无主体，共 8 个监控 × 4 类主体。
