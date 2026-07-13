# HTTP 流式保活设计

**日期：** 2026-07-14  
**状态：** 已批准  
**目标版本：** v0.1.58  

## 背景

AxonHub 已支持多种 HTTP 流式响应，但“流式请求”并不保证响应链路始终有字节传输。上游可能在建立连接、产生首个事件、重试或事件间计算阶段长时间静默。经过 Cloudflare 代理时，默认 120 秒 Proxy Read Timeout 会把这种静默请求中断为 524。

现有 `Connection: keep-alive` 只影响连接复用，不会产生 HTTP 响应体数据，无法防止该超时。AxonHub 的 `server.llm_stream_idle_timeout` 默认也是 120 秒，它用于判定上游流空闲，而不是向下游代理发送保活数据，两者不应混为一个设置。

本设计在不修改客户端、不改用 WebSocket、不使用 DNS-only 子域名的条件下，为兼容的 HTTP 文本流增加可配置、协议安全的下游保活。

## 目标

1. 在系统“流式响应”设置中，把 HTTP 流式保活设置放在现有 WebSocket 保活设置下方。
2. 默认关闭，避免升级后改变现有响应行为；管理员可设置间隔主动启用。
3. 在整个流式请求生命周期中覆盖下游静默，包括 `Process()` 返回前和真实流开始后的等待。
4. 心跳只写入客户端 HTTP 响应，不进入 LLM pipeline、转换器、持久化 chunks、用量统计或实时预览。
5. 根据下游协议生成安全的保活字节；不能安全插入字节的二进制流保持原样。
6. 保留现有上游 idle timeout 语义，并确保心跳不会重置它。

## 非目标

1. 不修改客户端或要求客户端识别新的业务事件。
2. 不新增异步任务、轮询接口或 WebSocket 路径。
3. 不调整 Cloudflare 配置。
4. 不向模型流中注入伪 `StreamEvent`。
5. 不为原始音频或其他二进制流插入保活字节。
6. 不改变 `server.llm_stream_idle_timeout` 的默认值或配置来源。

## 设置与持久化

扩展现有 `biz.StreamingSettings`：

```go
type StreamingSettings struct {
    WebSocketKeepaliveIntervalSeconds int `json:"web_socket_keepalive_interval_seconds"`
    HTTPStreamKeepaliveIntervalSeconds int `json:"http_stream_keepalive_interval_seconds"`
}
```

GraphQL 同步增加：

```graphql
type StreamingSettings {
  webSocketKeepaliveIntervalSeconds: Int!
  httpStreamKeepaliveIntervalSeconds: Int!
}

input UpdateStreamingSettingsInput {
  webSocketKeepaliveIntervalSeconds: Int
  httpStreamKeepaliveIntervalSeconds: Int
}
```

设置语义：

- `0`：禁用 HTTP 流式保活，也是默认值。
- 正整数：响应下游空闲达到该秒数时写入一次协议安全的保活字节。
- 负数在后端 normalization 中归零。
- 前端输入范围沿用 WebSocket 设置的 `0–3600` 秒。
- 前端文案建议管理员在 Cloudflare 场景使用 `20–30` 秒，但不自动填入或强制该值。

### 是否需要数据库变更

不需要 Ent schema、数据库字段或迁移。

现有 `system` 表通过通用 `key/value` 保存配置，`key = "streaming_settings"` 的 `value` 是 JSON。旧 JSON 缺少新字段时，Go 解码会得到零值 `0`，等同于禁用；下一次保存设置时自然写入新字段。因此升级和回滚均不依赖数据库迁移。

## 架构

### 1. 协议感知的保活策略

为每种 HTTP stream writer 指定一个保活策略，而不是在 pipeline 里创建心跳事件。

| 下游响应 | 保活负载 | 行为 |
|---|---|---|
| SSE（OpenAI、Anthropic、Responses、Gemini `alt=sse`） | `: keepalive\n\n` | SSE comment，标准客户端忽略 |
| Gemini 流式 JSON 数组 | `\n` | JSON 数组元素之间的合法空白 |
| AI SDK / Playground 文本流 | 仅在现有 framing 测试证明空行可被忽略后使用 `\n` | 不创建业务事件 |
| 原始音频及其他二进制流 | 无 | 禁止注入，避免损坏载荷 |

实现应把“写保活字节并 Flush”的能力放在 API 输出层。保活负载不得包装成 `httpclient.StreamEvent`。

### 2. 两阶段保活

#### 阶段一：等待 `Process()`

当请求已被识别为流式，且该 writer 支持保活、配置间隔大于零时：

1. 在受请求 context 控制的后台 goroutine 中执行 `processor.Process(ctx, genericReq)`。
2. handler goroutine独占 `gin.Context.Writer`，等待以下任一信号：
   - `Process()` 返回；
   - 保活 ticker 到期；
   - 请求 context 取消。
3. ticker 到期时，由 handler 写入该协议的保活负载并立即 `Flush()`。
4. `Process()` 返回真实 stream 后进入阶段二。
5. 后台 goroutine必须安装项目规则要求的顶层 panic recovery，并把 panic 转为错误结果，而不是令请求永久等待。

这样可以覆盖上游 HTTP 等待、首事件预读、空响应检测和重试期间的下游静默。

