// Repo view — IDE-like: file tree (left) + main panel (code/readme) with optional split.
function RepoView({ repoId, file, setRoute, openSplit, splitOpen, splitContent, closeSplit }) {
  const repo = REPOS.find(r => r.id === repoId) || REPOS[0] || { id: repoId, name: repoId, org: "", description: "", languageColor: "var(--fg-3)", visibility: "private", stars: 0 };
  const [active, setActive] = React.useState(file || "README.md");
  const [tab, setTab] = React.useState("code");      // code | readme | activity
  const [expanded, setExpanded] = React.useState(() => collectInitialExpanded(FILE_TREE));
  const [treeWidth, setTreeWidth] = React.useState(260);
  const [mainSplit, setMainSplit] = React.useState(0.55);

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
  const isOwner = !!(window.USERS && USERS.me && repo.org === USERS.me.handle);
  const [showSettings, setShowSettings] = React.useState(false);

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
      <RepoHeader repo={repo} setRoute={setRoute} openSplit={openSplit} isOwner={isOwner} onSettings={() => setShowSettings(true)} />
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
            <button className="btn ghost icon sm" title="Find in files"><Icons.Search size={12} /></button>
          </div>
          <div style={repoStyles.treeScroll}>
            {treeNodes.length === 0
              ? <div style={{ padding: "16px 12px", color: "var(--fg-3)", fontSize: 12 }}>Empty repository. Push some code to get started.</div>
              : <FileTree nodes={treeNodes} expanded={expanded} active={active} toggle={toggle} onPick={onPickFile} depth={0} />}
          </div>
        </div>

        {/* Tree resizer */}
        <div style={repoStyles.resizer} onMouseDown={startTreeResize} />

        {/* Main panel — possibly split */}
        <div style={repoStyles.mainArea}>
          {splitOpen ? (
            <>
              <div style={{ width: `calc(${mainSplit * 100}% - 3px)`, height: "100%", display: "flex", flexDirection: "column", overflow: "hidden" }}>
                <RepoMainPanel repo={repo} repoId={repoId} active={active} tab={tab} setTab={setTab} setActive={onPickFile} />
              </div>
              <div style={repoStyles.mainResizer} onMouseDown={startMainResize} />
              <div style={{ flex: 1, height: "100%", display: "flex", flexDirection: "column", overflow: "hidden", borderLeft: "1px solid var(--line)" }}>
                <SplitPanel content={splitContent} close={closeSplit} setRoute={setRoute} />
              </div>
            </>
          ) : (
            <div style={{ flex: 1, height: "100%", display: "flex", flexDirection: "column", overflow: "hidden" }}>
              <RepoMainPanel repo={repo} active={active} tab={tab} setTab={setTab} setActive={onPickFile} />
            </div>
          )}
        </div>
      </div>
    </div>
  );
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

