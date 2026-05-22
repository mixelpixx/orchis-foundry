// Open-a-PR — compare two branches and create a pull request.
function NewPRView({ route, setRoute }) {
  const [repo, setRepo] = React.useState((route && route.repo) || "");
  const [repos, setRepos] = React.useState([]);
  const [branches, setBranches] = React.useState([]);
  const [base, setBase] = React.useState("");
  const [head, setHead] = React.useState((route && route.head) || "");
  const [cmp, setCmp] = React.useState(null);
  const [title, setTitle] = React.useState("");
  const [body, setBody] = React.useState("");
  const [reviewer, setReviewer] = React.useState("");
  const [busy, setBusy] = React.useState(false);
  const [err, setErr] = React.useState("");
  const titleTouched = React.useRef(false);

  // If no repo given, offer a picker (owned repos).
  React.useEffect(() => {
    if (!repo && window.OrchisAPI) window.OrchisAPI.get("/v1/repos").then(rs => setRepos(rs || [])).catch(() => {});
  }, [repo]);

  // Load branches for the chosen repo; seed base = default branch.
  React.useEffect(() => {
    if (!repo || !window.OrchisAPI) return;
    window.OrchisAPI.get(`/v1/repos/${repo}/branches`).then(bs => {
      setBranches(bs || []);
      const def = (bs || []).find(b => b.isDefault);
      if (def && !base) setBase(def.name);
    }).catch(() => {});
  }, [repo]);

  // Compare whenever base+head are both set and differ.
  React.useEffect(() => {
    if (!repo || !base || !head || base === head || !window.OrchisAPI) { setCmp(null); return; }
    let cancelled = false;
    window.OrchisAPI.get(`/v1/repos/${repo}/compare?base=${encodeURIComponent(base)}&head=${encodeURIComponent(head)}`)
      .then(res => {
        if (cancelled) return;
        setCmp(res);
        if (!titleTouched.current && res.commits && res.commits.length) {
          setTitle(res.commits[0].message.split("\n")[0]);
        }
      })
      .catch(() => { if (!cancelled) setCmp(null); });
    return () => { cancelled = true; };
  }, [repo, base, head]);

  const create = async () => {
    if (!repo || !head || !title.trim()) return;
    setErr(""); setBusy(true);
    try {
      const pr = await window.OrchisAPI.post(`/v1/repos/${repo}/pulls`, { title: title.trim(), head, base, body });
      if (reviewer.trim()) {
        await window.OrchisAPI.post(`/v1/repos/${repo}/pulls/${pr.id}/request-review`, { reviewer: reviewer.trim() }).catch(() => {});
      }
      setRoute({ view: "pr", pr: pr.id, repo });
    } catch (e) { setErr("Could not open the PR — the head branch may have no new commits, or you lack write access."); setBusy(false); }
  };

  const branchOpts = branches.map(b => b.name);
  const canCompare = repo && base && head && base !== head;

  return (
    <div style={npStyles.scroll}>
      <div style={npStyles.page} className="fade-in">
        <div className="row" style={{ marginBottom: 16 }}>
          <Icons.PR size={18} style={{ color: "var(--accent)" }} />
          <h1 style={{ fontSize: 20, fontWeight: 500, margin: "0 0 0 8px" }}>Open a pull request</h1>
        </div>

        {err ? <div style={{ padding: "8px 12px", border: "1px solid var(--danger)", borderRadius: 6, color: "var(--danger)", fontSize: 12.5, marginBottom: 14 }}>{err}</div> : null}

        <div className="card" style={{ padding: 14, marginBottom: 16, display: "flex", flexDirection: "column", gap: 10 }}>
          {!route || !route.repo ? (
            <Field label="Repository">
              <select className="input" value={repo} onChange={e => { setRepo(e.target.value); setBase(""); setHead(""); }}>
                <option value="">Choose a repo…</option>
                {repos.map(r => <option key={r.id} value={r.id}>{r.id}</option>)}
              </select>
            </Field>
          ) : null}
          <div className="row" style={{ gap: 10, flexWrap: "wrap" }}>
            <div style={{ flex: 1, minWidth: 180 }}>
              <div className="section-title" style={{ marginBottom: 4 }}>Base (merge into)</div>
              <select className="input" value={base} onChange={e => setBase(e.target.value)} disabled={!repo}>
                <option value="">base…</option>
                {branchOpts.map(b => <option key={b} value={b}>{b}</option>)}
              </select>
            </div>
            <div style={{ alignSelf: "flex-end", padding: "0 4px 8px", color: "var(--fg-3)" }}>←</div>
            <div style={{ flex: 1, minWidth: 180 }}>
              <div className="section-title" style={{ marginBottom: 4 }}>Compare (head)</div>
              <select className="input" value={head} onChange={e => setHead(e.target.value)} disabled={!repo}>
                <option value="">head…</option>
                {branchOpts.map(b => <option key={b} value={b}>{b}</option>)}
              </select>
            </div>
          </div>
          {canCompare && cmp ? (
            <div className="subtle" style={{ fontSize: 12 }}>
              {cmp.aheadBy} commit{cmp.aheadBy === 1 ? "" : "s"} · <span style={{ color: "var(--accent)" }}>+{cmp.additions}</span> <span style={{ color: "var(--danger)" }}>−{cmp.deletions}</span> · {cmp.filesCount} file{cmp.filesCount === 1 ? "" : "s"}
            </div>
          ) : canCompare ? <div className="subtle" style={{ fontSize: 12 }}>Comparing…</div> : null}
        </div>

        {canCompare && cmp && cmp.aheadBy === 0 ? (
          <div className="card" style={{ padding: "16px 18px", color: "var(--fg-3)", fontSize: 13, marginBottom: 16 }}>
            <span className="mono">{head}</span> is not ahead of <span className="mono">{base}</span> — nothing to merge.
          </div>
        ) : null}

        {canCompare && cmp && cmp.aheadBy > 0 ? (
          <>
            <div className="card" style={{ padding: 14, marginBottom: 16, display: "flex", flexDirection: "column", gap: 10 }}>
              <Field label="Title"><input className="input" value={title} onChange={e => { titleTouched.current = true; setTitle(e.target.value); }} placeholder="Pull request title" /></Field>
              <Field label="Description">
                <textarea value={body} onChange={e => setBody(e.target.value)} placeholder="What does this change and why?" style={{
                  width: "100%", height: 110, padding: 10, border: "1px solid var(--line)", borderRadius: 6,
                  fontFamily: "inherit", fontSize: 13, background: "var(--bg-1)", color: "var(--fg)", resize: "vertical",
                }} />
              </Field>
              <div className="row" style={{ gap: 10 }}>
                <div style={{ flex: 1 }}>
                  <Field label="Request a reviewer (optional)"><input className="input" value={reviewer} onChange={e => setReviewer(e.target.value)} placeholder="handle" /></Field>
                </div>
                <button className="btn primary" style={{ alignSelf: "flex-end", height: 36 }} disabled={busy || !title.trim()} onClick={create}>{busy ? "Opening…" : "Create pull request"}</button>
              </div>
            </div>

            <div className="section-title" style={{ margin: "0 0 8px" }}>{cmp.commits.length} commit{cmp.commits.length === 1 ? "" : "s"}</div>
            <div className="card" style={{ marginBottom: 16, overflow: "hidden" }}>
              {cmp.commits.map((c, i) => (
                <div key={c.sha} className="row" style={{ gap: 10, padding: "9px 14px", borderBottom: i < cmp.commits.length - 1 ? "1px solid var(--line)" : "none" }}>
                  <Icons.Commit size={13} style={{ color: "var(--fg-2)" }} />
                  <span style={{ flex: 1, fontSize: 13, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{c.message.split("\n")[0]}</span>
                  <span className="mono subtle" style={{ fontSize: 11 }}>{c.short}</span>
                </div>
              ))}
            </div>

            <div className="section-title" style={{ margin: "0 0 8px" }}>Changes</div>
            <ReadOnlyDiff files={cmp.files} />
          </>
        ) : null}
      </div>
    </div>
  );
}

const npStyles = {
  scroll: { height: "100%", overflowY: "auto" },
  page: { maxWidth: 940, margin: "0 auto", padding: "28px 24px 60px" },
};

window.NewPRView = NewPRView;
