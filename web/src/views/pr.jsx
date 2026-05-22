// PR review screen — clean, dense, conversational comments inline.
function PRView({ prId, repo, setRoute }) {
  const [pr, setPr] = React.useState(() => PRS.find(p => p.id === prId) || PRS[0]);
  const [files, setFiles] = React.useState(null);
  const [commits, setCommits] = React.useState(null);
  const [checks, setChecks] = React.useState(null);
  const [comments, setComments] = React.useState(null);
  const [tab, setTab] = React.useState("files");        // conversation | files | commits | checks
  const [reviewing, setReviewing] = React.useState(false);
  const [reviewBody, setReviewBody] = React.useState("");
  const [reviewVerdict, setReviewVerdict] = React.useState("approve");
  const [openComment, setOpenComment] = React.useState(null);  // {fileIdx, lineKey}
  const [draftComments, setDraftComments] = React.useState({});

  const base = repo ? `/v1/repos/${repo}/pulls/${prId}` : null;
  const reload = React.useCallback(() => {
    if (!window.OrchisAPI || !base) return;
    window.OrchisAPI.get(base).then(setPr).catch(() => {});
    window.OrchisAPI.get(base + "/files").then(setFiles).catch(() => setFiles([]));
    window.OrchisAPI.get(base + "/commits").then(setCommits).catch(() => setCommits([]));
    window.OrchisAPI.get(base + "/checks").then(setChecks).catch(() => setChecks([]));
    window.OrchisAPI.get(base + "/comments").then(setComments).catch(() => setComments([]));
  }, [base]);
  React.useEffect(() => { reload(); }, [reload]);

  const [merging, setMerging] = React.useState(false);
  const [actionErr, setActionErr] = React.useState("");

  const doMerge = async (strategy) => {
    setActionErr(""); setMerging(true);
    try {
      await window.OrchisAPI.post(base + "/merge", { strategy: strategy || "merge" });
      reload();
    } catch (e) {
      setActionErr("Merge failed — likely a conflict, or you're not the repo owner.");
    } finally { setMerging(false); }
  };
  const doReview = async (verdict, body) => {
    if (!window.OrchisAPI || !base) return;
    await window.OrchisAPI.post(base + "/reviews", { verdict, body, comments: [] }).catch(() => {});
    reload();
  };
  const postComment = async (body, extra) => {
    if (!window.OrchisAPI || !base || !body.trim()) return;
    await window.OrchisAPI.post(base + "/comments", { body, ...(extra || {}) }).catch(() => {});
    reload();
  };

  // AI assist (uses the requesting user's configured model).
  const [modelOk, setModelOk] = React.useState(false);
  React.useEffect(() => {
    if (window.OrchisAPI) {
      window.OrchisAPI.get("/v1/me/scanner").then(c => setModelOk(!!(c.hasKey || c.baseUrl))).catch(() => {});
    }
  }, []);
  const [aiBusy, setAiBusy] = React.useState("");
  const [aiResult, setAiResult] = React.useState(null); // { label, text, canApply }
  const runAI = async (label, path, canApply) => {
    if (!base) return;
    setAiBusy(path); setAiResult(null);
    try {
      const r = await window.OrchisAPI.post(base + "/" + path, {});
      setAiResult({ label, text: r.summary || r.explanation || r.description || "", canApply: !!canApply });
    } catch (e) {
      setAiResult({ label, text: "_Could not run. Make sure your model is configured in Developer settings → Scanner._", canApply: false });
    } finally { setAiBusy(""); }
  };
  const applyDraft = async () => {
    if (!aiResult) return;
    await window.OrchisAPI.patch(base, { body: aiResult.text }).catch(() => {});
    setAiResult(null); reload();
  };

  if (!pr) return <div style={{ padding: 40 }} className="muted">Loading pull request…</div>;
  const statusLabel = (pr.status || "open").charAt(0).toUpperCase() + (pr.status || "open").slice(1);

  return (
    <div style={prStyles.shell}>
      {/* Header */}
      <div style={prStyles.header}>
        <button className="btn ghost icon sm" onClick={() => setRoute({ view: "prs" })} title="Back"><Icons.Chevron size={12} style={{ transform: "rotate(180deg)" }} /></button>
        <div style={{ minWidth: 0, flex: 1 }}>
          <div className="row" style={{ gap: 8, marginBottom: 2 }}>
            <span className="chip accent"><Icons.PR size={11} /> {statusLabel}</span>
            <span className="mono subtle" style={{ fontSize: 12 }}>{pr.repo} #{pr.id}</span>
            {(pr.labels || []).map(l => <span key={l.name} className={"chip " + l.color}>{l.name}</span>)}
          </div>
          <h1 style={prStyles.title}>{pr.title}</h1>
          <div className="row" style={{ gap: 8, marginTop: 6, color: "var(--fg-2)", fontSize: 12.5, flexWrap: "wrap" }}>
            <span className="avatar" style={{ background: pr.author.color, width: 18, height: 18, fontSize: 8 }}>{pr.author.initials}</span>
            <span><strong style={{ color: "var(--fg-1)", fontWeight: 500 }}>{pr.author.name}</strong> wants to merge</span>
            <span className="mono chip">{pr.commits} commits</span>
            <span>from</span>
            <span className="mono chip">{pr.branch}</span>
            <span>into</span>
            <span className="mono chip">{pr.base}</span>
            <span className="subtle">· updated {pr.updated}</span>
          </div>
        </div>

        <div className="row" style={{ gap: 6, flexShrink: 0 }}>
          {pr.status === "merged" ? (
            <span className="chip" style={{ color: "var(--purple, var(--accent))" }}>Merged</span>
          ) : (
            <>
              <button className="btn sm" onClick={() => setReviewing(true)}>
                <Icons.Check size={12} /> Review
              </button>
              <button className="btn primary sm" disabled={merging} onClick={() => doMerge("merge")}>
                <Icons.Branch size={12} /> {merging ? "Merging…" : "Merge"}
              </button>
            </>
          )}
        </div>
      </div>
      {actionErr ? <div style={{ padding: "8px 24px", color: "var(--danger)", fontSize: 12.5, background: "var(--bg-1)", borderBottom: "1px solid var(--line)" }}>{actionErr}</div> : null}

      {/* Compact status strip */}
      <div style={prStyles.statusStrip}>
        <Stat icon={<Icons.Check size={12} />} label="checks" value={`${pr.checks.passed} passed${pr.checks.pending ? `, ${pr.checks.pending} pending` : ''}`} good={pr.checks.failed === 0} />
        <Stat icon={<Icons.Eye size={12} />} label="reviewers" value={
          <span className="row" style={{ gap: -4 }}>
            {pr.reviewers.map((r, i) => (
              <span key={r.id} className="avatar" style={{ background: r.color, width: 18, height: 18, fontSize: 8, marginLeft: i ? -6 : 0, border: "2px solid var(--bg-1)" }}>{r.initials}</span>
            ))}
            <span style={{ marginLeft: 8 }}>{pr.reviewers.length} requested</span>
          </span>
        } />
        <Stat icon={<Icons.Diff size={12} />} label="diff" value={
          <span><span style={{ color: "var(--accent)" }}>+{pr.additions}</span> / <span style={{ color: "var(--danger)" }}>−{pr.deletions}</span> · {pr.files} files</span>
        } />
        <Stat icon={<Icons.Branch size={12} />} label="mergeable" value={<span style={{ color: "var(--accent)" }}>clean — no conflicts</span>} good />
        <span className="spacer" />
        <span className="muted" style={{ fontSize: 11.5 }}>{pr.comments} comments</span>
      </div>

      {/* AI assist */}
      {modelOk ? (
        <div className="row" style={{ gap: 6, padding: "8px 24px", borderBottom: "1px solid var(--line)", background: "var(--bg-1)" }}>
          <span className="subtle" style={{ fontSize: 11, textTransform: "uppercase", letterSpacing: "0.05em", marginRight: 4 }}>AI assist</span>
          <button className="btn sm" disabled={!!aiBusy} onClick={() => runAI("Summary", "summarize", false)}>{aiBusy === "summarize" ? "Summarizing…" : "Summarize"}</button>
          <button className="btn sm" disabled={!!aiBusy} onClick={() => runAI("Explanation", "explain", false)}>{aiBusy === "explain" ? "Explaining…" : "Explain diff"}</button>
          <button className="btn sm" disabled={!!aiBusy} onClick={() => runAI("Drafted description", "draft-description", true)}>{aiBusy === "draft-description" ? "Drafting…" : "Draft description"}</button>
          <span className="spacer" />
          <span className="subtle" style={{ fontSize: 11 }}>uses your configured model</span>
        </div>
      ) : null}
      {aiResult ? (
        <div style={{ padding: "12px 24px 16px", borderBottom: "1px solid var(--line)", background: "var(--bg-2)" }}>
          <div className="row" style={{ marginBottom: 8, gap: 8 }}>
            <strong style={{ fontSize: 13 }}>AI · {aiResult.label}</strong>
            <span className="spacer" />
            {aiResult.canApply ? <button className="btn sm" onClick={applyDraft}>Apply to PR description</button> : null}
            <button className="btn ghost icon sm" onClick={() => setAiResult(null)} title="Dismiss"><Icons.Close size={11} /></button>
          </div>
          <div style={{ fontSize: 13.5, lineHeight: 1.6, maxWidth: 860 }}>{parseMd(aiResult.text || "")}</div>
        </div>
      ) : null}

      {/* Tabs */}
      <div style={prStyles.tabs}>
        <PRTab active={tab === "conversation"} onClick={() => setTab("conversation")}>Conversation <span className="chip" style={{ height: 16, fontSize: 10 }}>{pr.comments}</span></PRTab>
        <PRTab active={tab === "files"} onClick={() => setTab("files")}>Files changed <span className="chip" style={{ height: 16, fontSize: 10 }}>{pr.files}</span></PRTab>
        <PRTab active={tab === "commits"} onClick={() => setTab("commits")}>Commits <span className="chip" style={{ height: 16, fontSize: 10 }}>{pr.commits}</span></PRTab>
        <PRTab active={tab === "checks"} onClick={() => setTab("checks")}>Checks <span className="chip" style={{ height: 16, fontSize: 10 }}>{pr.checks.passed + pr.checks.failed + pr.checks.pending}</span></PRTab>
      </div>

      <div style={prStyles.body}>
        {tab === "files" ? <FilesChanged files={files != null ? files : PR_DIFF_FILES} openComment={openComment} setOpenComment={setOpenComment} postComment={postComment} canComment={!!base} /> : null}
        {tab === "conversation" ? <Conversation pr={pr} comments={comments} postComment={postComment} /> : null}
        {tab === "commits" ? <CommitsList commits={commits} /> : null}
        {tab === "checks" ? <ChecksList checks={checks} /> : null}
      </div>

      {reviewing ? (
        <ReviewBar
          body={reviewBody}
          setBody={setReviewBody}
          verdict={reviewVerdict}
          setVerdict={setReviewVerdict}
          onCancel={() => setReviewing(false)}
          onSubmit={() => { doReview(reviewVerdict, reviewBody); setReviewing(false); setReviewBody(""); }}
        />
      ) : null}
    </div>
  );
}

