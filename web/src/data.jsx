// Mock data — kept small and realistic.

const USERS = {
  me: { id: "me", name: "Avery Park", handle: "avery", color: "oklch(60% 0.14 200)", initials: "AP" },
  jana: { id: "jana", name: "Jana Okafor", handle: "jana", color: "oklch(60% 0.14 30)", initials: "JO" },
  sam: { id: "sam", name: "Sam Riedl", handle: "sam", color: "oklch(60% 0.14 280)", initials: "SR" },
  noor: { id: "noor", name: "Noor Vance", handle: "noor", color: "oklch(60% 0.14 140)", initials: "NV" },
  lin: { id: "lin", name: "Lin Park-Soto", handle: "lin", color: "oklch(60% 0.14 70)", initials: "LP" },
  bot: { id: "bot", name: "ci-bot", handle: "ci-bot", color: "oklch(55% 0.04 250)", initials: "CI" },
};

const ORGS = [
  { id: "personal", name: "avery", kind: "user" },
  { id: "kelp", name: "kelp", kind: "org" },
  { id: "open-strata", name: "open-strata", kind: "org" },
];

const REPOS = [
  {
    id: "kelp/atlas",
    name: "atlas",
    org: "kelp",
    description: "Edge-deployed analytics router. Receives, batches, ships.",
    language: "Rust",
    languageColor: "oklch(60% 0.14 30)",
    stars: 1284,
    forks: 96,
    watchers: 24,
    visibility: "private",
    defaultBranch: "main",
    updated: "12 min ago",
    pinned: true,
  },
  {
    id: "kelp/atlas-ui",
    name: "atlas-ui",
    org: "kelp",
    description: "Web console for atlas. React + Vite.",
    language: "TypeScript",
    languageColor: "oklch(62% 0.12 240)",
    stars: 312,
    forks: 18,
    watchers: 9,
    visibility: "private",
    defaultBranch: "main",
    updated: "2 h ago",
    pinned: true,
  },
  {
    id: "kelp/ingest-spec",
    name: "ingest-spec",
    org: "kelp",
    description: "Versioned wire spec for atlas ingest.",
    language: "Markdown",
    languageColor: "oklch(60% 0.06 250)",
    stars: 41,
    forks: 4,
    watchers: 3,
    visibility: "private",
    defaultBranch: "main",
    updated: "yesterday",
    pinned: false,
  },
  {
    id: "open-strata/cordon",
    name: "cordon",
    org: "open-strata",
    description: "Tiny structured-log library with zero deps.",
    language: "Go",
    languageColor: "oklch(62% 0.12 200)",
    stars: 8421,
    forks: 412,
    watchers: 88,
    visibility: "public",
    defaultBranch: "main",
    updated: "3 d ago",
    pinned: true,
  },
  {
    id: "avery/dotfiles",
    name: "dotfiles",
    org: "avery",
    description: "Configs, scripts, and the ever-growing zshrc.",
    language: "Shell",
    languageColor: "oklch(70% 0.10 110)",
    stars: 12,
    forks: 0,
    watchers: 1,
    visibility: "public",
    defaultBranch: "main",
    updated: "1 w ago",
    pinned: false,
  },
];

// File tree for kelp/atlas
const FILE_TREE = [
  { type: "dir", name: ".github", path: ".github", children: [
    { type: "dir", name: "workflows", path: ".github/workflows", children: [
      { type: "file", name: "ci.yml", path: ".github/workflows/ci.yml", lang: "yaml" },
      { type: "file", name: "release.yml", path: ".github/workflows/release.yml", lang: "yaml" },
    ]},
  ]},
  { type: "dir", name: "crates", path: "crates", expanded: true, children: [
    { type: "dir", name: "atlas-core", path: "crates/atlas-core", expanded: true, children: [
      { type: "dir", name: "src", path: "crates/atlas-core/src", expanded: true, children: [
        { type: "file", name: "lib.rs", path: "crates/atlas-core/src/lib.rs", lang: "rust" },
        { type: "file", name: "router.rs", path: "crates/atlas-core/src/router.rs", lang: "rust", active: true },
        { type: "file", name: "batch.rs", path: "crates/atlas-core/src/batch.rs", lang: "rust" },
        { type: "file", name: "ship.rs", path: "crates/atlas-core/src/ship.rs", lang: "rust" },
      ]},
      { type: "file", name: "Cargo.toml", path: "crates/atlas-core/Cargo.toml", lang: "toml" },
    ]},
    { type: "dir", name: "atlas-cli", path: "crates/atlas-cli", children: [
      { type: "file", name: "main.rs", path: "crates/atlas-cli/src/main.rs", lang: "rust" },
    ]},
  ]},
  { type: "dir", name: "docs", path: "docs", children: [
    { type: "file", name: "architecture.md", path: "docs/architecture.md", lang: "markdown" },
    { type: "file", name: "deploy.md", path: "docs/deploy.md", lang: "markdown" },
  ]},
  { type: "file", name: ".gitignore", path: ".gitignore", lang: "text" },
  { type: "file", name: "Cargo.toml", path: "Cargo.toml", lang: "toml" },
  { type: "file", name: "Cargo.lock", path: "Cargo.lock", lang: "toml" },
  { type: "file", name: "LICENSE", path: "LICENSE", lang: "text" },
  { type: "file", name: "README.md", path: "README.md", lang: "markdown" },
];

