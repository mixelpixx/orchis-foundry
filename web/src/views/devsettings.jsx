// Developer settings — PATs, SSH keys, webhooks. Flat & fast.
function DevSettingsView({ route, setRoute }) {
  const [tab, setTab] = React.useState(route.tab || "tokens");
  const [newToken, setNewToken] = React.useState(route.newToken || false);
  const [newKey, setNewKey] = React.useState(route.newKey || false);
  const [newHook, setNewHook] = React.useState(route.newHook || false);
  const [generatedToken, setGeneratedToken] = React.useState(null);

  React.useEffect(() => {
    if (route.tab) setTab(route.tab);
    if (route.newToken) setNewToken(true);
    if (route.newKey) setNewKey(true);
    if (route.newHook) setNewHook(true);
  }, [route]);

  const tabs = [
    { id: "account", label: "Account", icon: <Icons.Eye /> },
    { id: "tokens", label: "Tokens", icon: <Icons.Key /> },
    { id: "ssh", label: "SSH keys", icon: <Icons.Key /> },
    { id: "security", label: "Security", icon: <Icons.Check /> },
    { id: "scanner", label: "Scanner", icon: <Icons.Bolt /> },
    { id: "webhooks", label: "Webhooks", icon: <Icons.Webhook /> },
    { id: "apps", label: "OAuth apps", icon: <Icons.Bolt /> },
    { id: "preferences", label: "Preferences", icon: <Icons.Settings /> },
  ];

  return (
    <div style={{ overflowY: "auto", height: "100%" }}>
      <div style={dsStyles.page} className="fade-in">
        <div style={{ marginBottom: 24 }}>
          <div style={{ fontSize: 11.5, color: "var(--fg-3)", textTransform: "uppercase", letterSpacing: "0.06em", marginBottom: 6 }}>Developer settings</div>
          <h1 style={{ fontSize: 26, fontWeight: 500, letterSpacing: "-0.02em", margin: 0 }}>Your dev surface</h1>
          <p className="muted" style={{ margin: "6px 0 0", fontSize: 14, maxWidth: 540 }}>
            Tokens, SSH keys, and webhooks — flat. No more digging four levels deep to rotate a PAT.
          </p>
        </div>

        <div style={dsStyles.layout}>
          <nav style={dsStyles.sideTabs}>
            {tabs.map(t => (
              <button key={t.id} onClick={() => setTab(t.id)} style={{
                ...dsStyles.sideTab,
                ...(tab === t.id ? dsStyles.sideTabActive : {}),
              }}>
                <span style={{ width: 16, color: tab === t.id ? "var(--accent)" : "var(--fg-2)" }}>{t.icon}</span>
                <span style={{ flex: 1, textAlign: "left" }}>{t.label}</span>
                {t.count != null ? <span className="chip" style={{ height: 18, fontSize: 10.5 }}>{t.count}</span> : null}
              </button>
            ))}
          </nav>

          <div style={dsStyles.main}>
            {tab === "account" ? <AccountPanel /> : null}
            {tab === "security" ? <SecurityPanel /> : null}
            {tab === "tokens" ? (
              <TokensPanel newToken={newToken} setNewToken={setNewToken} generated={generatedToken} setGenerated={setGeneratedToken} />
            ) : null}
            {tab === "ssh" ? <SSHPanel newKey={newKey} setNewKey={setNewKey} /> : null}
            {tab === "scanner" ? <ScannerPanel /> : null}
            {tab === "webhooks" ? <WebhooksPanel newHook={newHook} setNewHook={setNewHook} /> : null}
            {tab === "apps" ? <AppsPanel /> : null}
            {tab === "preferences" ? <PreferencesPanel /> : null}
          </div>
        </div>
      </div>
    </div>
  );
}