function PRTab({ active, onClick, children }) {
  return (
    <button onClick={onClick} style={{
      display: "inline-flex", alignItems: "center", gap: 6,
      height: 40, padding: "0 14px",
      background: "transparent", border: "none",
      borderBottom: active ? "2px solid var(--accent)" : "2px solid transparent",
      color: active ? "var(--fg)" : "var(--fg-2)",
      cursor: "pointer", font: "inherit", fontSize: 13, fontWeight: active ? 500 : 450,
    }}>{children}</button>
  );
}

function Stat({ icon, label, value, good }) {
  return (
    <div className="row" style={{ gap: 8 }}>
      <span style={{ color: good ? "var(--accent)" : "var(--fg-2)", display: "inline-flex" }}>{icon}</span>
      <span className="subtle" style={{ fontSize: 11, textTransform: "uppercase", letterSpacing: "0.05em" }}>{label}</span>
      <span style={{ fontSize: 12.5, color: "var(--fg-1)" }}>{value}</span>
    </div>
  );
}

function FilesChanged({ files, openComment, setOpenComment, postComment, canComment }) {
  return (
    <div style={{ padding: "16px 20px 40px", maxWidth: 1100, margin: "0 auto" }}>
      {files.map((f, fi) => (
        <FileDiff key={f.path} file={f} fi={fi} openComment={openComment} setOpenComment={setOpenComment} postComment={postComment} canComment={canComment} />
      ))}
    </div>
  );
}

