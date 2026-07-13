# 请求详情按需加载设计

**日期：** 2026-07-14

## 目标

降低用户查看请求记录时 AxonHub 服务端和浏览器的内存压力。打开请求详情路由时只加载元数据；请求、响应、流式 chunks 和 execution 正文仅在用户明确操作后加载，并在不再使用时及时离开浏览器查询缓存。

## 范围

本次调整覆盖项目级和管理员级请求详情路由、共享请求详情组件、实时响应预览，以及这些视图使用的 GraphQL operation。

不改变请求持久化方式、存储布局、保留策略、列表页字段、二进制内容下载和服务端实时流 registry。

## 当前问题

现有详情 operation 会立即选择 `requestHeaders`、`requestBody`、`responseBody` 和 `responseChunks`。详情组件还会一次查询最多十条 execution 及其完整请求和响应正文。gqlgen 会正确地只解析被选择的字段，但当前宽泛的字段选择迫使服务端在用户选择查看哪部分内容之前，就读取、解码并序列化所有正文。

React Query 随后用同一个 key 保存普通详情结果，使元数据和大正文共用生命周期。实时预览也在路由级启动，即使用户没有查看响应标签也会累计 chunks。

## 选定方案

将详情数据拆分为相互独立的 GraphQL operation 和 React Query 缓存项：

1. 请求元数据；
2. 主请求正文；
3. 主响应正文；
4. execution 摘要；
5. 单条 execution 正文。

该方案利用 gqlgen 现有字段选择行为避免后端改造：operation 未选择的字段不会执行对应 resolver 加载。它可以在不引入新的 REST 分页或流式协议的前提下，实现有效的内存隔离。

不采用单 query 配合 `@include`，因为元数据和正文仍然会共享同一个缓存对象与生命周期。暂不新增 REST 内容接口，因为它会引入更大规模的协议和 UI 改造；如果未来发现单条持久化正文即使按需加载仍然过大，再考虑该方案。

## 用户体验

详情视图提供四个顶层标签：

- **概览**：默认选中；只渲染现有概览卡片所需的元数据，不触发正文查询。
- **请求**：首次选中时加载 request headers 和 request body。
- **响应**：首次选中时加载 response body 和 response chunks；对于符合条件的处理中流式请求，则启动实时预览。
- **执行记录**：首次选中时只加载 execution 摘要。

每条 execution 初始只显示摘要。用户明确展开某条 execution 后，才加载其请求 headers/body 和响应 body/chunks。折叠时释放该 execution 正文 query。

每个区域独立显示加载失败和重试操作，不影响概览与其他标签的使用。

## 前端数据边界

### 请求元数据

元数据 operation 返回路由标题和概览卡片所需的字段，包括身份、时间、查看者有权访问的项目/渠道/API Key 关联、来源、模型、流式/状态/格式、存储标记、客户端 IP、延迟指标，以及现有视图需要的轻量 usage 摘要。

它绝不选择 request headers、request body、response body 或 response chunks。处理中的请求仍然只轮询轻量元数据。

### 主请求正文

独立 query 只返回 `id`、`requestHeaders` 和 `requestBody`。仅在“请求”标签激活时启用。它使用独立 key，并设置 `gcTime: 0`。

### 主响应正文

独立 query 只返回 `id`、`responseBody` 和 `responseChunks`。对于已持久化/已完成的响应，仅在“响应”标签激活时启用。它使用独立 key，并设置 `gcTime: 0`。

对于启用了实时预览的处理中流式请求，“响应”标签改用现有 preview endpoint。离开该标签时中止预览请求、销毁 batcher，并清空累积 chunk 的 ref。请求完成后只失效或重新获取相关的元数据和响应正文 query。

### Execution 摘要

Execution connection query 仅在“执行记录”标签激活时启用。它只返回卡片需要的字段：身份、时间、渠道摘要、模型/格式、状态/错误/状态码、摘要操作需要的 request URL、透传状态和延迟指标，不返回 request headers/body、response body 或 response chunks。

### Execution 正文

