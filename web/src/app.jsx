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

  // Server-side preferences: hydrate once on mount, then persist changes.
  const hydratingPrefs = React.useRef(false);
  const savePrefs = React.useCallback((partial) => {
    if (window.OrchisAPI && !hydratingPrefs.current) {
      window.OrchisAPI.patch("/v1/me/preferences", partial).catch(() => {});
    }
  }, []);

  const setTheme = (v) => {
    setThemeState(v);
    localStorage.setItem("orchis-theme", v);
    savePrefs({ theme: v });
  };

  // On load, pull saved preferences and apply them (without re-saving).
  React.useEffect(() => {
    if (!window.OrchisAPI) return;
    window.OrchisAPI.get("/v1/me/preferences").then((p) => {
      if (!p || typeof p !== "object") return;
      hydratingPrefs.current = true;
      if (p.theme === "light" || p.theme === "dark") { setThemeState(p.theme); localStorage.setItem("orchis-theme", p.theme); }
      const tw = {};
      ["accent", "density", "font"].forEach((k) => { if (p[k] != null) tw[k] = p[k]; });
      if (typeof p.showSplitTip === "boolean") tw.showSplitTip = p.showSplitTip;
      if (Object.keys(tw).length) setTweak(tw);
      setTimeout(() => { hydratingPrefs.current = false; }, 0);
    }).catch(() => {});
  }, []);

  // Persist tweak changes (accent/density/font/showSplitTip) to the account.
  React.useEffect(() => {
    const onTweak = (e) => savePrefs(e.detail || {});
    window.addEventListener("tweakchange", onTweak);
    return () => window.removeEventListener("tweakchange", onTweak);
  }, [savePrefs]);

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

  // Buttons should not retain focus after a mouse click. Without this, a clicked
  // button keeps focus and the global :focus-visible rule re-fires on the next
  // React re-render — painting a stray accent outline on the *previously* clicked
  // button when you click another. Suppressing focus-on-pointer makes every
  // button behave like the React-driven nav items; keyboard (Tab) focus is
  // untouched, so the accessibility ring still shows for keyboard users.
  React.useEffect(() => {
    const onMouseDown = (e) => {
      if (e.target.closest("input, textarea, select")) return; // keep field focus
      if (e.target.closest("button")) e.preventDefault();
    };
    document.addEventListener("mousedown", onMouseDown);
    return () => document.removeEventListener("mousedown", onMouseDown);
  }, []);

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
  else if (route.view === "newpr") view = <NewPRView route={route} setRoute={navigate} />;
  else if (route.view === "commit") view = <CommitView route={route} setRoute={navigate} />;
  else if (route.view === "user") view = <UserProfileView route={route} setRoute={navigate} />;
  else if (route.view === "settings") view = <DevSettingsView route={route} setRoute={navigate} />;
  else if (route.view === "admin") view = <AdminView route={route} setRoute={navigate} />;
  else if (route.view === "repo") view = <RepoView repoId={route.repo} file={route.file} setRoute={navigate} openSplit={openSplit} splitOpen={splitOpen} splitContent={splitContent} closeSplit={() => setSplitOpen(false)} />;
  else view = <DashboardView setRoute={navigate} openPalette={() => setPaletteOpen(true)} route={route} />;

  return (
    <div style={appStyles.outer}>
      <OrchisNav siteHandle="foundry" />
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
    </div>
  );
}

// (Tweak controls all come from tweaks-panel.jsx)

