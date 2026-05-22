// Sidebar — slim, vertical, with quick switch + pinned repos.
function Sidebar({ route, setRoute, openPalette, openTokens, theme, setTheme }) {
  const navItems = [
    { id: "home", label: "Home", icon: <Icons.Home /> },
    { id: "prs", label: "Pull requests", icon: <Icons.PR />, badge: 3 },
    { id: "search", label: "Search", icon: <Icons.Search /> },
    { id: "settings", label: "Developer", icon: <Icons.Key /> },
  ];

  const [pinnedState, setPinnedState] = React.useState(null);
  React.useEffect(() => {
    const load = () => {
      if (window.OrchisAPI) window.OrchisAPI.get("/v1/me/pinned").then(setPinnedState).catch(() => setPinnedState([]));
    };
    load();
    window.addEventListener("orchis:repos-changed", load);
    return () => window.removeEventListener("orchis:repos-changed", load);
  }, []);
  const pinned = pinnedState != null ? pinnedState : (window.OrchisAPI ? [] : REPOS.filter(r => r.pinned));

  return (
    <aside style={sbStyles.aside}>
      <div style={sbStyles.brand}>
        <div style={sbStyles.brandMark}>
          <Icons.Logo size={18} />
        </div>
        <div style={{ display: "flex", flexDirection: "column", lineHeight: 1.1 }}>
          <span style={{ fontWeight: 600, fontSize: 14 }}>Orchis</span>
          <span style={{ fontSize: 10.5, color: "var(--fg-3)" }}>avery / kelp</span>
        </div>
        <div className="spacer" />
        <NotificationsBell setRoute={setRoute} />
        <button className="btn ghost icon" onClick={() => setTheme(theme === "dark" ? "light" : "dark")} title="Toggle theme">
          {theme === "dark" ? <Icons.Sun /> : <Icons.Moon />}
        </button>
      </div>

      <div style={sbStyles.searchWrap}>
        <button style={sbStyles.searchBtn} onClick={openPalette}>
          <Icons.Search size={14} />
          <span style={{ color: "var(--fg-3)", flex: 1, textAlign: "left" }}>Jump to anything…</span>
          <span className="kbd">⌘K</span>
        </button>
      </div>

      <nav style={sbStyles.nav}>
        {navItems.map(item => (
          <button
            key={item.id}
            onClick={() => setRoute({ view: item.id })}
            style={{
              ...sbStyles.navItem,
              ...((route.view === item.id) ? sbStyles.navItemActive : {}),
            }}
          >
            <span style={{ width: 18, display: "inline-flex", color: route.view === item.id ? "var(--accent)" : "var(--fg-2)" }}>{item.icon}</span>
            <span style={{ flex: 1, textAlign: "left" }}>{item.label}</span>
            {item.badge ? <span className="chip accent" style={{ height: 18, fontSize: 10.5 }}>{item.badge}</span> : null}
            {item.kbd ? <span className="kbd">{item.kbd}</span> : null}
          </button>
        ))}
      </nav>

      <div style={{ padding: "16px 12px 6px" }}>
        <div className="section-title">Pinned</div>
      </div>
      <div style={sbStyles.pinned}>
        {pinned.map(r => (
          <button
            key={r.id}
            onClick={() => setRoute({ view: "repo", repo: r.id })}
            style={{
              ...sbStyles.navItem,
              ...((route.view === "repo" && route.repo === r.id) ? sbStyles.navItemActive : {}),
              paddingLeft: 12,
            }}
          >
            <span style={{ width: 8, height: 8, borderRadius: 2, background: r.languageColor, flexShrink: 0 }} />
            <span style={{ flex: 1, textAlign: "left", fontFamily: "var(--font-mono)", fontSize: 12.5, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
              {r.org}/{r.name}
            </span>
            {r.visibility === "private" ? <span className="subtle" style={{ fontSize: 10 }}>private</span> : null}
          </button>
        ))}
      </div>

      <div className="spacer" />

      <div style={sbStyles.footer}>
        <button style={sbStyles.user} onClick={() => setRoute({ view: "settings" })}>
          <span className="avatar" style={{ background: USERS.me.color }}>{USERS.me.initials}</span>
          <div style={{ display: "flex", flexDirection: "column", lineHeight: 1.15, textAlign: "left", flex: 1, minWidth: 0 }}>
            <span style={{ fontSize: 12.5, fontWeight: 500 }}>{USERS.me.name}</span>
            <span style={{ fontSize: 10.5, color: "var(--fg-3)" }}>@{USERS.me.handle}</span>
          </div>
          <Icons.Chevron size={12} style={{ color: "var(--fg-3)" }} />
        </button>
      </div>
    </aside>
  );
}

// NotificationsBell — global inbox affordance. Shows an unread badge, opens a
// dropdown of inbox items, and refreshes live via the SSE inbox:me topic.
// Unread is tracked client-side (localStorage seen-keys) — no backend change.
function NotificationsBell({ setRoute }) {
  const [items, setItems] = React.useState([]);
  const [open, setOpen] = React.useState(false);
  const [seen, setSeen] = React.useState(() => {
    try { return new Set(JSON.parse(localStorage.getItem("orchis-inbox-seen") || "[]")); } catch (e) { return new Set(); }
  });

  const itemKey = (it) => [it.title, it.repo || "", it.detail || ""].join("|");
  const persistSeen = (s) => { try { localStorage.setItem("orchis-inbox-seen", JSON.stringify([...s])); } catch (e) {} };

  const load = React.useCallback(() => {
    if (window.OrchisAPI) window.OrchisAPI.get("/v1/me/inbox").then(x => setItems(x || [])).catch(() => setItems([]));
  }, []);
  React.useEffect(() => { load(); }, [load]);
  React.useEffect(() => {
    if (!window.EventSource) return;
    const es = new EventSource("/v1/stream?topics=inbox:me");
    es.addEventListener("inbox.new", load);
    return () => es.close();
  }, [load]);

  const unread = items.filter(it => !seen.has(itemKey(it))).length;

  const toggle = () => {
    const next = !open;
    setOpen(next);
    if (next && items.length) {
      const s = new Set(seen);
      items.forEach(it => s.add(itemKey(it)));
      setSeen(s); persistSeen(s);
    }
  };
  const go = (it) => { setOpen(false); if (it.route) setRoute(it.route); };

  return (
    <div style={{ position: "relative" }}>
      <button className="btn ghost icon" onClick={toggle} title="Notifications" style={{ position: "relative" }}>
        <Icons.Bell />
        {unread > 0 ? (
          <span style={{ position: "absolute", top: 3, right: 3, minWidth: 14, height: 14, padding: "0 3px", borderRadius: 999, background: "var(--accent)", color: "var(--accent-fg)", fontSize: 9, fontWeight: 600, display: "flex", alignItems: "center", justifyContent: "center", lineHeight: 1 }}>{unread > 9 ? "9+" : unread}</span>
        ) : null}
      </button>
      {open ? (
        <>
          <div onClick={() => setOpen(false)} style={{ position: "fixed", inset: 0, zIndex: 90 }} />
          <div className="card fade-in" style={{ position: "absolute", top: "calc(100% + 6px)", right: 0, width: 320, maxHeight: 420, overflowY: "auto", zIndex: 91, boxShadow: "var(--shadow-lg)", padding: 6 }}>
            <div className="row" style={{ padding: "6px 8px 8px" }}>
              <span style={{ fontSize: 12.5, fontWeight: 600 }}>Notifications</span>
              <span className="spacer" />
              <span className="subtle" style={{ fontSize: 11 }}>{items.length} item{items.length === 1 ? "" : "s"}</span>
            </div>
            {items.length === 0 ? (
              <div style={{ padding: "16px 10px", color: "var(--fg-3)", fontSize: 12.5, textAlign: "center" }}>You're all caught up.</div>
            ) : items.map((it, i) => (
              <button key={i} onClick={() => go(it)} style={{ display: "flex", flexDirection: "column", gap: 2, width: "100%", textAlign: "left", padding: "9px 10px", borderRadius: 6, background: "transparent", border: "none", cursor: "pointer", font: "inherit", color: "inherit" }}>
                <div className="row" style={{ gap: 6 }}>
                  <span style={{ width: 6, height: 6, borderRadius: 999, background: it.kind === "warn" ? "var(--warn)" : "var(--accent)", flexShrink: 0 }} />
                  <span style={{ fontSize: 12.5, fontWeight: 500 }}>{it.title}</span>
                  <span className="spacer" />
                  <span className="mono subtle" style={{ fontSize: 10.5 }}>{it.repo}</span>
                </div>
                <div className="subtle" style={{ fontSize: 11.5, paddingLeft: 12, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{it.detail}</div>
              </button>
            ))}
          </div>
        </>
      ) : null}
    </div>
  );
}

const sbStyles = {
  aside: {
    width: 240,
    flexShrink: 0,
    height: "100%",
    display: "flex",
    flexDirection: "column",
    borderRight: "1px solid var(--line)",
    background: "var(--bg-1)",
  },
  brand: {
    display: "flex",
    alignItems: "center",
    gap: 10,
    padding: "12px 12px 10px",
    borderBottom: "1px solid var(--line)",
  },
  brandMark: {
    width: 28, height: 28,
    borderRadius: 7,
    background: "var(--accent)",
    color: "var(--accent-fg)",
    display: "flex", alignItems: "center", justifyContent: "center",
    flexShrink: 0,
  },
  searchWrap: {
    padding: "10px 10px 6px",
  },
  searchBtn: {
    display: "flex",
    alignItems: "center",
    gap: 8,
    width: "100%",
    height: 30,
    padding: "0 10px",
    background: "var(--bg-2)",
    border: "1px solid var(--line)",
    borderRadius: 6,
    color: "var(--fg-2)",
    cursor: "pointer",
    font: "inherit",
    fontSize: 12.5,
  },
  nav: {
    display: "flex",
    flexDirection: "column",
    gap: 2,
    padding: "4px 8px",
  },
  navItem: {
    display: "flex",
    alignItems: "center",
    gap: 10,
    height: 30,
    padding: "0 10px",
    borderRadius: 6,
    border: "1px solid transparent",
    background: "transparent",
    cursor: "pointer",
    color: "var(--fg-1)",
    fontSize: 13,
    font: "inherit",
    fontWeight: 450,
  },
  navItemActive: {
    background: "var(--bg-2)",
    color: "var(--fg)",
    borderColor: "var(--line)",
  },
  pinned: {
    display: "flex",
    flexDirection: "column",
    gap: 1,
    padding: "0 8px",
    overflowY: "auto",
  },
  footer: {
    borderTop: "1px solid var(--line)",
    padding: 8,
  },
  user: {
    display: "flex",
    alignItems: "center",
    gap: 10,
    width: "100%",
    padding: "6px 8px",
    background: "transparent",
    border: "1px solid transparent",
    borderRadius: 8,
    cursor: "pointer",
    font: "inherit",
  },
};

window.Sidebar = Sidebar;
