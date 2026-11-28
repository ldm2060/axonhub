# Codex 兼容支持 OpenAI 渠道 — 调研

日期: 2026-11-21。目标：让 Codex CLI 的 `/codex/*` 兼容流量可以由 OpenAI 类型渠道承接，而不强制依赖 Codex（ChatGPT OAuth）渠道。

## 现状链路（已工作的部分）

1. `/codex/*` 路由（`internal/server/codex_routes.go`）直接复用标准 OpenAI handlers：
   `POST /codex/v1/responses` → `handlers.OpenAI.CreateResponse` → 统一 orchestrator。
   **请求路由本身对渠道类型没有硬编码限制**。
2. 认证仿真已完备：`Whoami`/`AccountsCheck`（`internal/server/api/codex_compatibility.go`）
   伪造 ChatGPT `promax` 账号，与渠道类型无关。
3. 渠道选择支持协议转换兜底：Responses 请求的 `CapableAPIFormats(RequestTypeChat)`
   包含 chat-completions / anthropic / gemini（`llm/api_capability.go`），纯 chat 渠道
   也能经协议转换承接；WS↔HTTP 有互通 fallback（`orchestrator/select_endpoints.go:88`）。
4. 非 Codex/Fenno 渠道若挂了自定义 `openai/responses` 端点，出站自动用标准
   API-key responses transformer（`internal/server/biz/channel_llm.go:527-538`）——
   转换器已经存在，不需要新写出站。

## 两个硬依赖（真正拦住 OpenAI 渠道的地方）

1. **兼容开关强制要求 Codex 渠道**：`SetCodexCompatibilitySettings`
   （`internal/server/biz/system_codex.go:60-62`）在 `enabled` 时要求 `ChannelID`
   非空，且必须是 `TypeCodex` 渠道（`:71-73`）。没有 Codex 渠道连开关都打不开
   （实测 GraphQL 返回 "a Codex channel is required when enabled"）。
2. **模型目录强制走 Codex 渠道**：`fetchCodexCatalog`
   （`internal/server/biz/codex_catalog.go:110-113`）要求 catalog 源渠道
   `Type == TypeCodex`，否则 `/codex/models` 返回 503 "Codex catalog source is
   unavailable"。注意 `Channel.Get` 不按状态过滤——**禁用状态的 Codex 渠道也能当目录源**。

软差距：`TypeOpenai` 的默认端点表没有 `openai/responses`
（`internal/server/biz/channel_endpoint.go:195-217`），Responses 由
`openai_responses`/`codex`/`fenno` 等专门类型承担；openai 渠道可通过自定义端点补齐。

## 方案

### 方案 A — 纯配置，零代码（立即可用）

1. 建一个 Codex 渠道仅作目录源（凭据失效也没关系，目录能拉通即可；甚至可禁用）。
2. 建 `openai_responses` 渠道（`https://api.openai.com/v1` + API key），支持模型里
   加入 `gpt-5.x-codex` 系列（Codex CLI 请求的模型名必须命中渠道 supported models）。
3. 系统设置 → 集成与兼容 → 打开 Codex 兼容、目录源选第 1 步的渠道。
4. Codex CLI 按现有文档指向 `/codex` 入口；流量全部路由到 openai_responses 渠道。

缺点：仍需一个"壳" Codex 渠道；目录内容来自 ChatGPT 账号视图。

### 方案 B — 小改，放松目录源约束（推荐中期落地）

- `system_codex.go`：允许 `enabled && ChannelID == nil`；目录源渠道类型校验放宽到
  `TypeCodex | TypeOpenaiResponses`（或直接允许 nil）。
- `codex_catalog.go`：`ChannelID == nil` 时不再 503，退化为用内置/同步的目录元数据
  渲染 `ListEnabledModels`（fork 已有 model developers 同步数据与 npm 版本解析
  `resolveCodexClientVersion`，可离线拼出静态 catalog）。
- 前端 Codex 兼容设置页把"必选 Codex 渠道"改为可选说明。

涉及文件：`internal/server/biz/system_codex.go`、`internal/server/biz/codex_catalog.go`、
`internal/server/api/codex_compatibility.go`（503 分支）、前端 `codex-compatibility-settings.tsx`。

### 方案 C — 可选增强

给 `TypeOpenai` 增加可选的 `openai/responses` 默认端点开关（或渠道设置里一键
"启用 Responses 端点"），免去单独建 `openai_responses` 渠道；端点映射与出站已支持，
只需改 `defaultEndpointsForChannelType` + 渠道设置 UI。

## 推荐

短期用方案 A 验证；方案 B 作为正式实现（改动小、把"目录"与"渠道类型"解耦，
Codex 兼容从此可以纯 OpenAI 渠道运行）。方案 C 视用户反馈决定。

## 验证注意

- Codex CLI 请求的模型名（如 `gpt-5.1-codex`）必须出现在渠道 supported models 里，
  否则无候选渠道。
- 实例上 Codex 兼容默认关闭；本次调研曾在 8090 实例用 GraphQL 试开（被校验拒绝），
  未改动现有设置；调研用的临时 API Key（codex-research-probe）已软删除。
