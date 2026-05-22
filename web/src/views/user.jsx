// Public user profile — identity + their public repositories.
function UserProfileView({ route, setRoute }) {
  const handle = route.handle;
  const [data, setData] = React.useState(null);
  const [err, setErr] = React.useState(false);

  React.useEffect(() => {
    if (!window.OrchisAPI || !handle) return;
    setData(null); setErr(false);
    window.OrchisAPI.get(`/v1/users/${encodeURIComponent(handle)}`).then(setData).catch(() => setErr(true));
  }, [handle]);

  return (
    <div style={upStyles.scroll}>
      <div style={upStyles.page} className="fade-in">
        {err ? <div className="card" style={{ padding: 18, color: "var(--fg-3)", fontSize: 13 }}>User not found.</div> : null}
        {!data && !err ? <div className="muted" style={{ fontSize: 12.5 }}>Loading…</div> : null}
        {data ? (
          <>
            <div className="row" style={{ gap: 16, alignItems: "flex-start", marginBottom: 24 }}>
              {data.avatarUrl
                ? <img src={data.avatarUrl} alt="" style={{ width: 72, height: 72, borderRadius: 999, border: "1px solid var(--line)" }} />
                : <span className="avatar" style={{ background: data.color, width: 72, height: 72, fontSize: 26 }}>{data.initials}</span>}
              <div style={{ flex: 1, minWidth: 0 }}>
                <h1 style={{ fontSize: 22, fontWeight: 500, margin: 0 }}>{data.name || data.handle}</h1>
                <div className="muted" style={{ fontSize: 14, marginTop: 2 }}>@{data.handle}</div>
                {data.bio ? <p style={{ fontSize: 13.5, lineHeight: 1.5, margin: "10px 0 0", maxWidth: 600 }}>{data.bio}</p> : null}
                <div className="subtle" style={{ fontSize: 11.5, marginTop: 10 }}>Joined {data.memberSince}</div>
              </div>
            </div>

            <div className="section-title" style={{ marginBottom: 10 }}>Public repositories ({data.repos.length})</div>
            {data.repos.length === 0 ? (
              <div className="card" style={{ padding: "20px 18px", color: "var(--fg-3)", fontSize: 13 }}>No public repositories.</div>
            ) : (
              <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(280px, 1fr))", gap: 12 }}>
                {data.repos.map(rp => (
                  <button key={rp.id} className="card" onClick={() => setRoute({ view: "repo", repo: rp.id })}
                    style={{ padding: 14, textAlign: "left", cursor: "pointer", border: "1px solid var(--line)", background: "var(--bg-1)", display: "flex", flexDirection: "column", gap: 6 }}>
                    <div className="row" style={{ gap: 8 }}>
                      <span style={{ width: 9, height: 9, borderRadius: 2, background: rp.languageColor, flexShrink: 0 }} />
                      <span className="mono" style={{ fontSize: 13, fontWeight: 500, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{rp.name}</span>
                      <span className="spacer" />
                      <span className="row subtle" style={{ gap: 3, fontSize: 11.5 }}><Icons.Star size={11} /> {rp.stars}</span>
                    </div>
                    <div className="muted" style={{ fontSize: 12.5, minHeight: 18, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{rp.description || "No description"}</div>
                    <div className="subtle" style={{ fontSize: 11 }}>{rp.language || "—"} · updated {rp.updated}</div>
                  </button>
                ))}
              </div>
            )}
          </>
        ) : null}
      </div>
    </div>
  );
}

const upStyles = {
  scroll: { height: "100%", overflowY: "auto" },
  page: { maxWidth: 920, margin: "0 auto", padding: "32px 24px 60px" },
};

window.UserProfileView = UserProfileView;