按 execution ID 查询的 node operation 返回单条 execution 的 request headers/body、response body/chunks、request URL、格式，以及生成 cURL 所需的渠道字段。仅在对应 execution 展开时启用，并设置 `gcTime: 0`。

同时只允许展开一条 execution。展开另一条时折叠上一条，并移除上一条 execution 正文 query，从而约束浏览器正文保留量，同时保留现有全部功能。

## 组件结构

共享详情页持有当前顶层标签。它只获取一次元数据，并把元数据传给共享详情内容组件，避免路由标题和内容组件重复注册元数据 observer。

共享内容组件将正文展示委托给边界清晰的小单元：

- 请求正文面板；
- 响应正文面板；
- execution 摘要列表；
- execution 正文面板。

这些单元提供明确的 loading、error、retry 和 empty 状态。现有复制、下载、cURL、JSON viewer、response flow、音视频和 chunks dialog 功能在对应正文加载后继续可用。

Dialog 状态不得保留第二份正文副本。关闭 chunks dialog 时清空已选择的 execution chunks；主响应 dialog 直接读取当前响应正文结果。

## 缓存生命周期

- 元数据沿用应用默认缓存策略，使路由标题和概览导航保持响应迅速。
- 主请求正文、主响应正文和单条 execution 正文使用 `gcTime: 0`。
- 标签失活时按需显式移除非活动正文 query，保证及时释放而不是等待组件卸载。
- Execution 摘要不包含大字段，可沿用普通缓存策略。
- Query key 必须包含权限形状、项目作用域、管理员字段作用域和内容类型，避免数据跨作用域或与 quick-view key 冲突。

## 权限与作用域

项目路由继续传递 `X-Project-ID`。管理员路由继续使用系统级作用域，不隐式继承当前所选项目。所有拆分 operation 保留当前针对 project、channel、API Key 和 user 关联字段的权限选择逻辑。

正文不写入浏览器持久化存储。React Query 只在相关视图激活期间将其保存在内存。

## 错误与取消行为

- 每个正文 query 独立显示错误和重试操作。
- 导航离开或正文视图失活时，通过 TanStack Query 尽可能取消活动 fetch，并移除正文缓存项。
- “响应”标签失活或路由卸载时，实时预览必须中止 `fetch`、清理重连 timer、销毁 batcher 并清空 chunk ref。
- 实时请求完成后刷新元数据，并允许加载已持久化响应正文，不把大正文合并回元数据 query。

## 验证

自动化测试验证：

1. 元数据 operation 不包含大正文字段；
2. 请求和响应正文 operation 只选择各自目标字段；
3. Execution 摘要 operation 不包含正文字段；
4. 单条 execution 正文 operation 只针对一个 execution ID；
5. Query key 隔离元数据、正文类型、execution ID、项目作用域和权限形状；
6. 正文 query 在标签或 execution 激活前保持禁用，并使用立即垃圾回收；
7. 实时预览的启动和清理跟随“响应”标签可见性。

浏览器验证覆盖项目和管理员详情路由、四个标签、单条 execution 展开、复制/下载/cURL/chunks 操作、离开页面，以及条件允许时的处理中流式预览。网络检查必须确认“概览”不加载主正文，execution 展开前不加载 execution 正文。

运行时验证对比打开详情前、打开“概览”后、加载正文后，以及离开详情并强制 GC 后的 pprof heap 和进程内存。验收要求是“概览”不分配并保留请求/响应正文，关闭标签或离开详情后 live heap 不再保留对应正文。

## 成功标准

- 打开请求详情路由时，不执行选择 request/response 正文或 execution 正文的查询。
- 只有用户激活所属标签或展开 execution 后才获取对应正文。
- 浏览器同一时间最多保留一条 execution 正文。
- 离开正文视图后及时释放正文 query cache 和实时预览 buffer。
- 现有项目/管理员权限、实时预览、二进制下载、JSON 查看、复制、下载和 cURL 生成功能保持正常。
- 浏览器与 pprof 验证体现预期的内存分配和释放行为。
