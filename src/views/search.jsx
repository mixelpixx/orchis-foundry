// Code search view — full-text search across repo contents (git grep backend).
function SearchView({ route, setRoute }) {
  const [q, setQ] = React.useState((route && route.q) || "");
  const [lang, setLang] = React.useState("");
  const [repo, setRepo] = React.useState((route && route.repo) || "");
  const [hits, setHits] = React.useState(null);
  const [loading, setLoading] = React.useState(false);
  const inputRef = React.useRef(null);

  React.useEffect(() => { setTimeout(() => inputRef.current && inputRef.current.focus(), 20); }, []);

  React.useEffect(() => {
    if (!window.OrchisAPI || q.trim().length < 2) { setHits(q.trim() ? [] : null); return; }
    let cancelled = false;
    setLoading(true);
    const t = setTimeout(() => {
      const params = new URLSearchParams({ q: q.trim() });
      if (lang) params.set("lang", lang);
      if (repo) params.set("repo", repo);
      window.OrchisAPI.get("/v1/search/code?" + params.toString())
        .then(res => { if (!cancelled) { setHits(res || []); setLoading(false); } })
        .catch(() => { if (!cancelled) { setHits([]); setLoading(false); } });
    }, 200);
    return () => { cancelled = true; clearTimeout(t); };
  }, [q, lang, repo]);

  // Group hits by repo for display.
  const groups = React.useMemo(() => {
    const m = new Map();
    (hits || []).forEach(h => { if (!m.has(h.repo)) m.set(h.repo, []); m.get(h.repo).push(h); });
    return [...m.entries()].map(([name, items]) => ({ name, items }));
  }, [hits]);

  const langs = ["", "go", "js", "ts", "py", "rs", "rb", "java", "c", "cpp", "sh", "md", "json", "yaml", "html", "css"];

  return (
    <div style={searchStyles.scroll}>
      <div style={searchStyles.page} className="fade-in">
        <h1 style={{ fontSize: 22, fontWeight: 500, letterSpacing: "-0.015em", margin: "0 0 4px" }}>Code search</h1>
        <p className="muted" style={{ margin: "0 0 16px", fontSize: 13 }}>Literal full-text search across your repositories.</p>

        <div className="row" style={{ gap: 8, marginBottom: 14, flexWrap: "wrap" }}>
          <div className="row" style={{ gap: 8, flex: 1, minWidth: 240, border: "1px solid var(--line)", borderRadius: 8, padding: "0 10px", background: "var(--bg-1)" }}>
            <Icons.Search size={15} style={{ color: "var(--fg-2)" }} />
            <input ref={inputRef} value={q} onChange={e => setQ(e.target.value)} placeholder="Search code… (min 2 chars)"
              style={{ flex: 1, border: "none", outline: "none", background: "transparent", color: "var(--fg)", font: "inherit", fontSize: 14, height: 40 }} />
          </div>
          <select className="input" value={lang} onChange={e => setLang(e.target.value)} style={{ height: 40 }}>
            {langs.map(l => <option key={l} value={l}>{l === "" ? "Any language" : l}</option>)}
          </select>
          {repo ? <button className="btn sm" onClick={() => setRepo("")} title="Clear repo filter">{repo} ✕</button> : null}
        </div>

        {q.trim().length >= 2 && !loading && groups.length === 0 ? (
          <div className="card" style={{ padding: "20px 18px", color: "var(--fg-3)", fontSize: 13 }}>No matches.</div>
        ) : null}
        {loading ? <div className="muted" style={{ fontSize: 12.5, padding: "4px 2px" }}>Searching…</div> : null}

        {groups.map(g => (
          <div key={g.name} style={{ marginBottom: 18 }}>
            <div className="row" style={{ gap: 6, marginBottom: 6 }}>
              <Icons.Repo size={13} style={{ color: "var(--fg-2)" }} />
              <span className="mono" style={{ fontSize: 12.5, fontWeight: 500 }}>{g.name}</span>
              <span className="subtle" style={{ fontSize: 11 }}>{g.items.length} hit{g.items.length === 1 ? "" : "s"}</span>
            </div>
            <div className="card" style={{ overflow: "hidden" }}>
              {g.items.map((h, i) => (
                <button key={i} onClick={() => setRoute({ view: "repo", repo: h.repo, file: h.path })}
                  style={{ ...searchStyles.hit, borderBottom: i < g.items.length - 1 ? "1px solid var(--line)" : "none" }}>
                  <div className="row" style={{ gap: 8, minWidth: 0 }}>
                    <span className="mono subtle" style={{ fontSize: 11.5, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{h.path}</span>
                    <span className="mono subtle" style={{ fontSize: 11, flexShrink: 0 }}>:{h.line}</span>
                  </div>
                  <code style={searchStyles.snippet}>{highlight(h.text, q.trim())}</code>
                </button>
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

// highlight wraps case-insensitive literal matches of q in the snippet.
function highlight(text, q) {
  if (!q) return text;
  const lower = text.toLowerCase();
  const ql = q.toLowerCase();
  const out = [];
  let i = 0, k = 0;
  while (i < text.length) {
    const idx = lower.indexOf(ql, i);
    if (idx < 0) { out.push(text.slice(i)); break; }
    if (idx > i) out.push(text.slice(i, idx));
    out.push(<mark key={k++} style={{ background: "var(--accent-soft)", color: "var(--accent)", borderRadius: 3, padding: "0 1px" }}>{text.slice(idx, idx + q.length)}</mark>);
    i = idx + q.length;
  }
  return out;
}

const searchStyles = {
  scroll: { height: "100%", overflowY: "auto" },
  page: { maxWidth: 900, margin: "0 auto", padding: "28px 24px 60px" },
  hit: {
    display: "flex", flexDirection: "column", gap: 4, width: "100%", textAlign: "left",
    padding: "9px 12px", background: "transparent", border: "none", cursor: "pointer", font: "inherit", color: "inherit",
  },
  snippet: {
    fontFamily: "var(--font-mono)", fontSize: 12, color: "var(--fg-1)", whiteSpace: "pre",
    overflow: "hidden", textOverflow: "ellipsis",
  },
};

window.SearchView = SearchView;
