# Security

Orchis Foundry is built defensively. This document summarizes the security
posture honestly — both what's in place and the known limitations — so operators
can make an informed decision.

## In place
- **Credentials**: Personal access tokens are random, prefixed (`orc_pat_`),
  **Argon2id-hashed at rest**, shown once, scope-checked per request, and
  compared in constant time. Passwords are not used (OIDC sign-in only).
- **Sessions**: opaque, DB-backed, revocable; cookies are `HttpOnly`, `Secure`,
  `SameSite=Lax`, with IP/User-Agent binding and a 14-day TTL.
- **Web hardening**: strict CSP (`script-src 'self'`, no inline/eval/CDN),
  `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, a restrictive
  `Permissions-Policy`, and `Referrer-Policy`.
- **Injection**: all SQL is parameterized. Git is invoked via `exec.Command`
  (never a shell); ref and path inputs are allow-listed (`ValidRefName`,
  `validRepoPath`) and refs are resolved to SHAs before use.
- **XSS**: user-authored markdown (READMEs, comments) is HTML-escaped before any
  `dangerouslySetInnerHTML` injection; only `http(s)`/relative links are linkified.
- **SSRF**: outbound targets (webhook URLs, the BYO-model scanner base URL) are
  validated against loopback / RFC1918 / link-local / cloud-metadata ranges at
  configuration time and again before each request. Operators can opt in to
  internal targets with `ORCHIS_ALLOW_INTERNAL_TARGETS=1`.
- **Webhooks**: payloads are signed with HMAC-SHA256; secrets are not logged.
- **Posture**: MIT-licensed, no telemetry; the only outbound calls are to your
  configured OIDC provider and your configured model endpoint.

## Known limitations (see ROADMAP.md)
- Single-factor auth only — **no 2FA/MFA**.
- **No audit log** of administrative actions (compliance gap).
- No rate limiting / brute-force lockout on the API surface yet.
- SQLite single-node datastore (no clustering).

## Reporting a vulnerability
Email the maintainer (see repository owner) with details and reproduction steps.
Please do not open public issues for security reports.
