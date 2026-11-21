# 用户并发限制修复与验证（2026-10-04）

根因：WithConcurrencyLimit 原先在 HTTP 握手进入处理器前占用名额，直到 c.Next 返回才释放。WebSocket 处理器在连接存活期间不会返回，因此空闲连接也占用用户请求额度。

修改：

- WebSocket 握手只绑定用户身份和共享限制器，每次实际处理消息才申请名额，持有至响应流写完并关闭。
- HTTP 与 WebSocket 使用同一用户计数；HTTP 仍在读取请求体之前申请名额。
- HTTP 限流返回 429、Retry-After: 1；生产 Responses WebSocket 返回 response.failed，错误码为 concurrency_limit_exceeded，连接可继续使用。
- 限制器以同一互斥锁保护申请、释放及空闲项删除；动态修改上限保留原有在途计数。
- 通用 WebSocket 处理器在生成期间持续读取连接，断线取消上游任务；每次生成使用配置的请求超时。
- 专用 Responses 分发器按生成消息计数，本地预热不占额度，限流拒绝保留预热续接状态，支持原消息重试。当前生产 Responses 路由仍使用原来的通用串行处理器。

最终代码验证全部通过（Go 命令均设置 GOEXPERIMENT=jsonv2）：

- 根模块 go build ./...；llm 模块 go build ./...。
- 两个模块各自运行 golangci-lint run --timeout 10m --max-same-issues 50 ./...，均报告 0 issues。
- make test-backend-all：分别执行根模块和 llm 模块 go test ./...，全部通过。
- go test -race ./internal/server/middleware ./internal/server/api -run 'Concurrency' -count=1，通过；启用 CGO 并使用本机 MinGW GCC。
- go build -o axonhub.exe ./cmd/axonhub/，成功。

浏览器验证使用最终新二进制、独立 SQLite 测试库、Chromium 和本地模拟上游。浏览器 WebSocket 经本地转发层补充测试 API Key 请求头；网关正常执行鉴权。测试中的上限为 6：

| 场景 | 结果 |
| --- | --- |
| 7 条空闲 WebSocket，发送 HTTP POST | 200 |
| 1 个 WebSocket 生成运行，其余连接空闲，发送 POST | 200 |
| 6 个生成运行，发送新 POST | 429，Retry-After: 1 |
| 6 个生成运行，第 7 条连接发送生成消息 | response.failed / concurrency_limit_exceeded |
| 6 个生成仍运行，上限降至 5 | 新 POST 返回 429 |
| 上限增至 7 | 新 POST 返回 200 |
| 完成后使用原先被拒的连接重试 | response.completed |
| 上限改为 1，关闭正在生成的连接，再发送 POST | 上游取消，POST 返回 200 |

系统设置页面正常显示上限，未捕获 JavaScript 页面异常；控制台出现的是测试刻意触发的两个 HTTP 429。

结构化运行记录：concurrency-browser-verification.json。
完整双模块测试输出：concurrency-backend-tests.log。
浏览器验证脚本：verify-concurrency-browser.cjs（本地测试辅助文件，不纳入代码提交）。

验证完成后已停止独立测试服务，使用原 config.yml 和新 axonhub.exe 恢复本地服务；http://127.0.0.1:8090/health 返回 200。

以上运行证据来自本地模拟上游，未复核用户给出的生产原始日志，也不是线上部署结果。仅凭同秒 101 记录和计费表不能证明特定被拒时刻恰好有五条空闲连接；源码已确认这种占槽机制确实存在，回归测试和浏览器测试已验证修复后的行为。