// Code sample — router.rs
const ROUTER_RS = `use std::sync::Arc;
use tokio::sync::mpsc;
use tracing::{debug, info, warn};

use crate::batch::{Batch, BatchSink};
use crate::ship::Shipper;

/// Routes incoming events to the right batch sink based on the
/// declared destination in the event envelope.
///
/// Routing is intentionally simple: a static map keyed by the
/// destination id, populated at startup. Dynamic reconfig happens
/// out-of-band by swapping the whole router.
pub struct Router {
    sinks: Arc<dashmap::DashMap<String, BatchSink>>,
    fallback: Option<BatchSink>,
}

impl Router {
    pub fn new(fallback: Option<BatchSink>) -> Self {
        Self {
            sinks: Arc::new(dashmap::DashMap::new()),
            fallback,
        }
    }

    pub fn register(&self, dest: impl Into<String>, sink: BatchSink) {
        let dest = dest.into();
        info!(destination = %dest, "registering sink");
        self.sinks.insert(dest, sink);
    }

    pub async fn route(&self, env: Envelope) -> Result<(), RouteError> {
        let dest = env.destination();
        match self.sinks.get(dest) {
            Some(sink) => sink.send(env).await.map_err(Into::into),
            None => match &self.fallback {
                Some(fb) => {
                    warn!(destination = %dest, "no sink, using fallback");
                    fb.send(env).await.map_err(Into::into)
                }
                None => Err(RouteError::UnknownDestination(dest.to_string())),
            },
        }
    }
}
`;

const README_MD = `# atlas

> Edge-deployed analytics router. Receives, batches, ships.

Atlas is the routing brain behind the kelp analytics pipeline.
It runs at the edge, accepts events over HTTP or gRPC, batches
them by destination, and ships to downstream sinks with backpressure.

## Quickstart

\`\`\`sh
cargo install atlas-cli
atlas serve --config ./atlas.toml
\`\`\`

## Why another router?

- **Tiny** — single binary, ~12MB stripped.
- **Backpressure-aware** — no silent drops, no unbounded queues.
- **Observable by default** — every hop emits a structured span.

See [docs/architecture.md](docs/architecture.md) for the long version.
`;

// Pull requests
const PRS = [
  {
    id: 842,
    title: "router: switch fallback sink to bounded channel",
    repo: "kelp/atlas",
    author: USERS.jana,
    branch: "jana/bounded-fallback",
    base: "main",
    status: "open",
    reviewers: [USERS.me, USERS.sam],
    checks: { passed: 6, failed: 0, pending: 1 },
    additions: 142,
    deletions: 88,
    commits: 4,
    files: 6,
    updated: "8 min ago",
    description: "Replaces the unbounded `tokio::mpsc::unbounded_channel` in the fallback path with a bounded one (size: 4096). Backpressure now reaches the caller instead of silently growing memory.",
    labels: [
      { name: "backend", color: "info" },
      { name: "ready for review", color: "accent" },
    ],
    comments: 7,
  },
  {
    id: 839,
    title: "ui: tweak diff colors for high-contrast mode",
    repo: "kelp/atlas-ui",
    author: USERS.sam,
    branch: "sam/diff-contrast",
    base: "main",
    status: "open",
    reviewers: [USERS.me],
    checks: { passed: 4, failed: 0, pending: 0 },
    additions: 24,
    deletions: 12,
    commits: 2,
    files: 3,
    updated: "1 h ago",
    description: "Bumps the diff highlight delta so green/red are legible against the high-contrast theme.",
    labels: [{ name: "ui", color: "purple" }],
    comments: 2,
  },
  {
    id: 837,
    title: "ingest-spec: bump version to 0.6, add `trace_id` field",
    repo: "kelp/ingest-spec",
    author: USERS.noor,
    branch: "noor/spec-0.6",
    base: "main",
    status: "review",
    reviewers: [USERS.me, USERS.jana, USERS.lin],
    checks: { passed: 2, failed: 0, pending: 0 },
    additions: 312,
    deletions: 41,
    commits: 9,
    files: 12,
    updated: "yesterday",
    description: "Spec bump: adds optional `trace_id` to the envelope. Backwards compatible.",
    labels: [
      { name: "spec", color: "info" },
      { name: "breaking?", color: "warn" },
    ],
    comments: 14,
  },
];

