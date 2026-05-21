// Dashboard / home view.
function DashboardView({ setRoute, openPalette }) {
  const myPRs = PRS.filter(p => p.reviewers.some(r => r.id === "me") || p.author.id === "me");
  const today = new Date().toLocaleDateString(undefined, { weekday: "long", month: "long", day: "numeric" });
  const firstName = (USERS.me.name || USERS.me.handle || "there").split(" ")[0];
  const [showNewRepo, setShowNewRepo] = React.useState(false);

  return (
    <div style={dashStyles.scroll}>
      <div style={dashStyles.page} className="fade-in">
        <header style={dashStyles.header}>
          <div>
            <div style={{ fontSize: 11.5, color: "var(--fg-3)", textTransform: "uppercase", letterSpacing: "0.06em" }}>{today}</div>
            <h1 style={dashStyles.h1}>{greeting()}, {firstName}.</h1>
            <div style={{ color: "var(--fg-2)", fontSize: 14 }}>
              Welcome to Orchis Foundry.
            </div>
          </div>
          <div className="row" style={{ gap: 8 }}>
            <button className="btn" onClick={openPalette}><Icons.Search size={14} /> Quick action <span className="kbd" style={{ marginLeft: 4 }}>⌘K</span></button>
            <button className="btn primary" onClick={() => setShowNewRepo(true)}><Icons.Plus size={14} /> New repository</button>
          </div>
        </header>

        {showNewRepo ? <NewRepoModal onClose={() => setShowNewRepo(false)} setRoute={setRoute} /> : null}

        {/* Inbox — the 3 things */}
        <section style={dashStyles.section}>
          <div style={dashStyles.sectionHead}>
            <h2 style={dashStyles.h2}>Inbox</h2>
            <span className="muted" style={{ fontSize: 12 }}>Filtered to what actually needs you.</span>
          </div>
          <div style={dashStyles.inbox}>
            <InboxCard
              icon={<Icons.PR />}
              kind="accent"
              title="Review requested"
              repo="kelp/atlas"
              detail="#842 router: switch fallback sink to bounded channel"
              meta={[{ label: "by Jana", avatar: USERS.jana }, { label: "+142 -88", mono: true }, { label: "8 min ago" }]}
              cta="Open review"
              onClick={() => setRoute({ view: "pr", pr: 842 })}
            />
            <InboxCard
              icon={<Icons.Key />}
              kind="warn"
              title="Token expires in 14 days"
              repo="ci pipeline"
              detail="Rotate before it expires, or CI will stall."
              meta={[{ label: "scopes: repo:read, packages:write", mono: true }]}
              cta="Rotate token"
              onClick={() => setRoute({ view: "settings", tab: "tokens" })}
            />
            <InboxCard
              icon={<Icons.Issue />}
              kind="info"
              title="Mentioned in an issue"
              repo="kelp/atlas"
              detail="#911 Memory growth under fallback-only routing"
              meta={[{ label: "by Noor", avatar: USERS.noor }, { label: "yesterday" }]}
              cta="View issue"
              onClick={() => {}}
            />
          </div>
        </section>

        {/* Two columns */}
        <div style={dashStyles.cols}>
          <section style={dashStyles.section}>
            <div style={dashStyles.sectionHead}>
              <h2 style={dashStyles.h2}>Pinned</h2>
              <button className="btn ghost sm">Manage</button>
            </div>
            <div style={dashStyles.repoGrid}>
              {REPOS.filter(r => r.pinned).map(r => (
                <button key={r.id} className="card" style={dashStyles.repoCard} onClick={() => setRoute({ view: "repo", repo: r.id })}>
                  <div className="row" style={{ gap: 8 }}>
                    <Icons.Repo size={14} style={{ color: "var(--fg-2)" }} />
                    <span className="mono" style={{ fontSize: 13, fontWeight: 500 }}>{r.org}/{r.name}</span>
                    {r.visibility === "private" ? <span className="chip">private</span> : null}
                  </div>
                  <p style={{ color: "var(--fg-2)", fontSize: 13, margin: "8px 0 12px", lineHeight: 1.4 }}>{r.description}</p>
                  <div className="row" style={{ gap: 14, color: "var(--fg-2)", fontSize: 11.5 }}>
                    <span className="row" style={{ gap: 5 }}><span style={{ width: 8, height: 8, borderRadius: 2, background: r.languageColor }}/> {r.language}</span>
                    <span className="row" style={{ gap: 4 }}><Icons.Star size={12} /> {r.stars.toLocaleString()}</span>
                    <span className="row" style={{ gap: 4 }}><Icons.Fork size={12} /> {r.forks}</span>
                    <span className="spacer" />
                    <span className="subtle">{r.updated}</span>
                  </div>
                </button>
              ))}
            </div>
          </section>

          <section style={dashStyles.section}>
            <div style={dashStyles.sectionHead}>
              <h2 style={dashStyles.h2}>Activity</h2>
              <span className="muted" style={{ fontSize: 12 }}>Across all your repos</span>
            </div>
            <div className="card" style={{ padding: 4 }}>
              {ACTIVITY.map(a => (
                <div key={a.id} style={dashStyles.activityRow}>
                  <span className="avatar" style={{ background: a.actor.color, width: 20, height: 20, fontSize: 9 }}>{a.actor.initials}</span>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ fontSize: 13, lineHeight: 1.4, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                      <strong style={{ fontWeight: 500 }}>{a.actor.name}</strong>{" "}
                      <span className="muted">{activityVerb(a.kind)}</span>{" "}
                      <span className="mono" style={{ fontSize: 11.5 }}>{a.target}</span>
                    </div>
                    <div className="subtle" style={{ fontSize: 11.5, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{a.title}</div>
                  </div>
                  <span className="subtle" style={{ fontSize: 11 }}>{a.when}</span>
                </div>
              ))}
            </div>
          </section>
        </div>
      </div>
    </div>
  );
}

function activityVerb(k) {
  switch (k) {
    case "pr_review_requested": return "requested your review on";
    case "pr_approved": return "approved";
    case "commit": return "pushed to";
    case "issue_assigned": return "assigned you to";
    case "deploy": return "deployed";
    default: return "did something in";
  }
}

function InboxCard({ icon, kind = "info", title, repo, detail, meta = [], cta, onClick }) {
  const tone = {
    accent: { bg: "var(--accent-soft)", fg: "var(--accent)", border: "var(--accent-line)" },
    warn:   { bg: "color-mix(in oklab, var(--warn) 14%, var(--bg))", fg: "var(--warn)", border: "color-mix(in oklab, var(--warn) 35%, var(--bg))" },
    info:   { bg: "color-mix(in oklab, var(--info) 14%, var(--bg))", fg: "var(--info)", border: "color-mix(in oklab, var(--info) 35%, var(--bg))" },
  }[kind];

  return (
    <div className="card" style={dashStyles.inboxCard}>
      <div style={{ ...dashStyles.inboxIcon, background: tone.bg, color: tone.fg, borderColor: tone.border }}>{icon}</div>
      <div style={{ flex: 1, minWidth: 0 }}>
        <div className="row" style={{ gap: 8, marginBottom: 4 }}>
          <span style={{ fontWeight: 500, fontSize: 13 }}>{title}</span>
          <span className="mono subtle" style={{ fontSize: 11.5 }}>{repo}</span>
        </div>
        <div style={{ color: "var(--fg-1)", fontSize: 13, marginBottom: 6, lineHeight: 1.4 }}>{detail}</div>
        <div className="row" style={{ gap: 12, color: "var(--fg-2)", fontSize: 11.5, flexWrap: "wrap" }}>
          {meta.map((m, i) => (
            <span key={i} className="row" style={{ gap: 5 }}>
              {m.avatar ? <span className="avatar" style={{ background: m.avatar.color, width: 16, height: 16, fontSize: 8 }}>{m.avatar.initials}</span> : null}
              <span className={m.mono ? "mono" : ""}>{m.label}</span>
            </span>
          ))}
        </div>
      </div>
      <button className="btn" onClick={onClick}>{cta} <Icons.Chevron size={12} /></button>
    </div>
  );
}

const dashStyles = {
  scroll: { overflowY: "auto", height: "100%" },
  page: { maxWidth: 1180, margin: "0 auto", padding: "36px 36px 80px" },
  header: { display: "flex", alignItems: "flex-end", justifyContent: "space-between", marginBottom: 28, gap: 16, flexWrap: "wrap" },
  h1: { fontSize: 30, fontWeight: 500, letterSpacing: "-0.02em", margin: "6px 0 2px" },
  section: { marginBottom: 28 },
  sectionHead: { display: "flex", alignItems: "baseline", justifyContent: "space-between", marginBottom: 10, gap: 12 },
  h2: { fontSize: 13, fontWeight: 500, color: "var(--fg-3)", textTransform: "uppercase", letterSpacing: "0.07em", margin: 0 },
  inbox: { display: "flex", flexDirection: "column", gap: 8 },
  inboxCard: { display: "flex", alignItems: "center", gap: 14, padding: "14px 16px" },
  inboxIcon: {
    width: 32, height: 32, borderRadius: 8,
    border: "1px solid",
    display: "flex", alignItems: "center", justifyContent: "center",
    flexShrink: 0,
  },
  cols: {
    display: "grid",
    gridTemplateColumns: "1fr 1fr",
    gap: 24,
  },
  repoGrid: { display: "grid", gridTemplateColumns: "1fr", gap: 8 },
  repoCard: {
    padding: "14px 16px",
    textAlign: "left",
    cursor: "pointer",
    transition: "border-color 100ms, background 100ms",
    font: "inherit",
    color: "inherit",
  },
  activityRow: {
    display: "flex", alignItems: "center", gap: 10,
    padding: "10px 12px",
    borderBottom: "1px solid var(--line)",
    fontSize: 13,
  },
};

function greeting() {
  const h = new Date().getHours();
  if (h < 12) return "Good morning";
  if (h < 18) return "Good afternoon";
  return "Good evening";
}

// New repository modal — POSTs /v1/repos and navigates to the new repo.
function NewRepoModal({ onClose, setRoute }) {
  const [name, setName] = React.useState("");
  const [description, setDescription] = React.useState("");
  const [visibility, setVisibility] = React.useState("private");
  const [busy, setBusy] = React.useState(false);
  const [err, setErr] = React.useState("");

  const create = async () => {
    setErr(""); setBusy(true);
    try {
      const repo = await window.OrchisAPI.post("/v1/repos", { name: name.trim(), description, visibility });
      await window.loadRepos().catch(() => {});
      onClose();
      setRoute({ view: "repo", repo: repo.id });
    } catch (e) {
      setErr("Could not create repository. The name may already be taken, or it contains invalid characters.");
      setBusy(false);
    }
  };

  const valid = /^[A-Za-z0-9._-]{1,100}$/.test(name.trim());

  return (
    <div onClick={onClose} style={{
      position: "fixed", inset: 0, background: "color-mix(in oklab, var(--bg) 40%, transparent)",
      backdropFilter: "blur(2px)", zIndex: 200, display: "flex", alignItems: "center", justifyContent: "center", padding: 24,
    }}>
      <div onClick={e => e.stopPropagation()} className="card fade-in" style={{ width: 520, maxWidth: "92vw", padding: 22 }}>
        <h2 style={{ fontSize: 18, fontWeight: 500, margin: "0 0 4px" }}>New repository</h2>
        <p className="muted" style={{ fontSize: 13, margin: "0 0 18px" }}>Creates an empty repository under <span className="mono">{USERS.me.handle}/</span>. Push to it over HTTPS or SSH.</p>

        {err ? <div style={{ padding: "10px 12px", border: "1px solid var(--danger)", borderRadius: 8, color: "var(--danger)", fontSize: 13, marginBottom: 14 }}>{err}</div> : null}

        <div style={{ marginBottom: 14 }}>
          <div className="section-title" style={{ marginBottom: 6 }}>Name</div>
          <input className="input" autoFocus value={name} onChange={e => setName(e.target.value)} placeholder="kicad-mcp" />
        </div>
        <div style={{ marginBottom: 14 }}>
          <div className="section-title" style={{ marginBottom: 6 }}>Description <span className="muted" style={{ textTransform: "none", letterSpacing: 0 }}>(optional)</span></div>
          <input className="input" value={description} onChange={e => setDescription(e.target.value)} placeholder="What's in this repo?" />
        </div>
        <div style={{ marginBottom: 18 }}>
          <div className="section-title" style={{ marginBottom: 6 }}>Visibility</div>
          <div className="row" style={{ gap: 8 }}>
            {["private", "internal", "public"].map(v => (
              <label key={v} className="row" style={{
                gap: 8, padding: "8px 12px", border: "1px solid var(--line)", borderRadius: 6, cursor: "pointer", flex: 1,
                background: visibility === v ? "var(--accent-soft)" : "var(--bg-1)",
                borderColor: visibility === v ? "var(--accent-line)" : "var(--line)",
              }}>
                <input type="radio" name="vis" checked={visibility === v} onChange={() => setVisibility(v)} />
                <span style={{ textTransform: "capitalize", fontSize: 13 }}>{v}</span>
              </label>
            ))}
          </div>
        </div>

        <div className="row" style={{ gap: 8 }}>
          <button className="btn" onClick={onClose}>Cancel</button>
          <span className="spacer" />
          <button className="btn primary" disabled={!valid || busy} onClick={create}>{busy ? "Creating…" : "Create repository"}</button>
        </div>
      </div>
    </div>
  );
}

window.DashboardView = DashboardView;
