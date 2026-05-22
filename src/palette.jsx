// Command palette — the 3-step shortcut for everything.
function Palette({ open, onClose, onNavigate }) {
  const [q, setQ] = React.useState("");
  const [idx, setIdx] = React.useState(0);
  const inputRef = React.useRef(null);

  React.useEffect(() => {
    if (open) {
      setTimeout(() => inputRef.current?.focus(), 10);
      setQ(""); setIdx(0);
    }
  }, [open]);

  // Live search results from the server (repos / pulls / files).
  const [hits, setHits] = React.useState({ repos: [], pulls: [], files: [] });

  // Static actions — quick actions + navigation. These need local navigation
  // callbacks, so they're built client-side (the server supplies only data).
  const staticActions = React.useMemo(() => {
    const acts = [];
    // Quick actions (the killers — 1 step to PAT/SSH/webhook)
    acts.push({ id: "act-pat", group: "Quick action", icon: <Icons.Key />, label: "Create personal access token", hint: "PAT", kbd: "⌘T", run: () => onNavigate({ view: "settings", tab: "tokens", newToken: true }) });
    acts.push({ id: "act-ssh", group: "Quick action", icon: <Icons.Key />, label: "Add SSH key", hint: "SSH", run: () => onNavigate({ view: "settings", tab: "ssh", newKey: true }) });
    acts.push({ id: "act-wh", group: "Quick action", icon: <Icons.Webhook />, label: "Add webhook", run: () => onNavigate({ view: "settings", tab: "webhooks", newHook: true }) });
    acts.push({ id: "act-new-repo", group: "Quick action", icon: <Icons.Plus />, label: "New repository", kbd: "⌘N", run: () => onNavigate({ view: "home", newRepo: true }) });

    // Navigation
    acts.push({ id: "nav-home", group: "Go to", icon: <Icons.Home />, label: "Home", run: () => onNavigate({ view: "home" }) });
    acts.push({ id: "nav-prs", group: "Go to", icon: <Icons.PR />, label: "My pull requests", run: () => onNavigate({ view: "prs" }) });
    acts.push({ id: "nav-dev", group: "Go to", icon: <Icons.Settings />, label: "Developer settings", run: () => onNavigate({ view: "settings" }) });
    acts.push({ id: "nav-tokens", group: "Go to", icon: <Icons.Key />, label: "Tokens", run: () => onNavigate({ view: "settings", tab: "tokens" }) });
    acts.push({ id: "nav-ssh", group: "Go to", icon: <Icons.Key />, label: "SSH keys", run: () => onNavigate({ view: "settings", tab: "ssh" }) });
    acts.push({ id: "nav-wh", group: "Go to", icon: <Icons.Webhook />, label: "Webhooks", run: () => onNavigate({ view: "settings", tab: "webhooks" }) });
    return acts;
  }, [onNavigate]);

  // Fetch live hits (debounced) whenever the query changes while open.
  React.useEffect(() => {
    if (!open) return;
    let cancelled = false;
    const t = setTimeout(async () => {
      try {
        const res = await window.OrchisAPI.get("/v1/search/palette?q=" + encodeURIComponent(q));
        if (!cancelled) setHits({ repos: res.repos || [], pulls: res.pulls || [], files: res.files || [] });
      } catch (_) {
        if (!cancelled) setHits({ repos: [], pulls: [], files: [] });
      }
    }, q.trim() ? 140 : 0);
    return () => { cancelled = true; clearTimeout(t); };
  }, [q, open]);

  // Server hits mapped into action rows (run = navigate to the route).
  const serverActions = React.useMemo(() => {
    const acts = [];
    (hits.repos || []).forEach(r => {
      acts.push({ id: "repo-" + r.id, group: "Repositories", icon: <Icons.Repo />, label: r.label, sub: r.sub, run: () => onNavigate(r.route) });
    });
    (hits.pulls || []).forEach(p => {
      acts.push({ id: "pr-" + p.id, group: "Pull requests", icon: <Icons.PR />, label: p.label, sub: p.sub, run: () => onNavigate(p.route) });
    });
    (hits.files || []).forEach(f => {
      acts.push({ id: "file-" + f.id, group: "Files", icon: <Icons.File />, label: f.label, sub: f.sub, run: () => onNavigate(f.route) });
    });
    return acts;
  }, [hits, onNavigate]);

  // Filter the static actions client-side; server actions are already filtered.
  const filtered = React.useMemo(() => {
    let statics;
    if (!q.trim()) {
      statics = staticActions;
    } else {
      const ql = q.toLowerCase();
      statics = staticActions
        .map(a => {
          const hay = (a.label + " " + (a.sub || "") + " " + (a.hint || "") + " " + a.group).toLowerCase();
          if (!hay.includes(ql)) return null;
          const score = a.label.toLowerCase().startsWith(ql) ? 0 : a.label.toLowerCase().includes(ql) ? 1 : 2;
          return { ...a, _score: score };
        })
        .filter(Boolean)
        .sort((a, b) => a._score - b._score);
    }
    return [...statics, ...serverActions].slice(0, 40);
  }, [q, staticActions, serverActions]);

  // Group rendering
  const groups = React.useMemo(() => {
    const out = [];
    const seen = new Map();
    filtered.forEach(a => {
      if (!seen.has(a.group)) { seen.set(a.group, []); out.push({ name: a.group, items: seen.get(a.group) }); }
      seen.get(a.group).push(a);
    });
    return out;
  }, [filtered]);

  // Keyboard nav
  React.useEffect(() => {
    if (!open) return;
    const handler = (e) => {
      if (e.key === "Escape") { e.preventDefault(); onClose(); }
      else if (e.key === "ArrowDown") { e.preventDefault(); setIdx(i => Math.min(i + 1, filtered.length - 1)); }
      else if (e.key === "ArrowUp") { e.preventDefault(); setIdx(i => Math.max(i - 1, 0)); }
      else if (e.key === "Enter") { e.preventDefault(); const a = filtered[idx]; if (a) { a.run(); onClose(); } }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [open, filtered, idx, onClose]);

  if (!open) return null;

  let runningIdx = -1;

  return (
    <div style={palStyles.scrim} onClick={onClose}>
      <div style={palStyles.modal} onClick={e => e.stopPropagation()} className="fade-in">
        <div style={palStyles.inputRow}>
          <Icons.Search size={16} style={{ color: "var(--fg-2)" }} />
          <input
            ref={inputRef}
            value={q}
            onChange={e => { setQ(e.target.value); setIdx(0); }}
            placeholder="Type to do anything — create a token, open a repo, find a file…"
            style={palStyles.input}
          />
          <span className="kbd">esc</span>
        </div>
        <div style={palStyles.results}>
          {groups.length === 0 ? (
            <div style={{ padding: 24, textAlign: "center", color: "var(--fg-3)", fontSize: 13 }}>
              No matches. Try “token”, a repo name, or “webhook”.
            </div>
          ) : groups.map(g => (
            <div key={g.name}>
              <div style={palStyles.group}>{g.name}</div>
              {g.items.map(a => {
                runningIdx++;
                const active = runningIdx === idx;
                return (
                  <button
                    key={a.id}
                    onMouseEnter={() => setIdx(filtered.findIndex(x => x.id === a.id))}
                    onClick={() => { a.run(); onClose(); }}
                    style={{ ...palStyles.row, ...(active ? palStyles.rowActive : {}) }}
                  >
                    <span style={{ color: active ? "var(--accent)" : "var(--fg-2)", display: "inline-flex" }}>{a.icon}</span>
                    <span style={{ flex: 1, textAlign: "left", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                      {a.label}
                      {a.sub ? <span className="subtle" style={{ marginLeft: 8, fontFamily: "var(--font-mono)", fontSize: 11 }}>{a.sub}</span> : null}
                    </span>
                    {a.kbd ? <span className="kbd">{a.kbd}</span> : null}
                  </button>
                );
              })}
            </div>
          ))}
        </div>
        <div style={palStyles.footer}>
          <span className="row" style={{ gap: 6 }}><span className="kbd">↑</span><span className="kbd">↓</span><span className="subtle">navigate</span></span>
          <span className="row" style={{ gap: 6 }}><span className="kbd">↵</span><span className="subtle">select</span></span>
          <div className="spacer" />
          <span className="subtle" style={{ fontSize: 11 }}>Anything reachable in ≤ 3 steps.</span>
        </div>
      </div>
    </div>
  );
}

const palStyles = {
  scrim: {
    position: "fixed", inset: 0,
    background: "color-mix(in oklab, var(--bg) 60%, rgba(0,0,0,0.4))",
    backdropFilter: "blur(2px)",
    zIndex: 1000,
    display: "flex",
    alignItems: "flex-start",
    justifyContent: "center",
    paddingTop: "12vh",
  },
  modal: {
    width: "min(640px, 92vw)",
    background: "var(--bg-1)",
    border: "1px solid var(--line-strong)",
    borderRadius: 12,
    boxShadow: "var(--shadow-lg)",
    overflow: "hidden",
    display: "flex",
    flexDirection: "column",
    maxHeight: "70vh",
  },
  inputRow: {
    display: "flex",
    alignItems: "center",
    gap: 10,
    padding: "12px 14px",
    borderBottom: "1px solid var(--line)",
  },
  input: {
    flex: 1,
    border: "none",
    outline: "none",
    background: "transparent",
    color: "var(--fg)",
    font: "inherit",
    fontSize: 15,
  },
  results: {
    overflowY: "auto",
    padding: "6px 6px 8px",
    flex: 1,
  },
  group: {
    fontSize: 10.5,
    fontWeight: 500,
    color: "var(--fg-3)",
    textTransform: "uppercase",
    letterSpacing: "0.07em",
    padding: "10px 12px 4px",
  },
  row: {
    display: "flex",
    alignItems: "center",
    gap: 10,
    width: "100%",
    height: 34,
    padding: "0 10px",
    borderRadius: 6,
    border: "none",
    background: "transparent",
    color: "var(--fg)",
    fontSize: 13,
    cursor: "pointer",
    font: "inherit",
  },
  rowActive: {
    background: "var(--bg-2)",
  },
  footer: {
    display: "flex",
    alignItems: "center",
    gap: 14,
    padding: "8px 14px",
    borderTop: "1px solid var(--line)",
    fontSize: 11,
    color: "var(--fg-2)",
    background: "var(--bg)",
  },
};

window.Palette = Palette;
