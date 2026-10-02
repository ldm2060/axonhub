# 将 AxonHub 作为 OIDC Provider

AxonHub 可以作为 **OpenID Connect (OIDC) 身份提供商**，让第三方应用接入「通过 AxonHub 登录」。本文档面向对接方开发者，说明如何完成接入。

这与 [OIDC 集成](oidc.md) 的方向相反：后者是 AxonHub 用户通过 Google、Logto 等外部提供商登录。

## 支持范围

- 授权码流程（Authorization Code）+ PKCE（必须使用 `code_challenge_method=S256`，所有客户端类型均强制要求）
- Scope：`openid`（必需）、`profile`、`email`
- `id_token` 使用 RS256 签名，公钥通过 JWKS 发布
- UserInfo 端点（不透明访问令牌）
- 机密客户端（带密钥）与公开客户端（仅 PKCE，适用于 SPA、原生应用）

暂不支持：refresh token、隐式流程（implicit）、`client_credentials`、`id_token_hint` / `prompt` 等可选参数。

## 前置条件

AxonHub 必须配置 `server.public_url`，指向其对外可访问地址。发现文档、回调地址与 `iss` 声明都由它生成，未配置时授权端点会直接报错。

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

所有支持 OIDC 发现的标准客户端库都可以直接使用，把 issuer 配置为 `https://axonhub.example.com` 即可（库会自动读取上述端点）。

## 注册客户端（管理员操作）

1. 以系统所有者身份登录 AxonHub，打开 **管理 → OAuth 应用**。
2. 点击 **新建应用**，填写：
   - **名称**：显示在授权同意页上，对接方用户会看到。
   - **客户端类型**：
     - `机密客户端`：服务端应用，能安全保管密钥，用 `client_secret` 认证；
     - `公开客户端（PKCE）`：SPA、移动/桌面应用等无法保管密钥的场景，只用 PKCE。
   - **回调地址**：每行一个，**必须与对接方实际请求的 `redirect_uri` 完全一致**（协议、主机、端口、路径、结尾斜杠都参与比较）。
3. 创建后立即复制生成的 **客户端密钥**——它只显示一次。之后可在应用菜单中轮换（轮换后旧密钥立即失效）。

## 对接方配置清单

| 配置项 | 值 |
|---|---|
| Issuer / 发现地址 | `https://axonhub.example.com` |
| Client ID | 创建应用时生成（`ahc_` 前缀） |
| Client Secret | 创建/轮换时显示一次（`ahs_` 前缀）；公开客户端没有 |
| Redirect URI | 与注册值完全一致的地址，例如 `https://app.example.com/oauth/callback` |
| Scope | `openid profile email`（按需裁剪，`openid` 必需） |

## 授权流程

### 1. 发起登录：跳转授权端点

应用在服务端生成并保存 `state`、`nonce` 与 PKCE 的 `code_verifier`，然后让浏览器跳转到：

```
GET https://axonhub.example.com/oauth2/authorize
  ?client_id=ahc_xxxxxxxx
  &redirect_uri=https%3A%2F%2Fapp.example.com%2Foauth%2Fcallback
  &response_type=code
  &scope=openid%20profile%20email
  &state=<随机值，原样返回>
  &nonce=<随机值，写入 id_token>
  &code_challenge=<BASE64URL(SHA256(code_verifier))，无填充>
  &code_challenge_method=S256
```

| 参数 | 必填 | 说明 |
|---|---|---|
| `client_id` | 是 | 应用创建时生成 |
| `redirect_uri` | 是 | 必须与注册值完全一致 |
| `response_type` | 是 | 仅支持 `code` |
| `scope` | 是 | 必须包含 `openid`，可用 `profile`、`email` |
| `state` | 建议 | 防 CSRF，原样回传 |
| `nonce` | 建议 | 会写入 `id_token`，用于防重放 |
| `code_challenge` | 是 | PKCE challenge，S256 |
| `code_challenge_method` | 是 | 必须为 `S256` |

