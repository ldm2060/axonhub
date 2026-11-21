# 网关用户并发与流式生命周期体检

检查日期：2026-10-04。仓库：`C:\PythonProject\axonhub`，分支 `unstable`。

本轮提交：`10e2e39d`（统一并发与流收尾）、`93abe962`（首个响应等待超时）。工作树干净，未推送远端。本地服务已恢复原始配置，最终健康检查与浏览器访问通过。

**结论：非 Codex 请求也受影响，而且问题超过“WebSocket 空闲连接占槽”这一项。** 本次沿真实路由、中间件、用户身份、HTTP/WS 处理、流读取、下游写出和资源收尾逐层检查，并修复了下列并发相关问题。

## 已确认并修复

| 编号 | 问题与影响 | 本次处理及主要位置 |
|---|---|---|
| C01 | Gemini 原生 `/gemini/:version`、`/v1beta` 和 JWT Playground 未接入用户限制器。用户可跨协议或切换网页入口绕开限额。 | 三个入口接入同一限制器；[routes.go](C:/PythonProject/axonhub/internal/server/routes.go:177)，Gemini 两组分别在 321、335 行。 |
| C02 | 模型列表、视频任务查询与取消也占生成请求额度。生成打满后，发现模型和查询/取消任务可能被误拒。 | 普通 GET/HEAD/DELETE/OPTIONS 不占生成额度，真正的 GET WebSocket 升级仍按每次生成计数；[concurrency.go](C:/PythonProject/axonhub/internal/server/middleware/concurrency.go:126)。 |
| C03 | API Key 已有 UserID，但用户实体未加载时退回 key ID 计数。同一用户的多把 key 可能拆成多份额度。 | 优先使用已认证用户；缺少用户实体时仍按 API Key.UserID 归属，仅无属主 key 独立计数；[concurrency.go](C:/PythonProject/axonhub/internal/server/middleware/concurrency.go:168)。 |
| C04 | 关闭限制时不记录在途请求，重新启用限制可能在旧请求上叠加一份新额度。 | 限制值为 0 时继续跟踪活动请求；配置热更新保留原有计数，完成后清理；`ConcurrencyLimitConfig.acquire`。 |
| C05 | 所有入口统一返回 OpenAI 429 JSON，Anthropic/Gemini 客户端无法获得原生错误结构。 | Anthropic 返回顶层 `type:error`；Gemini 返回数值 `code:429` 和 `RESOURCE_EXHAUSTED`；HTTP 保留 `Retry-After:1`；`concurrencyLimitError`。 |
| C06 | 限制器位于 trace 中间件之后，拒绝前可能已把大请求体读入内存。 | 限制器移到 Thread/Trace 之前，认证之后；[routes.go](C:/PythonProject/axonhub/internal/server/routes.go:233)。 |
| C07 | `/anthropic/v1/messages` WebSocket 别名继承整条 HTTP 连接超时。长时间空闲后发起新请求会受旧连接截止时间影响。 | 与其他 WS 路由同组，只对每次消息处理应用超时；`routes.go` 的 WebSocket 注册组。 |
| C08 | 普通 HTTP 路由统一套用“非流式请求总超时”，覆盖了处理层对流的区分；Gemini 却没有这一层，跨协议不一致。 | 流式请求依赖空闲超时；非流式仍保留总超时。对视频、Jina、TypeSafe、任务与模型查询等单独保留超时；补齐 moderation/alpha-search 处理器超时设置。 |
| C09 | HTTP 心跳处理提前返回后，后台 Process 晚返回的流可能无人关闭。 | 使用交接与放弃信号，无法交给响应处理器的流由后台关闭；[chat.go](C:/PythonProject/axonhub/internal/server/api/chat.go:215)。 |
| C10 | 取消时可能同时存在后台 Next 与主线程 drain；超时/写失败时 Close 也可能与 Next/Current 重叠，影响持久化和资源释放。 | 统一 waiter；先取消上游上下文，再等待 pending read 交还，最后 drain/Close；SSE、Gemini JSON、AI SDK、二进制及 WS 共用该收尾规则；[stream_timeout.go](C:/PythonProject/axonhub/internal/server/api/stream_timeout.go:129)。 |
| C11 | 专用 SSE 心跳分支绕开 StreamIdleTimeout。心跳在发，但上游已停顿时可以继续占槽。 | 专用心跳与普通流共用 waiter，心跳不重置上游空闲计时；保留 OpenAI 注释心跳和 Anthropic ping 格式，移除重复 reader。 |
| C12 | WS 写失败未可靠退出连接，生成 deadline 有静默结束路径；非 SSE writer 部分忽略写错误，且缺少单次写出 deadline。 | WS 返回写错误并退出、发送生成超时错误帧，WS 单次写出限 10 秒；Gemini JSON、AI SDK、二进制和 HTTP 等待心跳复用 30 秒写出保护，检查写入和刷新失败；[ws.go](C:/PythonProject/axonhub/internal/server/api/ws.go:102)、[chat.go](C:/PythonProject/axonhub/internal/server/api/chat.go:819)。 |
| C13 | StreamIdleTimeout 原先只在 Process 返回后生效；上游迟迟不返回首包时尚未进入流 writer，可能长期占槽。 | 处理阶段增加独立空闲计时，覆盖 HTTP、Playground 和 WS 的首个响应等待；超时取消上游并关闭晚返回的流，成功交接后由后续 chunk 空闲计时接管。 |