#### 阶段二：转发真实 stream

stream writer 在等待 `stream.Next()` 时同时监听：

- 下一个真实事件；
- HTTP 保活 ticker；
- `server.llm_stream_idle_timeout`；
- 请求 context 取消。

真实事件到达后：

1. 按原协议写出事件；
2. 立即 `Flush()`；
3. 从该时刻重新计算下一次下游空闲保活。

保活到期后：

1. 写入协议安全负载；
2. 立即 `Flush()`；
3. 重新计算下一次保活；
4. 不重置上游 stream idle timeout。

这一区分很重要：

- HTTP keepalive 只表示“下游代理连接仍应保持”；
- stream idle timeout 表示“上游是否已经长时间没有真实事件”。

### 3. Writer 所有权与并发

`gin.Context.Writer` 只能由 handler/writer 所在 goroutine写入。`Process()` 和阻塞的 `stream.Next()` 可以在后台执行，但后台 goroutine不得写 HTTP 响应。

每次等待最多保留一个正在执行的 `Next()`。context 取消或 idle timeout 时关闭 stream，以解除阻塞；结果 channel 使用容量 1，避免后台返回时永久阻塞。所有手工 goroutine均须遵循项目 panic recovery 规则。

## 错误处理和 HTTP 状态

### 第一条保活之前失败

响应尚未提交，维持现有行为：返回原本的 HTTP 状态和协议错误正文，例如 400、429 或 500。

### 第一条保活之后失败

第一次写保活会提交 `200 OK`，HTTP 状态不可再修改。后续失败必须按当前下游 writer 的流内错误格式写出并 Flush：

- SSE 使用现有错误事件格式；
- Gemini JSON 数组写入合法的错误元素并闭合数组；
- AI SDK 文本流使用现有协议错误帧；
- 若客户端已断开，只记录并结束，不再写响应。

这是“不修改客户端且提前向代理发送响应字节”的固有限制。默认值为 0 可确保未主动启用的实例完全保留原有状态码行为。

## HTTP headers 和代理缓冲

支持保活的文本流应确保：

- `Cache-Control: no-cache, no-transform`；
- 每次保活和真实事件后调用 `Flush()`；
- 不设置固定 `Content-Length`；
- SSE 保持 `Content-Type: text/event-stream`；
- 如果部署链路还有 Nginx 等会缓冲的反向代理，可增加 `X-Accel-Buffering: no`，该 header 对 Cloudflare 本身无副作用。

`Connection: keep-alive` 可以保留用于兼容 HTTP/1.1，但不能作为本功能是否成功的判断依据。

## 前端

在现有 `StreamingSettings` 卡片中：

1. 保留当前 WebSocket 保活输入。
2. 紧接其下新增“HTTP 流式保活间隔”数字输入。
3. 输入 `0` 或留空表示禁用。
4. 描述明确说明：该设置只在兼容的 HTTP 文本流空闲时发送透明保活，不适用于原始二进制流；Cloudflare 场景建议 20–30 秒。
5. 更新 GraphQL query、mutation input、TypeScript interfaces、form state 和提交逻辑。
6. 同步更新英文与简体中文 locale 文件。

## 测试

### Biz 和持久化兼容

- 负 HTTP 间隔 normalization 为 0。
- 正值保持不变。
- 只含旧 WebSocket 字段的 JSON 解码后 HTTP 间隔为 0。
- 新旧字段 JSON round-trip 正确。

### API writer

- 设置为 0 时不产生任何额外字节。
- SSE 空闲达到间隔后输出 `: keepalive\n\n` 并继续等待真实事件。
- 真实 SSE 事件会重置下游保活计时。
- 心跳不会重置上游 idle timeout。
- `Process()` 返回前能够发送保活。
- `Process()` 在首次保活前失败时保留原 HTTP 状态。
- `Process()` 在首次保活后失败时输出流内错误。
- 客户端取消会停止 ticker、关闭 stream 并释放 goroutine。
- Gemini JSON 保活后最终响应仍是合法 JSON。
- AI SDK / Playground 只有在协议测试证明空行兼容后才启用对应策略。
- 二进制 writer 即使配置非零也不写保活字节。

### GraphQL 与前端

- GraphQL 查询返回新字段，mutation 可保存并回读。
- 前端加载旧设置时显示 0。
- 保存后 query cache 刷新并显示新值。
- 在浏览器验证字段位置、中文/英文说明、保存和重新加载。

### 端到端代理行为

用一个测试 stream 在超过保活间隔后才返回首事件，并通过真实 HTTP 客户端观察：

1. 客户端在真实事件之前收到保活字节；
2. 连接保持打开；
3. 最终真实事件与终止事件完整；
4. AxonHub 请求记录中不包含保活；
5. idle timeout 仍按真实上游事件而非保活计算。

## 验证与提交

实现后按仓库规则：

1. 运行相关 Go 单元测试和前端测试（若已有对应测试入口）。
2. 运行 `make test-backend-all`。
3. 浏览器验证系统设置的加载、保存和刷新。
4. 启动真实流式请求，观察空闲期间的响应字节和最终完整响应。
5. 确认工作区没有 `.exe` 等构建产物。
6. 提交代码变更。
