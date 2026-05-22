// Commit detail — message, author, and the commit's diff.
function CommitView({ route, setRoute }) {
  const repo = route.repo;
  const sha = route.sha;
  const [data, setData] = React.useState(null);
  const [err, setErr] = React.useState(false);

  React.useEffect(() => {
    if (!window.OrchisAPI || !repo || !sha) return;
    setData(null); setErr(false);
    window.OrchisAPI.get(`/v1/repos/${repo}/commits/${sha}`)
      .then(setData).catch(() => setErr(true));
  }, [repo, sha]);

  return (
    <div style={cvStyles.scroll}>
      <div style={cvStyles.page} className="fade-in">
        <button className="btn ghost sm" style={{ marginBottom: 12 }} onClick={() => setRoute({ view: "repo", repo })}>← {repo}</button>
        {err ? <div className="card" style={{ padding: 18, color: "var(--fg-3)", fontSize: 13 }}>Commit not found.</div> : null}
        {!data && !err ? <div className="muted" style={{ fontSize: 12.5 }}>Loading…</div> : null}
        {data ? (
          <>
            <div className="card" style={{ padding: 16, marginBottom: 16 }}>
              <div style={{ fontSize: 16, fontWeight: 500, whiteSpace: "pre-wrap", marginBottom: 8 }}>{data.message}</div>
              <div className="row" style={{ gap: 10, fontSize: 12, color: "var(--fg-2)" }}>
                <Icons.Commit size={13} />
                <span className="mono">{data.short}</span>
                <span>·</span>
                <span>{data.author}</span>
                <span className="subtle">{data.email}</span>
                <span className="spacer" />
                <span style={{ color: "var(--accent)" }}>+{data.additions}</span>
                <span style={{ color: "var(--danger)" }}>−{data.deletions}</span>
              </div>
            </div>
            <ReadOnlyDiff files={data.files} />
          </>
        ) : null}
      </div>
    </div>
  );
}

const cvStyles = {
  scroll: { height: "100%", overflowY: "auto" },
  page: { maxWidth: 940, margin: "0 auto", padding: "24px 24px 60px" },
};

window.CommitView = CommitView;