### 2. AxonHub 侧：登录与授权同意

- 如果用户尚未登录 AxonHub，会先跳到登录页，登录后自动回到授权同意页；
- 同意页展示请求方名称、回调域名、申请的 scope 列表，以及当前登录账号；
- 用户点击「允许」后，浏览器被重定向回 `redirect_uri`，携带 `code` 与 `state`；
- 点击「拒绝」时，重定向携带 `error=access_denied` 与 `state`。

### 3. 换取令牌

应用在服务端用授权码调用令牌端点（不要在浏览器里做这一步）：

```
POST https://axonhub.example.com/oauth2/token
Content-Type: application/x-www-form-urlencoded

grant_type=authorization_code
&code=<上一步拿到的 code>
&redirect_uri=<与授权请求一致>
&client_id=ahc_xxxxxxxx
&client_secret=<机密客户端必填>
&code_verifier=<第一步生成的 verifier>
```

客户端认证支持两种方式，任选其一：
- `client_secret_post`：`client_id`、`client_secret` 放在表单里（上面示例）；
- `client_secret_basic`：`Authorization: Basic base64(client_id:client_secret)`。

公开客户端没有密钥，只传 `client_id` 与 `code_verifier`。

成功响应（`200`，`Cache-Control: no-store`）：

```json
{
  "access_token": "aho_xxxxxxxx...",
  "id_token": "eyJhbGciOiJSUzI1NiIs...",
  "token_type": "Bearer",
  "expires_in": 3600,
  "scope": "openid profile email"
}
```

### 4. 校验 id_token 并建立会话

- 用 JWKS 校验签名，算法固定为 `RS256`，按 `kid` 选择公钥；
- 校验 `iss`（等于 AxonHub 的 `server.public_url`）、`aud`（等于自己的 `client_id`）、`exp`；
- 校验 `nonce` 与第一步保存的值一致；
- `sub` 是用户在 AxonHub 中的稳定唯一标识，**请以它作为账号关联键**（`sub` 不会因邮箱、姓名变化而改变）。

### 5. 获取用户资料（可选）

```
GET https://axonhub.example.com/oauth2/userinfo
Authorization: Bearer aho_xxxxxxxx...
```

返回的 claims 与 `id_token` 中一致（按已授权的 scope 裁剪）。

## 令牌与有效期

| 项目 | 说明 |
|---|---|
| 授权请求（同意页） | 10 分钟内有效 |
| 授权码 | 一次性使用，5 分钟过期，绑定 client / redirect_uri / PKCE |
| Access Token | 不透明字符串（`aho_` 前缀，**不是 JWT，不要解析**），1 小时有效，仅用于 `/oauth2/userinfo` |
| ID Token | JWT（RS256），1 小时有效 |
| Refresh Token | **不支持**；令牌过期后重新走一次授权流程 |

## Claims

| Claim | 所需 Scope | 说明 |
|---|---|---|
| `sub` | `openid` | 稳定的 AxonHub 用户 ID |
| `iss` / `aud` / `iat` / `exp` / `auth_time` / `nonce` | `openid` | 标准声明；`nonce` 仅在请求中带了才返回 |
| `email`、`email_verified` | `email` | `email_verified` 反映邮箱在 AxonHub 中是否已验证 |
| `name`、`given_name`、`family_name`、`preferred_username`、`picture` | `profile` | `picture` 为 AxonHub 头像地址 |

## 错误处理

**授权端点**（错误分两类）：

| 情况 | 表现 |
|---|---|
| `client_id` 不存在/已禁用，或 `redirect_uri` 未注册 | `400` 纯文本错误页，**不跳转**（防止授权码泄露到未注册地址） |
| `response_type` 不为 `code` | 跳回 `redirect_uri`：`error=unsupported_response_type` |
| `scope` 缺失、不含 `openid` 或包含未知值 | 跳回 `redirect_uri`：`error=invalid_scope` |
| 缺少 PKCE 参数或 `code_challenge_method` 不为 `S256` | 跳回 `redirect_uri`：`error=invalid_request` |
| 用户拒绝 | 跳回 `redirect_uri`：`error=access_denied` |

