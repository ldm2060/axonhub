# Using AxonHub as an OIDC Provider

AxonHub can act as an **OpenID Connect (OIDC) provider**, so third-party applications can implement "Sign in with AxonHub". This guide is written for the integrating developer.

This is the opposite direction of [OIDC integration](oidc.md), which lets AxonHub users sign in through external providers such as Google or Logto.

## What is supported

- Authorization Code flow with PKCE (`code_challenge_method=S256`, required for every client type)
- Scopes: `openid` (required), `profile`, `email`
- `id_token` signed with RS256; public keys published via JWKS
- UserInfo endpoint with opaque access tokens
- Confidential clients (client secret) and public clients (PKCE only, for SPAs and native apps)

Not supported: refresh tokens, implicit flow, `client_credentials`, and optional parameters such as `id_token_hint` / `prompt`.

## Requirements

AxonHub must have `server.public_url` set to its externally reachable URL. The discovery document, redirects and the `iss` claim are all built from it, and the authorization endpoint fails without it.

```yaml
server:
  public_url: "https://axonhub.example.com"
```

## Endpoints

| Endpoint | Purpose |
|---|---|
| `/.well-known/openid-configuration` | OIDC discovery document |
| `/oauth2/authorize` | Authorization endpoint (browser redirect) |
| `/oauth2/token` | Token endpoint (authorization code exchange) |
| `/oauth2/userinfo` | UserInfo endpoint (Bearer access token) |
| `/oauth2/jwks.json` | Signing keys (JWKS) |

Any standard OIDC client library that supports discovery can be used directly by setting the issuer to `https://axonhub.example.com`.

## Registering a client (administrator)

1. Sign in to AxonHub as the system owner and open **Admin → OAuth Applications**.
2. Click **New application** and fill in:
   - **Name** — shown on the consent screen to the third-party application's users.
   - **Client type**:
     - `Confidential` — server-side applications that can keep a secret; they authenticate with `client_secret`.
     - `Public (PKCE)` — SPAs and native apps that cannot keep a secret; PKCE only.
   - **Redirect URIs** — one per line. They must match the `redirect_uri` the application actually sends **exactly** (scheme, host, port, path and trailing slash all count).
3. Copy the generated **client secret** immediately — it is shown only once. It can be rotated later from the application menu (the old secret stops working at once).

## Configuration checklist for the integrating application

| Setting | Value |
|---|---|
| Issuer / discovery URL | `https://axonhub.example.com` |
| Client ID | Generated on creation (`ahc_` prefix) |
| Client Secret | Shown once on creation/rotation (`ahs_` prefix); public clients have none |
| Redirect URI | Exactly the registered value, e.g. `https://app.example.com/oauth/callback` |
| Scopes | `openid profile email` (trim as needed; `openid` is required) |

## Authorization flow

### 1. Start sign-in: redirect to the authorization endpoint

The application generates and stores `state`, `nonce` and the PKCE `code_verifier` server-side, then sends the browser to:

```
GET https://axonhub.example.com/oauth2/authorize
  ?client_id=ahc_xxxxxxxx
  &redirect_uri=https%3A%2F%2Fapp.example.com%2Foauth%2Fcallback
  &response_type=code
  &scope=openid%20profile%20email
  &state=<random, echoed back>
  &nonce=<random, embedded in the id_token>
  &code_challenge=<BASE64URL(SHA256(code_verifier)), no padding>
  &code_challenge_method=S256
```

| Parameter | Required | Notes |
|---|---|---|
| `client_id` | Yes | Generated when the application was registered |
| `redirect_uri` | Yes | Must exactly match a registered URI |
| `response_type` | Yes | Only `code` is supported |
| `scope` | Yes | Must include `openid`; `profile` and `email` are optional |
| `state` | Recommended | CSRF protection; echoed back unchanged |
| `nonce` | Recommended | Embedded in the `id_token` for replay protection |
| `code_challenge` | Yes | PKCE challenge, S256 |
| `code_challenge_method` | Yes | Must be `S256` |

### 2. AxonHub side: login and consent

- If the user is not signed in to AxonHub, they are sent to the sign-in page first and return to the consent page afterwards.
- The consent page shows the requesting application, its callback host, the requested scopes and the signed-in account.
- On **Allow**, the browser is redirected back to `redirect_uri` with `code` and `state`.
- On **Deny**, the redirect carries `error=access_denied` and `state`.

### 3. Exchange the code for tokens

The application calls the token endpoint from its backend (never from the browser):

```
POST https://axonhub.example.com/oauth2/token
Content-Type: application/x-www-form-urlencoded

grant_type=authorization_code
&code=<code from step 2>
&redirect_uri=<same value as in the authorization request>
&client_id=ahc_xxxxxxxx
&client_secret=<required for confidential clients>
&code_verifier=<verifier generated in step 1>
```

Client authentication supports either method:
- `client_secret_post`: `client_id` and `client_secret` in the form body (as above);
- `client_secret_basic`: `Authorization: Basic base64(client_id:client_secret)`.

Public clients have no secret and send only `client_id` and `code_verifier`.

Successful response (`200`, `Cache-Control: no-store`):

