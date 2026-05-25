// Repo view — IDE-like: file tree (left) + main panel (code/readme) with optional split.
function RepoView({ repoId, file, setRoute, openSplit, splitOpen, splitContent, closeSplit }) {
  const repo = REPOS.find(r => r.id === repoId) || REPOS[0] || { id: repoId, name: repoId, org: "", description: "", languageColor: "var(--fg-3)", visibility: "private", stars: 0 };
  const [active, setActive] = React.useState(file || "README.md");
  const [tab, setTab] = React.useState("code");      // code | readme | activity
  const [expanded, setExpanded] = React.useState(() => collectInitialExpanded(FILE_TREE));
  const [treeWidth, setTreeWidth] = React.useState(260);
  const [mainSplit, setMainSplit] = React.useState(0.55);

  // Files the user pinned into LLM context (persisted per-repo in localStorage).
  const [pins, setPins] = React.useState(() => loadPins(repoId));
  React.useEffect(() => { setPins(loadPins(repoId)); }, [repoId]);
  const togglePinFile = React.useCallback((path) => {
    setPins(prev => {
      const next = prev.includes(path) ? prev.filter(x => x !== path) : [...prev, path];
      savePins(repoId, next);
      return next;
    });
  }, [repoId]);

  // Live file tree for this repo (falls back to the mock when offline).
  const [tree, setTree] = React.useState(null);
  React.useEffect(() => {
    let cancelled = false;
    setTree(null);
    if (window.OrchisAPI && repoId) {
      window.OrchisAPI.get(`/v1/repos/${repoId}/tree`)
        .then(t => { if (!cancelled) setTree(t || []); })
        .catch(() => { if (!cancelled) setTree(null); });
    }
    return () => { cancelled = true; };
  }, [repoId]);
  const treeNodes = tree || FILE_TREE;
  // Manage (settings + collaborators) is available to the owner or an admin
  // collaborator. repo.role comes from the API (owner|admin|write|read|"").
  const isOwner = (repo.role === "owner" || repo.role === "admin")
    || !!(window.USERS && USERS.me && repo.org === USERS.me.handle);
  const [showSettings, setShowSettings] = React.useState(false);

  // Live star/pin state (seeded from the loaded repo, refreshed from the API).
  const [meta, setMeta] = React.useState({ stars: repo.stars || 0, starred: !!repo.starred, pinned: !!repo.pinned });
  React.useEffect(() => {
    if (window.OrchisAPI && repoId) {
      window.OrchisAPI.get(`/v1/repos/${repoId}`)
        .then(r => setMeta({ stars: r.stars || 0, starred: !!r.starred, pinned: !!r.pinned }))
        .catch(() => {});
    }
  }, [repoId]);
  const toggleStar = async () => {
    if (!window.OrchisAPI) return;
    const next = !meta.starred;
    setMeta(m => ({ ...m, starred: next, stars: Math.max(0, m.stars + (next ? 1 : -1)) }));
    try { await (next ? window.OrchisAPI.put(`/v1/me/stars/${repoId}`) : window.OrchisAPI.del(`/v1/me/stars/${repoId}`)); } catch (e) {}
  };
  const togglePin = async () => {
    if (!window.OrchisAPI) return;
    const next = !meta.pinned;
    setMeta(m => ({ ...m, pinned: next }));
    try {
      await (next ? window.OrchisAPI.put(`/v1/me/pinned/${repoId}`) : window.OrchisAPI.del(`/v1/me/pinned/${repoId}`));
      window.dispatchEvent(new CustomEvent("orchis:repos-changed"));
    } catch (e) {}
  };

  React.useEffect(() => {
    if (file) setActive(file);
  }, [file]);

  const toggle = (path) => setExpanded(e => ({ ...e, [path]: !e[path] }));

  const onPickFile = (node) => {
    setActive(node.path);
    setTab("code");
  };

  const startTreeResize = (e) => {
    e.preventDefault();
    const startX = e.clientX;
    const startW = treeWidth;
    const move = (ev) => {
      const w = Math.max(180, Math.min(420, startW + (ev.clientX - startX)));
      setTreeWidth(w);
    };
    const up = () => {
      window.removeEventListener("mousemove", move);
      window.removeEventListener("mouseup", up);
    };
    window.addEventListener("mousemove", move);
    window.addEventListener("mouseup", up);
  };

  const startMainResize = (e) => {
    e.preventDefault();
    const containerEl = e.currentTarget.parentElement;
    const rect = containerEl.getBoundingClientRect();
    const move = (ev) => {
      const p = Math.max(0.25, Math.min(0.8, (ev.clientX - rect.left) / rect.width));
      setMainSplit(p);
    };
    const up = () => {
      window.removeEventListener("mousemove", move);
      window.removeEventListener("mouseup", up);
    };
    window.addEventListener("mousemove", move);
    window.addEventListener("mouseup", up);
  };

  return (
    <div style={repoStyles.shell}>
      <RepoHeader repo={repo} setRoute={setRoute} openSplit={openSplit} isOwner={isOwner} onSettings={() => setShowSettings(true)} meta={meta} toggleStar={toggleStar} togglePin={togglePin} />
      {showSettings ? <RepoSettingsModal repo={repo} onClose={() => setShowSettings(false)} setRoute={setRoute} /> : null}

      <div style={repoStyles.body}>
        {/* File tree */}
        <div style={{ ...repoStyles.tree, width: treeWidth }}>
          <div style={repoStyles.treeHead}>
            <span className="row" style={{ gap: 6, color: "var(--fg-1)" }}>
              <Icons.Branch size={13} />
              <span style={{ fontFamily: "var(--font-mono)", fontSize: 12 }}>main</span>
              <Icons.ChevronDown size={11} style={{ color: "var(--fg-3)" }} />
            </span>
            <button className="btn ghost icon sm" title="Find in files" onClick={() => setRoute({ view: "search", repo: repoId })}><Icons.Search size={12} /></button>
          </div>
          <div style={repoStyles.treeScroll}>
            {treeNodes.length === 0
              ? <div style={{ padding: "16px 12px", color: "var(--fg-3)", fontSize: 12 }}>Empty repository. Push some code to get started.</div>
              : <FileTree nodes={treeNodes} expanded={expanded} active={active} toggle={toggle} onPick={onPickFile} depth={0} pins={pins} onPinPath={togglePinFile} />}
          </div>
        </div>

        {/* Tree resizer */}
        <div style={repoStyles.resizer} onMouseDown={startTreeResize} />

        {/* Main panel — possibly split */}
        <div style={repoStyles.mainArea}>
          {splitOpen ? (
            <>
              <div style={{ width: `calc(${mainSplit * 100}% - 3px)`, height: "100%", display: "flex", flexDirection: "column", overflow: "hidden" }}>
                <RepoMainPanel repo={repo} repoId={repoId} active={active} tab={tab} setTab={setTab} setActive={onPickFile} setRoute={setRoute} />
              </div>
              <div style={repoStyles.mainResizer} onMouseDown={startMainResize} />
              <div style={{ flex: 1, height: "100%", display: "flex", flexDirection: "column", overflow: "hidden", borderLeft: "1px solid var(--line)" }}>
                <SplitPanel content={splitContent} close={closeSplit} setRoute={setRoute} />
              </div>
            </>
          ) : (
            <div style={{ flex: 1, height: "100%", display: "flex", flexDirection: "column", overflow: "hidden" }}>
              <RepoMainPanel repo={repo} repoId={repoId} active={active} tab={tab} setTab={setTab} setActive={onPickFile} setRoute={setRoute} />
            </div>
          )}
        </div>

        {/* AI chat sidebar — toggleable; dormant strip when disabled. */}
        <ChatDock repoId={repoId} repo={repo} pins={pins} onUnpin={togglePinFile} />
      </div>
    </div>
  );
}

// Pin persistence — per-repo, client-side (localStorage). Survives reloads;
// sent with every chat request so pinned files stay in the model's context.
function loadPins(repoId) {
  try { return JSON.parse(localStorage.getItem("orchis:pins:" + repoId) || "[]"); }
  catch (e) { return []; }
}
function savePins(repoId, pins) {
  try { localStorage.setItem("orchis:pins:" + repoId, JSON.stringify(pins)); } catch (e) {}
}

function collectInitialExpanded(nodes, acc = {}) {
  nodes.forEach(n => {
    if (n.type === "dir" && n.expanded) {
      acc[n.path] = true;
      if (n.children) collectInitialExpanded(n.children, acc);
    }
  });
  return acc;
}

function FileTree({ nodes, expanded, active, toggle, onPick, depth, pins, onPinPath }) {
  const pinned = pins || [];
  return nodes.map(n => {
    const isOpen = expanded[n.path];
    const isActive = n.type === "file" && active === n.path;
    const isPinned = pinned.includes(n.path);
    return (
      <React.Fragment key={n.path}>
        <button
          style={{
            ...repoStyles.treeRow,
            ...(isActive ? repoStyles.treeRowActive : {}),
            paddingLeft: 6 + depth * 14,
          }}
          onClick={() => n.type === "dir" ? toggle(n.path) : onPick(n)}
          onContextMenu={onPinPath && n.type === "file" ? (e) => { e.preventDefault(); onPinPath(n.path); } : undefined}
          title={n.type === "file" && onPinPath ? "Right-click to pin/unpin for AI context" : undefined}
        >
          {n.type === "dir"
            ? <Icons.Chevron size={11} style={{ color: "var(--fg-3)", transform: isOpen ? "rotate(90deg)" : "none", transition: "transform 80ms" }} />
            : <span style={{ width: 11 }} />
          }
          {n.type === "dir"
            ? <Icons.Folder size={13} style={{ color: "var(--fg-2)" }} />
            : <span style={{ ...repoStyles.fileGlyph, background: langColor(n.lang) }} />
          }
          <span style={{ fontFamily: "var(--font-mono)", fontSize: 12, color: isActive ? "var(--fg)" : "var(--fg-1)", flex: 1 }}>{n.name}</span>
          {isPinned ? <Icons.Pin size={11} style={{ color: "var(--accent)", flexShrink: 0 }} /> : null}
        </button>
        {n.type === "dir" && isOpen && n.children
          ? <FileTree nodes={n.children} expanded={expanded} active={active} toggle={toggle} onPick={onPick} depth={depth + 1} pins={pins} onPinPath={onPinPath} />
          : null}
      </React.Fragment>
    );
  });
}