function TokensPanel({ newToken, setNewToken, generated, setGenerated }) {
  if (generated) {
    return (
      <div className="fade-in">
        <div className="card" style={{ padding: 20, borderColor: "var(--accent-line)", background: "var(--accent-soft)" }}>
          <div className="row" style={{ gap: 8, marginBottom: 10 }}>
            <Icons.Check size={14} style={{ color: "var(--accent)" }} />
            <span style={{ fontWeight: 500, color: "var(--accent)" }}>Token created — copy it now.</span>
          </div>
          <p className="muted" style={{ margin: "0 0 12px", fontSize: 13 }}>You won't be able to see this token again. It's only shown here.</p>
          <div style={{
            display: "flex", alignItems: "center", gap: 10,
            padding: "12px 14px", borderRadius: 8,
            background: "var(--bg)", border: "1px solid var(--line-strong)",
            fontFamily: "var(--font-mono)", fontSize: 13,
          }}>
            <span style={{ flex: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{generated.value}</span>
            <button className="btn sm" onClick={() => navigator.clipboard?.writeText(generated.value).catch(()=>{})}><Icons.Copy size={12} /> Copy</button>
          </div>
          <button className="btn primary sm" style={{ marginTop: 14 }} onClick={() => setGenerated(null)}>I've saved it</button>
        </div>
      </div>
    );
  }

  // Live token list. Reloads when we return from the "generated" view (i.e.
  // after creating/rotating a token). Falls back to the mock when offline.
  const [tokens, setTokens] = React.useState(null);
  const reload = React.useCallback(() => {
    if (window.OrchisAPI) {
      window.OrchisAPI.get("/v1/me/tokens").then(setTokens).catch(() => setTokens([]));
    }
  }, []);
  React.useEffect(() => { if (!generated) reload(); }, [generated, reload]);

  const list = tokens != null ? tokens : (window.OrchisAPI ? [] : TOKENS);

  const revoke = async (id) => {
    try { await window.OrchisAPI.del("/v1/me/tokens/" + id); reload(); } catch (e) {}
  };
  const rotate = async (id) => {
    try {
      const created = await window.OrchisAPI.post("/v1/me/tokens/" + id + "/rotate", {});
      setGenerated({ name: created.name, value: created.token });
    } catch (e) {}
  };

  return (
    <>
      <div style={dsStyles.head}>
        <div>
          <h2 style={dsStyles.h2}>Personal access tokens</h2>
          <p className="muted" style={dsStyles.subtitle}>Use these for git over HTTPS, CI pipelines, and API access.</p>
        </div>
        <button className="btn primary" onClick={() => setNewToken(true)}><Icons.Plus size={14} /> New token</button>
      </div>

      {newToken ? <NewTokenForm onCancel={() => setNewToken(false)} onCreate={(t) => { setNewToken(false); setGenerated(t); }} /> : null}

      {list.length === 0 ? (
        <div className="card" style={{ marginTop: 14, padding: "20px 18px", color: "var(--fg-3)", fontSize: 13 }}>
          No tokens yet. Create one to push over HTTPS or hit the API.
        </div>
      ) : (
      <div className="card" style={{ marginTop: 14 }}>
        {list.map((t, i) => (
          <div key={t.id} style={{
            display: "flex", alignItems: "center", gap: 16,
            padding: "14px 18px",
            borderBottom: i < list.length - 1 ? "1px solid var(--line)" : "none",
            opacity: t.expired ? 0.55 : 1,
          }}>
            <Icons.Key size={16} style={{ color: t.expiringSoon ? "var(--warn)" : t.expired ? "var(--fg-3)" : "var(--fg-2)" }} />
            <div style={{ flex: 1, minWidth: 0 }}>
              <div className="row" style={{ gap: 8, marginBottom: 4 }}>
                <span style={{ fontSize: 14, fontWeight: 500 }}>{t.name}</span>
                {t.expiringSoon ? <span className="chip warn">expires {t.expires}</span> : null}
                {t.expired ? <span className="chip">expired</span> : null}
              </div>
              <div className="row" style={{ gap: 6, color: "var(--fg-2)", fontSize: 11.5, flexWrap: "wrap" }}>
                {(t.scopes || []).map(s => <span key={s} className="mono chip">{s}</span>)}
              </div>
              <div className="subtle" style={{ fontSize: 11.5, marginTop: 6 }}>
                Created {t.created || "—"} · Last used {t.lastUsed || "never"}{t.expires ? ` · Expires ${t.expires}` : ""}
              </div>
            </div>
            <div className="row" style={{ gap: 6 }}>
              <button className="btn sm" onClick={() => rotate(t.id)}>Rotate</button>
              <button className="btn ghost sm" style={{ color: "var(--danger)" }} onClick={() => revoke(t.id)}>Revoke</button>
            </div>
          </div>
        ))}
      </div>
      )}

      <p className="subtle" style={{ marginTop: 14, fontSize: 12 }}>
        <Icons.Bolt size={11} style={{ verticalAlign: "-1px", marginRight: 4 }} />
        Tip: <span className="kbd">⌘K</span> → "create token" — anywhere in Orchis.
      </p>
    </>
  );
}

function NewTokenForm({ onCancel, onCreate }) {
  const [name, setName] = React.useState("");
  const [expires, setExpires] = React.useState("90");
  const [scopes, setScopes] = React.useState({ "repo:read": true });
  const [busy, setBusy] = React.useState(false);
  const [err, setErr] = React.useState("");

  const submit = async () => {
    setErr(""); setBusy(true);
    const selected = Object.keys(scopes).filter(k => scopes[k]);
    const expires_in_days = expires === "never" ? 0 : parseInt(expires, 10);
    try {
      if (window.OrchisAPI) {
        const created = await window.OrchisAPI.post("/v1/me/tokens", { name: name.trim(), scopes: selected, expires_in_days });
        onCreate({ name: created.name, value: created.token });
      } else {
        onCreate({ name, value: "orc_pat_demo_offline_token_value" });
      }
    } catch (e) {
      setErr("Could not create token. Try again.");
      setBusy(false);
    }
  };

  const SCOPE_GROUPS = [
    { label: "Repositories", scopes: [
      { id: "repo:read", desc: "Read code, issues, PRs" },
      { id: "repo:write", desc: "Push, comment, merge" },
      { id: "repo:admin", desc: "Manage settings, webhooks" },
    ]},
    { label: "Actions & packages", scopes: [
      { id: "actions:read", desc: "Read CI runs and logs" },
      { id: "packages:write", desc: "Publish packages" },
    ]},
    { label: "Account", scopes: [
      { id: "user:read", desc: "Profile, email, orgs" },
    ]},
  ];

  return (
    <div className="card fade-in" style={{ padding: 18, marginTop: 14, borderColor: "var(--accent-line)" }}>
      <h3 style={{ fontSize: 15, fontWeight: 500, margin: "0 0 14px" }}>New personal access token</h3>

      <div style={{ display: "grid", gridTemplateColumns: "1fr 220px", gap: 14, marginBottom: 16 }}>
        <Field label="Name" hint="What is this token for?">
          <input className="input" value={name} onChange={e => setName(e.target.value)} placeholder="e.g. macbook — atlas dev" autoFocus />
        </Field>
        <Field label="Expires">
          <select className="input" value={expires} onChange={e => setExpires(e.target.value)}>
            <option value="30">30 days</option>
            <option value="60">60 days</option>
            <option value="90">90 days</option>
            <option value="365">1 year</option>
            <option value="never">No expiration (not recommended)</option>
          </select>
        </Field>
      </div>

      <div style={{ marginBottom: 14 }}>
        <div className="section-title" style={{ marginBottom: 8 }}>Scopes</div>
        <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
          {SCOPE_GROUPS.map(g => (
            <div key={g.label}>
              <div className="muted" style={{ fontSize: 11.5, marginBottom: 6 }}>{g.label}</div>
              <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 6 }}>
                {g.scopes.map(s => (
                  <label key={s.id} className="row" style={{
                    gap: 10, padding: "8px 10px", border: "1px solid var(--line)", borderRadius: 6,
                    cursor: "pointer",
                    background: scopes[s.id] ? "var(--accent-soft)" : "var(--bg-1)",
                    borderColor: scopes[s.id] ? "var(--accent-line)" : "var(--line)",
                  }}>
                    <input type="checkbox" checked={!!scopes[s.id]} onChange={() => setScopes(p => ({ ...p, [s.id]: !p[s.id] }))} />
                    <div style={{ flex: 1 }}>
                      <div className="mono" style={{ fontSize: 12.5 }}>{s.id}</div>
                      <div className="subtle" style={{ fontSize: 11 }}>{s.desc}</div>
                    </div>
                  </label>
                ))}
              </div>
            </div>
          ))}
        </div>
      </div>

      {err ? <div style={{ padding: "8px 12px", border: "1px solid var(--danger)", borderRadius: 6, color: "var(--danger)", fontSize: 12.5, marginBottom: 12 }}>{err}</div> : null}

      <div className="row" style={{ gap: 8 }}>
        <button className="btn" onClick={onCancel}>Cancel</button>
        <span className="spacer" />
        <button className="btn primary" disabled={!name.trim() || busy} onClick={submit}>
          {busy ? "Generating…" : "Generate token"}
        </button>
      </div>
    </div>
  );
}

