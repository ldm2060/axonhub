# Fix: IP blocklist did not match the request-log IP

Date: 2026-10-11

## Symptom

Banning an IP from the request-log quick action (or the security settings) did not block the client — requests kept succeeding.

## Root cause

Two different IP sources were used:

- Request log (`llm/httpclient.getClientIP`) records the first `X-Forwarded-For` entry, else `X-Real-IP`, else the TCP peer — unconditionally.
- Blocklist middleware (`internal/server/middleware/ip_blocklist.go`) checked only `c.ClientIP()` (Gin, trusted-proxy aware; with empty `server.trusted_proxies` this is the direct TCP peer).

Commit f4fffa76 (#2424) removed the XFF/X-Real-IP candidates from `clientIPCandidates` to harden allowlists. That also changed the blocklist: behind any reverse proxy (or with any XFF-bearing client) the logged IP never equals the checked peer IP, so bans never matched.

Reproduced locally: request from 127.0.0.1 with `X-Forwarded-For: 203.0.113.7` was logged as `203.0.113.7` and passed the middleware even with that IP banned.

## Fix

`WithIPBlocklist` now uses a dedicated `blocklistIPCandidates` that extends the trusted `c.ClientIP()` with exactly the candidate the request log records (first XFF entry, else X-Real-IP, deduped). Allowlist paths (`clientIPCandidates`, used by API-key `allowed_ips`) are unchanged and remain trusted-only — untrusted header values can only over-block the requester in a deny-list, never grant access.

Note: allowlists behind a reverse proxy additionally need `server.trusted_proxies` configured, otherwise `c.ClientIP()` is the proxy address; that is a config matter, not a code bug.

## Verification

- Unit tests added (`TestBlocklistIPCandidates`); both modules build/lint/test clean.
- Live: with `203.0.113.7` banned, requests with `X-Forwarded-For: 203.0.113.7` or `X-Real-IP: 203.0.113.7` → 403; no header or a different XFF → 200.

Also fixed a pre-existing gci import-order lint failure in `llm/transformer/openai/decisions/outbound.go` (from upstream merge 655b6c4c).