function langColor(lang) {
  return {
    rust: "oklch(60% 0.14 30)",
    typescript: "oklch(62% 0.12 240)",
    yaml: "oklch(65% 0.10 90)",
    toml: "oklch(55% 0.08 30)",
    markdown: "oklch(60% 0.06 250)",
    go: "oklch(62% 0.12 200)",
    text: "var(--fg-3)",
  }[lang] || "var(--fg-3)";
}

// ChatDock — the conversational AI sidebar. Per-user toggleable: when disabled
// it collapses to a thin dormant strip (a one-click "Enable" away). When
// enabled, it's a slide-out panel that chats against the repo, using the files
// the user pinned in the tree as context.
function ChatDock({ repoId, repo, pins, onUnpin }) {
  const [enabled, setEnabled] = React.useState(null);   // null = loading
  const [open, setOpen] = React.useState(false);
  const [messages, setMessages] = React.useState([]);   // {role, content}
  const [chatId, setChatId] = React.useState(0);
  const [input, setInput] = React.useState("");
  const [busy, setBusy] = React.useState(false);
  const [err, setErr] = React.useState("");

  React.useEffect(() => {
    if (window.OrchisAPI) {
      window.OrchisAPI.get("/v1/me/preferences")
        .then(p => setEnabled(p.aiChat !== false))
        .catch(() => setEnabled(true));
    } else { setEnabled(false); }
  }, []);

  const setPref = async (on) => {
    setEnabled(on);
    if (!on) setOpen(false);
    try { await window.OrchisAPI.patch("/v1/me/preferences", { aiChat: on }); } catch (e) {}
  };

  const send = async () => {
    const text = input.trim();
    if (!text || busy) return;
    const next = [...messages, { role: "user", content: text }];
    setMessages(next); setInput(""); setBusy(true); setErr("");
    try {
      const res = await window.OrchisAPI.post(`/v1/repos/${repoId}/chat`, { chatId, messages: next, pinned: pins || [] });
      if (res.chatId) setChatId(res.chatId);
      setMessages(m => [...m, { role: "assistant", content: res.message }]);
    } catch (e) {
      setErr("The model call failed. Configure a model in Developer settings → Scanner.");
    } finally { setBusy(false); }
  };

  if (enabled === null) return null;

  // Dormant strip — disabled, or enabled-but-closed.
  if (!enabled || !open) {
    return (
      <div style={{ width: 40, borderLeft: "1px solid var(--line)", display: "flex", flexDirection: "column", alignItems: "center", paddingTop: 12, gap: 10, background: "var(--bg-1)" }}>
        <button className="btn ghost icon sm" title={enabled ? "Open AI chat" : "AI chat is off"}
          onClick={() => enabled ? setOpen(true) : setPref(true)} style={{ opacity: enabled ? 1 : 0.5 }}>
          <Icons.Bolt size={14} />
        </button>
        <span style={{ writingMode: "vertical-rl", fontSize: 10.5, color: "var(--fg-3)", letterSpacing: "0.04em" }}>
          {enabled ? "AI chat" : "AI chat · off"}
        </span>
        {!enabled ? (
          <button className="btn ghost sm" style={{ writingMode: "vertical-rl", fontSize: 10, height: "auto", padding: "6px 2px" }} onClick={() => setPref(true)}>Enable</button>
        ) : null}
      </div>
    );
  }

  return (
    <div style={{ width: 340, borderLeft: "1px solid var(--line)", display: "flex", flexDirection: "column", background: "var(--bg-1)", minHeight: 0 }}>
      <div className="row" style={{ gap: 8, padding: "8px 12px", borderBottom: "1px solid var(--line)" }}>
        <Icons.Bolt size={13} style={{ color: "var(--accent)" }} />
        <span style={{ fontSize: 12.5, fontWeight: 600 }}>AI chat</span>
        <span className="spacer" />
        <button className="btn ghost icon sm" title="Turn off (collapses to a strip)" onClick={() => setPref(false)}><Icons.Settings size={12} /></button>
        <button className="btn ghost icon sm" title="Collapse" onClick={() => setOpen(false)}><Icons.Close size={12} /></button>
      </div>

      {pins && pins.length ? (
        <div style={{ padding: "6px 10px", borderBottom: "1px solid var(--line)", display: "flex", flexWrap: "wrap", gap: 5 }}>
          <span className="subtle" style={{ fontSize: 10.5, width: "100%", marginBottom: 2 }}>Pinned context ({pins.length})</span>
          {pins.map(p => (
            <span key={p} className="chip" style={{ height: 18, fontSize: 10, gap: 4 }}>
              {p.split("/").pop()}
              <span onClick={() => onUnpin && onUnpin(p)} style={{ cursor: "pointer", color: "var(--fg-3)" }} title={"Unpin " + p}>×</span>
            </span>
          ))}
        </div>
      ) : (
        <div className="subtle" style={{ padding: "8px 12px", fontSize: 11, borderBottom: "1px solid var(--line)" }}>
          Tip: right-click a file in the tree to pin it as context.
        </div>
      )}

      <div style={{ flex: 1, overflowY: "auto", padding: 12, display: "flex", flexDirection: "column", gap: 10, minHeight: 0 }}>
        {messages.length === 0 ? <div className="subtle" style={{ fontSize: 12 }}>Ask about this repository. Pinned files are sent as context.</div> : null}
        {messages.map((m, i) => (
          <div key={i} style={{ alignSelf: m.role === "user" ? "flex-end" : "flex-start", maxWidth: "92%",
            background: m.role === "user" ? "var(--accent-soft)" : "var(--bg-2)", border: "1px solid var(--line)",
            borderRadius: 8, padding: "7px 10px", fontSize: 12.5, lineHeight: 1.5 }}>
            {m.role === "assistant" ? <div className="md-mini">{parseMd(m.content)}</div> : m.content}
          </div>
        ))}
        {busy ? <div className="subtle" style={{ fontSize: 12 }}>Thinking…</div> : null}
      </div>

      {err ? <div style={{ padding: "6px 12px", fontSize: 11.5, color: "var(--danger)" }}>{err}</div> : null}
      <div className="row" style={{ gap: 6, padding: 10, borderTop: "1px solid var(--line)" }}>
        <input className="input" value={input} onChange={e => setInput(e.target.value)}
          onKeyDown={e => { if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); send(); } }}
          placeholder="Ask about this repo…" style={{ flex: 1 }} disabled={busy} />
        <button className="btn primary sm" onClick={send} disabled={busy || !input.trim()}>Send</button>
      </div>
    </div>
  );
}

