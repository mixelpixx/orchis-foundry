// Main app — shell, routing, command palette, tweaks.

const TWEAK_DEFAULTS = /*EDITMODE-BEGIN*/{
  "accent": "spring",
  "density": "default",
  "font": "geist",
  "showSplitTip": true
}/*EDITMODE-END*/;

const ACCENTS = {
  spring: { name: "Spring",   accent: "oklch(62% 0.14 150)", soft: "oklch(94% 0.04 150)", line: "oklch(80% 0.08 150)", darkAccent: "oklch(72% 0.16 150)", darkSoft: "oklch(28% 0.06 150)", darkLine: "oklch(45% 0.1 150)" },
  cobalt: { name: "Cobalt",   accent: "oklch(60% 0.16 250)", soft: "oklch(94% 0.04 250)", line: "oklch(80% 0.08 250)", darkAccent: "oklch(70% 0.18 250)", darkSoft: "oklch(28% 0.07 250)", darkLine: "oklch(45% 0.12 250)" },
  ember:  { name: "Ember",    accent: "oklch(62% 0.16 40)",  soft: "oklch(94% 0.04 40)",  line: "oklch(80% 0.09 40)",  darkAccent: "oklch(72% 0.17 40)",  darkSoft: "oklch(30% 0.07 40)",  darkLine: "oklch(45% 0.12 40)" },
  violet: { name: "Violet",   accent: "oklch(58% 0.17 300)", soft: "oklch(94% 0.04 300)", line: "oklch(78% 0.08 300)", darkAccent: "oklch(70% 0.18 300)", darkSoft: "oklch(28% 0.07 300)", darkLine: "oklch(45% 0.12 300)" },
};

const FONTS = {
  geist: { sans: '"Geist", ui-sans-serif, -apple-system, sans-serif', mono: '"Geist Mono", ui-monospace, monospace', label: "Geist" },
  serifMix: { sans: '"Fraunces", "Geist", serif', mono: '"Geist Mono", monospace', label: "Editorial" },
  mono: { sans: '"Geist Mono", monospace', mono: '"Geist Mono", monospace', label: "All mono" },
};