function SSHPanel({ newKey, setNewKey }) {
  const [name, setName] = React.useState("");
  const [body, setBody] = React.useState("");
  const [busy, setBusy] = React.useState(false);
  const [err, setErr] = React.useState("");
  const [keys, setKeys] = React.useState(null);

  const reload = React.useCallback(() => {
    if (window.OrchisAPI) {
      window.OrchisAPI.get("/v1/me/ssh-keys").then(setKeys).catch(() => setKeys([]));
    }
  }, []);
  React.useEffect(() => { reload(); }, [reload]);

  const list = keys != null ? keys : (window.OrchisAPI ? [] : SSH_KEYS);

  const add = async () => {
    setErr(""); setBusy(true);
    try {
      await window.OrchisAPI.post("/v1/me/ssh-keys", { name: name.trim(), key: body.trim() });
      setName(""); setBody(""); setNewKey(false); reload();
    } catch (e) {
      setErr("Could not add key — check it's a valid public key and isn't already registered.");
    } finally { setBusy(false); }
  };
  const remove = async (id) => {
    try { await window.OrchisAPI.del("/v1/me/ssh-keys/" + id); reload(); } catch (e) {}
  };

  return (
    <>
      <div style={dsStyles.head}>
        <div>
          <h2 style={dsStyles.h2}>SSH keys</h2>
          <p className="muted" style={dsStyles.subtitle}>For pushing and pulling over SSH — one paste, you're set. Clone with <span className="mono">ssh://git@foundry.orchis.ai:2222/&lt;you&gt;/&lt;repo&gt;.git</span></p>
        </div>
        <button className="btn primary" onClick={() => setNewKey(true)}><Icons.Plus size={14} /> Add SSH key</button>
      </div>

      {newKey ? (
        <div className="card fade-in" style={{ padding: 18, marginTop: 14 }}>
          <h3 style={{ fontSize: 15, fontWeight: 500, margin: "0 0 12px" }}>Add SSH key</h3>
          {err ? <div style={{ padding: "8px 12px", border: "1px solid var(--danger)", borderRadius: 6, color: "var(--danger)", fontSize: 12.5, marginBottom: 12 }}>{err}</div> : null}
          <Field label="Title" hint="What machine is this?">
            <input className="input" value={name} onChange={e => setName(e.target.value)} placeholder="e.g. macbook-air" autoFocus />
          </Field>
          <div style={{ height: 12 }} />
          <Field label="Key" hint="Paste your public key (begins with ssh-ed25519 or ssh-rsa)">
            <textarea value={body} onChange={e => setBody(e.target.value)} placeholder="ssh-ed25519 AAAAC3Nz…" style={{
              width: "100%", height: 120,
              padding: 10, border: "1px solid var(--line)", borderRadius: 6,
              fontFamily: "var(--font-mono)", fontSize: 12.5,
              background: "var(--bg-1)", color: "var(--fg)", resize: "vertical",
            }} />
          </Field>
          <div className="row" style={{ gap: 8, marginTop: 14 }}>
            <button className="btn" onClick={() => { setNewKey(false); setErr(""); }}>Cancel</button>
            <span className="spacer" />
            <button className="btn primary" onClick={add} disabled={!name || !body || busy}>{busy ? "Adding…" : "Add key"}</button>
          </div>
        </div>
      ) : null}

      {list.length === 0 ? (
        <div className="card" style={{ marginTop: 14, padding: "20px 18px", color: "var(--fg-3)", fontSize: 13 }}>
          No SSH keys yet. Add one to push over SSH.
        </div>
      ) : (
      <div className="card" style={{ marginTop: 14 }}>
        {list.map((k, i) => (
          <div key={k.id} style={{
            display: "flex", alignItems: "center", gap: 16,
            padding: "14px 18px",
            borderBottom: i < list.length - 1 ? "1px solid var(--line)" : "none",
          }}>
            <Icons.Key size={16} style={{ color: "var(--fg-2)" }} />
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ fontSize: 14, fontWeight: 500, marginBottom: 4 }}>{k.name}</div>
              <div className="mono subtle" style={{ fontSize: 11.5, overflow: "hidden", textOverflow: "ellipsis" }}>{k.fingerprint}</div>
              <div className="subtle" style={{ fontSize: 11.5, marginTop: 4 }}>Added {k.created || k.added || "—"} · Last used {k.lastUsed || "never"}</div>
            </div>
            <button className="btn ghost sm" style={{ color: "var(--danger)" }} onClick={() => remove(k.id)}>Remove</button>
          </div>
        ))}
      </div>
      )}
    </>
  );
}