function RepoHeader({ repo, setRoute, openSplit, isOwner, onSettings, meta, toggleStar, togglePin }) {
  const [copied, setCopied] = React.useState(false);
  const cloneURL = (typeof window !== "undefined" ? window.location.origin : "") + "/" + repo.org + "/" + repo.name + ".git";
  const copyClone = () => {
    navigator.clipboard?.writeText(cloneURL).then(() => { setCopied(true); setTimeout(() => setCopied(false), 1500); }).catch(() => {});
  };
  return (
    <div style={repoStyles.header}>
      <div className="row" style={{ gap: 10, minWidth: 0, flex: 1 }}>
        <button className="btn ghost icon sm" onClick={() => setRoute({ view: "home" })} title="Back"><Icons.Chevron size={12} style={{ transform: "rotate(180deg)" }} /></button>
        <span style={{ width: 10, height: 10, borderRadius: 3, background: repo.languageColor, flexShrink: 0 }} />
        <span style={{ fontFamily: "var(--font-mono)", fontSize: 14, fontWeight: 500, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
          <span className="muted">{repo.org}/</span>{repo.name}
        </span>
        {repo.visibility === "private" ? <span className="chip">private</span> : <span className="chip">public</span>}
        <span className="muted" style={{ fontSize: 12.5, marginLeft: 8, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{repo.description}</span>
      </div>

      <div className="row" style={{ gap: 6, flexShrink: 0 }}>
        <button className="btn sm" onClick={() => openSplit({ type: "prs", repo: repo.id })} title="Open PRs in split">
          <Icons.PR size={12} /> PRs <span className="chip" style={{ height: 16, fontSize: 10 }}>3</span>
        </button>
        <button className="btn sm" onClick={() => setRoute({ view: "newpr", repo: repo.id })} title="Open a pull request">
          <Icons.Plus size={12} /> PR
        </button>
        <button className="btn sm" onClick={() => openSplit({ type: "issues", repo: repo.id })}>
          <Icons.Issue size={12} /> Issues
        </button>
        <button className="btn sm" onClick={() => openSplit({ type: "actions", repo: repo.id })}>
          <Icons.Bolt size={12} /> Actions
        </button>
        <span style={{ width: 1, height: 18, background: "var(--line)", margin: "0 4px" }} />
        <button className="btn sm" title={meta && meta.starred ? "Unstar" : "Star"} onClick={toggleStar}
          style={meta && meta.starred ? { color: "var(--accent)", borderColor: "var(--accent-line)", background: "var(--accent-soft)" } : {}}>
          <Icons.Star size={12} /> {((meta ? meta.stars : repo.stars) || 0).toLocaleString()}
        </button>
        <button className="btn sm" title={meta && meta.pinned ? "Unpin from sidebar" : "Pin to sidebar"} onClick={togglePin}
          style={meta && meta.pinned ? { color: "var(--accent)", borderColor: "var(--accent-line)", background: "var(--accent-soft)" } : {}}>
          <Icons.Pin size={12} /> {meta && meta.pinned ? "Pinned" : "Pin"}
        </button>
        {isOwner ? <button className="btn sm" title="Repository settings" onClick={onSettings}><Icons.Settings size={12} /></button> : null}
        <button className="btn primary sm" onClick={copyClone} title={cloneURL}><Icons.Copy size={12} /> {copied ? "Copied!" : "Clone"}</button>
      </div>
    </div>
  );
}

// Owner-only repo settings: rename, description, visibility, default branch, delete.
function RepoSettingsModal({ repo, onClose, setRoute }) {
  const [name, setName] = React.useState(repo.name);
  const [description, setDescription] = React.useState(repo.description || "");
  const [visibility, setVisibility] = React.useState(repo.visibility || "private");
  const [defaultBranch, setDefaultBranch] = React.useState(repo.defaultBranch || "main");
  const [branches, setBranches] = React.useState([]);
  const [busy, setBusy] = React.useState(false);
  const [err, setErr] = React.useState("");
  const [confirmDelete, setConfirmDelete] = React.useState("");

  const [newBranch, setNewBranch] = React.useState("");
  const [branchBusy, setBranchBusy] = React.useState(false);
  const reloadBranches = React.useCallback(() => {
    if (window.OrchisAPI) window.OrchisAPI.get(`/v1/repos/${repo.id}/branches`).then(b => setBranches(b || [])).catch(() => {});
  }, [repo.id]);
  React.useEffect(() => { reloadBranches(); }, [reloadBranches]);

  const createBranch = async () => {
    if (!newBranch.trim()) return;
    setErr(""); setBranchBusy(true);
    try { await window.OrchisAPI.post(`/v1/repos/${repo.id}/branches`, { name: newBranch.trim(), from: defaultBranch }); setNewBranch(""); reloadBranches(); }
    catch (e) { setErr("Could not create branch — the name may be invalid or already exist."); }
    finally { setBranchBusy(false); }
  };
  const deleteBranch = async (b) => {
    setErr("");
    try { await window.OrchisAPI.del(`/v1/repos/${repo.id}/branches/${b}`); reloadBranches(); }
    catch (e) { setErr("Could not delete branch — you can't delete the default branch."); }
  };

  // Collaborators
  const [collabs, setCollabs] = React.useState([]);
  const [collabHandle, setCollabHandle] = React.useState("");
  const [collabRole, setCollabRole] = React.useState("write");
  const reloadCollabs = React.useCallback(() => {
    if (window.OrchisAPI) window.OrchisAPI.get(`/v1/repos/${repo.id}/collaborators`).then(c => setCollabs(c || [])).catch(() => {});
  }, [repo.id]);
  React.useEffect(() => { reloadCollabs(); }, [reloadCollabs]);
  const addCollab = async () => {
    if (!collabHandle.trim()) return;
    setErr("");
    try { await window.OrchisAPI.put(`/v1/repos/${repo.id}/collaborators/${collabHandle.trim()}`, { role: collabRole }); setCollabHandle(""); reloadCollabs(); }
    catch (e) { setErr("Could not add collaborator — check the handle exists."); }
  };
  const removeCollab = async (h) => {
    try { await window.OrchisAPI.del(`/v1/repos/${repo.id}/collaborators/${h}`); reloadCollabs(); } catch (e) {}
  };

  const save = async () => {
    setErr(""); setBusy(true);
    try {
      const updated = await window.OrchisAPI.patch(`/v1/repos/${repo.id}`, {
        name: name.trim(), description, visibility, defaultBranch,
      });
      await window.loadRepos().catch(() => {});
      onClose();
      // Repo id is org/name; if renamed, navigate to the new id.
      if (updated && updated.id && updated.id !== repo.id) setRoute({ view: "repo", repo: updated.id });
    } catch (e) {
      setErr("Could not save — the name may be taken, the default branch may not exist, or the value is invalid.");
      setBusy(false);
    }
  };

  const del = async () => {
    setBusy(true);
    try {
      await window.OrchisAPI.del(`/v1/repos/${repo.id}`);
      await window.loadRepos().catch(() => {});
      onClose();
      setRoute({ view: "home" });
    } catch (e) { setErr("Delete failed."); setBusy(false); }
  };

  return (
    <div onClick={onClose} style={{ position: "fixed", inset: 0, background: "color-mix(in oklab, var(--bg) 40%, transparent)", backdropFilter: "blur(2px)", zIndex: 200, display: "flex", alignItems: "center", justifyContent: "center", padding: 24 }}>
      <div onClick={e => e.stopPropagation()} className="card fade-in" style={{ width: 560, maxWidth: "92vw", maxHeight: "88vh", overflowY: "auto", padding: 22 }}>
        <h2 style={{ fontSize: 18, fontWeight: 500, margin: "0 0 4px" }}>Repository settings</h2>
        <p className="muted" style={{ fontSize: 13, margin: "0 0 18px" }}><span className="mono">{repo.org}/{repo.name}</span></p>

        {err ? <div style={{ padding: "10px 12px", border: "1px solid var(--danger)", borderRadius: 8, color: "var(--danger)", fontSize: 13, marginBottom: 14 }}>{err}</div> : null}

        <div style={{ marginBottom: 14 }}>
          <div className="section-title" style={{ marginBottom: 6 }}>Name</div>
          <input className="input" value={name} onChange={e => setName(e.target.value)} />
        </div>
        <div style={{ marginBottom: 14 }}>
          <div className="section-title" style={{ marginBottom: 6 }}>Description</div>
          <input className="input" value={description} onChange={e => setDescription(e.target.value)} placeholder="What's in this repo?" />
        </div>
        <div style={{ marginBottom: 14 }}>
          <div className="section-title" style={{ marginBottom: 6 }}>Default branch</div>
          {branches.length > 0 ? (
            <select className="input" value={defaultBranch} onChange={e => setDefaultBranch(e.target.value)}>
              {branches.map(b => <option key={b.name} value={b.name}>{b.name}</option>)}
            </select>
          ) : <input className="input" value={defaultBranch} onChange={e => setDefaultBranch(e.target.value)} />}
        </div>
        <div style={{ marginBottom: 18 }}>
          <div className="section-title" style={{ marginBottom: 6 }}>Visibility</div>
          <div className="row" style={{ gap: 8 }}>
            {["private", "internal", "public"].map(v => (
              <label key={v} className="row" style={{ gap: 8, padding: "8px 12px", border: "1px solid var(--line)", borderRadius: 6, cursor: "pointer", flex: 1, background: visibility === v ? "var(--accent-soft)" : "var(--bg-1)", borderColor: visibility === v ? "var(--accent-line)" : "var(--line)" }}>
                <input type="radio" name="vis" checked={visibility === v} onChange={() => setVisibility(v)} />
                <span style={{ textTransform: "capitalize", fontSize: 13 }}>{v}</span>
              </label>
            ))}
          </div>
        </div>

        <div className="row" style={{ gap: 8, marginBottom: 24 }}>
          <button className="btn" onClick={onClose}>Cancel</button>
          <span className="spacer" />
          <button className="btn primary" disabled={busy || !name.trim()} onClick={save}>{busy ? "Saving…" : "Save changes"}</button>
        </div>

        <div style={{ marginBottom: 24 }}>
          <div className="section-title" style={{ marginBottom: 8 }}>Branches</div>
          <div className="card" style={{ overflow: "hidden", marginBottom: 10 }}>
            {branches.length === 0 ? (
              <div style={{ padding: "12px 14px", color: "var(--fg-3)", fontSize: 12.5 }}>No branches yet — push some code.</div>
            ) : branches.map((b, i) => (
              <div key={b.name} className="row" style={{ gap: 10, padding: "9px 14px", borderBottom: i < branches.length - 1 ? "1px solid var(--line)" : "none" }}>
                <Icons.Branch size={13} style={{ color: "var(--fg-2)" }} />
                <span className="mono" style={{ fontSize: 12.5, flex: 1 }}>{b.name}</span>
                {b.name === (repo.defaultBranch || defaultBranch) ? <span className="chip" style={{ height: 18, fontSize: 10 }}>default</span> : (
                  <button className="btn ghost sm" style={{ color: "var(--danger)" }} onClick={() => deleteBranch(b.name)}>Delete</button>
                )}
              </div>
            ))}
          </div>
          <div className="row" style={{ gap: 8 }}>
            <input className="input" value={newBranch} onChange={e => setNewBranch(e.target.value)}
              onKeyDown={e => { if (e.key === "Enter") createBranch(); }}
              placeholder={"new-branch (from " + defaultBranch + ")"} style={{ flex: 1 }} />
            <button className="btn" disabled={!newBranch.trim() || branchBusy} onClick={createBranch}>{branchBusy ? "Creating…" : "Create branch"}</button>
          </div>
        </div>

        <div style={{ marginBottom: 24 }}>
          <div className="section-title" style={{ marginBottom: 8 }}>Collaborators</div>
          <div className="card" style={{ overflow: "hidden", marginBottom: 10 }}>
            <div className="row" style={{ gap: 10, padding: "9px 14px", borderBottom: collabs.length ? "1px solid var(--line)" : "none" }}>
              <span className="avatar" style={{ background: "var(--accent)", width: 22, height: 22, fontSize: 9, color: "var(--accent-fg)" }}>{(repo.org || "?").slice(0, 2).toUpperCase()}</span>
              <span style={{ flex: 1, fontSize: 13 }}>{repo.org} <span className="subtle">(owner)</span></span>
              <span className="chip" style={{ height: 18, fontSize: 10 }}>owner</span>
            </div>
            {collabs.map((c, i) => (
              <div key={c.handle} className="row" style={{ gap: 10, padding: "9px 14px", borderBottom: i < collabs.length - 1 ? "1px solid var(--line)" : "none" }}>
                <span className="avatar" style={{ background: c.color, width: 22, height: 22, fontSize: 9 }}>{c.initials}</span>
                <span style={{ flex: 1, fontSize: 13 }}>{c.handle} <span className="subtle">{c.name}</span></span>
                <span className="chip" style={{ height: 18, fontSize: 10 }}>{c.role}</span>
                <button className="btn ghost sm" style={{ color: "var(--danger)" }} onClick={() => removeCollab(c.handle)}>Remove</button>
              </div>
            ))}
          </div>
          <div className="row" style={{ gap: 8 }}>
            <input className="input" value={collabHandle} onChange={e => setCollabHandle(e.target.value)} placeholder="user handle" style={{ flex: 1 }} onKeyDown={e => { if (e.key === "Enter") addCollab(); }} />
            <select className="input" value={collabRole} onChange={e => setCollabRole(e.target.value)} style={{ width: 110 }}>
              <option value="read">read</option>
              <option value="write">write</option>
              <option value="admin">admin</option>
            </select>
            <button className="btn" disabled={!collabHandle.trim()} onClick={addCollab}>Add</button>
          </div>
        </div>

        <div style={{ border: "1px solid var(--danger)", borderRadius: 8, padding: 14 }}>
          <div style={{ fontWeight: 500, color: "var(--danger)", fontSize: 13, marginBottom: 6 }}>Danger zone</div>
          <p className="muted" style={{ fontSize: 12.5, margin: "0 0 10px" }}>Deleting a repository is permanent — code, PRs, and history are gone. Type <span className="mono">{repo.name}</span> to confirm.</p>
          <div className="row" style={{ gap: 8 }}>
            <input className="input" value={confirmDelete} onChange={e => setConfirmDelete(e.target.value)} placeholder={repo.name} style={{ maxWidth: 240 }} />
            <button className="btn" style={{ borderColor: "var(--danger)", color: "var(--danger)" }} disabled={busy || confirmDelete !== repo.name} onClick={del}>Delete this repository</button>
          </div>
        </div>
      </div>
    </div>
  );
}

function RepoMainPanel({ repo, repoId, active, tab, setTab, setActive, setRoute }) {
  return (
    <>
      <div style={repoStyles.subtabs}>
        <Subtab active={tab === "code"} onClick={() => setTab("code")} icon={<Icons.Code size={12} />}>Code</Subtab>
        <Subtab active={tab === "readme"} onClick={() => setTab("readme")} icon={<Icons.Book size={12} />}>Readme</Subtab>
        <Subtab active={tab === "commits"} onClick={() => setTab("commits")} icon={<Icons.Commit size={12} />}>Commits</Subtab>
        <Subtab active={tab === "proposals"} onClick={() => setTab("proposals")} icon={<Icons.Diff size={12} />}>Proposals</Subtab>
        <Subtab active={tab === "activity"} onClick={() => setTab("activity")} icon={<Icons.Activity size={12} />}>Activity</Subtab>
        <Subtab active={tab === "releases"} onClick={() => setTab("releases")} icon={<Icons.Tag size={12} />}>Releases</Subtab>
      </div>

      <div style={{ flex: 1, minHeight: 0, overflowY: "auto" }}>
        {tab === "code" ? <CodeView repoId={repoId} path={active} repo={repo} setRoute={setRoute} /> : null}
        {tab === "readme" ? <ReadmeView repoId={repoId} /> : null}
        {tab === "commits" ? <RepoCommitsList repoId={repoId} repo={repo} setRoute={setRoute} /> : null}
        {tab === "proposals" ? <ProposalsView repoId={repoId} repo={repo} setRoute={setRoute} /> : null}
        {tab === "activity" ? <RecentActivityView repoId={repoId} /> : null}
        {tab === "releases" ? <ReleasesView repo={repo} repoId={repoId} /> : null}
      </div>
    </>
  );
}

// RepoCommitsList — commit history for the repo's default branch; rows open the
// commit detail view.
function RepoCommitsList({ repoId, repo, setRoute }) {
  const [commits, setCommits] = React.useState(null);
  React.useEffect(() => {
    if (window.OrchisAPI && repoId) window.OrchisAPI.get(`/v1/repos/${repoId}/commits?limit=100`).then(setCommits).catch(() => setCommits([]));
  }, [repoId]);
  const list = commits || [];
  return (
    <div style={{ padding: "20px 24px", maxWidth: 900 }}>
      {commits == null ? <div className="muted" style={{ fontSize: 12.5 }}>Loading…</div> : null}
      {commits != null && list.length === 0 ? <div className="card" style={{ padding: "20px 18px", color: "var(--fg-3)", fontSize: 13 }}>No commits yet.</div> : null}
      {list.length > 0 ? (
        <div className="card" style={{ overflow: "hidden" }}>
          {list.map((c, i) => (
            <button key={c.sha} onClick={() => setRoute({ view: "commit", repo: repoId, sha: c.sha })}
              style={{ display: "flex", alignItems: "center", gap: 12, width: "100%", textAlign: "left", padding: "11px 14px", background: "transparent", border: "none", borderBottom: i < list.length - 1 ? "1px solid var(--line)" : "none", cursor: "pointer", font: "inherit", color: "inherit" }}>
              <Icons.Commit size={14} style={{ color: "var(--fg-2)" }} />
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ fontSize: 13.5, fontWeight: 450, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{c.message.split("\n")[0]}</div>
                <div className="subtle" style={{ fontSize: 11.5, marginTop: 2 }}>{c.author} · {relativeDate(c.date)}</div>
              </div>
              <span className="mono subtle" style={{ fontSize: 11.5 }}>{c.short}</span>
            </button>
          ))}
        </div>
      ) : null}
    </div>
  );
}

// ProposalsView — the human-in-the-loop gate. Lists AI/code change proposals;
// a writer reviews the diff and Accepts (applies to a feature branch + opens a
// PR) or Rejects. Nothing touches the repo until Accept is clicked.
function ProposalsView({ repoId, repo, setRoute }) {
  const [items, setItems] = React.useState(null);
  const [open, setOpen] = React.useState(null);   // expanded proposal id
  const [busy, setBusy] = React.useState(false);
  const [note, setNote] = React.useState("");
  const canWrite = repo && ["owner", "admin", "write"].includes(repo.role);

  const reload = React.useCallback(() => {
    if (window.OrchisAPI && repoId) window.OrchisAPI.get(`/v1/repos/${repoId}/proposals`).then(setItems).catch(() => setItems([]));
  }, [repoId]);
  React.useEffect(() => { reload(); }, [reload]);

  const act = async (id, verb) => {
    setBusy(true); setNote("");
    try {
      const res = await window.OrchisAPI.post(`/v1/repos/${repoId}/proposals/${id}/${verb}`, {});
      if (verb === "accept") setNote(`Accepted → committed to ${res.branch}${res.pullNumber ? `, opened PR #${res.pullNumber}` : ""}.`);
      else setNote("Proposal rejected.");
      reload();
    } catch (e) { setNote(verb === "accept" ? "Accept failed — the patch may not apply cleanly." : "Reject failed."); }
    finally { setBusy(false); }
  };

  const list = items || [];
  const pill = (st) => ({ pending: "var(--accent)", accepted: "var(--ok, var(--accent))", rejected: "var(--fg-3)" }[st] || "var(--fg-3)");

  return (
    <div style={{ padding: "20px 24px", maxWidth: 900 }}>
      <div className="row" style={{ marginBottom: 12 }}>
        <div>
          <h2 style={{ fontSize: 15, fontWeight: 600, margin: 0 }}>Change proposals</h2>
          <p className="subtle" style={{ fontSize: 12, margin: "2px 0 0" }}>AI- or tool-proposed edits. Review the diff, then Accept (applies to a new branch + opens a PR) or Reject. Nothing is applied until you accept.</p>
        </div>
      </div>
      {note ? <div className="card" style={{ padding: "8px 12px", marginBottom: 10, fontSize: 12.5, color: "var(--accent)" }}>{note}</div> : null}
      {items == null ? <div className="muted" style={{ fontSize: 12.5 }}>Loading…</div> : null}
      {items != null && list.length === 0 ? <div className="card" style={{ padding: "20px 18px", color: "var(--fg-3)", fontSize: 13 }}>No proposals yet.</div> : null}
      {list.map(p => (
        <div key={p.id} className="card" style={{ marginBottom: 10, overflow: "hidden" }}>
          <button onClick={() => setOpen(open === p.id ? null : p.id)}
            style={{ display: "flex", alignItems: "center", gap: 10, width: "100%", textAlign: "left", padding: "11px 14px", background: "transparent", border: "none", cursor: "pointer", font: "inherit", color: "inherit" }}>
            <Icons.Diff size={14} style={{ color: "var(--fg-2)" }} />
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ fontSize: 13.5, fontWeight: 450 }}>{p.title}</div>
              <div className="subtle" style={{ fontSize: 11.5, marginTop: 2 }}>by {p.author && p.author.name} · {p.when}{p.pullNumber ? ` · PR #${p.pullNumber}` : ""}</div>
            </div>
            <span className="chip" style={{ height: 18, fontSize: 10.5, color: pill(p.status), borderColor: "currentColor" }}>{p.status}</span>
          </button>
          {open === p.id ? (
            <div style={{ borderTop: "1px solid var(--line)" }}>
              {p.summary ? <div style={{ padding: "10px 14px", fontSize: 12.5, color: "var(--fg-2)" }}>{p.summary}</div> : null}
              <pre style={{ margin: 0, padding: "12px 14px", overflowX: "auto", fontSize: 11.5, lineHeight: 1.5, fontFamily: "var(--font-mono)", background: "var(--bg-1)", maxHeight: 360 }}>
                {p.patch ? p.patch : (p.changes || []).map(c => (c.delete ? `- delete ${c.path}\n` : `+ ${c.path}\n${c.content}\n`)).join("\n")}
              </pre>
              {canWrite && p.status === "pending" ? (
                <div className="row" style={{ gap: 8, padding: 12, borderTop: "1px solid var(--line)" }}>
                  <span className="spacer" />
                  <button className="btn ghost" disabled={busy} onClick={() => act(p.id, "reject")}>Reject</button>
                  <button className="btn primary" disabled={busy} onClick={() => act(p.id, "accept")}>{busy ? "Applying…" : "Accept changes"}</button>
                </div>
              ) : p.pullNumber ? (
                <div className="row" style={{ padding: 12, borderTop: "1px solid var(--line)" }}>
                  <span className="spacer" />
                  <button className="btn ghost sm" onClick={() => setRoute && setRoute({ view: "pr", pr: p.pullNumber, repo: repoId })}>Open PR #{p.pullNumber}</button>
                </div>
              ) : null}
            </div>
          ) : null}
        </div>
      ))}
    </div>
  );
}

// relativeDate renders an ISO date compactly (the API already sends UTC ISO).
function relativeDate(iso) {
  if (!iso) return "";
  const d = new Date(iso);
  if (isNaN(d)) return iso;
  return d.toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" });
}

function Subtab({ active, onClick, icon, children }) {
  return (
    <button onClick={onClick} style={{
      ...repoStyles.subtab,
      color: active ? "var(--fg)" : "var(--fg-2)",
      borderBottomColor: active ? "var(--accent)" : "transparent",
    }}>
      {icon}{children}
    </button>
  );
}

function CodeView({ repoId, path, repo, setRoute }) {
  const [blob, setBlob] = React.useState(null);
  const [err, setErr] = React.useState(false);
  const [reload, setReload] = React.useState(0);
  // Blame + outline + edit state.
  const [blame, setBlame] = React.useState(null);
  const [showBlame, setShowBlame] = React.useState(false);
  const [outline, setOutline] = React.useState(null);
  const [showOutline, setShowOutline] = React.useState(false);
  const [editing, setEditing] = React.useState(false);
  const [draft, setDraft] = React.useState("");
  const [msg, setMsg] = React.useState("");
  const [toNew, setToNew] = React.useState(false);
  const [newBranch, setNewBranch] = React.useState("");
  const [busy, setBusy] = React.useState(false);
  const [note, setNote] = React.useState("");

  const canWrite = repo && ["owner", "admin", "write"].includes(repo.role);
  const defBranch = (repo && repo.defaultBranch) || "main";

  React.useEffect(() => {
    let cancelled = false;
    setBlob(null); setErr(false); setBlame(null); setShowBlame(false);
    setOutline(null); setShowOutline(false); setEditing(false); setNote("");
    if (window.OrchisAPI && repoId && path && path !== "README.md") {
      window.OrchisAPI.get(`/v1/repos/${repoId}/blob?path=${encodeURIComponent(path)}`)
        .then(b => { if (!cancelled) setBlob(b); })
        .catch(() => { if (!cancelled) setErr(true); });
    }
    return () => { cancelled = true; };
  }, [repoId, path, reload]);

  const toggleBlame = () => {
    if (!showBlame && blame == null && window.OrchisAPI) {
      window.OrchisAPI.get(`/v1/repos/${repoId}/blame?path=${encodeURIComponent(path)}`)
        .then(setBlame).catch(() => setBlame([]));
    }
    setShowBlame(v => !v); setShowOutline(false);
  };

  const toggleOutline = () => {
    if (!showOutline && outline == null && window.OrchisAPI) {
      window.OrchisAPI.get(`/v1/repos/${repoId}/outline?path=${encodeURIComponent(path)}`)
        .then(setOutline).catch(() => setOutline([]));
    }
    setShowOutline(v => !v); setShowBlame(false);
  };

  const startEdit = () => {
    setDraft(blob ? blob.content : "");
    setMsg("Update " + path);
    setToNew(false); setNewBranch(""); setNote(""); setShowBlame(false);
    setEditing(true);
  };

  const commit = async () => {
    setBusy(true); setNote("");
    const branch = toNew ? newBranch.trim() : defBranch;
    try {
      const res = await window.OrchisAPI.post(`/v1/repos/${repoId}/commits`, {
        branch, message: msg, changes: [{ path, content: draft }],
      });
      setEditing(false);
      setNote(`Committed ${res.sha ? res.sha.slice(0, 7) : ""} to ${res.branch}.`);
      if (toNew) {
        // Don't silently change what the viewer sees; just confirm the branch.
      } else {
        setReload(n => n + 1); // refresh the blob on the same branch
      }
    } catch (e) {
      setNote("Commit failed — you may lack write access, or the branch moved.");
    } finally { setBusy(false); }
  };

  if (path === "README.md") return <ReadmeView repoId={repoId} />;

  // Offline prototype fallback: the original hardcoded router.rs sample.
  if (!window.OrchisAPI && path === "crates/atlas-core/src/router.rs") {
    return (
      <div style={repoStyles.code}>
        <div style={repoStyles.codeBar}>
          <span className="mono" style={{ color: "var(--fg-2)" }}>{path}</span>
          <span className="spacer" />
          <span className="muted" style={{ fontSize: 11.5 }}>56 lines · 1.4 KB · Rust</span>
          <button className="btn ghost sm icon"><Icons.Copy size={12} /></button>
        </div>
        <CodeBlock code={ROUTER_RS} />
      </div>
    );
  }

  if (blob) {
    return (
      <div style={repoStyles.code}>
        <div style={repoStyles.codeBar}>
          <span className="mono" style={{ color: "var(--fg-2)" }}>{path}</span>
          <span className="spacer" />
          {!editing ? <span className="muted" style={{ fontSize: 11.5 }}>{blob.lines} lines · {fmtBytes(blob.size)} · {blob.lang}</span> : null}
          {!editing ? (
            <button className={"btn ghost sm" + (showBlame ? " active" : "")} onClick={toggleBlame} title="Toggle blame">
              <Icons.Activity size={12} /> Blame
            </button>
          ) : null}
          {!editing && window.OrchisAPI ? (
            <button className={"btn ghost sm" + (showOutline ? " active" : "")} onClick={toggleOutline} title="Show symbol outline">
              <Icons.Code size={12} /> Outline
            </button>
          ) : null}
          {canWrite && !editing ? (
            <button className="btn ghost sm" onClick={startEdit} title="Edit this file"><Icons.Edit size={12} /> Edit</button>
          ) : null}
        </div>

        {note ? <div style={{ padding: "8px 14px", fontSize: 12, color: "var(--accent)", borderBottom: "1px solid var(--line)" }}>{note}</div> : null}

        {editing ? (
          <div style={{ display: "flex", flexDirection: "column", minHeight: 0, flex: 1 }}>
            <textarea value={draft} onChange={e => setDraft(e.target.value)} spellCheck={false}
              style={{ flex: 1, minHeight: 300, resize: "vertical", border: "none", outline: "none", padding: "12px 16px",
                fontFamily: "var(--font-mono)", fontSize: 12.5, lineHeight: 1.6, background: "var(--bg-1)", color: "var(--fg)" }} />
            <div style={{ borderTop: "1px solid var(--line)", padding: 12, display: "flex", flexDirection: "column", gap: 8 }}>
              <div className="row" style={{ gap: 8 }}>
                <input className="input" value={msg} onChange={e => setMsg(e.target.value)} placeholder="Commit message" style={{ flex: 1 }} />
                <button className="btn ghost sm" disabled={busy} title="Draft a Conventional Commit message with your model"
                  onClick={async () => {
                    try { const res = await window.OrchisAPI.post(`/v1/repos/${repoId}/commits/draft-message`, { changes: [{ path, content: draft }] }); if (res && res.message) setMsg(res.message); }
                    catch (e) { setNote("Couldn't draft a message — is a model configured in Developer settings?"); }
                  }}>
                  <Icons.Bolt size={12} /> Generate
                </button>
              </div>
              <div className="row" style={{ gap: 12, fontSize: 12.5, flexWrap: "wrap" }}>
                <label className="row" style={{ gap: 6, cursor: "pointer" }}>
                  <input type="radio" checked={!toNew} onChange={() => setToNew(false)} /> Commit to <span className="mono">{defBranch}</span>
                </label>
                <label className="row" style={{ gap: 6, cursor: "pointer" }}>
                  <input type="radio" checked={toNew} onChange={() => setToNew(true)} /> New branch
                </label>
                {toNew ? <input className="input" value={newBranch} onChange={e => setNewBranch(e.target.value)} placeholder="feature/my-edit" style={{ flex: 1, minWidth: 160 }} /> : null}
              </div>
              <div className="row" style={{ gap: 8 }}>
                <span className="spacer" />
                <button className="btn ghost" onClick={() => setEditing(false)} disabled={busy}>Cancel</button>
                <button className="btn primary" onClick={commit} disabled={busy || !msg.trim() || (toNew && !newBranch.trim())}>{busy ? "Committing…" : "Commit changes"}</button>
              </div>
            </div>
          </div>
        ) : showBlame && blame ? (
          <BlameBlock code={blob.content} blame={blame} repoId={repoId} setRoute={setRoute} />
        ) : showOutline && outline ? (
          <OutlineList symbols={outline} />
        ) : (
          <CodeBlock code={blob.content} />
        )}
      </div>
    );
  }

  return (
    <div style={repoStyles.code}>
      <div style={repoStyles.codeBar}>
        <span className="mono" style={{ color: "var(--fg-2)" }}>{path}</span>
        <span className="spacer" />
      </div>
      <div className="placeholder" style={{ margin: 20, height: 220 }}>
        {err ? `[ could not load ${path} ]` : `[ loading ${path}… ]`}
      </div>
    </div>
  );
}

// OutlineList renders a file's structural symbols (AST-aware chunking). Lets a
// reader (or the LLM, via the same endpoint) scan names before whole bodies.
function OutlineList({ symbols }) {
  const kindColor = { func: "var(--accent)", method: "var(--accent)", class: "oklch(64% 0.12 30)", struct: "oklch(64% 0.12 30)", enum: "oklch(64% 0.12 30)", interface: "oklch(60% 0.12 280)", type: "oklch(60% 0.12 280)" };
  if (!symbols.length) return <div className="subtle" style={{ padding: 20, fontSize: 12.5 }}>No symbols detected (or unsupported language).</div>;
  return (
    <div style={{ padding: "10px 4px" }}>
      {symbols.map((s, i) => (
        <div key={i} className="row" style={{ gap: 10, padding: "6px 16px", fontSize: 12.5 }}>
          <span className="chip" style={{ height: 17, fontSize: 9.5, color: kindColor[s.kind] || "var(--fg-3)", borderColor: "currentColor", minWidth: 52, justifyContent: "center" }}>{s.kind}</span>
          <span className="mono" style={{ flex: 1 }}>{s.name}</span>
          <span className="subtle" style={{ fontSize: 11 }}>L{s.lineStart}{s.lineEnd > s.lineStart ? `–${s.lineEnd}` : ""}</span>
        </div>
      ))}
    </div>
  );
}

// BlameBlock renders the file with a per-line authorship gutter. Clicking a
// line's sha opens that commit.
function BlameBlock({ code, blame, repoId, setRoute }) {
  const lines = code.split("\n");
  const by = {};
  blame.forEach(b => { by[b.line] = b; });
  return (
    <pre style={repoStyles.pre}>
      {lines.map((ln, i) => {
        const b = by[i + 1];
        return (
          <div key={i} style={repoStyles.codeLine}>
            <span
              onClick={() => b && setRoute && setRoute({ view: "commit", repo: repoId, sha: b.sha })}
              title={b ? `${b.summary} — ${b.author}, ${b.when}` : ""}
              style={{ display: "inline-block", width: 168, flexShrink: 0, paddingRight: 10, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap",
                color: "var(--fg-3)", fontSize: 10.5, cursor: b ? "pointer" : "default", borderRight: "1px solid var(--line)" }}>
              {b ? <><span className="mono" style={{ color: "var(--accent)" }}>{b.short}</span> {b.author} · {b.when}</> : ""}
            </span>
            <span style={{ ...repoStyles.lineNo, width: 40 }}>{i + 1}</span>
            <code style={{ whiteSpace: "pre", fontFamily: "var(--font-mono)" }}>{highlightRust(ln)}</code>
          </div>
        );
      })}
    </pre>
  );
}

function fmtBytes(n) {
  if (n == null) return "";
  if (n < 1024) return n + " B";
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + " KB";
  return (n / (1024 * 1024)).toFixed(1) + " MB";
}

function CodeBlock({ code }) {
  const lines = code.split("\n");
  return (
    <pre style={repoStyles.pre}>
      {lines.map((ln, i) => (
        <div key={i} style={repoStyles.codeLine}>
          <span style={repoStyles.lineNo}>{i + 1}</span>
          <code style={{ whiteSpace: "pre", fontFamily: "var(--font-mono)" }}>{highlightRust(ln)}</code>
        </div>
      ))}
    </pre>
  );
}

// Very lightweight Rust-ish syntax highlight.
function highlightRust(line) {
  const tokens = [];
  let i = 0;
  const push = (text, color) => tokens.push(<span key={tokens.length} style={color ? { color } : null}>{text}</span>);

  // Comments
  const commentIdx = line.indexOf("//");
  if (commentIdx !== -1) {
    const before = line.slice(0, commentIdx);
    const after = line.slice(commentIdx);
    return <>{highlightRust(before)}<span style={{ color: "var(--fg-3)", fontStyle: "italic" }}>{after}</span></>;
  }

  const keywords = ["use", "pub", "fn", "impl", "struct", "let", "mut", "async", "await", "self", "Self", "match", "None", "Some", "Ok", "Err", "Result", "Option", "if", "else", "return", "true", "false", "in"];
  const regex = /([A-Za-z_][A-Za-z0-9_]*)|("(?:[^"\\]|\\.)*")|(\/\/.*$)|(\b\d+\b)|(\s+)|([^A-Za-z0-9_"\s]+)/g;
  let m;
  while ((m = regex.exec(line)) !== null) {
    const tok = m[0];
    if (m[1]) {
      if (keywords.includes(tok)) push(tok, "oklch(60% 0.14 280)");
      else if (/^[A-Z]/.test(tok)) push(tok, "oklch(64% 0.12 30)");
      else push(tok);
    } else if (m[2]) push(tok, "oklch(60% 0.12 130)");
    else if (m[4]) push(tok, "oklch(65% 0.13 200)");
    else if (m[6]) push(tok, "var(--fg-2)");
    else push(tok);
  }
  return tokens;
}

