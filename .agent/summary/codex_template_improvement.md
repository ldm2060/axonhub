# Codex/ChatGPT 模板改进：使用分组功能

## 改进前后对比

### 改进前（平铺显示）
```
┌─────────────────────────────────┐
│ ChatGPT               [活跃]    │
├─────────────────────────────────┤
│ 5h Window Used %: 45%           │
│ 5h Reset At: 2h                 │
│ 7d Window Used %: 30%           │
│ 7d Reset At: 5d 3h              │
│ Plan Type: [Pro Lite]           │
│ Limit Reached: [false]          │
│ Extra Feature: Extended Thinking│
│ Extra 5h Used %: 80%            │
│ Extra 5h Reset: 1h 30m          │
│ Extra 7d Used %: 60%            │
│ Extra 7d Reset: 4d 2h           │
│ Extra Feature 2: Image Gener... │
│ Extra 2 5h Used %: 25%          │
│ Extra 2 5h Reset: 3h            │
│ Extra 2 7d Used %: 15%          │
│ Extra 2 7d Reset: 6d 1h         │
└─────────────────────────────────┘
```

### 改进后（分组显示）
```
┌─────────────────────────────────┐
│ ChatGPT               [活跃]    │
├─────────────────────────────────┤
│ Plan Type: [Pro Lite]           │
│ Limit Reached: [false]          │
├ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─│
│ PRIMARY                         │
│ 5h Window Used %: 45%           │
│ ▓▓▓▓▓░░░░░ 45%                  │
│ 5h Reset At: 2h                 │
│ 7d Window Used %: 30%           │
│ ▓▓▓░░░░░░░ 30%                  │
│ 7d Reset At: 5d 3h              │
├ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─│
│ EXTENDED THINKING               │← 动态标题
│ 5h Window Used %: 80%           │
│ ▓▓▓▓▓▓▓▓░░ 80%                  │
│ 5h Reset At: 1h 30m             │
│ 7d Window Used %: 60%           │
│ ▓▓▓▓▓▓░░░░ 60%                  │
│ 7d Reset At: 4d 2h              │
├ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─│
│ IMAGE GENERATION                │← 动态标题
│ 5h Window Used %: 25%           │
│ ▓▓▓░░░░░░░ 25%                  │
│ 5h Reset At: 3h                 │
│ 7d Window Used %: 15%           │
│ ▓░░░░░░░░░ 15%                  │
│ 7d Reset At: 6d 1h              │
└─────────────────────────────────┘
```

## 改进内容

### 1. 分组结构
- **Primary 组**：主配额（所有用户共有）
- **Additional_0 组**：第一个计量功能（动态）
- **Additional_1 组**：第二个计量功能（动态）

### 2. 动态分组标题
使用 `GroupLabelRef` 引用 `additional_X_name` 变量的值作为分组标题：
- 当 API 返回 `"metered_feature": "extended_thinking"` 时，显示 "EXTENDED THINKING"
- 当 API 返回 `"metered_feature": "image_generation"` 时，显示 "IMAGE GENERATION"

### 3. 字段优化
- 移除了冗余的 `additional_X_name` 字段（现在作为分组标题使用）
- 简化了标签文本（去掉 "Extra"、"Extra 2" 前缀）
- 将 Plan Type 和 Limit Reached 移到顶部，便于快速识别账户状态

## 技术实现

### 变量配置（未变）
```go
Variables: []Variable{
    // ... 主配额变量 ...
    {Key: "additional_0_name", Path: "$.additional_rate_limits[0].metered_feature", Type: "jsonpath"},
    {Key: "additional_0_primary_pct", Path: "$.additional_rate_limits[0].rate_limit.primary_window.used_percent", Type: "jsonpath"},
    // ...
}
```

### 展示字段配置（改进后）
```go
DisplayFields: []DisplayField{
    // 顶部：账户状态
    {Key: "plan_type", Label: "Plan Type", ValueRef: "plan_type", Format: "text", DisplayOrder: 0, Badge: "plan"},
    {Key: "limit_reached", Label: "Limit Reached", ValueRef: "limit_reached", Format: "text", DisplayOrder: 1, Badge: "limit"},
    
    // Primary 分组
    {Key: "primary_used_pct", Label: "5h Window Used %", ValueRef: "primary_used_pct", Format: "percentage", DisplayOrder: 2, Group: "primary"},
    {Key: "primary_reset", Label: "5h Reset At", ValueRef: "primary_reset", Format: "datetime", DisplayOrder: 3, Group: "primary"},
    
    // Additional_0 分组（动态标题）
    {Key: "additional_0_primary_pct", Label: "5h Window Used %", ValueRef: "additional_0_primary_pct", Format: "percentage", 
     DisplayOrder: 6, Group: "additional_0", GroupLabelRef: "additional_0_name"},
    // ...
}
```

## 向后兼容性

✅ **完全兼容**：
- 已有渠道的自定义配置会保留
- `enrichDisplayFieldsFromTemplate` 函数会自动填充缺失的分组字段
- 用户可以随时编辑或覆盖分组配置

## 适用场景

此改进特别适合以下用户：
- **ChatGPT Pro Lite** 用户（有 extended_thinking 等计量功能）
- **ChatGPT Pro** 用户（有多个高级功能配额）
- **ChatGPT Team** 用户（可能有更多计量功能）

Free 和 Plus 用户通常没有 additional_rate_limits，所以只会看到 Primary 分组。

## 其他可改进的模板

### GitHub Copilot
当前已使用固定分组 `quota_snapshots`，可以进一步细化：
- `quota_snapshots_chat` 组
- `quota_snapshots_completions` 组  
- `quota_snapshots_premium` 组

### Apertis
可以分为：
- `subscription` 组（订阅周期配额）
- `payg` 组（按需付费余额）

这些改进可以在后续迭代中逐步完成。