function WebhooksPanel({ newHook, setNewHook }) {
  const EVENTS = ["push", "pull_request", "issue", "release", "deploy", "comment"];
  const ownedRepos = REPOS.filter(r => r.org === USERS.me.handle);

  const [hooks, setHooks] = React.useState(null);
  const [url, setUrl] = React.useState("");
  const [repo, setRepo] = React.useState(ownedRepos[0] ? ownedRepos[0].id : "");
  const [events, setEvents] = React.useState(["push"]);
  const [secret, setSecret] = React.useState("");
  const [busy, setBusy] = React.useState(false);
  const [err, setErr] = React.useState("");
  const [tested, setTested] = React.useState("");

  const reload = React.useCallback(() => {
    if (window.OrchisAPI) window.OrchisAPI.get("/v1/me/webhooks").then(setHooks).catch(() => setHooks([]));
  }, []);
  React.useEffect(() => { reload(); }, [reload]);
  React.useEffect(() => { if (!repo && ownedRepos[0]) setRepo(ownedRepos[0].id); }, [ownedRepos.length]);

  const list = hooks != null ? hooks : (window.OrchisAPI ? [] : WEBHOOKS);

  const toggleEvent = (e) => setEvents(evs => evs.includes(e) ? evs.filter(x => x !== e) : [...evs, e]);

  const add = async () => {
    setErr("");
    if (!repo) { setErr("Create a repo first — webhooks attach to a repo."); return; }
    if (events.length === 0) { setErr("Pick at least one event."); return; }
    setBusy(true);
    try {
      await window.OrchisAPI.post("/v1/repos/" + repo + "/webhooks", { url: url.trim(), events, secret: secret.trim() });
      setUrl(""); setSecret(""); setEvents(["push"]); setNewHook(false); reload();
    } catch (e) {
      setErr("Could not create webhook — check the URL is http(s) and you own the repo.");
    } finally { setBusy(false); }
  };
  const test = async (w) => {
    setTested("");
    try { await window.OrchisAPI.post("/v1/repos/" + w.repo + "/webhooks/" + w.id + "/test"); setTested(w.id); setTimeout(() => setTested(""), 2500); } catch (e) {}
  };
  const remove = async (w) => {
    try { await window.OrchisAPI.del("/v1/repos/" + w.repo + "/webhooks/" + w.id); reload(); } catch (e) {}
  };

  return (
    <>
      <div style={dsStyles.head}>
        <div>
          <h2 style={dsStyles.h2}>Webhooks</h2>
          <p className="muted" style={dsStyles.subtitle}>Get a signed POST when something happens. Across all your repos in one place. Each delivery carries <span className="mono">X-Orchis-Event</span>, <span className="mono">X-Orchis-Delivery</span>, and (if a secret is set) an <span className="mono">X-Orchis-Signature</span> HMAC.</p>
        </div>
        <button className="btn primary" onClick={() => { setErr(""); setNewHook(true); }}><Icons.Plus size={14} /> New webhook</button>
      </div>

      {newHook ? (
        <div className="card fade-in" style={{ padding: 18, marginTop: 14 }}>
          <h3 style={{ fontSize: 15, fontWeight: 500, margin: "0 0 12px" }}>New webhook</h3>
          {err ? <div style={{ padding: "8px 12px", border: "1px solid var(--danger)", borderRadius: 6, color: "var(--danger)", fontSize: 12.5, marginBottom: 12 }}>{err}</div> : null}
          <Field label="Payload URL">
            <input className="input" value={url} onChange={e => setUrl(e.target.value)} placeholder="https://hooks.example.com/orchis" autoFocus />
          </Field>
          <div style={{ height: 12 }} />
          <Field label="Repo">
            {ownedRepos.length === 0 ? (
              <div className="subtle" style={{ fontSize: 12.5 }}>You don't own any repos yet. Create one first.</div>
            ) : (
              <select className="input" value={repo} onChange={e => setRepo(e.target.value)}>
                {ownedRepos.map(r => <option key={r.id} value={r.id}>{r.id}</option>)}
              </select>
            )}
          </Field>
          <div style={{ height: 12 }} />
          <Field label="Secret" hint="Optional — used to HMAC-sign each payload">
            <input className="input" value={secret} onChange={e => setSecret(e.target.value)} placeholder="(optional signing secret)" />
          </Field>
          <div style={{ height: 12 }} />
          <Field label="Events">
            <div className="row" style={{ gap: 6, flexWrap: "wrap" }}>
              {EVENTS.map(e => (
                <label key={e} className="chip" style={{ cursor: "pointer", padding: "0 10px", height: 24 }}>
                  <input type="checkbox" checked={events.includes(e)} onChange={() => toggleEvent(e)} style={{ marginRight: 6 }} />{e}
                </label>
              ))}
            </div>
          </Field>
          <div className="row" style={{ gap: 8, marginTop: 14 }}>
            <button className="btn" onClick={() => { setNewHook(false); setErr(""); }}>Cancel</button>
            <span className="spacer" />
            <button className="btn primary" onClick={add} disabled={!url || !repo || busy}>{busy ? "Adding…" : "Add webhook"}</button>
          </div>
        </div>
      ) : null}

      {list.length === 0 ? (
        <div className="card" style={{ marginTop: 14, padding: "20px 18px", color: "var(--fg-3)", fontSize: 13 }}>
          No webhooks yet. Add one to get a POST on push, PR, and more.
        </div>
      ) : (
      <div className="card" style={{ marginTop: 14 }}>
        {list.map((w, i) => (
          <div key={w.id} style={{
            display: "flex", alignItems: "center", gap: 16,
            padding: "14px 18px",
            borderBottom: i < list.length - 1 ? "1px solid var(--line)" : "none",
          }}>
            <span style={{
              width: 8, height: 8, borderRadius: 999,
              background: w.status === "ok" ? "var(--accent)" : "var(--warn)",
            }} />
            <div style={{ flex: 1, minWidth: 0 }}>
              <div className="mono" style={{ fontSize: 13, marginBottom: 4, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{w.url}</div>
              <div className="row" style={{ gap: 6, flexWrap: "wrap" }}>
                <span className="mono subtle" style={{ fontSize: 11.5 }}>{w.repo}</span>
                {w.events.map(e => <span key={e} className="chip" style={{ height: 18, fontSize: 10.5 }}>{e}</span>)}
              </div>
              <div className="subtle" style={{ fontSize: 11.5, marginTop: 4 }}>Last delivery {w.lastDelivery}</div>
            </div>
            <button className="btn sm" onClick={() => test(w)}>{tested === w.id ? "Sent ✓" : "Test"}</button>
            <button className="btn ghost sm" style={{ color: "var(--danger)" }} onClick={() => remove(w)}>Delete</button>
          </div>
        ))}
      </div>
      )}
    </>
  );
}

// Account — edit your display name + bio. Avatar and email come from sign-in.
function AccountPanel() {
  const [me, setMe] = React.useState(null);
  const [name, setName] = React.useState("");
  const [bio, setBio] = React.useState("");
  const [busy, setBusy] = React.useState(false);
  const [saved, setSaved] = React.useState(false);
  const [err, setErr] = React.useState("");
  const fileRef = React.useRef(null);
  const [avatarBusy, setAvatarBusy] = React.useState(false);

  React.useEffect(() => {
    if (window.OrchisAPI) {
      window.OrchisAPI.get("/v1/me").then(u => { setMe(u); setName(u.name || ""); setBio(u.bio || ""); }).catch(() => {});
    } else if (window.USERS) {
      setMe(USERS.me); setName(USERS.me.name || ""); setBio(USERS.me.bio || "");
    }
  }, []);

  if (!me) return <div className="muted" style={{ padding: 20 }}>Loading…</div>;

  const applyMe = (u) => {
    setMe(u);
    if (window.USERS && USERS.me) { USERS.me.name = u.name; USERS.me.bio = u.bio; USERS.me.initials = u.initials; USERS.me.avatarUrl = u.avatarUrl; }
    window.dispatchEvent(new CustomEvent("orchis:me-changed"));
  };

  const save = async () => {
    setErr(""); setSaved(false); setBusy(true);
    try {
      const u = await window.OrchisAPI.patch("/v1/me", { name: name.trim(), bio: bio.trim() });
      applyMe(u); setSaved(true);
    } catch (e) { setErr("Could not save — name must be 1–80 chars and bio ≤ 280."); }
    finally { setBusy(false); }
  };

  const uploadAvatar = async (file) => {
    if (!file) return;
    setErr(""); setAvatarBusy(true);
    try {
      const fd = new FormData(); fd.append("avatar", file);
      const r = await fetch("/v1/me/avatar", { method: "POST", credentials: "same-origin", body: fd });
      if (!r.ok) throw new Error();
      applyMe(await r.json());
    } catch (e) { setErr("Could not upload — use a png/jpeg/gif/webp under 512 KB."); }
    finally { setAvatarBusy(false); if (fileRef.current) fileRef.current.value = ""; }
  };
  const removeAvatar = async () => {
    setErr("");
    try {
      const r = await fetch("/v1/me/avatar", { method: "DELETE", credentials: "same-origin" });
      if (r.ok) applyMe(await r.json());
    } catch (e) {}
  };

  return (
    <>
      <div style={dsStyles.head}>
        <div>
          <h2 style={dsStyles.h2}>Account</h2>
          <p className="muted" style={dsStyles.subtitle}>Your public identity on this instance. Avatar and email come from your GitHub sign-in.</p>
        </div>
      </div>

      {err ? <div style={{ padding: "8px 12px", border: "1px solid var(--danger)", borderRadius: 6, color: "var(--danger)", fontSize: 12.5, margin: "14px 0" }}>{err}</div> : null}
      {saved ? <div style={{ padding: "8px 12px", border: "1px solid var(--accent-line)", background: "var(--accent-soft)", borderRadius: 6, color: "var(--accent)", fontSize: 12.5, margin: "14px 0" }}>Saved.</div> : null}

      <div className="card" style={{ padding: 18, marginTop: 14, display: "flex", flexDirection: "column", gap: 16 }}>
        <div className="row" style={{ gap: 14 }}>
          {me.avatarUrl
            ? <img src={me.avatarUrl} alt="" style={{ width: 56, height: 56, borderRadius: 999, border: "1px solid var(--line)", objectFit: "cover" }} />
            : <span className="avatar" style={{ background: me.color, width: 56, height: 56, fontSize: 20 }}>{me.initials}</span>}
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ fontSize: 15, fontWeight: 500 }}>{me.handle}</div>
            <div className="subtle" style={{ fontSize: 12.5 }}>{me.email || "no email on file"}</div>
          </div>
          <input ref={fileRef} type="file" accept="image/png,image/jpeg,image/gif,image/webp" style={{ display: "none" }} onChange={e => uploadAvatar(e.target.files && e.target.files[0])} />
          <button className="btn sm" disabled={avatarBusy} onClick={() => fileRef.current && fileRef.current.click()}>{avatarBusy ? "Uploading…" : "Change photo"}</button>
          {me.avatarUrl ? <button className="btn ghost sm" style={{ color: "var(--danger)" }} onClick={removeAvatar}>Remove</button> : null}
        </div>
        <Field label="Display name">
          <input className="input" value={name} onChange={e => { setName(e.target.value); setSaved(false); }} maxLength={80} />
        </Field>
        <Field label="Bio" hint="A short line about you (≤ 280 chars). Shown on your profile.">
          <textarea value={bio} onChange={e => { setBio(e.target.value); setSaved(false); }} maxLength={280} placeholder="What you work on…" style={{
            width: "100%", height: 80, padding: 10, border: "1px solid var(--line)", borderRadius: 6,
            fontFamily: "inherit", fontSize: 13, background: "var(--bg-1)", color: "var(--fg)", resize: "vertical",
          }} />
        </Field>
        <div className="row">
          <span className="spacer" />
          <button className="btn primary" disabled={busy || !name.trim()} onClick={save}>{busy ? "Saving…" : "Save changes"}</button>
        </div>
      </div>
    </>
  );
}

