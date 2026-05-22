# Changelog

Versions are the build stamps deployed to `foundry.orchis.ai`
(`orchis-foundry --version`). Dates approximate.

## 0.21.0-rbac
Per-repo collaborators with `read` / `write` / `admin` roles (owner = implicit
admin). ACL threaded through HTTP + SSH push, merge, branches, releases, repo
settings, webhooks, PR edit/request-review, SSE topics, reads, and the repo
list. Collaborator CRUD endpoints + a Collaborators panel in repo settings.

## 0.20.0-prflow-commits-notifs
- **Open a PR from the UI** — branch compare (`GET …/compare`) → title/body →
  request reviewer → create.
- **Commit history + detail** — `GET …/commits/{sha}` (parent diff; empty-tree
  fallback for root commits), a Commits subtab, and a commit view.
- **Notifications** — sidebar bell with live (SSE) unread badge + dropdown.

## 0.19.0-search
`GET /v1/search/repos` + `/v1/search/users`; a "Search code for …" action in
the ⌘K palette.

## 0.18.0-releases
Releases: publish from a tag (name/notes/prerelease), list, delete, download
source `.tar.gz` (`git archive`). Releases subtab.

## 0.17.0-branches
Branch management: create / delete branches (safe ref-name validation,
default-branch protection), in the repo settings modal.

## 0.16.0-codesearch-inbox
- **Code search** via `git grep` on bare repos (`GET /v1/search/code`),
  injection-safe (literal `-e`, sha-resolved refs, lang allowlist, caps).
- Request-review endpoint + live "review requested" inbox over SSE.

## 0.15.0-realtime
Server-Sent Events (`/v1/stream`): in-process pub/sub hub, ACL'd topics, live
PR comment/review/merge + check updates.

## 0.14.0-hardening
Precompiled frontend bundle (no in-browser Babel / CDN), CSP tightened to
`script-src 'self'`, daily SQLite backup timer, golden-file test on the PR diff
shape.

## 0.13.0-pins-stars-reviews
Repo pins + stars (real counts/toggles, live sidebar); batched PR reviews
(stage line comments, submit with a verdict).

## 0.12.x-issues / linecomments
Issue tracker (open/comment/assign/close) + honest Actions panel; inline
PR diff-line comments wired to the comment API.

## 0.11.0-palette-webhooks
Real ⌘K command palette search; webhooks (per-repo CRUD + HMAC-signed delivery
worker with backoff).

## 0.10.0-reposettings
Repo settings + lifecycle (rename / visibility / default branch / delete),
tags, the real dashboard (activity feed + inbox).

## Earlier (milestones A–B)
Bootstrap + embedded frontend; DB + GitHub OIDC + sessions; repos (read);
personal access tokens (the 3-step flow); git over HTTPS + post-receive hook;
embedded SSH git server (:2222); PR read (diff/commits/comments); the BYO-model
supply-chain scanner as a PR check; PR write (comments/reviews/merge); the
LLM-friendly context endpoints (`/llms.txt`, `/v1/openapi.json`, repo/PR pack)
and in-PR AI assist.