前一次提交 `afd6cf26` 已把原有共用 WS 处理器改成按消息计数。本轮继续验证 OpenAI Chat、Anthropic、Gemini 的入口、别名和生命周期，未把生产 `/v1/responses` 切换到另一个未注册的专用 dispatcher。


## 现在的计数语义

- 同一用户跨 key、跨协议、HTTP/WS、JWT Playground 共用一份额度。
- 普通生成请求从准入开始占槽，覆盖上游处理、响应流和收尾；正常完成、出错、取消和超时都会进入释放路径。
- WebSocket 握手与空闲等待不占生成槽；每次生成消息占槽。连接本身并不等于一次正在进行的生成。
- SSE 心跳不会新占槽，也不会把上游空闲计时重新归零。
- 模型发现、任务查询和 DELETE 取消不占生成槽。
- 关闭并重新启用限制时，仍能识别先前已开始、尚未结束的请求。
- 额度在流收尾/持久化完成后释放。客户端观察到断开或上游 socket 关闭，与服务器清理完成之间可能有短暂间隔；验证使用有界重试确认释放，不假设同一个瞬间完成。

## 验证证据

**真实浏览器与本地网关进程。** 使用隔离 SQLite 测试库、两把同用户测试 API Key、JWT、真实路由与本地模拟上游；没有调用付费生产模型。测试服务使用非流式总超时 1 秒、流空闲超时 2 秒、SSE 心跳间隔 200 毫秒，刻意放大超时语义差异。

| 验证层 | 覆盖 | 结果 |
|---|---|---|
| HTTP 完整流生命周期 | OpenAI Chat SSE、Anthropic SSE、Anthropic 别名 SSE、Gemini SSE、Gemini 别名 JSON 流、JWT Playground，共 6 种 | 在途请求跨第二把 key 触发 429；流超过非流式总超时仍可继续；正常结束、客户端取消后恢复准入。 |
| 真实路由准入 | 24 个生成路由，含 Chat/Completions/Responses/Compact、embeddings、moderations、search、images、video、audio、Jina、TypeSafe、Doubao、Gemini、Playground | 达到额度后全部 429，HTTP 有 Retry-After；Anthropic/Gemini 原生错误结构正确。 |
| 模型发现 | `/v1/models`、`/anthropic/v1/models`、Gemini 两种前缀 | 额度满时均 200。 |
| 查询/取消准入 | OpenAI 视频和 Doubao 任务的 GET/DELETE | 不再被并发限制返回 429。使用不存在的测试任务 ID，实际业务返回 500；这不代表成功查询/删除真实任务。 |
| WS 完整生命周期 | OpenAI Chat、Anthropic 正式/别名、Gemini 正式/别名，共 5 种 | 空闲连接超过非流式超时仍可用；共享额度、拒绝后同连接重试、断开释放通过。 |
| 首个响应等待 | 上述 6 种 HTTP 流，上游收到请求后不返回响应头/内容 | 等待约 2 秒触发初始空闲超时，上游被取消、额度释放；单元测试在修复前可稳定复现等待超出空闲阈值。 |
| 上游空闲超时 | 上述 6 种 HTTP 流 | 上游停顿触发回收，恢复准入；OpenAI/Anthropic 持续发心跳也不能掩盖空闲超时。 |
| 热启用配置 | 先关闭限制并开始流，再启用限制 | 已有流计入额度，新请求正确 429。 |
| 普通非流式超时 | 模拟上游不返回非流式请求 | 仍触发超时，收尾后额度恢复。当前错误状态为 500，见附带发现。 |
| 浏览器运行 | 系统设置页面及恢复普通配置后的页面 | 页面无未捕获异常；本地 `/health` 返回 healthy。 |