// Diff for PR #842
const PR_DIFF_FILES = [
  {
    path: "crates/atlas-core/src/router.rs",
    additions: 18,
    deletions: 22,
    hunks: [
      {
        header: "@@ -12,7 +12,7 @@ pub struct Router {",
        lines: [
          { type: "ctx", num: ["12", "12"], text: "/// out-of-band by swapping the whole router." },
          { type: "ctx", num: ["13", "13"], text: "pub struct Router {" },
          { type: "ctx", num: ["14", "14"], text: "    sinks: Arc<dashmap::DashMap<String, BatchSink>>," },
          { type: "del", num: ["15", " "], text: "    fallback: Option<mpsc::UnboundedSender<Envelope>>," },
          { type: "add", num: [" ", "15"], text: "    fallback: Option<BatchSink>," },
          { type: "ctx", num: ["16", "16"], text: "}" },
          { type: "ctx", num: ["17", "17"], text: "" },
        ],
      },
      {
        header: "@@ -34,9 +34,13 @@ impl Router {",
        lines: [
          { type: "ctx", num: ["34", "34"], text: "    pub async fn route(&self, env: Envelope) -> Result<(), RouteError> {" },
          { type: "ctx", num: ["35", "35"], text: "        let dest = env.destination();" },
          { type: "ctx", num: ["36", "36"], text: "        match self.sinks.get(dest) {" },
          { type: "del", num: ["37", " "], text: "            Some(sink) => sink.send(env).await.map_err(Into::into)," },
          { type: "add", num: [" ", "37"], text: "            Some(sink) => sink.send(env).await.map_err(Into::into)," },
          { type: "ctx", num: ["38", "38"], text: "            None => match &self.fallback {" },
          { type: "del", num: ["39", " "], text: "                Some(fb) => fb.send(env).map_err(|_| RouteError::FallbackClosed)," },
          { type: "add", num: [" ", "39"], text: "                Some(fb) => {" },
          { type: "add", num: [" ", "40"], text: "                    warn!(destination = %dest, \"no sink, using fallback\");" },
          { type: "add", num: [" ", "41"], text: "                    fb.send(env).await.map_err(Into::into)" },
          { type: "add", num: [" ", "42"], text: "                }" },
          { type: "ctx", num: ["40", "43"], text: "                None => Err(RouteError::UnknownDestination(dest.to_string()))," },
          { type: "ctx", num: ["41", "44"], text: "            }," },
          { type: "ctx", num: ["42", "45"], text: "        }" },
        ],
      },
    ],
    comments: [
      {
        line: 41,
        side: "right",
        author: USERS.me,
        when: "3 min ago",
        body: "Could we surface this as a metric too? `atlas_fallback_total{destination=...}` would help us catch misconfigured pipelines.",
        suggestion: null,
      },
    ],
  },
  {
    path: "crates/atlas-core/src/batch.rs",
    additions: 64,
    deletions: 34,
    hunks: [
      {
        header: "@@ -1,8 +1,12 @@",
        lines: [
          { type: "del", num: ["1", " "], text: "use tokio::sync::mpsc::{unbounded_channel, UnboundedSender};" },
          { type: "add", num: [" ", "1"], text: "use tokio::sync::mpsc::{channel, Sender};" },
          { type: "ctx", num: ["2", "2"], text: "use tracing::debug;" },
          { type: "ctx", num: ["3", "3"], text: "" },
          { type: "add", num: [" ", "4"], text: "/// Default backpressure depth — events block once this many" },
          { type: "add", num: [" ", "5"], text: "/// envelopes are queued for a single destination." },
          { type: "add", num: [" ", "6"], text: "const DEFAULT_DEPTH: usize = 4096;" },
        ],
      },
    ],
    comments: [],
  },
  {
    path: "crates/atlas-core/src/ship.rs",
    additions: 12,
    deletions: 8,
    hunks: [],
    comments: [],
  },
];