跳回时都会原样携带 `state` 与 `error_description`。

**令牌端点**（JSON 错误体）：

| 情况 | HTTP | `error` |
|---|---|---|
| `client_secret` 错误，或客户端已禁用/删除 | `401` | `invalid_client` |
| 授权码无效/过期/已使用、`redirect_uri` 不符、PKCE 校验失败 | `400` | `invalid_grant` |
| `grant_type` 不是 `authorization_code` | `400` | `unsupported_grant_type` |
| 缺少 `code` | `400` | `invalid_request` |

**UserInfo 端点**：

| 情况 | HTTP | `error` |
|---|---|---|
| 未携带 Bearer 令牌 | `400` | `invalid_request` |
| 令牌无效/过期（含 `WWW-Authenticate: Bearer error="invalid_token"`） | `401` | `invalid_token` |

## 代码示例

### Node.js（Express + jose）

```js
// npm i express jose
import express from 'express';
import crypto from 'node:crypto';
import { createRemoteJWKSet, jwtVerify } from 'jose';

const ISSUER = 'https://axonhub.example.com';
const CLIENT_ID = process.env.AXONHUB_CLIENT_ID;
const CLIENT_SECRET = process.env.AXONHUB_CLIENT_SECRET;
const REDIRECT_URI = 'https://app.example.com/oauth/callback';

const metadata = await fetch(`${ISSUER}/.well-known/openid-configuration`).then((r) => r.json());
const jwks = createRemoteJWKSet(new URL(metadata.jwks_uri));

const app = express();
const pending = new Map(); // 示例用内存存储；生产环境请换成会话存储

app.get('/auth/login', (req, res) => {
  const state = crypto.randomBytes(16).toString('hex');
  const nonce = crypto.randomBytes(16).toString('hex');
  const verifier = crypto.randomBytes(32).toString('base64url');
  const challenge = crypto.createHash('sha256').update(verifier).digest('base64url');

  pending.set(state, { verifier, nonce });

  const url = new URL(metadata.authorization_endpoint);
  url.searchParams.set('client_id', CLIENT_ID);
  url.searchParams.set('redirect_uri', REDIRECT_URI);
  url.searchParams.set('response_type', 'code');
  url.searchParams.set('scope', 'openid profile email');
  url.searchParams.set('state', state);
  url.searchParams.set('nonce', nonce);
  url.searchParams.set('code_challenge', challenge);
  url.searchParams.set('code_challenge_method', 'S256');
  res.redirect(url.toString());
});

app.get('/oauth/callback', async (req, res) => {
  const { code, state, error, error_description } = req.query;
  if (error) return res.status(400).send(`${error}: ${error_description ?? ''}`);

  const saved = pending.get(state);
  if (!saved) return res.status(400).send('invalid state');
  pending.delete(state);

  const tokenRes = await fetch(metadata.token_endpoint, {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body: new URLSearchParams({
      grant_type: 'authorization_code',
      code,
      redirect_uri: REDIRECT_URI,
      client_id: CLIENT_ID,
      client_secret: CLIENT_SECRET,
      code_verifier: saved.verifier,
    }),
  });
  if (!tokenRes.ok) return res.status(502).send(await tokenRes.text());
  const tokens = await tokenRes.json();

  const { payload } = await jwtVerify(tokens.id_token, jwks, {
    issuer: metadata.issuer,
    audience: CLIENT_ID,
  });
  if (payload.nonce !== saved.nonce) return res.status(400).send('invalid nonce');

  const userinfo = await fetch(metadata.userinfo_endpoint, {
    headers: { Authorization: `Bearer ${tokens.access_token}` },
  }).then((r) => r.json());

  // payload.sub 为稳定用户标识
  res.json({ sub: payload.sub, email: payload.email, name: payload.name, userinfo });
});
```

### Python（requests + PyJWT）