function App() {
  const [theme, setThemeState] = React.useState(() => {
    return localStorage.getItem("orchis-theme") || (window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light");
  });
  const [t, setTweak] = useTweaks(TWEAK_DEFAULTS);
  const [route, setRoute] = React.useState({ view: "home" });
  const [paletteOpen, setPaletteOpen] = React.useState(false);
  const [splitOpen, setSplitOpen] = React.useState(true);
  const [splitContent, setSplitContent] = React.useState({ type: "prs", repo: "kelp/atlas" });

  const setTheme = (v) => {
    setThemeState(v);
    localStorage.setItem("orchis-theme", v);
  };

  // Apply theme + density + accent to document
  React.useEffect(() => {
    document.documentElement.setAttribute("data-theme", theme);
  }, [theme]);

  React.useEffect(() => {
    document.documentElement.setAttribute("data-density", t.density);
  }, [t.density]);

  React.useEffect(() => {
    const a = ACCENTS[t.accent] || ACCENTS.spring;
    const root = document.documentElement;
    if (theme === "dark") {
      root.style.setProperty("--accent", a.darkAccent);
      root.style.setProperty("--accent-soft", a.darkSoft);
      root.style.setProperty("--accent-line", a.darkLine);
    } else {
      root.style.setProperty("--accent", a.accent);
      root.style.setProperty("--accent-soft", a.soft);
      root.style.setProperty("--accent-line", a.line);
    }
  }, [t.accent, theme]);

  React.useEffect(() => {
    const f = FONTS[t.font] || FONTS.geist;
    document.documentElement.style.setProperty("--font-sans", f.sans);
    document.documentElement.style.setProperty("--font-mono", f.mono);
  }, [t.font]);

  // Cmd+K palette
  React.useEffect(() => {
    const handler = (e) => {
      if ((e.metaKey || e.ctrlKey) && (e.key === "k" || e.key === "K")) {
        e.preventDefault();
        setPaletteOpen(o => !o);
      }
      if ((e.metaKey || e.ctrlKey) && e.key === "\\") {
        e.preventDefault();
        if (route.view === "repo") setSplitOpen(s => !s);
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [route.view]);

  const navigate = (next) => {
    setRoute(next);
    if (next.newToken || next.newKey || next.newHook) {
      // already encoded
    }
  };

  const openSplit = (content) => {
    setSplitContent(content);
    setSplitOpen(true);
  };

  // Render current view
  let view;
  if (route.view === "home") view = <DashboardView setRoute={navigate} openPalette={() => setPaletteOpen(true)} route={route} />;
  else if (route.view === "prs") view = <PRsView setRoute={navigate} />;
  else if (route.view === "pr") view = <PRView prId={route.pr} repo={route.repo} setRoute={navigate} />;
  else if (route.view === "search") view = <SearchView route={route} setRoute={navigate} />;
  else if (route.view === "settings") view = <DevSettingsView route={route} setRoute={navigate} />;
  else if (route.view === "repo") view = <RepoView repoId={route.repo} file={route.file} setRoute={navigate} openSplit={openSplit} splitOpen={splitOpen} splitContent={splitContent} closeSplit={() => setSplitOpen(false)} />;
  else view = <DashboardView setRoute={navigate} openPalette={() => setPaletteOpen(true)} route={route} />;

  return (
    <div style={appStyles.shell}>
      <Sidebar
        route={route}
        setRoute={navigate}
        openPalette={() => setPaletteOpen(true)}
        theme={theme}
        setTheme={setTheme}
      />
      <main style={appStyles.main}>
        {view}
      </main>

      <Palette open={paletteOpen} onClose={() => setPaletteOpen(false)} onNavigate={navigate} />

      {/* Toast: keyboard hint, dismissible */}
      {t.showSplitTip && route.view === "repo" ? (
        <div style={appStyles.tip} className="fade-in">
          <Icons.Bolt size={12} style={{ color: "var(--accent)" }} />
          <span>Try <span className="kbd">⌘\</span> to toggle split. <span className="kbd">⌘K</span> reaches anything.</span>
          <button className="btn ghost icon sm" onClick={() => setTweak({ showSplitTip: false })}><Icons.Close size={11} /></button>
        </div>
      ) : null}

      <TweaksPanel title="Tweaks">
        <TweakSection label="Theme">
          <TweakRadio
            label="Mode"
            value={theme}
            onChange={setTheme}
            options={[
              { label: "Light", value: "light" },
              { label: "Dark", value: "dark" },
            ]}
          />
          <TweakSelect
            label="Accent"
            value={t.accent}
            onChange={(v) => setTweak({ accent: v })}
            options={Object.entries(ACCENTS).map(([k, v]) => ({ label: v.name, value: k }))}
          />
        </TweakSection>

        <TweakSection label="Layout">
          <TweakRadio
            label="Density"
            value={t.density}
            onChange={(v) => setTweak({ density: v })}
            options={[
              { label: "Compact", value: "compact" },
              { label: "Default", value: "default" },
              { label: "Cozy", value: "cozy" },
            ]}
          />
          <TweakSelect
            label="Font"
            value={t.font}
            onChange={(v) => setTweak({ font: v })}
            options={Object.entries(FONTS).map(([k, v]) => ({ label: v.label, value: k }))}
          />
        </TweakSection>

        <TweakSection label="Behavior">
          <TweakToggle label="Keyboard tip" value={t.showSplitTip} onChange={(v) => setTweak({ showSplitTip: v })} />
        </TweakSection>

        <TweakSection label="Try">
          <TweakButton label="Open ⌘K palette" onClick={() => setPaletteOpen(true)} />
          <TweakButton label="Create a token (3-step)" onClick={() => navigate({ view: "settings", tab: "tokens", newToken: true })} />
        </TweakSection>
      </TweaksPanel>
    </div>
  );
}

// (Tweak controls all come from tweaks-panel.jsx)

const appStyles = {
  shell: { display: "flex", height: "100%", overflow: "hidden", background: "var(--bg)" },
  main: { flex: 1, position: "relative", minWidth: 0, overflow: "hidden" },
  tip: {
    position: "absolute",
    bottom: 16, left: "50%",
    transform: "translateX(-50%)",
    display: "flex", alignItems: "center", gap: 10,
    padding: "8px 12px",
    background: "var(--bg-1)",
    border: "1px solid var(--line-strong)",
    borderRadius: 999,
    boxShadow: "var(--shadow-md)",
    fontSize: 12,
    color: "var(--fg-1)",
    zIndex: 100,
  },
};

// --- Auth gate -------------------------------------------------------------
// Wraps the app: checks /v1/me, shows a sign-in screen on 401, hydrates the
// real user into USERS.me on success. If the fetch fails entirely (no backend,
// e.g. opening index.html from disk), falls back to the mock so the prototype
// still works standalone.
function SignIn() {
  return (
    <div style={{
      position: "fixed", inset: 0, display: "flex", alignItems: "center",
      justifyContent: "center", background: "var(--bg)", color: "var(--fg)",
      fontFamily: "var(--font-sans)",
    }}>
      <div style={{
        width: 360, maxWidth: "90vw", padding: 32,
        border: "1px solid var(--line)", borderRadius: 14, background: "var(--bg-1)",
        textAlign: "center",
      }}>
        <div style={{ fontSize: 22, fontWeight: 600, letterSpacing: "-0.02em", marginBottom: 6 }}>
          Orchis Foundry
        </div>
        <div style={{ fontSize: 13.5, color: "var(--fg-2)", marginBottom: 24 }}>
          A calm, self-hosted code platform. Sign in to continue.
        </div>
        <a href="/v1/auth/oidc/github" style={{
          display: "flex", alignItems: "center", justifyContent: "center", gap: 10,
          padding: "11px 16px", borderRadius: 10, textDecoration: "none",
          background: "var(--fg)", color: "var(--bg)", fontSize: 14, fontWeight: 500,
        }}>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
            <path d="M12 2a10 10 0 0 0-3.16 19.49c.5.09.68-.22.68-.48v-1.7c-2.78.6-3.37-1.34-3.37-1.34-.45-1.16-1.11-1.46-1.11-1.46-.91-.62.07-.6.07-.6 1 .07 1.53 1.03 1.53 1.03.89 1.53 2.34 1.09 2.91.83.09-.65.35-1.09.63-1.34-2.22-.25-4.55-1.11-4.55-4.94 0-1.09.39-1.99 1.03-2.69-.1-.25-.45-1.27.1-2.65 0 0 .84-.27 2.75 1.02a9.6 9.6 0 0 1 5 0c1.91-1.29 2.75-1.02 2.75-1.02.55 1.38.2 2.4.1 2.65.64.7 1.03 1.6 1.03 2.69 0 3.84-2.34 4.69-4.57 4.93.36.31.68.92.68 1.85v2.74c0 .27.18.58.69.48A10 10 0 0 0 12 2Z"/>
          </svg>
          Sign in with GitHub
        </a>
      </div>
    </div>
  );
}

function Root() {
  const [state, setState] = React.useState("loading"); // loading | authed | signin
  React.useEffect(() => {
    fetch("/v1/me", { credentials: "same-origin" })
      .then((r) => {
        if (r.status === 401) { setState("signin"); return null; }
        if (!r.ok) throw new Error("me failed");
        return r.json();
      })
      .then(async (me) => {
        if (me) {
          // Hydrate the mock user object in place so the (untouched) views see
          // the real signed-in identity.
          Object.assign(USERS.me, {
            id: me.id || "me",
            name: me.name || me.handle,
            handle: me.handle,
            initials: me.initials || "?",
            color: me.color || USERS.me.color,
            avatarUrl: me.avatarUrl || "",
            isAdmin: !!me.isAdmin,
          });
          // Replace the mock repo list with the user's real repos.
          try { await window.loadRepos(); } catch (e) { /* keep mock on failure */ }
          setState("authed");
        }
      })
      .catch(() => {
        // No backend reachable (standalone prototype) — keep the mock.
        setState("authed");
      });
  }, []);

  if (state === "loading") {
    return <div style={{ position: "fixed", inset: 0, background: "var(--bg)" }} />;
  }
  if (state === "signin") return <SignIn />;
  return <App />;
}

ReactDOM.createRoot(document.getElementById("root")).render(<Root />);

// Mobile responsive — collapse sidebar on small screens
const respCss = document.createElement("style");
respCss.textContent = `
@media (max-width: 760px) {
  aside { width: 56px !important; }
  aside [style*="font-size: 14px"], aside [style*="font-size:14px"] { display: none; }
  aside button span:not(:first-child) { display: none; }
  aside button { justify-content: center !important; }
  aside > div:nth-of-type(2) { display: none; }
  aside nav button { padding: 0 !important; width: 36px; height: 36px; }
  aside [class*="section-title"] { display: none; }
}
`;
document.head.appendChild(respCss);