// Activity feed
const ACTIVITY = [
  { id: 1, kind: "pr_review_requested", actor: USERS.jana, target: "kelp/atlas#842", title: "router: switch fallback sink to bounded channel", when: "8 min ago" },
  { id: 2, kind: "pr_approved", actor: USERS.lin, target: "kelp/atlas-ui#839", title: "ui: tweak diff colors for high-contrast mode", when: "1 h ago" },
  { id: 3, kind: "commit", actor: USERS.me, target: "kelp/atlas / main", title: "docs: clarify shipping retries", when: "3 h ago" },
  { id: 4, kind: "issue_assigned", actor: USERS.noor, target: "kelp/atlas#911", title: "Memory growth under fallback-only routing", when: "yesterday" },
  { id: 5, kind: "deploy", actor: USERS.bot, target: "kelp/atlas / production", title: "v0.14.2 shipped", when: "yesterday" },
];

// Personal access tokens
const TOKENS = [
  { id: "tok_1", name: "macbook — atlas dev", scopes: ["repo:read", "repo:write", "actions:read"], created: "2 weeks ago", lastUsed: "8 min ago", expires: "in 87 days" },
  { id: "tok_2", name: "ci pipeline", scopes: ["repo:read", "packages:write"], created: "3 months ago", lastUsed: "2 h ago", expires: "in 14 days", expiringSoon: true },
  { id: "tok_3", name: "old laptop", scopes: ["repo:read"], created: "1 year ago", lastUsed: "4 months ago", expires: "expired", expired: true },
];

const SSH_KEYS = [
  { id: "ssh_1", name: "macbook-air", fingerprint: "SHA256:k9d2…aZqL", added: "2 weeks ago", lastUsed: "today" },
  { id: "ssh_2", name: "linux-tower", fingerprint: "SHA256:p3xN…7gFq", added: "8 months ago", lastUsed: "3 d ago" },
];

const WEBHOOKS = [
  { id: "wh_1", url: "https://hooks.kelp.dev/ci", events: ["push", "pull_request"], repo: "kelp/atlas", lastDelivery: "8 min ago", status: "ok" },
  { id: "wh_2", url: "https://internal.kelp.dev/deploy", events: ["release"], repo: "kelp/atlas", lastDelivery: "yesterday", status: "ok" },
  { id: "wh_3", url: "https://hooks.kelp.dev/ci", events: ["push"], repo: "kelp/atlas-ui", lastDelivery: "1 h ago", status: "warn" },
];

Object.assign(window, {
  USERS, ORGS, REPOS,
  FILE_TREE, ROUTER_RS, README_MD,
  PRS, PR_DIFF_FILES,
  ACTIVITY, TOKENS, SSH_KEYS, WEBHOOKS,
});

// --- Real API wiring -------------------------------------------------------
// Small fetch helper + loaders that replace mock data in place. When the
// backend is unreachable (e.g. opening the prototype from disk), the mock
// constants above remain as the fallback.
const OrchisAPI = {
  async get(path) {
    const r = await fetch(path, { credentials: "same-origin" });
    if (!r.ok) throw new Error(path + " -> " + r.status);
    return r.json();
  },
  async post(path, body) {
    const r = await fetch(path, {
      method: "POST", credentials: "same-origin",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body || {}),
    });
    if (!r.ok) throw new Error(path + " -> " + r.status);
    return r.json();
  },
  async del(path) {
    const r = await fetch(path, { method: "DELETE", credentials: "same-origin" });
    if (!r.ok && r.status !== 204) throw new Error(path + " -> " + r.status);
    return true;
  },
};

// Replace REPOS contents in place with the live repo list.
async function loadRepos() {
  const repos = await OrchisAPI.get("/v1/repos");
  REPOS.length = 0;
  (repos || []).forEach((r) => REPOS.push(r));
  return REPOS;
}

window.OrchisAPI = OrchisAPI;
window.loadRepos = loadRepos;
