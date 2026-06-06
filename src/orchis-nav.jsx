// OrchisNav — cross-site navigation bar for orchis.ai sub-sites.
// Mirrors the canonical nav in orchis-landing/src/partials/nav.html. The
// markup, link order, and behaviour must stay in sync with that file.
//
// Mounted at the top of Foundry's app shell; sits above the existing
// vertical Sidebar (which keeps doing Foundry-internal navigation).
//
// Admin probe: a small fetch against admin.orchis.ai/v1/me. The nginx IP
// allowlist in front of admin makes this succeed only from the operator's
// home IP; everywhere else the fetch errors and the admin link is never
// rendered. The URL appears in the bundled source (a view-source reveal),
// but never in rendered HTML.
function OrchisNav({ siteHandle = "foundry" }) {
  const [showAdmin, setShowAdmin] = React.useState(false);

  React.useEffect(() => {
    if (typeof window === "undefined") return;
    // Don't probe when running on the admin console itself.
    if (window.location && window.location.hostname === "admin.orchis.ai") return;
    if (navigator.connection && navigator.connection.saveData) return;

    let cancelled = false;
    fetch("https://admin.orchis.ai/v1/me", {
      method: "GET",
      credentials: "include",
      mode: "cors",
      cache: "no-store",
    })
      .then((r) => (r.ok ? r.json() : null))
      .then((me) => {
        if (!cancelled && me && me.handle) setShowAdmin(true);
      })
      .catch(() => {
        // Not the operator, or admin offline. Do nothing.
      });
    return () => { cancelled = true; };
  }, []);

  const LINKS = [
    { id: "foundry",   label: "Foundry",   href: "https://foundry.orchis.ai" },
    { id: "downloads", label: "Downloads", href: "https://downloads.orchis.ai" },
    { id: "forum",     label: "Forum",     href: "https://forum.orchis.ai" },
    { id: "kicad",     label: "KiCAD-MCP", href: "https://kicad.orchis.ai" },
  ];

  return (
    <nav style={navStyles.bar} aria-label="Site navigation">
      <a href="https://orchis.ai" style={navStyles.brand} aria-label="orchis.ai home">
        <span style={navStyles.mark} aria-hidden="true">
          <span style={navStyles.markInner} />
        </span>
        <span>orchis<span style={navStyles.dot}>.</span>ai</span>
      </a>
      <ul style={navStyles.links} role="list">
        {LINKS.map((l) => {
          const active = l.id === siteHandle;
          return (
            <li key={l.id} style={{ listStyle: "none" }}>
              <a
                href={l.href}
                style={active ? navStyles.linkActive : navStyles.link}
              >
                {l.label}
              </a>
            </li>
          );
        })}
      </ul>
      <div style={navStyles.aux}>
        {showAdmin ? (
          <a href="https://admin.orchis.ai" style={navStyles.admin} title="Open admin console">
            <span style={navStyles.adminDot} aria-hidden="true" />
            admin
          </a>
        ) : null}
      </div>
    </nav>
  );
}

const navStyles = {
  bar: {
    display: "flex",
    alignItems: "center",
    gap: 20,
    height: 44,
    padding: "0 16px",
    background: "var(--bg-1)",
    borderBottom: "1px solid var(--line)",
    fontSize: 13,
    flexShrink: 0,
    zIndex: 50,
  },
  brand: {
    display: "inline-flex",
    alignItems: "center",
    gap: 8,
    color: "var(--fg)",
    textDecoration: "none",
    fontWeight: 600,
    fontSize: 13.5,
    letterSpacing: "-0.01em",
    flexShrink: 0,
  },
  mark: {
    width: 12,
    height: 12,
    background: "var(--accent)",
    borderRadius: 2,
    transform: "rotate(45deg)",
    position: "relative",
    display: "inline-block",
  },
  markInner: {
    position: "absolute",
    inset: 3,
    background: "var(--bg-1)",
    borderRadius: 1,
  },
  dot: { color: "var(--accent)" },
  links: {
    display: "flex",
    alignItems: "center",
    gap: 4,
    listStyle: "none",
    margin: 0,
    padding: 0,
    flex: 1,
  },
  link: {
    display: "inline-block",
    padding: "5px 10px",
    borderRadius: 4,
    color: "var(--fg-2)",
    textDecoration: "none",
    fontWeight: 500,
    fontSize: 12.5,
    border: "1px solid transparent",
  },
  linkActive: {
    display: "inline-block",
    padding: "5px 10px",
    borderRadius: 4,
    color: "var(--fg)",
    textDecoration: "none",
    fontWeight: 500,
    fontSize: 12.5,
    border: "1px solid var(--line)",
    background: "var(--bg-2)",
  },
  aux: {
    display: "inline-flex",
    alignItems: "center",
    gap: 8,
    flexShrink: 0,
  },
  admin: {
    display: "inline-flex",
    alignItems: "center",
    gap: 6,
    padding: "4px 10px",
    borderRadius: 4,
    color: "var(--accent)",
    border: "1px solid var(--accent)",
    textDecoration: "none",
    fontFamily: "var(--font-mono)",
    fontSize: 11,
    fontWeight: 500,
    letterSpacing: "0.04em",
  },
  adminDot: {
    display: "inline-block",
    width: 6,
    height: 6,
    borderRadius: "50%",
    background: "var(--accent)",
  },
};

window.OrchisNav = OrchisNav;