```json
{
  "access_token": "aho_xxxxxxxx...",
  "id_token": "eyJhbGciOiJSUzI1NiIs...",
  "token_type": "Bearer",
  "expires_in": 3600,
  "scope": "openid profile email"
}
```

### 4. Verify the id_token and establish a session

- Verify the signature with the JWKS; the algorithm is always `RS256` and the key is selected by `kid`.
- Validate `iss` (equals AxonHub's `server.public_url`), `aud` (equals your `client_id`) and `exp`.
- Validate `nonce` against the value saved in step 1.
- `sub` is the user's stable identifier in AxonHub — **use it as the account linkage key** (it does not change when the email or name changes).

### 5. Fetch the profile (optional)

```
GET https://axonhub.example.com/oauth2/userinfo
Authorization: Bearer aho_xxxxxxxx...
```

The claims match those in the `id_token`, filtered by the granted scopes.

## Tokens and lifetimes

| Item | Notes |
|---|---|
| Authorization request (consent page) | Valid for 10 minutes |
| Authorization code | Single-use, expires in 5 minutes, bound to the client, redirect URI and PKCE challenge |
| Access token | Opaque string (`aho_` prefix, **not a JWT — do not parse it**), valid 1 hour, only usable with `/oauth2/userinfo` |
| ID token | JWT (RS256), valid 1 hour |
| Refresh token | **Not supported**; run the authorization flow again when tokens expire |

## Claims

| Claim | Scope | Notes |
|---|---|---|
| `sub` | `openid` | Stable AxonHub user ID |
| `iss` / `aud` / `iat` / `exp` / `auth_time` / `nonce` | `openid` | Standard claims; `nonce` is returned only when requested |
| `email`, `email_verified` | `email` | `email_verified` reflects whether the address was verified in AxonHub |
| `name`, `given_name`, `family_name`, `preferred_username`, `picture` | `profile` | `picture` points to the AxonHub avatar |

## Error handling

**Authorization endpoint** (two classes of errors):

| Case | Behaviour |
|---|---|
| Unknown/disabled `client_id`, or unregistered `redirect_uri` | `400` plain-text error page, **no redirect** (so codes can never leak to an unregistered URI) |
| `response_type` is not `code` | Redirect back: `error=unsupported_response_type` |
| `scope` missing, without `openid`, or with unknown values | Redirect back: `error=invalid_scope` |
| PKCE parameters missing or `code_challenge_method` is not `S256` | Redirect back: `error=invalid_request` |
| User denies | Redirect back: `error=access_denied` |

Redirects always carry the original `state` plus `error_description`.

**Token endpoint** (JSON error body):

| Case | HTTP | `error` |
|---|---|---|
| Wrong `client_secret`, or client disabled/deleted | `401` | `invalid_client` |
| Code invalid/expired/used, `redirect_uri` mismatch, PKCE failure | `400` | `invalid_grant` |
| `grant_type` is not `authorization_code` | `400` | `unsupported_grant_type` |
| Missing `code` | `400` | `invalid_request` |

**UserInfo endpoint**:

| Case | HTTP | `error` |
|---|---|---|
| No bearer token | `400` | `invalid_request` |
| Invalid/expired token (with `WWW-Authenticate: Bearer error="invalid_token"`) | `401` | `invalid_token` |

## Code samples

### Node.js (Express + jose)

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
const pending = new Map(); // in-memory for the example; use a session store in production

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

  // payload.sub is the stable user identifier
  res.json({ sub: payload.sub, email: payload.email, name: payload.name, userinfo });
});
```

### Python (requests + PyJWT)

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
    # store state -> {verifier, nonce} in the server-side session
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

### Quick manual check (curl)

```bash
# 1) Open the authorization URL in a browser (with all parameters above);
#    after login and consent, take the code from the callback URL.

# 2) Exchange the code
curl -s https://axonhub.example.com/oauth2/token \
  -d grant_type=authorization_code \
  -d code="<code>" \
  -d redirect_uri="https://app.example.com/oauth/callback" \
  -d client_id="ahc_xxx" \
  -d client_secret="ahs_xxx" \
  -d code_verifier="<verifier>"

# 3) Fetch the profile
curl -s https://axonhub.example.com/oauth2/userinfo \
  -H "Authorization: Bearer <access_token>"
```

## Security and deployment notes

- **Exact redirect URI matching**: the registered and requested values must be identical character for character; this is the first line of defence against authorization-code interception.
- **PKCE is mandatory**: every client (confidential included) must send `code_challenge`; codes are single-use and bound to the client, redirect URI and PKCE challenge.
- **Multi-instance deployments need Redis**: authorization codes and access tokens live in the cache (`cache.mode`). With the default in-memory cache, a restart invalidates pending codes and issued tokens, and instances cannot share them; use `cache.mode: redis` in production.
- **Signing keys**: generated automatically on first use and stored in the system settings table; they survive restarts. Always verify `id_token`s through the JWKS rather than pinning a public key.
- **Keep tokens off the frontend**: the `client_secret` and the token exchange belong on the server; public clients have no secret but must still use PKCE.
- **Logout semantics**: signing out of AxonHub does not end sessions in third-party applications; they manage their own sessions and logout.
