# Using AxonHub as an OIDC Provider

AxonHub can act as an **OpenID Connect (OIDC) provider**, so third-party applications can implement "Sign in with AxonHub" for your users.

This is the opposite direction of [OIDC integration](oidc.md), which lets AxonHub users sign in through external providers such as Google or Logto.

## What is supported

- Authorization Code flow with PKCE (`code_challenge_method=S256`, required)
- Scopes: `openid` (required), `profile`, `email`
- `id_token` signed with RS256; public keys published via JWKS
- UserInfo endpoint with opaque access tokens
- Confidential clients (client secret) and public clients (PKCE only)

Refresh tokens, implicit flow and `client_credentials` are not supported.

## Requirements

Set `server.public_url` to the externally reachable URL of your AxonHub instance. The discovery document, redirects and issuer claim are all built from it, and the OIDC flow will not start without it.

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

## Registering a client

1. Sign in as the system owner and open **Admin → OAuth Applications**.
2. Click **New application** and fill in:
   - **Name** — shown on the consent screen.
   - **Client type** — `Confidential` for server-side applications that can keep a secret; `Public (PKCE)` for SPAs and native apps.
   - **Redirect URIs** — one per line. Redirect URIs are matched exactly, so avoid trailing slashes unless you register them explicitly.
3. Copy the generated **client secret** — it is shown only once. Client IDs and secrets can be rotated later from the application menu.

Disabled or deleted applications can no longer complete the flow; existing users are not signed out of the third-party application, but no new sign-ins can be authorized.

## Connecting a third-party application

Configure the application with:

- Issuer / discovery URL: `https://axonhub.example.com`
- Authorization endpoint: `https://axonhub.example.com/oauth2/authorize`
- Token endpoint: `https://axonhub.example.com/oauth2/token`
- UserInfo endpoint: `https://axonhub.example.com/oauth2/userinfo`
- JWKS URL: `https://axonhub.example.com/oauth2/jwks.json`
- Redirect URI: the exact URI registered above, e.g. `https://app.example.com/oauth/callback`
- Scopes: `openid profile email`

The flow is standard:

1. The application redirects the browser to `/oauth2/authorize` with `client_id`, `redirect_uri`, `response_type=code`, `scope`, `state`, `nonce` and the PKCE `code_challenge`.
2. If the user is not signed in to AxonHub, they are asked to sign in first, then shown a consent screen listing the requesting application and the scopes.
3. After approval, the browser is redirected back to the application with `code` and `state`. Denying redirects back with `error=access_denied`.
4. The application exchanges the code at `/oauth2/token` (with `client_secret` for confidential clients, `code_verifier` for PKCE).
5. The response contains `access_token`, `id_token`, `token_type`, `expires_in` and `scope`. Verify the `id_token` against the JWKS; call `/oauth2/userinfo` for profile claims.

## Claims

| Claim | Scope | Notes |
|---|---|---|
| `sub` | `openid` | Stable AxonHub user ID |
| `email`, `email_verified` | `email` | `email_verified` reflects whether the address was verified in AxonHub |
| `name`, `given_name`, `family_name`, `preferred_username`, `picture` | `profile` | |

## Security notes

- The authorization endpoint requires PKCE for every client type.
- Authorization codes are single-use, expire after 5 minutes, and are bound to the client, redirect URI and PKCE challenge.
- Access tokens and authorization requests are stored in the configured cache (`cache.mode`). With the default in-memory cache, a restart invalidates pending codes and tokens, and multi-instance deployments must use Redis for the token and code stores to be shared.
- Signing keys are generated on first use and stored in the system settings table.