function ReadmeView({ repoId }) {
  const [md, setMd] = React.useState(null);
  React.useEffect(() => {
    let cancelled = false;
    if (window.OrchisAPI && repoId) {
      window.OrchisAPI.get(`/v1/repos/${repoId}/readme`)
        .then(r => { if (!cancelled) setMd(r && r.content != null ? r.content : ""); })
        .catch(() => { if (!cancelled) setMd(null); });
    }
    return () => { cancelled = true; };
  }, [repoId]);

  // md === null → not loaded / offline: use the mock. md === "" → no README.
  const content = md != null ? md : README_MD;
  if (content === "") {
    return <div style={{ ...repoStyles.readme, color: "var(--fg-3)" }}>No README in this repository.</div>;
  }
  return (
    <div style={repoStyles.readme}>
      {parseMd(content)}
    </div>
  );
}

function parseMd(md) {
  const out = [];
  const lines = md.split("\n");
  let i = 0;
  let key = 0;
  while (i < lines.length) {
    const ln = lines[i];
    if (ln.startsWith("# ")) { out.push(<h1 key={key++} style={repoStyles.mdH1}>{ln.slice(2)}</h1>); i++; }
    else if (ln.startsWith("## ")) { out.push(<h2 key={key++} style={repoStyles.mdH2}>{ln.slice(3)}</h2>); i++; }
    else if (ln.startsWith("> ")) { out.push(<blockquote key={key++} style={repoStyles.mdQuote}>{ln.slice(2)}</blockquote>); i++; }
    else if (ln.startsWith("```")) {
      const start = i + 1;
      let end = start;
      while (end < lines.length && !lines[end].startsWith("```")) end++;
      out.push(<pre key={key++} style={repoStyles.mdCode}>{lines.slice(start, end).join("\n")}</pre>);
      i = end + 1;
    }
    else if (ln.startsWith("- ")) {
      const items = [];
      while (i < lines.length && lines[i].startsWith("- ")) { items.push(lines[i].slice(2)); i++; }
      out.push(<ul key={key++} style={repoStyles.mdUl}>{items.map((it, j) => <li key={j} dangerouslySetInnerHTML={{__html: inlineMd(it)}}/>)}</ul>);
    }
    else if (ln.trim() === "") { i++; }
    else { out.push(<p key={key++} style={repoStyles.mdP} dangerouslySetInnerHTML={{__html: inlineMd(ln)}}/>); i++; }
  }
  return out;
}

