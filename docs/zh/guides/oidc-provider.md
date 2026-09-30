# 将 AxonHub 作为 OIDC Provider

AxonHub 可以作为 **OpenID Connect (OIDC) 身份提供商**，让第三方应用接入「通过 AxonHub 登录」。

这与 [OIDC 集成](oidc.md) 的方向相反：后者是 AxonHub 用户通过 Google、Logto 等外部提供商登录。

## 支持范围

- 授权码流程 + PKCE（必须使用 `code_challenge_method=S256`）
- Scope：`openid`（必需）、`profile`、`email`
- `id_token` 使用 RS256 签名，公钥通过 JWKS 发布
- UserInfo 端点（Bearer 访问令牌）
- 机密客户端（带密钥）与公开客户端（仅 PKCE）

暂不支持 refresh token、隐式流程和 `client_credentials`。

## 前置条件

必须配置 `server.public_url`，指向 AxonHub 对外可访问的地址。发现文档、回调地址和 issuer 声明都由它生成，未配置时无法发起 OIDC 流程。

```yaml
server:
  public_url: "https://axonhub.example.com"
```

## 端点

| 端点 | 用途 |
|---|---|
| `/.well-known/openid-configuration` | OIDC 发现文档 |
| `/oauth2/authorize` | 授权端点（浏览器跳转） |
| `/oauth2/token` | 令牌端点（授权码换取令牌） |
| `/oauth2/userinfo` | UserInfo 端点（Bearer 访问令牌） |
| `/oauth2/jwks.json` | 签名公钥（JWKS） |

## 注册客户端

1. 以系统所有者身份登录，打开 **管理 → OAuth 应用**。
2. 点击 **新建应用**，填写：
   - **名称**：显示在授权同意页上。
   - **客户端类型**：服务端应用可选 `机密客户端`（能保管密钥）；SPA 和原生应用选 `公开客户端（PKCE）`。
   - **回调地址**：每行一个，必须完全匹配，注意结尾斜杠需要显式注册。
3. 复制生成的 **客户端密钥**——它只显示一次。之后可以在应用菜单中轮换。

禁用或删除应用后，该应用无法再完成登录流程；已登录的第三方会话不会失效，但无法再发起新的授权。

## 第三方应用接入

在第三方应用中配置：

- Issuer / 发现地址：`https://axonhub.example.com`
- 授权端点：`https://axonhub.example.com/oauth2/authorize`
- 令牌端点：`https://axonhub.example.com/oauth2/token`
- UserInfo 端点：`https://axonhub.example.com/oauth2/userinfo`
- JWKS 地址：`https://axonhub.example.com/oauth2/jwks.json`
- 回调地址：上面注册的完全一致的地址，例如 `https://app.example.com/oauth/callback`
- Scope：`openid profile email`

流程为标准 OIDC：

1. 应用把浏览器重定向到 `/oauth2/authorize`，携带 `client_id`、`redirect_uri`、`response_type=code`、`scope`、`state`、`nonce` 以及 PKCE 的 `code_challenge`。
2. 如果用户尚未登录 AxonHub，会先要求登录，然后显示授权同意页，列出请求方和申请的 scope。
3. 用户同意后，浏览器带着 `code` 和 `state` 跳回应用；拒绝则带 `error=access_denied` 跳回。
4. 应用在 `/oauth2/token` 用授权码换取令牌（机密客户端带 `client_secret`，PKCE 带 `code_verifier`）。
5. 响应包含 `access_token`、`id_token`、`token_type`、`expires_in` 和 `scope`。请用 JWKS 校验 `id_token`；用户资料通过 `/oauth2/userinfo` 获取。

## Claims

| Claim | 所需 Scope | 说明 |
|---|---|---|
| `sub` | `openid` | 稳定的 AxonHub 用户 ID |
| `email`、`email_verified` | `email` | `email_verified` 反映邮箱在 AxonHub 中是否已验证 |
| `name`、`given_name`、`family_name`、`preferred_username`、`picture` | `profile` | |

## 安全注意事项

- 授权端点对所有类型的客户端都强制 PKCE。
- 授权码一次性使用，5 分钟过期，并绑定客户端、回调地址和 PKCE challenge。
- 访问令牌与授权请求存放在配置的缓存中（`cache.mode`）。默认使用内存缓存时，重启会使待处理的授权码和令牌失效；多实例部署需要配置 Redis，否则不同实例之间无法共享授权码和令牌。
- 签名密钥在首次使用时生成，保存在系统设置表中。