// Security — active sessions: see them, revoke one, sign out everywhere else.
function SecurityPanel() {
  const [sessions, setSessions] = React.useState(null);
  const [busy, setBusy] = React.useState(false);
  const reload = React.useCallback(() => {
    if (window.OrchisAPI) window.OrchisAPI.get("/v1/me/sessions").then(setSessions).catch(() => setSessions([]));
  }, []);
  React.useEffect(() => { reload(); }, [reload]);
  const list = sessions || [];
  const revoke = async (id) => { try { await window.OrchisAPI.del("/v1/me/sessions/" + id); reload(); } catch (e) {} };
  const revokeOthers = async () => {
    setBusy(true);
    try { await window.OrchisAPI.post("/v1/me/sessions/revoke-others"); reload(); } catch (e) {} finally { setBusy(false); }
  };
  const others = list.filter(s => !s.current).length;

  return (
    <>
      <div style={dsStyles.head}>
        <div>
          <h2 style={dsStyles.h2}>Security</h2>
          <p className="muted" style={dsStyles.subtitle}>Active browser sessions on your account. Revoke any you don't recognize.</p>
        </div>
        {others > 0 ? <button className="btn" disabled={busy} onClick={revokeOthers}>{busy ? "Signing out…" : "Sign out other sessions"}</button> : null}
      </div>

      {sessions == null ? <div className="muted" style={{ padding: 20 }}>Loading…</div> : (
      <div className="card" style={{ marginTop: 14 }}>
        {list.length === 0 ? (
          <div style={{ padding: "18px", color: "var(--fg-3)", fontSize: 13 }}>No active sessions.</div>
        ) : list.map((s, i) => (
          <div key={s.id} style={{ display: "flex", alignItems: "center", gap: 14, padding: "14px 18px", borderBottom: i < list.length - 1 ? "1px solid var(--line)" : "none" }}>
            <Icons.Check size={16} style={{ color: s.current ? "var(--accent)" : "var(--fg-3)" }} />
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ fontSize: 13.5, fontWeight: 500 }}>
                Browser session {s.current ? <span className="chip accent" style={{ height: 18, fontSize: 10.5, marginLeft: 6 }}>this device</span> : null}
              </div>
              <div className="subtle" style={{ fontSize: 11.5, marginTop: 3 }}>Started {s.created} · last active {s.lastSeen}</div>
            </div>
            {s.current ? null : <button className="btn ghost sm" style={{ color: "var(--danger)" }} onClick={() => revoke(s.id)}>Revoke</button>}
          </div>
        ))}
      </div>
      )}
    </>
  );
}