function FileTree({ nodes, expanded, active, toggle, onPick, depth }) {
  return nodes.map(n => {
    const isOpen = expanded[n.path];
    const isActive = n.type === "file" && active === n.path;
    return (
      <React.Fragment key={n.path}>
        <button
          style={{
            ...repoStyles.treeRow,
            ...(isActive ? repoStyles.treeRowActive : {}),
            paddingLeft: 6 + depth * 14,
          }}
          onClick={() => n.type === "dir" ? toggle(n.path) : onPick(n)}
        >
          {n.type === "dir"
            ? <Icons.Chevron size={11} style={{ color: "var(--fg-3)", transform: isOpen ? "rotate(90deg)" : "none", transition: "transform 80ms" }} />
            : <span style={{ width: 11 }} />
          }
          {n.type === "dir"
            ? <Icons.Folder size={13} style={{ color: "var(--fg-2)" }} />
            : <span style={{ ...repoStyles.fileGlyph, background: langColor(n.lang) }} />
          }
          <span style={{ fontFamily: "var(--font-mono)", fontSize: 12, color: isActive ? "var(--fg)" : "var(--fg-1)" }}>{n.name}</span>
        </button>
        {n.type === "dir" && isOpen && n.children
          ? <FileTree nodes={n.children} expanded={expanded} active={active} toggle={toggle} onPick={onPick} depth={depth + 1} />
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

function RepoHeader({ repo, setRoute, openSplit, isOwner, onSettings }) {
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
        <button className="btn sm" onClick={() => openSplit({ type: "issues", repo: repo.id })}>
          <Icons.Issue size={12} /> Issues
        </button>
        <button className="btn sm" onClick={() => openSplit({ type: "actions", repo: repo.id })}>
          <Icons.Bolt size={12} /> Actions
        </button>
        <span style={{ width: 1, height: 18, background: "var(--line)", margin: "0 4px" }} />
        <button className="btn sm" title="Star"><Icons.Star size={12} /> {(repo.stars || 0).toLocaleString()}</button>
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

  React.useEffect(() => {
    if (window.OrchisAPI) {
      window.OrchisAPI.get(`/v1/repos/${repo.id}/branches`).then(b => setBranches(b || [])).catch(() => {});
    }
  }, [repo.id]);

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

function RepoMainPanel({ repo, repoId, active, tab, setTab, setActive }) {
  return (
    <>
      <div style={repoStyles.subtabs}>
        <Subtab active={tab === "code"} onClick={() => setTab("code")} icon={<Icons.Code size={12} />}>Code</Subtab>
        <Subtab active={tab === "readme"} onClick={() => setTab("readme")} icon={<Icons.Book size={12} />}>Readme</Subtab>
        <Subtab active={tab === "activity"} onClick={() => setTab("activity")} icon={<Icons.Activity size={12} />}>Activity</Subtab>
        <span className="spacer" />
        <span className="row" style={{ gap: 8, color: "var(--fg-2)", fontSize: 12 }}>
          <Icons.Commit size={12} />
          <span className="mono">a3f9c12</span>
          <span className="subtle">by Jana, 12 min ago</span>
        </span>
      </div>

      <div style={{ flex: 1, minHeight: 0, overflowY: "auto" }}>
        {tab === "code" ? <CodeView repoId={repoId} path={active} /> : null}
        {tab === "readme" ? <ReadmeView repoId={repoId} /> : null}
        {tab === "activity" ? <RecentActivityView /> : null}
      </div>
    </>
  );
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

function CodeView({ repoId, path }) {
  const [blob, setBlob] = React.useState(null);
  const [err, setErr] = React.useState(false);

  React.useEffect(() => {
    let cancelled = false;
    setBlob(null); setErr(false);
    if (window.OrchisAPI && repoId && path && path !== "README.md") {
      window.OrchisAPI.get(`/v1/repos/${repoId}/blob?path=${encodeURIComponent(path)}`)
        .then(b => { if (!cancelled) setBlob(b); })
        .catch(() => { if (!cancelled) setErr(true); });
    }
    return () => { cancelled = true; };
  }, [repoId, path]);

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
          <span className="muted" style={{ fontSize: 11.5 }}>{blob.lines} lines · {fmtBytes(blob.size)} · {blob.lang}</span>
          <button className="btn ghost sm icon"><Icons.Copy size={12} /></button>
        </div>
        <CodeBlock code={blob.content} />
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

function inlineMd(s) {
  return s
    .replace(/`([^`]+)`/g, "<code style=\"font-family: var(--font-mono); background: var(--bg-2); padding: 1px 5px; border-radius: 4px; font-size: 0.9em;\">$1</code>")
    .replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>")
    .replace(/\[([^\]]+)\]\(([^)]+)\)/g, "<a href=\"#\" style=\"color: var(--accent); text-decoration: none;\">$1</a>");
}

function RecentActivityView() {
  return (
    <div style={{ padding: "20px 24px" }}>
      <div className="card" style={{ padding: 4 }}>
        {ACTIVITY.slice(0, 4).map(a => (
          <div key={a.id} style={{ display: "flex", alignItems: "center", gap: 10, padding: "10px 12px", borderBottom: "1px solid var(--line)", fontSize: 13 }}>
            <span className="avatar" style={{ background: a.actor.color, width: 20, height: 20, fontSize: 9 }}>{a.actor.initials}</span>
            <div style={{ flex: 1 }}>
              <strong style={{ fontWeight: 500 }}>{a.actor.name}</strong> <span className="muted">— {a.title}</span>
            </div>
            <span className="subtle" style={{ fontSize: 11 }}>{a.when}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

// Split panel — shows PRs, Issues, or Actions while you're in the code.
function SplitPanel({ content, close, setRoute }) {
  const [pulls, setPulls] = React.useState(null);
  React.useEffect(() => {
    if (content.type === "prs" && window.OrchisAPI && content.repo) {
      window.OrchisAPI.get(`/v1/repos/${content.repo}/pulls?state=open`).then(setPulls).catch(() => setPulls([]));
    }
  }, [content.type, content.repo]);
  const prList = pulls != null ? pulls : PRS.filter(p => p.repo === content.repo);
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
        {content.type === "prs" ? (
          <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
            {prList.length === 0 ? <div className="subtle" style={{ fontSize: 12, padding: 8 }}>No open pull requests.</div> : null}
            {prList.map(p => (
              <button key={p.id} className="card" style={repoStyles.prMini} onClick={() => setRoute({ view: "pr", pr: p.id, repo: content.repo })}>
                <Icons.PR size={14} style={{ color: "var(--accent)" }} />
                <div style={{ flex: 1, minWidth: 0, textAlign: "left" }}>
                  <div style={{ fontSize: 13, fontWeight: 450, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{p.title}</div>
                  <div className="subtle" style={{ fontSize: 11, marginTop: 2 }}>#{p.id} · by {p.author.name} · {p.updated}</div>
                </div>
                <span className="chip" style={{ height: 18, fontSize: 10.5 }}>+{p.additions} −{p.deletions}</span>
              </button>
            ))}
          </div>
        ) : content.type === "issues" ? (
          <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
            <IssueMini num={911} title="Memory growth under fallback-only routing" assignee={USERS.me} />
            <IssueMini num={908} title="Document the `--no-fallback` flag in deploy.md" assignee={USERS.jana} />
            <IssueMini num={902} title="Add Prometheus metric for dropped envelopes" assignee={null} />
            <IssueMini num={891} title="Crash on empty destination string" assignee={USERS.sam} closed />
          </div>
        ) : (
          <ActionsList />
        )}
      </div>
    </>
  );
}

function IssueMini({ num, title, assignee, closed }) {
  return (
    <div className="card" style={repoStyles.prMini}>
      <Icons.Issue size={14} style={{ color: closed ? "var(--purple)" : "var(--info)" }} />
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ fontSize: 13, fontWeight: 450, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{title}</div>
        <div className="subtle" style={{ fontSize: 11, marginTop: 2 }}>#{num}{closed ? " · closed" : ""}</div>
      </div>
      {assignee ? <span className="avatar" style={{ background: assignee.color, width: 18, height: 18, fontSize: 8 }}>{assignee.initials}</span> : null}
    </div>
  );
}

function ActionsList() {
  const runs = [
    { id: 1, name: "ci", branch: "main", commit: "a3f9c12", status: "ok", dur: "2m 41s", when: "12 min ago", actor: USERS.bot },
    { id: 2, name: "ci", branch: "jana/bounded-fallback", commit: "8c1bba0", status: "pending", dur: "running…", when: "8 min ago", actor: USERS.jana },
    { id: 3, name: "release", branch: "main", commit: "f4218a1", status: "ok", dur: "5m 03s", when: "yesterday", actor: USERS.bot },
    { id: 4, name: "ci", branch: "sam/diff-contrast", commit: "11a229c", status: "fail", dur: "1m 12s", when: "yesterday", actor: USERS.sam },
  ];
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
      {runs.map(r => (
        <div key={r.id} className="card" style={repoStyles.prMini}>
          <span style={{
            width: 8, height: 8, borderRadius: 999,
            background: r.status === "ok" ? "var(--accent)" : r.status === "fail" ? "var(--danger)" : "var(--warn)",
            boxShadow: r.status === "pending" ? "0 0 0 3px color-mix(in oklab, var(--warn) 25%, transparent)" : "none",
          }} />
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ fontSize: 13, fontWeight: 450 }}>{r.name} <span className="mono subtle" style={{ fontSize: 11.5 }}>· {r.branch}@{r.commit}</span></div>
            <div className="subtle" style={{ fontSize: 11, marginTop: 2 }}>{r.dur} · {r.when}</div>
          </div>
          <span className="avatar" style={{ background: r.actor.color, width: 18, height: 18, fontSize: 8 }}>{r.actor.initials}</span>
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