const appStyles = {
  outer: { display: "flex", flexDirection: "column", height: "100%", overflow: "hidden", background: "var(--bg)" },
  shell: { display: "flex", flex: 1, minHeight: 0, overflow: "hidden", background: "var(--bg)" },
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
  const [methods, setMethods] = React.useState(null);
  const [mode, setMode] = React.useState("login");   // "login" | "signup"
  const [email, setEmail] = React.useState("");
  const [name, setName] = React.useState("");
  const [password, setPassword] = React.useState("");
  const [busy, setBusy] = React.useState(false);
  const [err, setErr] = React.useState("");

  React.useEffect(() => {
    fetch("/v1/auth/methods", { credentials: "same-origin" })
      .then(r => r.json())
      .then(setMethods)
      .catch(() => setMethods({ localLogin: true, localSignup: false, providers: [] }));
  }, []);

  const submit = async (e) => {
    e.preventDefault();
    setBusy(true); setErr("");
    const signup = mode === "signup";
    const body = signup ? { email, name, password } : { identifier: email, password };
    try {
      const r = await fetch(signup ? "/v1/auth/signup" : "/v1/auth/login", {
        method: "POST", credentials: "same-origin",
        headers: { "Content-Type": "application/json" }, body: JSON.stringify(body),
      });
      if (!r.ok) { const j = await r.json().catch(() => ({})); throw new Error(j.error || "Sign-in failed"); }
      window.location.assign("/");
    } catch (e2) { setErr(e2.message); setBusy(false); }
  };

  const m = methods || { localLogin: true, localSignup: false, providers: [] };
  const showForm = m.localLogin || (mode === "signup" && m.localSignup);
  const hasProviders = m.providers && m.providers.length > 0;

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
        <div style={{ fontSize: 13.5, color: "var(--fg-2)", marginBottom: 20 }}>
          A calm, self-hosted code platform.
        </div>

        {showForm ? (
          <form onSubmit={submit} style={{ display: "flex", flexDirection: "column", gap: 10, textAlign: "left" }}>
            <input className="input" type="email" placeholder="Email" autoComplete="email" required
              value={email} onChange={e => setEmail(e.target.value)} />
            {mode === "signup" ? (
              <input className="input" placeholder="Display name (optional)"
                value={name} onChange={e => setName(e.target.value)} />
            ) : null}
            <input className="input" type="password" placeholder="Password" autoComplete={mode === "signup" ? "new-password" : "current-password"} required
              value={password} onChange={e => setPassword(e.target.value)} />
            {err ? <div style={{ color: "var(--danger)", fontSize: 12.5 }}>{err}</div> : null}
            <button className="btn primary" type="submit" disabled={busy} style={{ justifyContent: "center" }}>
              {busy ? "…" : (mode === "signup" ? "Create account" : "Sign in")}
            </button>
          </form>
        ) : null}

        {m.localSignup ? (
          <button className="btn ghost sm" style={{ marginTop: 8 }}
            onClick={() => { setMode(mode === "signup" ? "login" : "signup"); setErr(""); }}>
            {mode === "signup" ? "Have an account? Sign in" : "Create an account"}
          </button>
        ) : null}

        {hasProviders ? (
          <>
            {showForm ? (
              <div style={{ display: "flex", alignItems: "center", gap: 8, margin: "16px 0 4px", color: "var(--fg-3)", fontSize: 11.5 }}>
                <span style={{ flex: 1, height: 1, background: "var(--line)" }} /> or <span style={{ flex: 1, height: 1, background: "var(--line)" }} />
              </div>
            ) : null}
            <div style={{ display: "flex", flexDirection: "column", gap: 8, marginTop: 12 }}>
              {m.providers.map(p => (
                <a key={p.id} href={"/v1/auth/oidc/" + p.id} style={{
                  display: "flex", alignItems: "center", justifyContent: "center", gap: 10,
                  padding: "10px 16px", borderRadius: 10, textDecoration: "none",
                  background: "var(--fg)", color: "var(--bg)", fontSize: 14, fontWeight: 500,
                }}>Continue with {p.label}</a>
              ))}
            </div>
          </>
        ) : null}

        {!showForm && !m.localSignup && !hasProviders ? (
          <div style={{ marginTop: 18, fontSize: 13, color: "var(--fg-2)" }}>
            No sign-in methods are enabled. Contact your administrator.
          </div>
        ) : null}
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
            bio: me.bio || "",
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