// Scanner — per-user, BYO endpoint. Drives the orchis-scan PR check.
function ScannerPanel() {
  const [cfg, setCfg] = React.useState(null);
  const [saved, setSaved] = React.useState(false);
  const [err, setErr] = React.useState("");

  React.useEffect(() => {
    if (window.OrchisAPI) {
      window.OrchisAPI.get("/v1/me/scanner")
        .then(c => setCfg({ ...c, apiKey: "" }))
        .catch(() => setCfg({ enabled: false, provider: "anthropic", baseUrl: "", model: "", hasKey: false, apiKey: "", contextBudget: 8000, maxOutputTokens: 2048, pruneGlobs: "" }));
    } else {
      setCfg({ enabled: false, provider: "anthropic", baseUrl: "", model: "", hasKey: false, apiKey: "", contextBudget: 8000, maxOutputTokens: 2048, pruneGlobs: "" });
    }
  }, []);

  if (!cfg) return <div className="muted" style={{ padding: 20 }}>Loading…</div>;

  const set = (k, v) => { setCfg(c => ({ ...c, [k]: v })); setSaved(false); };
  const save = async () => {
    setErr(""); setSaved(false);
    try {
      const body = {
        enabled: cfg.enabled, provider: cfg.provider, baseUrl: cfg.baseUrl, model: cfg.model,
        contextBudget: Number(cfg.contextBudget) || 8000,
        maxOutputTokens: Number(cfg.maxOutputTokens) || 2048,
        pruneGlobs: cfg.pruneGlobs || "",
      };
      if (cfg.apiKey) body.apiKey = cfg.apiKey;
      const res = await window.OrchisAPI.put("/v1/me/scanner", body);
      setCfg({ ...res, apiKey: "" });
      setSaved(true);
    } catch (e) { setErr("Could not save. Check the fields and try again."); }
  };

  const isAnthropic = cfg.provider === "anthropic";

  return (
    <>
      <div style={dsStyles.head}>
        <div>
          <h2 style={dsStyles.h2}>Your model (scanner + AI assist)</h2>
          <p className="muted" style={dsStyles.subtitle}>
            One model config, used two ways: it powers the <span className="mono">orchis-scan</span> supply-chain check on
            PRs in repos you own, and the in-PR <strong>AI assist</strong> (Summarize / Explain diff / Draft description).
            Use Claude, or point at any OpenAI-compatible endpoint you host (Ollama, LM Studio, vLLM, llama.cpp).
            The "enable scanning" toggle below only controls the automatic scan; AI assist works whenever a model is set.
          </p>
        </div>
      </div>

      {err ? <div style={{ padding: "8px 12px", border: "1px solid var(--danger)", borderRadius: 6, color: "var(--danger)", fontSize: 12.5, margin: "14px 0" }}>{err}</div> : null}
      {saved ? <div style={{ padding: "8px 12px", border: "1px solid var(--accent-line)", background: "var(--accent-soft)", borderRadius: 6, color: "var(--accent)", fontSize: 12.5, margin: "14px 0" }}>Saved.</div> : null}

      <div className="card" style={{ padding: 18, marginTop: 14, display: "flex", flexDirection: "column", gap: 16 }}>
        <label className="row" style={{ gap: 10, cursor: "pointer" }}>
          <input type="checkbox" checked={cfg.enabled} onChange={e => set("enabled", e.target.checked)} />
          <span style={{ fontSize: 14, fontWeight: 500 }}>Enable scanning on my repos' PRs</span>
        </label>

        <Field label="Provider">
          <select className="input" value={cfg.provider} onChange={e => set("provider", e.target.value)}>
            <option value="anthropic">Anthropic (Claude Sonnet / Opus)</option>
            <option value="openai_compatible">OpenAI-compatible endpoint (Ollama / LM Studio / vLLM / OpenAI / …)</option>
          </select>
        </Field>

        {!isAnthropic ? (
          <Field label="Base URL" hint="OpenAI-compatible base, e.g. http://your-host:11434/v1 (Ollama) or https://api.openai.com/v1">
            <input className="input" value={cfg.baseUrl} onChange={e => set("baseUrl", e.target.value)} placeholder="http://localhost:11434/v1" />
          </Field>
        ) : null}

        <Field label="Model" hint={isAnthropic ? "e.g. claude-sonnet-4-5 or claude-opus-4-1 (blank = sonnet)" : "e.g. qwen2.5-coder:14b, llama3.1, gpt-4o-mini"}>
          <input className="input" value={cfg.model} onChange={e => set("model", e.target.value)} placeholder={isAnthropic ? "claude-sonnet-4-5" : "qwen2.5-coder"} />
        </Field>

        <Field label="API key" hint={cfg.hasKey ? "A key is saved. Leave blank to keep it; type to replace." : (isAnthropic ? "Your Anthropic API key (sk-ant-…)" : "Optional — many local servers need no key")}>
          <input className="input" type="password" value={cfg.apiKey} onChange={e => set("apiKey", e.target.value)} placeholder={cfg.hasKey ? "•••••••• (saved)" : ""} autoComplete="off" />
        </Field>

        <div style={{ borderTop: "1px solid var(--line)", paddingTop: 14 }}>
          <div className="section-title" style={{ marginBottom: 4 }}>Context budget</div>
          <p className="subtle" style={{ fontSize: 11.5, margin: "0 0 10px" }}>
            Cap how much context is fed to the model — protects local VRAM and cloud cost. Pruning strips
            dependencies, build output, and lockfiles before anything reaches the model.
          </p>
          <Field label={`Input budget — ~${Number(cfg.contextBudget).toLocaleString()} tokens`} hint="Approximate cap on context sent per request (1k–200k).">
            <input type="range" min="1000" max="64000" step="1000" value={cfg.contextBudget}
              onChange={e => set("contextBudget", e.target.value)} style={{ width: "100%" }} />
          </Field>
          <Field label="Max output tokens" hint="Generation cap passed to the provider (256–32000).">
            <input className="input" type="number" min="256" max="32000" step="256" value={cfg.maxOutputTokens}
              onChange={e => set("maxOutputTokens", e.target.value)} />
          </Field>
          <Field label="Extra prune globs" hint="Newline- or comma-separated path globs to also exclude, e.g. *.snap, generated/*, *.pb.go">
            <textarea className="input" rows={2} value={cfg.pruneGlobs}
              onChange={e => set("pruneGlobs", e.target.value)} placeholder="*.snap, generated/*" style={{ resize: "vertical", fontFamily: "var(--font-mono)" }} />
          </Field>
        </div>

        <div className="row">
          <span className="subtle" style={{ fontSize: 11.5 }}>Your key is stored server-side and never shown back to the browser.</span>
          <span className="spacer" />
          <button className="btn primary" onClick={save}>Save scanner</button>
        </div>
      </div>

      <p className="subtle" style={{ marginTop: 14, fontSize: 12 }}>
        <Icons.Bolt size={11} style={{ verticalAlign: "-1px", marginRight: 4 }} />
        On each PR open or push, the scanner reviews the diff and posts a result. It never blocks merges by default —
        it's a visible check you can act on.
      </p>
    </>
  );
}