// escapeHtml neutralizes user-authored content before it is injected via
// dangerouslySetInnerHTML. inlineMd escapes the whole line first, then layers
// trusted markdown HTML (code/bold/links) on top — so a README or comment
// containing `<img onerror=…>` renders as inert text, not script.
function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}

function inlineMd(s) {
  return escapeHtml(s)
    .replace(/`([^`]+)`/g, "<code style=\"font-family: var(--font-mono); background: var(--bg-2); padding: 1px 5px; border-radius: 4px; font-size: 0.9em;\">$1</code>")
    .replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>")
    // Render real links, but only for http(s)/root-relative URLs — anything else
    // (e.g. javascript:) is kept as plain text to avoid an injection vector via
    // dangerouslySetInnerHTML.
    .replace(/\[([^\]]+)\]\(([^)]+)\)/g, (m, text, url) => {
      const u = url.trim();
      if (!/^(https?:\/\/|\/)/i.test(u)) return text;
      return "<a href=\"" + u + "\" target=\"_blank\" rel=\"noreferrer noopener\" style=\"color: var(--accent); text-decoration: none;\">" + text + "</a>";
    });
}

function RecentActivityView({ repoId }) {
  const [activity, setActivity] = React.useState(null);
  React.useEffect(() => {
    if (window.OrchisAPI && repoId) {
      window.OrchisAPI.get(`/v1/repos/${repoId}/activity`).then(setActivity).catch(() => setActivity([]));
    }
  }, [repoId]);
  const items = activity != null ? activity : (window.OrchisAPI ? [] : ACTIVITY.slice(0, 4));
  return (
    <div style={{ padding: "20px 24px" }}>
      {items.length === 0 ? (
        <div className="card" style={{ padding: "20px 18px", color: "var(--fg-3)", fontSize: 13 }}>No activity yet.</div>
      ) : (
      <div className="card" style={{ padding: 4 }}>
        {items.map(a => (
          <div key={a.id} style={{ display: "flex", alignItems: "center", gap: 10, padding: "10px 12px", borderBottom: "1px solid var(--line)", fontSize: 13 }}>
            <span className="avatar" style={{ background: a.actor.color, width: 20, height: 20, fontSize: 9 }}>{a.actor.initials}</span>
            <div style={{ flex: 1 }}>
              <strong style={{ fontWeight: 500 }}>{a.actor.name}</strong> <span className="muted">{activityVerb(a.kind)} {a.target}</span>
              <div className="subtle" style={{ fontSize: 11.5 }}>{a.title}</div>
            </div>
            <span className="subtle" style={{ fontSize: 11 }}>{a.when}</span>
          </div>
        ))}
      </div>
      )}
    </div>
  );
}

// Split panel — shows PRs, Issues, or Actions while you're in the code.
function SplitPanel({ content, close, setRoute }) {
  return (
    <>
      <div style={repoStyles.splitHead}>
        <Icons.Split size={13} style={{ color: "var(--fg-2)" }} />
        <span style={{ fontSize: 12.5, fontWeight: 500, textTransform: "capitalize" }}>{content.type}</span>
        <span className="mono subtle" style={{ fontSize: 11 }}>{content.repo}</span>
        <span className="spacer" />
        <button className="btn ghost icon sm" onClick={close} title="Close split"><Icons.Close size={11} /></button>
      </div>
      <div style={{ flex: 1, overflowY: "auto", padding: 12 }}>
        {content.type === "prs" ? <PRsSplit repo={content.repo} setRoute={setRoute} />
          : content.type === "issues" ? <IssuesSplit repo={content.repo} />
          : <ActionsSplit repo={content.repo} />}
      </div>
    </>
  );
}

function PRsSplit({ repo, setRoute }) {
  const [pulls, setPulls] = React.useState(null);
  React.useEffect(() => {
    if (window.OrchisAPI && repo) window.OrchisAPI.get(`/v1/repos/${repo}/pulls?state=open`).then(setPulls).catch(() => setPulls([]));
  }, [repo]);
  const prList = pulls != null ? pulls : (window.OrchisAPI ? [] : PRS.filter(p => p.repo === repo));
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
      {prList.length === 0 ? <div className="subtle" style={{ fontSize: 12, padding: 8 }}>No open pull requests.</div> : null}
      {prList.map(p => (
        <button key={p.id} className="card" style={repoStyles.prMini} onClick={() => setRoute({ view: "pr", pr: p.id, repo })}>
          <Icons.PR size={14} style={{ color: "var(--accent)" }} />
          <div style={{ flex: 1, minWidth: 0, textAlign: "left" }}>
            <div style={{ fontSize: 13, fontWeight: 450, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{p.title}</div>
            <div className="subtle" style={{ fontSize: 11, marginTop: 2 }}>#{p.id} · by {p.author.name} · {p.updated}</div>
          </div>
          <span className="chip" style={{ height: 18, fontSize: 10.5 }}>+{p.additions} −{p.deletions}</span>
        </button>
      ))}
    </div>
  );
}

function IssuesSplit({ repo }) {
  const [items, setItems] = React.useState(null);
  const [selected, setSelected] = React.useState(null);   // issue number
  const [creating, setCreating] = React.useState(false);
  const [title, setTitle] = React.useState("");
  const [body, setBody] = React.useState("");
  const [busy, setBusy] = React.useState(false);

  const reload = React.useCallback(() => {
    if (window.OrchisAPI && repo) window.OrchisAPI.get(`/v1/repos/${repo}/issues`).then(setItems).catch(() => setItems([]));
  }, [repo]);
  React.useEffect(() => { reload(); }, [reload]);

  const list = items != null ? items : [];

  const create = async () => {
    if (!title.trim()) return;
    setBusy(true);
    try {
      await window.OrchisAPI.post(`/v1/repos/${repo}/issues`, { title: title.trim(), body: body.trim() });
      setTitle(""); setBody(""); setCreating(false); reload();
    } catch (e) {} finally { setBusy(false); }
  };

  if (selected != null) {
    return <IssueDetail repo={repo} num={selected} onBack={() => { setSelected(null); reload(); }} />;
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
      {window.OrchisAPI ? (
        creating ? (
          <div className="card" style={{ padding: 12, display: "flex", flexDirection: "column", gap: 8 }}>
            <input className="input" value={title} onChange={e => setTitle(e.target.value)} placeholder="Issue title" autoFocus />
            <textarea value={body} onChange={e => setBody(e.target.value)} placeholder="Describe it… (optional)" style={{
              width: "100%", height: 70, padding: 8, border: "1px solid var(--line)", borderRadius: 6,
              fontFamily: "inherit", fontSize: 12.5, background: "var(--bg-1)", color: "var(--fg)", resize: "vertical",
            }} />
            <div className="row" style={{ gap: 8 }}>
              <button className="btn sm" onClick={() => { setCreating(false); setTitle(""); setBody(""); }}>Cancel</button>
              <span className="spacer" />
              <button className="btn primary sm" onClick={create} disabled={!title.trim() || busy}>{busy ? "Opening…" : "Open issue"}</button>
            </div>
          </div>
        ) : (
          <button className="btn sm" style={{ alignSelf: "flex-start", marginBottom: 2 }} onClick={() => setCreating(true)}>
            <Icons.Plus size={12} /> New issue
          </button>
        )
      ) : null}

      {list.length === 0 ? <div className="subtle" style={{ fontSize: 12, padding: 8 }}>No issues yet.</div> : null}
      {list.map(it => (
        <button key={it.number} className="card" style={repoStyles.prMini} onClick={() => setSelected(it.number)}>
          <Icons.Issue size={14} style={{ color: it.closed ? "var(--purple)" : "var(--info)" }} />
          <div style={{ flex: 1, minWidth: 0, textAlign: "left" }}>
            <div style={{ fontSize: 13, fontWeight: 450, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{it.title}</div>
            <div className="subtle" style={{ fontSize: 11, marginTop: 2 }}>#{it.number}{it.closed ? " · closed" : ""} · {it.comments || 0} comment{it.comments === 1 ? "" : "s"}</div>
          </div>
          {it.assignee ? <span className="avatar" style={{ background: it.assignee.color, width: 18, height: 18, fontSize: 8 }}>{it.assignee.initials}</span> : null}
        </button>
      ))}
    </div>
  );
}

function IssueDetail({ repo, num, onBack }) {
  const [issue, setIssue] = React.useState(null);
  const [comments, setComments] = React.useState([]);
  const [draft, setDraft] = React.useState("");
  const [busy, setBusy] = React.useState(false);

  const load = React.useCallback(() => {
    if (!window.OrchisAPI) return;
    window.OrchisAPI.get(`/v1/repos/${repo}/issues/${num}`).then(setIssue).catch(() => setIssue(null));
    window.OrchisAPI.get(`/v1/repos/${repo}/issues/${num}/comments`).then(setComments).catch(() => setComments([]));
  }, [repo, num]);
  React.useEffect(() => { load(); }, [load]);

  const comment = async () => {
    if (!draft.trim()) return;
    setBusy(true);
    try { await window.OrchisAPI.post(`/v1/repos/${repo}/issues/${num}/comments`, { body: draft.trim() }); setDraft(""); load(); }
    catch (e) {} finally { setBusy(false); }
  };
  const toggleState = async () => {
    if (!issue) return;
    setBusy(true);
    try { const r = await window.OrchisAPI.patch(`/v1/repos/${repo}/issues/${num}`, { state: issue.closed ? "open" : "closed" }); setIssue(r); }
    catch (e) {} finally { setBusy(false); }
  };

  if (!issue) return <div className="subtle" style={{ fontSize: 12, padding: 8 }}>Loading…</div>;

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
      <button className="btn ghost sm" style={{ alignSelf: "flex-start" }} onClick={onBack}>← All issues</button>
      <div className="row" style={{ gap: 8, alignItems: "flex-start" }}>
        <Icons.Issue size={16} style={{ color: issue.closed ? "var(--purple)" : "var(--info)", marginTop: 2 }} />
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ fontSize: 15, fontWeight: 500 }}>{issue.title}</div>
          <div className="subtle" style={{ fontSize: 11.5, marginTop: 3 }}>
            #{issue.number} · <span className="chip" style={{ height: 18, fontSize: 10.5, color: issue.closed ? "var(--purple)" : "var(--accent)" }}>{issue.closed ? "closed" : "open"}</span> · opened by {issue.author.name} · {issue.created}
          </div>
        </div>
      </div>
      {issue.body ? <div className="card" style={{ padding: 12, fontSize: 13, whiteSpace: "pre-wrap", lineHeight: 1.5 }}>{issue.body}</div> : null}

      {comments.map((c, i) => (
        <div key={i} className="card" style={{ padding: 12 }}>
          <div className="row" style={{ gap: 8, marginBottom: 6 }}>
            <span className="avatar" style={{ background: c.author.color, width: 18, height: 18, fontSize: 8 }}>{c.author.initials}</span>
            <span style={{ fontSize: 12.5, fontWeight: 500 }}>{c.author.name}</span>
            <span className="subtle" style={{ fontSize: 11 }}>{c.when}</span>
          </div>
          <div style={{ fontSize: 13, whiteSpace: "pre-wrap", lineHeight: 1.5 }}>{c.body}</div>
        </div>
      ))}

      {window.OrchisAPI ? (
        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
          <textarea value={draft} onChange={e => setDraft(e.target.value)} placeholder="Leave a comment…" style={{
            width: "100%", height: 64, padding: 8, border: "1px solid var(--line)", borderRadius: 6,
            fontFamily: "inherit", fontSize: 12.5, background: "var(--bg-1)", color: "var(--fg)", resize: "vertical",
          }} />
          <div className="row" style={{ gap: 8 }}>
            <button className="btn sm" onClick={toggleState} disabled={busy}>{issue.closed ? "Reopen" : "Close issue"}</button>
            <span className="spacer" />
            <button className="btn primary sm" onClick={comment} disabled={!draft.trim() || busy}>Comment</button>
          </div>
        </div>
      ) : null}
    </div>
  );
}

function ActionsSplit({ repo }) {
  const [runs, setRuns] = React.useState(null);
  React.useEffect(() => {
    if (window.OrchisAPI && repo) window.OrchisAPI.get(`/v1/repos/${repo}/checks`).then(setRuns).catch(() => setRuns([]));
  }, [repo]);
  const list = runs != null ? runs : [];
  if (runs != null && list.length === 0) {
    return (
      <div className="card" style={{ padding: 16, fontSize: 12.5, color: "var(--fg-2)", lineHeight: 1.55 }}>
        <div style={{ fontWeight: 500, color: "var(--fg)", marginBottom: 6 }}>No check runs yet</div>
        Foundry doesn't execute workflows itself. Check runs appear here from the built-in <span className="mono">orchis-scan</span> supply-chain check, and from any external CI you wire up via webhooks + the checks API.
      </div>
    );
  }
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
      {list.map((r, i) => (
        <div key={i} className="card" style={repoStyles.prMini}>
          <span style={{
            width: 8, height: 8, borderRadius: 999,
            background: r.status === "ok" ? "var(--accent)" : r.status === "fail" ? "var(--danger)" : r.status === "cancelled" ? "var(--fg-3)" : "var(--warn)",
            boxShadow: r.status === "pending" || r.status === "queued" ? "0 0 0 3px color-mix(in oklab, var(--warn) 25%, transparent)" : "none",
          }} />
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ fontSize: 13, fontWeight: 450 }}>{r.name} <span className="mono subtle" style={{ fontSize: 11.5 }}>· {r.sha}</span></div>
            <div className="subtle" style={{ fontSize: 11, marginTop: 2, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{r.detail || r.status}{r.when ? " · " + r.when : ""}</div>
          </div>
          {r.externalUrl ? <a className="btn ghost sm" href={r.externalUrl} target="_blank" rel="noreferrer">View</a> : null}
        </div>
      ))}
    </div>
  );
}

// Releases — list + (owner) publish + download source tarball.
function ReleasesView({ repo, repoId }) {
  const isOwner = !!(window.USERS && USERS.me && repo.org === USERS.me.handle);
  const [items, setItems] = React.useState(null);
  const [creating, setCreating] = React.useState(false);
  const [tag, setTag] = React.useState("");
  const [name, setName] = React.useState("");
  const [body, setBody] = React.useState("");
  const [prerelease, setPrerelease] = React.useState(false);
  const [busy, setBusy] = React.useState(false);
  const [err, setErr] = React.useState("");

  const reload = React.useCallback(() => {
    if (window.OrchisAPI && repoId) window.OrchisAPI.get(`/v1/repos/${repoId}/releases`).then(setItems).catch(() => setItems([]));
  }, [repoId]);
  React.useEffect(() => { reload(); }, [reload]);
  const list = items != null ? items : [];

  const publish = async () => {
    if (!tag.trim()) return;
    setErr(""); setBusy(true);
    try {
      await window.OrchisAPI.post(`/v1/repos/${repoId}/releases`, { tag: tag.trim(), name: name.trim(), body: body.trim(), prerelease });
      setTag(""); setName(""); setBody(""); setPrerelease(false); setCreating(false); reload();
    } catch (e) { setErr("Could not publish — the tag may be invalid or already released."); }
    finally { setBusy(false); }
  };
  const remove = async (t) => {
    try { await window.OrchisAPI.del(`/v1/repos/${repoId}/releases/${t}`); reload(); } catch (e) {}
  };

  return (
    <div style={{ padding: "20px 24px", maxWidth: 820 }}>
      <div className="row" style={{ marginBottom: 14 }}>
        <h2 style={{ fontSize: 16, fontWeight: 500, margin: 0 }}>Releases</h2>
        <span className="spacer" />
        {isOwner && window.OrchisAPI ? <button className="btn primary sm" onClick={() => { setErr(""); setCreating(c => !c); }}><Icons.Tag size={12} /> Draft a release</button> : null}
      </div>

      {err ? <div style={{ padding: "8px 12px", border: "1px solid var(--danger)", borderRadius: 6, color: "var(--danger)", fontSize: 12.5, marginBottom: 12 }}>{err}</div> : null}

      {creating ? (
        <div className="card fade-in" style={{ padding: 16, marginBottom: 16, display: "flex", flexDirection: "column", gap: 10 }}>
          <div className="row" style={{ gap: 8 }}>
            <input className="input" value={tag} onChange={e => setTag(e.target.value)} placeholder="tag e.g. v1.0.0" style={{ flex: 1 }} />
            <input className="input" value={name} onChange={e => setName(e.target.value)} placeholder="release name (optional)" style={{ flex: 1 }} />
          </div>
          <textarea value={body} onChange={e => setBody(e.target.value)} placeholder="Release notes (markdown)…" style={{
            width: "100%", height: 100, padding: 10, border: "1px solid var(--line)", borderRadius: 6,
            fontFamily: "inherit", fontSize: 13, background: "var(--bg-1)", color: "var(--fg)", resize: "vertical",
          }} />
          <label className="row" style={{ gap: 8, fontSize: 12.5, cursor: "pointer" }}>
            <input type="checkbox" checked={prerelease} onChange={e => setPrerelease(e.target.checked)} /> Mark as pre-release
          </label>
          <div className="row" style={{ gap: 8 }}>
            <span className="subtle" style={{ fontSize: 11.5 }}>Tagged from <span className="mono">{repo.defaultBranch || "main"}</span> if the tag doesn't exist yet.</span>
            <span className="spacer" />
            <button className="btn sm" onClick={() => setCreating(false)}>Cancel</button>
            <button className="btn primary sm" disabled={!tag.trim() || busy} onClick={publish}>{busy ? "Publishing…" : "Publish release"}</button>
          </div>
        </div>
      ) : null}

      {list.length === 0 ? (
        <div className="card" style={{ padding: "20px 18px", color: "var(--fg-3)", fontSize: 13 }}>No releases yet.</div>
      ) : list.map(rel => (
        <div key={rel.tag} className="card" style={{ padding: 16, marginBottom: 12 }}>
          <div className="row" style={{ gap: 8, marginBottom: 6 }}>
            <Icons.Tag size={14} style={{ color: "var(--accent)" }} />
            <span style={{ fontSize: 15, fontWeight: 500 }}>{rel.name}</span>
            <span className="chip" style={{ height: 18, fontSize: 10.5 }}>{rel.tag}</span>
            {rel.prerelease ? <span className="chip warn" style={{ height: 18, fontSize: 10.5 }}>pre-release</span> : null}
            <span className="mono subtle" style={{ fontSize: 11 }}>{rel.sha}</span>
            <span className="spacer" />
            <span className="subtle" style={{ fontSize: 11.5 }}>{rel.created}</span>
          </div>
          {rel.body ? <div style={{ fontSize: 13, lineHeight: 1.5, whiteSpace: "pre-wrap", color: "var(--fg-1)", margin: "4px 0 10px" }}>{rel.body}</div> : null}
          <div className="row" style={{ gap: 8 }}>
            <a className="btn sm" href={rel.tarball} download><Icons.File size={12} /> Source (.tar.gz)</a>
            {rel.author ? <span className="subtle" style={{ fontSize: 11.5 }}>by {rel.author.name}</span> : null}
            <span className="spacer" />
            {isOwner ? <button className="btn ghost sm" style={{ color: "var(--danger)" }} onClick={() => remove(rel.tag)}>Delete</button> : null}
          </div>
        </div>
      ))}
    </div>
  );
}

const repoStyles = {
  shell: { display: "flex", flexDirection: "column", height: "100%", overflow: "hidden" },
  header: {
    display: "flex",
    alignItems: "center",
    gap: 10,
    padding: "10px 16px",
    borderBottom: "1px solid var(--line)",
    background: "var(--bg-1)",
    minHeight: 52,
  },
  body: { flex: 1, display: "flex", minHeight: 0, overflow: "hidden" },
  tree: {
    flexShrink: 0,
    background: "var(--bg-1)",
    display: "flex",
    flexDirection: "column",
    minWidth: 180,
  },
  treeHead: {
    display: "flex", alignItems: "center", justifyContent: "space-between",
    padding: "8px 10px",
    borderBottom: "1px solid var(--line)",
  },
  treeScroll: { flex: 1, overflowY: "auto", padding: "4px 4px 12px" },
  treeRow: {
    display: "flex", alignItems: "center", gap: 6,
    width: "100%",
    height: 26,
    paddingRight: 6,
    background: "transparent",
    border: "none",
    cursor: "pointer",
    borderRadius: 4,
    font: "inherit",
    color: "var(--fg-1)",
  },
  treeRowActive: { background: "var(--accent-soft)" },
  fileGlyph: { width: 8, height: 8, borderRadius: 2, flexShrink: 0 },
  resizer: { width: 6, cursor: "col-resize", background: "transparent", borderLeft: "1px solid var(--line)" },
  mainArea: { flex: 1, display: "flex", minWidth: 0, overflow: "hidden" },
  mainResizer: { width: 6, cursor: "col-resize", background: "var(--bg)", borderLeft: "1px solid var(--line)", borderRight: "1px solid var(--line)" },
  subtabs: {
    display: "flex", alignItems: "center", gap: 2,
    padding: "0 16px",
    borderBottom: "1px solid var(--line)",
    background: "var(--bg-1)",
    height: 38,
  },
  subtab: {
    height: 38,
    padding: "0 12px",
    background: "transparent",
    border: "none",
    borderBottom: "2px solid transparent",
    cursor: "pointer",
    font: "inherit",
    fontSize: 12.5,
    fontWeight: 450,
    color: "var(--fg-2)",
    display: "inline-flex", alignItems: "center", gap: 6,
  },
  code: { display: "flex", flexDirection: "column", height: "100%" },
  codeBar: {
    display: "flex", alignItems: "center", gap: 12,
    padding: "6px 14px",
    fontSize: 12,
    color: "var(--fg-2)",
    borderBottom: "1px solid var(--line)",
    background: "var(--bg-1)",
  },
  pre: { margin: 0, padding: "10px 0", fontFamily: "var(--font-mono)", fontSize: 12.5, lineHeight: 1.55, background: "var(--bg)", overflowX: "auto" },
  codeLine: { display: "flex", paddingRight: 16 },
  lineNo: {
    display: "inline-block", textAlign: "right",
    width: 44, paddingRight: 14,
    color: "var(--fg-3)", fontSize: 11,
    userSelect: "none",
    flexShrink: 0,
  },
  readme: { padding: "28px 36px 60px", maxWidth: 820, fontSize: 14.5, lineHeight: 1.65, color: "var(--fg-1)" },
  mdH1: { fontSize: 28, fontWeight: 500, letterSpacing: "-0.02em", margin: "0 0 16px", paddingBottom: 12, borderBottom: "1px solid var(--line)", color: "var(--fg)" },
  mdH2: { fontSize: 18, fontWeight: 500, margin: "28px 0 12px", color: "var(--fg)" },
  mdP: { margin: "0 0 14px" },
  mdQuote: { borderLeft: "3px solid var(--accent-line)", paddingLeft: 14, margin: "0 0 18px", color: "var(--fg-2)", fontStyle: "italic" },
  mdCode: { background: "var(--bg-2)", border: "1px solid var(--line)", borderRadius: 8, padding: "12px 14px", fontFamily: "var(--font-mono)", fontSize: 12.5, margin: "0 0 16px", whiteSpace: "pre", overflowX: "auto" },
  mdUl: { margin: "0 0 16px 0", padding: "0 0 0 22px" },
  splitHead: {
    display: "flex", alignItems: "center", gap: 8,
    padding: "0 12px", height: 38,
    borderBottom: "1px solid var(--line)",
    background: "var(--bg-1)",
  },
  prMini: {
    display: "flex", alignItems: "center", gap: 10,
    padding: "10px 12px",
    cursor: "pointer",
    font: "inherit",
    color: "inherit",
    textAlign: "left",
  },
};

window.RepoView = RepoView;