**自动化验证。** 所有 Go 命令均带 `GOEXPERIMENT=jsonv2`。

- 根模块 `go build ./...`、`llm/` 内 `go build ./...`：通过。
- 两个模块 `golangci-lint run --timeout 10m --max-same-issues 50 ./...`：均 0 issues。
- `make test-backend-all`：包含根模块与 `llm/` 各自的 `go test ./...`，通过。
- `go test -race ./internal/server/api ./internal/server/middleware`：通过；Windows CGO 使用本机 MinGW。
- 重新编译 `axonhub.exe`，实际重启并运行浏览器矩阵；随后恢复原始本地配置。
- 新增/扩展回归覆盖用户归属、动态启用、GET/DELETE、原生错误格式、非 Responses WS、晚返回流回收、取消 drain 交接、超时错误、心跳下空闲超时、Next/Close 不重叠、非 SSE 写失败停止消费。

复核文件：

- [浏览器矩阵原始结果](C:/PythonProject/axonhub/.agent/summary/gateway-browser-verification.json)
- [浏览器验证脚本](C:/PythonProject/axonhub/.agent/summary/verify-gateway-concurrency-browser.cjs)
- [系统设置截图](C:/PythonProject/axonhub/.agent/summary/gateway-concurrency-settings.png)
- [全量后端测试记录](C:/PythonProject/axonhub/.agent/summary/gateway-backend-tests.log)
- [竞态检测记录](C:/PythonProject/axonhub/.agent/summary/gateway-race-tests.log)
- [新增生命周期回归测试](C:/PythonProject/axonhub/internal/server/api/gateway_concurrency_test.go)

## 保留边界及附带发现

1. **限制仍是单进程计数。** `ConcurrencyLimitConfig` 是进程内 mutex/map。若线上有多副本网关，“6”不能自动解释为所有副本合计 6；需要共享租约/计数才能满足集群总额度。本轮没有引入分布式限制器，也未假定生产是单副本。
2. **异步视频限制的是提交请求。** 创建任务的 HTTP 请求结束即释放槽，不持续统计远端任务整个运行周期。若产品要限制“同时生成的视频任务数”，需要单独的任务并发语义，不能靠连接存活时间间接计数。
3. **错误呈现还有独立问题。** 本地实测不存在的视频/豆包任务查询与删除返回 500；非流式处理超时也返回 500。Gemini JSON 流的 idle timeout 分支会结束数组，但没有像 SSE 一样发送明确错误事件；本轮验证了资源释放，未把它当作错误语义修复完成。这些应分别规范为任务不存在、超时及流内失败的明确结果。
4. **连接资源与生成额度是两个限制。** 空闲 WS 不占生成槽后，连接总数/速率的保护仍应独立配置；本轮没有把连接数量重新混入生成额度。
5. 本轮覆盖本地源码、真实认证/路由和本地模拟上游。未回放生产 user:38 的原始 nginx/数据库现场，也未执行多副本压力测试或逐个付费供应商的音视频/图像生成。图像、音视频、Jina、TypeSafe 的路由准入已实测，二进制流生命周期有单元回归；这不等价于所有供应商生成成功的端到端验证。


