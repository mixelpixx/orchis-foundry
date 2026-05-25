# Roadmap

Orchis Foundry today is a focused, self-hosted Git + code-review platform for
solo developers and small teams (see the scope matrix in the README). The items
below are the deliberate next steps to grow it toward larger/team and regulated
deployments. They are **not implemented yet** — listed here honestly, with rough
effort, so a prospective owner can scope and prioritize.

Foundations that make these cheaper: the schema already contains unused `orgs`
and `org_members` tables, and auth/ACL is centralized (`internal/auth`,
`internal/api/acl.go`).

## Already shipped
- **Local email+password** accounts (Argon2id).
- **Generic OIDC** (Microsoft Entra ID, Google, Okta, Keycloak, …) + GitHub.
- **Admin panel**: toggle sign-in methods; create / disable (deprovision) /
  grant-admin / reset-password users.

## Identity & access (next)
- **SAML SSO** for IdPs that require it (some Okta/ADFS setups). *(~2–3 weeks)*
- **Audit log** of admin/auth actions (common compliance requirement). *(~1–2 weeks)*
- **SCIM** auto-provisioning + **invite links** + self-service password reset
  (reset needs email — see below). *(~2 weeks)*
- **2FA / MFA** enforcement. *(~1 week)*

## Collaboration at scale
- **Organizations & teams** — wire the existing `orgs`/`org_members` schema and
  add team-based, bulk repo permissions. *(~2–3 weeks)*
- **Branch protection / required reviews** — block merges until N approvals and
  required checks pass. *(~1–2 weeks)*

## Platform & operations
- **PostgreSQL backend** — for higher concurrency / multi-node; SQL is already
  parameterized and migration-driven. *(~2–3 weeks)*
- **Email notifications** (review requests, mentions, expiring tokens). *(~1 week)*
- **Prometheus `/metrics`** + richer request tracing. *(~1 week)*
- **Rate limiting / abuse protection** on auth and API. *(~3–5 days)*

## Larger bets
- Native CI/CD ("Actions"), package registry, and Git LFS. *(each multi-week)*

> Effort estimates assume one experienced Go/React engineer familiar with the
> codebase and are intended for planning, not commitment.