function AppsPanel() {
  // OAuth-app authorization isn't part of the platform yet, so there's no
  // /v1/me/oauth-apps endpoint. Show an honest empty state when running against
  // a real backend; the sample rows only appear in the offline prototype.
  const demo = [
    { name: "kelp-deploy-bot", scopes: ["repo:read", "actions:read"], by: "kelp", last: "today" },
    { name: "atlas-dashboard", scopes: ["repo:read"], by: "open-strata", last: "2 weeks ago" },
  ];
  const apps = window.OrchisAPI ? [] : demo;
  return (
    <div>
      <h2 style={dsStyles.h2}>OAuth apps</h2>
      <p className="muted" style={dsStyles.subtitle}>Apps you've authorized to act on your behalf.</p>
      {apps.length === 0 ? (
        <div className="card" style={{ marginTop: 14, padding: "20px 18px", color: "var(--fg-3)", fontSize: 13 }}>
          No authorized OAuth apps. Apps you grant access to will appear here.
        </div>
      ) : (
      <div className="card" style={{ marginTop: 14 }}>
        {apps.map((a, i) => (
          <div key={a.name} style={{ display: "flex", alignItems: "center", gap: 16, padding: "14px 18px", borderBottom: i < apps.length - 1 ? "1px solid var(--line)" : "none" }}>
            <div style={{ width: 32, height: 32, borderRadius: 8, background: "var(--bg-2)", display: "flex", alignItems: "center", justifyContent: "center" }}>
              <Icons.Bolt size={14} />
            </div>
            <div style={{ flex: 1 }}>
              <div style={{ fontSize: 14, fontWeight: 500 }}>{a.name}</div>
              <div className="subtle" style={{ fontSize: 11.5 }}>by {a.by} · last used {a.last}</div>
              <div className="row" style={{ gap: 6, marginTop: 6 }}>
                {a.scopes.map(s => <span key={s} className="mono chip" style={{ fontSize: 10.5 }}>{s}</span>)}
              </div>
            </div>
          </div>
        ))}
      </div>
      )}
    </div>
  );
}