// anchorFor maps a diff line to the (line, side) a comment should attach to:
// added/context lines anchor to the right (new) side; deleted lines to the
// left (old) side. Returns null for lines with no usable line number.
function anchorFor(ln) {
  const oldN = parseInt(ln.num[0], 10);
  const newN = parseInt(ln.num[1], 10);
  if (ln.type === "del") return Number.isNaN(oldN) ? null : { line: oldN, side: "left" };
  return Number.isNaN(newN) ? null : { line: newN, side: "right" };
}

function FileDiff({ file, fi, openComment, setOpenComment, postComment, canComment }) {
  const [collapsed, setCollapsed] = React.useState(false);
  const [busy, setBusy] = React.useState(false);

  const submitComment = async (ln, body) => {
    const a = anchorFor(ln);
    if (!a || !body.trim()) { setOpenComment(null); return; }
    setBusy(true);
    try {
      await postComment(body.trim(), { path: file.path, line: a.line, side: a.side });
      setOpenComment(null);
    } finally { setBusy(false); }
  };

  return (
    <div className="card" style={{ marginBottom: 16, overflow: "hidden" }}>
      <div style={prStyles.fileHead} onClick={() => setCollapsed(c => !c)}>
        <Icons.Chevron size={11} style={{ color: "var(--fg-3)", transform: collapsed ? "none" : "rotate(90deg)", transition: "transform 80ms" }} />
        <span className="mono" style={{ fontSize: 12.5, fontWeight: 500 }}>{file.path}</span>
        <span className="chip" style={{ height: 18 }}><span style={{ color: "var(--accent)" }}>+{file.additions}</span></span>
        <span className="chip" style={{ height: 18 }}><span style={{ color: "var(--danger)" }}>−{file.deletions}</span></span>
        <span className="spacer" />
        <span className="muted" style={{ fontSize: 11.5 }}>{file.comments.length} comment{file.comments.length === 1 ? "" : "s"}</span>
        <button className="btn ghost sm" onClick={(e) => { e.stopPropagation(); }} title="View file">View</button>
      </div>
      {collapsed ? null : (
        <div style={{ overflowX: "auto" }}>
          {file.hunks.length === 0 ? (
            <div style={{ padding: 20, textAlign: "center", color: "var(--fg-3)", fontSize: 12.5 }}>Load diff…</div>
          ) : file.hunks.map((h, hi) => (
            <div key={hi}>
              <div style={prStyles.hunkHead}><span className="mono" style={{ fontSize: 11.5, color: "var(--fg-2)" }}>{h.header}</span></div>
              {h.lines.map((ln, li) => {
                const lineKey = `${fi}-${hi}-${li}`;
                const a = anchorFor(ln);
                const inlineComments = (file.comments || []).filter(c =>
                  a && c.line === a.line && (c.side || "right") === a.side);
                return (
                  <React.Fragment key={li}>
                    <DiffLine ln={ln} canComment={canComment && !!a} onComment={() => setOpenComment(lineKey)} />
                    {inlineComments.map((c, ci) => <InlineComment key={ci} c={c} />)}
                    {openComment === lineKey ? (
                      <NewCommentBox busy={busy} onCancel={() => setOpenComment(null)} onSubmit={(body) => submitComment(ln, body)} />
                    ) : null}
                  </React.Fragment>
                );
              })}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function DiffLine({ ln, onComment, canComment }) {
  const bg = ln.type === "add" ? "color-mix(in oklab, var(--accent) 10%, var(--bg))"
    : ln.type === "del" ? "color-mix(in oklab, var(--danger) 10%, var(--bg))"
    : "transparent";
  const marker = ln.type === "add" ? "+" : ln.type === "del" ? "−" : " ";
  const markerColor = ln.type === "add" ? "var(--accent)" : ln.type === "del" ? "var(--danger)" : "var(--fg-3)";

  return (
    <div style={{ ...prStyles.diffLine, background: bg }} className="diff-line">
      <span style={prStyles.diffNum}>{ln.num[0]}</span>
      <span style={prStyles.diffNum}>{ln.num[1]}</span>
      {canComment ? (
        <button style={prStyles.diffAdd} onClick={onComment} title="Add a comment on this line">
          <Icons.Plus size={10} />
        </button>
      ) : <span style={{ width: 18 }} />}
      <span style={{ width: 16, textAlign: "center", color: markerColor, fontFamily: "var(--font-mono)" }}>{marker}</span>
      <code style={prStyles.diffCode}>{ln.text}</code>
    </div>
  );
}

function InlineComment({ c }) {
  return (
    <div style={prStyles.inlineComment}>
      <div className="row" style={{ gap: 8, marginBottom: 6 }}>
        <span className="avatar" style={{ background: c.author.color, width: 20, height: 20, fontSize: 9 }}>{c.author.initials}</span>
        <span style={{ fontSize: 12.5, fontWeight: 500 }}>{c.author.name}</span>
        <span className="subtle" style={{ fontSize: 11.5 }}>{c.when}</span>
        {c.side ? <span className="mono subtle" style={{ fontSize: 10.5 }}>· {c.side === "left" ? "old" : "new"} L{c.line}</span> : null}
      </div>
      <div style={{ fontSize: 13, lineHeight: 1.5, color: "var(--fg-1)", whiteSpace: "pre-wrap" }}>{c.body}</div>
    </div>
  );
}

function NewCommentBox({ onCancel, onSubmit, busy }) {
  const [v, setV] = React.useState("");
  const submit = () => { if (v.trim()) onSubmit(v); };
  return (
    <div style={prStyles.inlineComment}>
      <textarea
        value={v}
        onChange={e => setV(e.target.value)}
        onKeyDown={e => { if ((e.metaKey || e.ctrlKey) && e.key === "Enter") { e.preventDefault(); submit(); } }}
        placeholder="Leave a comment… ⌘+Enter to submit"
        style={{
          width: "100%", minHeight: 76, padding: "8px 10px",
          border: "1px solid var(--line)", borderRadius: 6,
          background: "var(--bg)", color: "var(--fg)",
          fontFamily: "inherit", fontSize: 13,
          resize: "vertical",
        }}
        autoFocus
      />
      <div className="row" style={{ gap: 6, marginTop: 8 }}>
        <button className="btn sm" onClick={onCancel}>Cancel</button>
        <span className="spacer" />
        <button className="btn primary sm" onClick={submit} disabled={!v.trim() || busy}>{busy ? "Posting…" : "Comment"}</button>
      </div>
    </div>
  );
}

function Conversation({ pr, comments, postComment }) {
  const [draft, setDraft] = React.useState("");
  // Top-level comments only (no line/path) — line comments live in the diff.
  const topLevel = (comments || []).filter(c => c.line == null && !c.path);
  return (
    <div style={{ maxWidth: 820, margin: "0 auto", padding: "20px 24px 40px" }}>
      <ConvBlock author={pr.author} when="">
        <p style={{ margin: 0, lineHeight: 1.55 }}>{pr.description || <span className="muted">No description provided.</span>}</p>
      </ConvBlock>
      {topLevel.map((c, i) => (
        <ConvBlock key={i} author={c.author} when={c.when}>
          <p style={{ margin: 0, lineHeight: 1.55 }}>{c.body}</p>
        </ConvBlock>
      ))}

      <div className="card" style={{ marginTop: 18, padding: 14 }}>
        <textarea value={draft} onChange={e => setDraft(e.target.value)} placeholder="Leave a comment…" style={{
          width: "100%", minHeight: 90, padding: 10, border: "1px solid var(--line)", borderRadius: 6,
          background: "var(--bg)", color: "var(--fg)", fontFamily: "inherit", fontSize: 13, resize: "vertical",
        }} />
        <div className="row" style={{ marginTop: 10, gap: 6 }}>
          <span className="muted" style={{ fontSize: 11.5 }}>Supports markdown</span>
          <span className="spacer" />
          <button className="btn primary sm" disabled={!draft.trim()} onClick={() => { postComment && postComment(draft); setDraft(""); }}>Comment</button>
        </div>
      </div>
    </div>
  );
}

function ConvBlock({ author, when, children }) {
  return (
    <div style={{ display: "flex", gap: 12, marginBottom: 16 }}>
      <span className="avatar" style={{ background: author.color, width: 28, height: 28, fontSize: 11, flexShrink: 0 }}>{author.initials}</span>
      <div className="card" style={{ flex: 1, padding: 12 }}>
        <div className="row" style={{ gap: 8, marginBottom: 8 }}>
          <span style={{ fontSize: 13, fontWeight: 500 }}>{author.name}</span>
          <span className="subtle" style={{ fontSize: 11.5 }}>{when}</span>
        </div>
        <div style={{ fontSize: 13.5, color: "var(--fg-1)" }}>{children}</div>
      </div>
    </div>
  );
}

function CommitsList({ commits }) {
  // Real commits: {sha, short, message, author, email, date}. Fallback to mock when null (offline).
  const mock = [
    { short: "8c1bba0", message: "router: log a warning when fallback is used", author: "Jana", date: "" },
    { short: "0aa9ee4", message: "router: switch fallback sink to bounded channel", author: "Jana", date: "" },
  ];
  const list = commits != null ? commits : mock;
  return (
    <div style={{ maxWidth: 820, margin: "0 auto", padding: "20px 24px" }}>
      {list.length === 0 ? (
        <div className="card" style={{ padding: "20px", color: "var(--fg-3)", fontSize: 13 }}>No commits ahead of base.</div>
      ) : (
      <div className="card" style={{ padding: 4 }}>
        {list.map((c, i) => (
          <div key={c.sha || i} style={{ display: "flex", alignItems: "center", gap: 12, padding: "10px 14px", borderBottom: i < list.length - 1 ? "1px solid var(--line)" : "none" }}>
            <Icons.Commit size={14} style={{ color: "var(--fg-3)" }} />
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ fontSize: 13, fontWeight: 450, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{(c.message || "").split("\n")[0]}</div>
              <div className="subtle" style={{ fontSize: 11.5, marginTop: 2 }}>{c.author}{c.date ? " · " + (c.date || "").slice(0, 10) : ""}</div>
            </div>
            <span className="mono chip">{c.short || (c.sha || "").slice(0, 7)}</span>
          </div>
        ))}
      </div>
      )}
    </div>
  );
}

function ChecksList({ checks }) {
  // Real checks: {name, status, detail, externalUrl}. Fallback to mock when null.
  const mock = [
    { name: "ci / build", status: "ok", detail: "" },
    { name: "ci / test", status: "ok", detail: "" },
  ];
  const list = checks != null ? checks : mock;
  return (
    <div style={{ maxWidth: 820, margin: "0 auto", padding: "20px 24px" }}>
      {list.length === 0 ? (
        <div className="card" style={{ padding: "20px", color: "var(--fg-3)", fontSize: 13 }}>No checks have run on the latest commit yet.</div>
      ) : (
      <div className="card" style={{ padding: 4 }}>
        {list.map((c, i) => (
          <div key={c.name} style={{ display: "flex", alignItems: "flex-start", gap: 12, padding: "10px 14px", borderBottom: i < list.length - 1 ? "1px solid var(--line)" : "none" }}>
            <span style={{
              width: 8, height: 8, borderRadius: 999, marginTop: 5, flexShrink: 0,
              background: c.status === "ok" ? "var(--accent)" : c.status === "fail" ? "var(--danger)" : "var(--warn)",
              boxShadow: c.status === "pending" ? "0 0 0 3px color-mix(in oklab, var(--warn) 25%, transparent)" : "none",
            }} />
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ fontSize: 13 }}>{c.name} <span className="subtle" style={{ fontSize: 11 }}>· {c.status}</span></div>
              {c.detail ? <div className="subtle" style={{ fontSize: 11.5, marginTop: 3, whiteSpace: "pre-wrap" }}>{c.detail}</div> : null}
            </div>
            {c.externalUrl ? <a className="btn ghost sm" href={c.externalUrl} target="_blank" rel="noreferrer">Details</a> : null}
          </div>
        ))}
      </div>
      )}
    </div>
  );
}

function ReviewBar({ body, setBody, verdict, setVerdict, onCancel, onSubmit }) {
  return (
    <div style={prStyles.reviewBar}>
      <div className="row" style={{ gap: 8, marginBottom: 8 }}>
        <span style={{ fontWeight: 500, fontSize: 13 }}>Submit review</span>
        <span className="spacer" />
        <button className="btn ghost icon sm" onClick={onCancel}><Icons.Close size={11} /></button>
      </div>
      <textarea
        value={body}
        onChange={e => setBody(e.target.value)}
        placeholder="Summary of your review (optional)"
        style={{
          width: "100%", minHeight: 80, padding: 10,
          border: "1px solid var(--line)", borderRadius: 6,
          background: "var(--bg)", color: "var(--fg)",
          fontFamily: "inherit", fontSize: 13, resize: "vertical",
        }}
      />
      <div className="row" style={{ gap: 10, marginTop: 10 }}>
        <label className="row" style={{ gap: 6, cursor: "pointer" }}>
          <input type="radio" checked={verdict === "comment"} onChange={() => setVerdict("comment")} /> Comment
        </label>
        <label className="row" style={{ gap: 6, cursor: "pointer" }}>
          <input type="radio" checked={verdict === "approve"} onChange={() => setVerdict("approve")} />
          <span style={{ color: "var(--accent)" }}>Approve</span>
        </label>
        <label className="row" style={{ gap: 6, cursor: "pointer" }}>
          <input type="radio" checked={verdict === "changes"} onChange={() => setVerdict("changes")} />
          <span style={{ color: "var(--danger)" }}>Request changes</span>
        </label>
        <span className="spacer" />
        <button className="btn primary sm" onClick={onSubmit}>Submit review</button>
      </div>
    </div>
  );
}

const prStyles = {
  shell: { display: "flex", flexDirection: "column", height: "100%", overflow: "hidden" },
  header: {
    display: "flex", alignItems: "flex-start", gap: 12,
    padding: "16px 24px",
    borderBottom: "1px solid var(--line)",
    background: "var(--bg-1)",
  },
  title: { fontSize: 22, fontWeight: 500, letterSpacing: "-0.015em", margin: 0, lineHeight: 1.25 },
  statusStrip: {
    display: "flex", alignItems: "center", gap: 22,
    padding: "10px 24px",
    borderBottom: "1px solid var(--line)",
    background: "var(--bg-1)",
    flexWrap: "wrap",
  },
  tabs: {
    display: "flex", alignItems: "center", gap: 2,
    padding: "0 16px",
    borderBottom: "1px solid var(--line)",
    background: "var(--bg-1)",
    height: 40,
  },
  body: { flex: 1, overflowY: "auto" },
  fileHead: {
    display: "flex", alignItems: "center", gap: 10,
    padding: "10px 14px",
    background: "var(--bg-2)",
    borderBottom: "1px solid var(--line)",
    cursor: "pointer",
  },
  hunkHead: {
    padding: "6px 14px",
    background: "color-mix(in oklab, var(--info) 6%, var(--bg))",
    color: "var(--fg-2)",
    borderBottom: "1px solid var(--line)",
  },
  diffLine: {
    display: "flex", alignItems: "stretch",
    fontFamily: "var(--font-mono)", fontSize: 12.5,
    lineHeight: 1.6,
    position: "relative",
  },
  diffNum: {
    display: "inline-block",
    width: 50, paddingRight: 10, textAlign: "right",
    color: "var(--fg-3)", fontSize: 11,
    userSelect: "none",
    flexShrink: 0,
    borderRight: "1px solid var(--line)",
  },
  diffAdd: {
    width: 16, height: 18, marginTop: 1,
    border: "none", background: "var(--accent)",
    color: "var(--accent-fg)",
    display: "inline-flex", alignItems: "center", justifyContent: "center",
    cursor: "pointer",
    opacity: 0,
    borderRadius: 3,
    flexShrink: 0,
    marginLeft: 4, marginRight: 2,
    alignSelf: "center",
    transition: "opacity 80ms",
  },
  diffCode: { whiteSpace: "pre", padding: "0 12px", color: "var(--fg-1)", flex: 1 },
  inlineComment: {
    padding: "12px 16px 14px 70px",
    background: "var(--bg-1)",
    borderTop: "1px solid var(--line)",
    borderBottom: "1px solid var(--line)",
  },
  reviewBar: {
    position: "absolute",
    right: 24, bottom: 24,
    width: 420,
    background: "var(--bg-1)",
    border: "1px solid var(--line-strong)",
    borderRadius: 12,
    boxShadow: "var(--shadow-lg)",
    padding: 14,
    zIndex: 50,
  },
};

// Hover state for diff add button
const prStyleTag = document.createElement("style");
prStyleTag.textContent = `
.diff-line:hover button { opacity: 0.7 !important; }
.diff-line button:hover { opacity: 1 !important; }
`;
document.head.appendChild(prStyleTag);

window.PRView = PRView;

// PR list view — sidebar 'pull requests'
function PRsView({ setRoute }) {
  const [filter, setFilter] = React.useState("review-requested");
  const [pulls, setPulls] = React.useState(null);

  React.useEffect(() => {
    if (window.OrchisAPI) {
      window.OrchisAPI.get("/v1/pulls?filter=" + encodeURIComponent(filter))
        .then(setPulls).catch(() => setPulls([]));
    }
  }, [filter]);

  const list = pulls != null ? pulls : (window.OrchisAPI ? [] : PRS);

  return (
    <div style={{ overflowY: "auto", height: "100%" }}>
      <div style={{ maxWidth: 1100, margin: "0 auto", padding: "32px 32px 60px" }} className="fade-in">
        <h1 style={{ fontSize: 26, fontWeight: 500, letterSpacing: "-0.02em", margin: "0 0 6px" }}>Pull requests</h1>
        <p className="muted" style={{ marginTop: 0, fontSize: 14 }}>Across all your repos.</p>

        <div className="row" style={{ gap: 6, margin: "20px 0 14px", flexWrap: "wrap" }}>
          {[
            { id: "review-requested", label: "Review requested" },
            { id: "yours", label: "Yours" },
            { id: "mentioned", label: "Mentioned" },
            { id: "all", label: "All open" },
          ].map(f => (
            <button key={f.id} onClick={() => setFilter(f.id)} className="btn sm" style={filter === f.id ? { background: "var(--accent-soft)", color: "var(--accent)", borderColor: "var(--accent-line)" } : {}}>
              {f.label}
            </button>
          ))}
        </div>

        {list.length === 0 ? (
          <div className="card" style={{ padding: "24px 18px", color: "var(--fg-3)", fontSize: 13.5 }}>
            No pull requests here yet. Push a branch and open one with <span className="mono">POST /v1/repos/&lt;you&gt;/&lt;repo&gt;/pulls</span>.
          </div>
        ) : (
        <div className="card" style={{ overflow: "hidden" }}>
          {list.map(p => (
            <button key={p.repo + "#" + p.id} onClick={() => setRoute({ view: "pr", pr: p.id, repo: p.repo })} style={{
              display: "flex", alignItems: "center", gap: 12,
              padding: "14px 16px",
              background: "transparent", border: "none", borderBottom: "1px solid var(--line)",
              width: "100%", textAlign: "left", cursor: "pointer", font: "inherit", color: "inherit",
            }}>
              <Icons.PR size={15} style={{ color: "var(--accent)", flexShrink: 0 }} />
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ fontSize: 14, fontWeight: 450, marginBottom: 3 }}>{p.title}</div>
                <div className="row" style={{ gap: 10, color: "var(--fg-2)", fontSize: 11.5, flexWrap: "wrap" }}>
                  <span className="mono">{p.repo} #{p.id}</span>
                  <span>by {p.author.name}</span>
                  <span>updated {p.updated}</span>
                  {(p.labels || []).map(l => <span key={l.name} className={"chip " + l.color}>{l.name}</span>)}
                </div>
              </div>
              <span className="row" style={{ gap: -4 }}>
                {(p.reviewers || []).map((r, i) => (
                  <span key={r.id} className="avatar" style={{ background: r.color, width: 20, height: 20, fontSize: 9, marginLeft: i ? -6 : 0, border: "2px solid var(--bg-1)" }}>{r.initials}</span>
                ))}
              </span>
              <span className="chip" style={{ height: 20 }}><span style={{ color: "var(--accent)" }}>+{p.additions}</span> <span style={{ color: "var(--danger)" }}>−{p.deletions}</span></span>
            </button>
          ))}
        </div>
        )}
      </div>
    </div>
  );
}

window.PRsView = PRsView;