```python
# pip install requests PyJWT cryptography
import base64, hashlib, secrets
from urllib.parse import urlencode

import jwt  # PyJWT
import requests

ISSUER = "https://axonhub.example.com"
CLIENT_ID = "..."
CLIENT_SECRET = "..."
REDIRECT_URI = "https://app.example.com/oauth/callback"

metadata = requests.get(f"{ISSUER}/.well-known/openid-configuration").json()
jwks_client = jwt.PyJWKClient(metadata["jwks_uri"])


def build_login_url() -> tuple[str, dict]:
    verifier = secrets.token_urlsafe(48)
    challenge = base64.urlsafe_b64encode(
        hashlib.sha256(verifier.encode()).digest()
    ).rstrip(b"=").decode()
    state, nonce = secrets.token_urlsafe(16), secrets.token_urlsafe(16)

    params = {
        "client_id": CLIENT_ID,
        "redirect_uri": REDIRECT_URI,
        "response_type": "code",
        "scope": "openid profile email",
        "state": state,
        "nonce": nonce,
        "code_challenge": challenge,
        "code_challenge_method": "S256",
    }
    # 将 state -> {verifier, nonce} 存入服务端会话
    return f"{metadata['authorization_endpoint']}?{urlencode(params)}", {"state": state, "verifier": verifier, "nonce": nonce}


def handle_callback(code: str, session: dict) -> dict:
    resp = requests.post(
        metadata["token_endpoint"],
        data={
            "grant_type": "authorization_code",
            "code": code,
            "redirect_uri": REDIRECT_URI,
            "client_id": CLIENT_ID,
            "client_secret": CLIENT_SECRET,
            "code_verifier": session["verifier"],
        },
        timeout=10,
    )
    resp.raise_for_status()
    tokens = resp.json()

    signing_key = jwks_client.get_signing_key_from_jwt(tokens["id_token"])
    claims = jwt.decode(
        tokens["id_token"],
        signing_key.key,
        algorithms=["RS256"],
        audience=CLIENT_ID,
        issuer=ISSUER,
    )
    assert claims["nonce"] == session["nonce"]

    userinfo = requests.get(
        metadata["userinfo_endpoint"],
        headers={"Authorization": f"Bearer {tokens['access_token']}"},
        timeout=10,
    ).json()
    return {"claims": claims, "userinfo": userinfo}
```

### 快速手工验证（curl）

```bash
# 1) 浏览器打开授权地址（含上表全部参数），完成登录与授权后从回调地址取 code

# 2) 换取令牌
curl -s https://axonhub.example.com/oauth2/token \
  -d grant_type=authorization_code \
  -d code="<code>" \
  -d redirect_uri="https://app.example.com/oauth/callback" \
  -d client_id="ahc_xxx" \
  -d client_secret="ahs_xxx" \
  -d code_verifier="<verifier>"

# 3) 获取用户资料
curl -s https://axonhub.example.com/oauth2/userinfo \
  -H "Authorization: Bearer <access_token>"
```

## 安全与部署注意事项

- **回调地址严格匹配**：注册值与请求值必须逐字符一致，这是防止授权码被劫持的第一道防线。
- **PKCE 强制**：所有客户端（包括机密客户端）都必须携带 `code_challenge`；授权码一次性使用并绑定 client、redirect_uri 与 PKCE。
- **多实例部署必须使用 Redis**：授权码与访问令牌存放在缓存中（`cache.mode`）。默认内存缓存下，重启会使待兑换的授权码与已签发的访问令牌失效，多实例之间也无法共享；生产多实例请配置 `cache.mode: redis`。
- **签名密钥**：首次使用时自动生成并保存在系统设置中，重启不变化；`id_token` 请始终通过 JWKS 校验，不要固定写死公钥。
- **令牌不要落前端**：`client_secret` 与令牌交换必须在服务端完成；公开客户端没有密钥，但同样要使用 PKCE。
- **登出语义**：AxonHub 侧的登出不会使第三方应用的会话失效；第三方应用需要自行维护会话与登出。