function PreferencesPanel() {
  // Real, persisted preferences only. These map to backend pref keys
  // (internal/api/preferences.go boolPrefs) and are saved to the account via
  // PATCH /v1/me/preferences. Both default to on when unset (opt-out).
  const [prefs, setPrefs] = React.useState(null);
  React.useEffect(() => {
    if (window.OrchisAPI) {
      window.OrchisAPI.get("/v1/me/preferences").then(p => setPrefs(p || {})).catch(() => setPrefs({}));
    } else { setPrefs({}); }
  }, []);
  const set = (key, val) => {
    setPrefs(p => ({ ...p, [key]: val }));
    if (window.OrchisAPI) window.OrchisAPI.patch("/v1/me/preferences", { [key]: val }).catch(() => {});
  };
  if (!prefs) return <div className="muted" style={{ padding: 20 }}>Loading…</div>;
  const on = (k) => prefs[k] !== false; // unset ⇒ enabled

  return (
    <div>
      <h2 style={dsStyles.h2}>Preferences</h2>
      <p className="muted" style={dsStyles.subtitle}>Tune Orchis to your workflow. Changes save to your account.</p>
      <div className="card" style={{ marginTop: 14, padding: 4 }}>
        <PrefRow label="Show keyboard hints (⌘K / split tip)" value={on("showSplitTip")} onChange={v => set("showSplitTip", v)} />
        <PrefRow label="AI chat sidebar in repositories" value={on("aiChat")} onChange={v => set("aiChat", v)} last />
      </div>
      <p className="subtle" style={{ marginTop: 12, fontSize: 11.5 }}>Theme, accent, density, and font live in the Tweaks panel and also sync to your account.</p>
    </div>
  );
}

function PrefRow({ label, value, onChange, last }) {
  return (
    <div style={{ display: "flex", alignItems: "center", padding: "10px 14px", borderBottom: last ? "none" : "1px solid var(--line)" }}>
      <span style={{ flex: 1, fontSize: 13 }}>{label}</span>
      <button onClick={() => onChange(!value)} aria-pressed={value} style={{
        width: 36, height: 20, borderRadius: 999,
        border: "1px solid " + (value ? "var(--accent-line)" : "var(--line-strong)"),
        background: value ? "var(--accent)" : "var(--bg-2)",
        cursor: "pointer", padding: 0,
        position: "relative",
      }}>
        <span style={{
          position: "absolute", top: 1, left: value ? 17 : 1,
          width: 16, height: 16, borderRadius: 999,
          background: value ? "var(--accent-fg)" : "var(--fg-2)",
          transition: "left 120ms",
        }} />
      </button>
    </div>
  );
}

function Field({ label, hint, children }) {
  return (
    <div>
      <div className="row" style={{ gap: 8, marginBottom: 6 }}>
        <label style={{ fontSize: 12.5, fontWeight: 500 }}>{label}</label>
        {hint ? <span className="subtle" style={{ fontSize: 11.5 }}>{hint}</span> : null}
      </div>
      {children}
    </div>
  );
}

const dsStyles = {
  page: { maxWidth: 1080, margin: "0 auto", padding: "36px 36px 80px" },
  layout: { display: "grid", gridTemplateColumns: "220px 1fr", gap: 32 },
  sideTabs: { display: "flex", flexDirection: "column", gap: 2 },
  sideTab: {
    display: "flex", alignItems: "center", gap: 10,
    height: 34, padding: "0 10px",
    background: "transparent", borderWidth: 1, borderStyle: "solid", borderColor: "transparent",
    borderRadius: 6, cursor: "pointer", color: "var(--fg-1)",
    fontSize: 13, font: "inherit", fontWeight: 450,
    transition: "background 80ms, color 80ms",
  },
  sideTabActive: {
    background: "var(--accent-soft)", color: "var(--accent)", borderColor: "var(--accent-line)", fontWeight: 500,
  },
  main: { minWidth: 0 },
  head: { display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 12, flexWrap: "wrap" },
  h2: { fontSize: 18, fontWeight: 500, margin: "0 0 4px", letterSpacing: "-0.01em" },
  subtitle: { margin: 0, fontSize: 13 },
};

window.DevSettingsView = DevSettingsView;
