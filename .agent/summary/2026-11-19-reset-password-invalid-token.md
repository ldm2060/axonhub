# 重置密码总是报 "Invalid or expired reset token"

日期：2026-11-19
分支：unstable

## 现象

用户通过邮件里的重置链接打开 `/reset-password?token=...`，填新密码提交后，
无论重试多少次都返回 `Invalid or expired reset token`（HTTP 400）。

## 根因

不是一个 bug，是两个 bug 叠在一起：

1. **`UserService.ResetPassword` 缺 Ent 隐私旁路。**
   `/auth/reset-password` 是未认证的公开路由，`c.Request.Context()` 里没有任何
   authz principal。`ResetPassword` 直接用 `s.entFromContext(ctx).User.UpdateOneID(...)`
   写库，被 `User.Policy()` 的 `MutationPolicy` 拒绝，返回 500
   `Failed to reset password`。
   同文件里语义等价的 `UpdateSelfPassword`、`GetByEmail` 都包了
   `authz.RunWithSystemBypass`，只有这三个公开入口漏了：
   `ResetPassword`、`MarkEmailVerified`、`ActivateUser`
   （后两个是 `/auth/verify-email` 用的，同样坏）。

2. **令牌在动作成功之前就被消费掉了。**
   原 handler 的顺序是 `ValidateToken → ConsumeToken → ResetPassword`。
   因为第 1 点，`ResetPassword` 必然失败，但令牌已经在第 2 步被标记
   `consumed_at`。一次性令牌被烧掉，之后每次重试
   `ValidateToken` 都过不了 `ConsumedAtIsNil()`，于是稳定复现
   `Invalid or expired reset token` —— 这就是"总是"的原因。

复现顺序（本地实测）：

```
POST /auth/forgot-password            → 200，写入 reset_password 令牌
POST /auth/reset-password             → 500 Failed to reset password（令牌已 consumed）
POST /auth/reset-password（同一令牌） → 400 Invalid or expired reset token
```

## 修复

`internal/server/biz/user.go`
- `ResetPassword` / `MarkEmailVerified` / `ActivateUser` 全部包进
  `authz.RunWithSystemBypassVoid`，缓存失效仍用外层 ctx。
- `MarkEmailVerified` 的 `time.Now()` 换成 `xtime.UTCNow()`，与令牌链路一致。

`internal/server/api/email_token.go`
- `ResetPassword`：改成 `ValidateToken → ResetPassword → ConsumeToken`。
  密码改成功之后再烧令牌；消费失败只记日志不失败请求（密码已经改了，
  再报错会让用户以为没成功）。
- `VerifyEmail`：同样的顺序调整，抽出 `consumeVerificationToken` 辅助函数，
  各成功分支调用它；失败仅记录。修掉之前"验证写库一失败，链接即失效"的同类陷阱。

## 验证

单元测试（新增 `internal/server/biz/email_token_public_flow_test.go`）：
- 用**裸 context**（等价 Gin 未认证请求）跑 CreateToken → ValidateToken →
  ResetPassword 全链路，断言密码真的被改写、令牌随后一次性失效。
- `MarkEmailVerified` + `ActivateUser` 在裸 context 下生效，
  `email_verified_at` 时区为 UTC。
- 重置失败时令牌不被消费，可以重试。

端到端（本地 8095 + 独立 DB 副本）：
- 重置：forgot 200 → reset 200 → 新密码登录 200 → 旧密码登录 401。
- 验证：verify-email 302 `?verified=1` → 用户 activated + verified；
  重放同一链接 302 `?verified=0`。
- 浏览器实测 `/reset-password?token=...` 表单提交，出现
  "密码重置成功 / 您的密码已重置"，API 侧确认密码已变更。

`go build ./...`、`go build ./...`（llm）、`gofmt`、`go vet`、
`go test ./internal/server/...` 全部通过。

## 影响面

这三个方法只有 `internal/server/api/email_token.go` 三个公开 handler 在调
（已 grep 确认），不会影响任何已认证流程。
