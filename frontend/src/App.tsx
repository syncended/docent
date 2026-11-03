import { useCallback, useEffect, useRef, useState } from "react";
import { Events } from "@wailsio/runtime";
import { ArrowLeft, CaseSensitive, ChevronDown, ChevronRight, File, PanelLeftClose, PanelLeftOpen, FolderPlus, LoaderCircle, Regex, Search, Settings as SettingsIcon, Shapes, Sparkles, SquareFunction, TextSearch, type LucideIcon } from "lucide-react";
import * as api from "./api";
import type { Hit, Progress, Status, Tab } from "./api";
import { Code } from "./Code";
import { Settings } from "./Settings";
import { AgentAnswer, applyEvent, newRun, stepLabel, type Run } from "./Agent";
import { altLabel, cmd, hotkeyLabel, isMac, modLabel } from "./keys";

// shortcut switches to the tab when typed into an empty query.
const tabs: { tab: Tab; label: string; icon: LucideIcon; shortcut?: string }[] = [
  { tab: "all", label: "All", icon: Search },
  { tab: "types", label: "Types", icon: Shapes, shortcut: "#" },
  { tab: "symbols", label: "Symbols", icon: SquareFunction, shortcut: "@" },
  { tab: "files", label: "Files", icon: File, shortcut: "/" },
  { tab: "text", label: "Text", icon: TextSearch },
  { tab: "agent", label: "Agent", icon: Sparkles, shortcut: "?" },
];

type Row = { type: "header"; label: string } | { type: "note"; text: string } | { type: "hit"; hit: Hit } | { type: "agent"; query: string };

// A list that replaces search results, e.g. usages opened from the viewer.
type List = { title: string; hits: Hit[] };

type Viewer = { hit: Hit; content: string };

export function App() {
  const [mode, setMode] = useState<"palette" | "viewer" | "settings">("palette");
  const [tab, setTab] = useState<Tab>("all");
  const [query, setQuery] = useState("");
  const [regex, setRegex] = useState(false);
  const [matchCase, setMatchCase] = useState(false);
  const [run, setRun] = useState<Run | null>(null);
  // sourcesOpen is null until the user toggles it: then sources are shown
  // while the agent searches and collapse once the answer starts.
  const [sourcesOpen, setSourcesOpen] = useState<boolean | null>(null);
  const runId = useRef(0);
  const [rows, setRows] = useState<Row[]>([]);
  const [selected, setSelected] = useState(0);
  const [list, setList] = useState<List | null>(null);
  const [status, setStatus] = useState<Status | null>(null);
  const [progress, setProgress] = useState<Progress | null>(null);
  const [indexVersion, setIndexVersion] = useState(0);
  const [notice, setNotice] = useState("");
  const [viewer, setViewer] = useState<Viewer | null>(null);
  const [dirty, setDirty] = useState(false);
  const [discardArmed, setDiscardArmed] = useState(false);
  const input = useRef<HTMLInputElement>(null);
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const dirtyRef = useRef(false);
  dirtyRef.current = dirty;
  // stepResult moves the viewer to the next or previous result; the global
  // key handler, registered once, calls the latest version.
  const stepResult = useRef<(dir: 1 | -1) => void>(() => {});

  const focusInput = () => setTimeout(() => (input.current?.focus(), input.current?.select()), 0);

  useEffect(() => {
    api.getStatus().then(setStatus);
    const offs = [
      Events.On("palette:shown", () => {
        if (!dirtyRef.current) setMode("palette");
        focusInput();
      }),
      Events.On("settings:open", () => setMode("settings")),
      Events.On("agent:event", (e) => {
        const { run: id, event } = e.data as { run: number; event: api.AgentEvent };
        if (id === runId.current) setRun((r) => r && applyEvent(r, event));
      }),
      Events.On("index:progress", (e) => setProgress(e.data as Progress)),
      Events.On("index:done", () => {
        setProgress(null);
        api.getStatus().then(setStatus);
        setIndexVersion((v) => v + 1);
      }),
    ];
    // Cmd+W hides the window in any mode; unsaved edits stay in the viewer.
    // Cmd+, opens settings.
    const onKey = (e: KeyboardEvent) => {
      if (!cmd(e) || e.shiftKey) return;
      if (e.altKey) {
        if (e.key === "ArrowDown" || e.key === "ArrowUp") {
          e.preventDefault();
          stepResult.current(e.key === "ArrowDown" ? 1 : -1);
        }
        return;
      }
      if (e.key.toLowerCase() === "w") {
        e.preventDefault();
        api.hide();
      } else if (e.key === ",") {
        e.preventDefault();
        if (!dirtyRef.current) setMode("settings");
      }
    };
    window.addEventListener("keydown", onKey, true);
    focusInput();
    return () => (offs.forEach((off) => off()), window.removeEventListener("keydown", onKey, true));
  }, []);

  // Search as the query changes.
  useEffect(() => {
    if (list) return;
    const q = query.trim();
    if (!q || tab === "agent") {
      setRows([]);
      return;
    }
    const ctl = new AbortController();
    const timer = setTimeout(async () => {
      try {
        setRows(await search(tab, q, regex, matchCase, ctl.signal));
        setSelected(0);
        setNotice("");
      } catch (e) {
        if (!ctl.signal.aborted) setNotice((e as Error).message);
      }
    }, 40);
    return () => (clearTimeout(timer), ctl.abort());
  }, [tab, query, regex, matchCase, list, indexVersion]);

  const agentRows: Row[] = (run?.steps ?? []).flatMap((step): Row[] =>
    "note" in step
      ? [{ type: "note", text: step.note }]
      : [{ type: "header", label: stepLabel(step) }, ...step.hits.map((hit): Row => ({ type: "hit", hit }))],
  );
  const agentView = tab === "agent" && !list;
  // contextRows is the search a file was opened from, shown next to the viewer.
  const contextRows: Row[] = list ? list.hits.map((hit) => ({ type: "hit", hit })) : agentView ? agentRows : rows;
  const contextHits = contextRows.flatMap((r) => (r.type === "hit" ? [r.hit] : []));
  const showSources = sourcesOpen ?? run?.status === "running";
  const shownRows: Row[] = list ? list.hits.map((hit) => ({ type: "hit", hit })) : agentView ? (showSources ? agentRows : []) : rows;
  const selectable = shownRows.flatMap((r, i) => (r.type === "header" || r.type === "note" ? [] : [i]));
  const current = shownRows[selectable[Math.min(selected, selectable.length - 1)]];
  const currentHit = current?.type === "hit" ? current.hit : null;

  // The window is a Spotlight-like bar until there is something to show below it.
  const noRoots = status !== null && status.roots.length === 0;
  const layout: api.Layout =
    mode === "viewer" ? "viewer" : mode === "settings" || list || shownRows.length > 0 || (tab === "agent" && run) ? "results" : "compact";
  useEffect(() => {
    api.setLayout(layout);
  }, [layout]);

  const openHit = useCallback(async (hit: Hit) => {
    try {
      const content = await api.readFile(hit.root, hit.path);
      setViewer({ hit, content });
      setDirty(false);
      setDiscardArmed(false);
      setMode("viewer");
      setNotice("");
    } catch (e) {
      setNotice((e as Error).message);
    }
  }, []);

  const ask = async (question: string) => {
    question = question.trim();
    if (!question) return;
    setTab("agent");
    setSelected(0);
    setRun(newRun(question));
    setSourcesOpen(null);
    try {
      const id = await api.askAgent(question);
      runId.current = id;
      setRun((r) => r && { ...r, id });
    } catch (e) {
      setRun((r) => r && { ...r, status: "error", error: (e as Error).message });
    }
  };

  const openLocation = (root: string, path: string, line: number) => {
    const rootName = status?.roots.find((r) => r.id === root)?.name ?? root;
    openHit({ tab: "files", root, rootName, path, line, column: 0, title: path.slice(path.lastIndexOf("/") + 1) });
  };

  const activate = (row?: Row) => {
    if (row?.type === "hit") openHit(row.hit);
    if (row?.type === "agent") ask(row.query);
  };

  const onKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    const shortcut = tabs.find((t) => t.shortcut === e.key);
    if (shortcut && query === "" && shortcut.tab !== tab) {
      e.preventDefault();
      setList(null);
      setTab(shortcut.tab);
      return;
    }
    switch (e.key) {
      case "Tab": {
        e.preventDefault();
        const i = tabs.findIndex((t) => t.tab === tab);
        setList(null);
        setTab(tabs[(i + (e.shiftKey ? tabs.length - 1 : 1)) % tabs.length].tab);
        return;
      }
      case "ArrowDown":
        e.preventDefault();
        setSelected((s) => Math.min(s + 1, selectable.length - 1));
        return;
      case "ArrowUp":
        e.preventDefault();
        setSelected((s) => Math.max(s - 1, 0));
        return;
      case "Enter":
        e.preventDefault();
        // A new question starts the agent; on the same question Enter opens the selected finding.
        if (tab === "agent" && !list && run?.question !== query.trim()) ask(query);
        else activate(current);
        return;
      case "Escape":
        e.preventDefault();
        if (list) setList(null);
        else api.hide();
        return;
    }
    if (e.altKey && e.code === "KeyR" && tab === "text") {
      e.preventDefault();
      setRegex((r) => !r);
    }
    if (e.altKey && e.code === "KeyC" && tab === "text") {
      e.preventDefault();
      setMatchCase((c) => !c);
    }
  };

  const backToPalette = () => {
    if (dirty && !discardArmed) {
      setDiscardArmed(true);
      setNotice(`Unsaved changes: ${modLabel}S to save, Esc again to discard`);
      return;
    }
    setDirty(false);
    setDiscardArmed(false);
    setNotice("");
    setMode("palette");
    focusInput();
  };

  const save = async (text: string) => {
    if (!viewer) return;
    try {
      await api.writeFile(viewer.hit.root, viewer.hit.path, text);
      setDirty(false);
      setDiscardArmed(false);
      setNotice(`Saved ${viewer.hit.path}`);
    } catch (e) {
      setNotice((e as Error).message);
    }
  };

  // addFolder is the first-run path: pick a folder and start indexing it
  // without going through settings.
  const addFolder = async () => {
    try {
      const path = await api.pickFolder();
      if (!path) return focusInput();
      const s = await api.getSettings();
      await api.putSettings({ ...s, roots: [...s.roots, { path, include: null, exclude: null }] });
      setStatus(await api.getStatus());
      focusInput();
    } catch (e) {
      setNotice((e as Error).message);
    }
  };

  stepResult.current = (dir) => {
    if (mode !== "viewer" || !viewer || contextHits.length === 0) return;
    if (dirty) return setNotice("Save or discard changes before leaving the file");
    const i = contextHits.findIndex((h) => sameHit(h, viewer.hit));
    const next = contextHits[(i + dir + contextHits.length) % contextHits.length];
    if (next) openHit(next);
  };

  const openFromContext = (hit: Hit) => {
    if (dirty) return setNotice("Save or discard changes before leaving the file");
    openHit(hit);
  };

  const showList = (title: string, hits: Hit[]) => {
    setList({ title, hits });
    setSelected(0);
    setMode("palette");
    focusInput();
  };

  const goToDefinition = async (word: string) => {
    if (dirty) return setNotice("Save or discard changes before leaving the file");
    const hits = await api.definitions(word);
    if (hits.length === 0) setNotice(`No definition found for ${word}`);
    else if (hits.length === 1) openHit(hits[0]);
    else showList(`Definitions of ${word}`, hits);
  };

  const findUsages = async (word: string) => {
    if (dirty) return setNotice("Save or discard changes before leaving the file");
    const hits = await api.usages(word);
    if (hits.length === 0) setNotice(`No usages found for ${word}`);
    else showList(`Usages of ${word}`, hits);
  };

  const footer = (
    <footer className="status">
      <span>{notice || statusLine(status, progress)}</span>
      <span>{status && hotkeyLabel(status.hotkey)}</span>
    </footer>
  );

  return (
    <div className={`app ${layout}`}>
      {mode === "palette" ? (
        <>
          <div className="search drag">
            <Search className="search-icon" size={20} strokeWidth={2} aria-hidden />
            <input
              ref={input}
              className="query"
              value={query}
              placeholder={noRoots ? "Add a folder to start searching" : placeholder(tab)}
              spellCheck={false}
              onChange={(e) => (setList(null), setQuery(e.target.value))}
              onKeyDown={onKeyDown}
            />
          </div>
          <nav className="tabs">
            {noRoots ? (
              <>
                <button className="tab active" onClick={addFolder}>
                  <FolderPlus size={14} strokeWidth={2} />
                  Add folder…
                </button>
                <button className="tab" onClick={() => setMode("settings")}>
                  <SettingsIcon size={14} strokeWidth={2} />
                  Settings {modLabel},
                </button>
                {notice && <span className="tab-note">{notice}</span>}
              </>
            ) : list ? (
              <span className="tab active">{list.title} · Esc to go back</span>
            ) : (
              tabs.map((t) => (
                <button key={t.tab} className={t.tab === tab ? "tab active" : "tab"} onClick={() => (setTab(t.tab), focusInput())}>
                  <t.icon size={14} strokeWidth={2} />
                  {t.label}
                </button>
              ))
            )}
            {tab === "text" && !list && !noRoots && (
              <>
                <button className={regex ? "tab active" : "tab"} onClick={() => (setRegex(!regex), focusInput())}>
                  <Regex size={14} strokeWidth={2} />
                  Regex
                </button>
                <button className={matchCase ? "tab active" : "tab"} onClick={() => (setMatchCase(!matchCase), focusInput())}>
                  <CaseSensitive size={14} strokeWidth={2} />
                  Match case
                </button>
              </>
            )}
            {progress && (
              <span className="tab progress">
                <LoaderCircle size={14} strokeWidth={2} className="spin" />
                {progress.root} {Math.round((100 * progress.done) / Math.max(progress.total, 1))}%
              </span>
            )}
            {!noRoots && (
              <button className={progress ? "tab gear after-progress" : "tab gear"} title={`Settings (${modLabel},)`} onClick={() => setMode("settings")}>
                <SettingsIcon size={14} strokeWidth={2} />
              </button>
            )}
          </nav>
          {layout === "results" && (
            <div className="card">
              <main className="body">
                <section className="results">
                  {agentView && run && <AgentAnswer run={run} onOpen={openLocation} />}
                  {agentView && run && run.steps.length > 0 && (
                    <button className="sources-toggle" onClick={() => (setSourcesOpen(!showSources), focusInput())}>
                      {showSources ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
                      Sources · {sourcesSummary(run)}
                    </button>
                  )}
                  {shownRows.map((row, i) => (
                      <RowView
                        key={i}
                        row={row}
                        selected={row === current}
                        onSelect={() => setSelected(selectable.indexOf(i))}
                        onOpen={() => activate(row)}
                      />
                    ))}
                </section>
                {!agentView && <section className="preview">{currentHit && <Preview hit={currentHit} />}</section>}
              </main>
              {footer}
            </div>
          )}
        </>
      ) : mode === "settings" ? (
        <div className="card">
          <Settings
            onClose={() => (setMode("palette"), focusInput())}
            onSaved={(message) => {
              setNotice(message);
              api.getStatus().then(setStatus);
              setMode("palette");
              focusInput();
            }}
          />
        </div>
      ) : (
        viewer && (
          <div className="card">
            <header className="drag viewer-header">
              <button className="icon-button" title={sidebarOpen ? "Hide search" : "Show search"} onClick={() => setSidebarOpen(!sidebarOpen)}>
                {sidebarOpen ? <PanelLeftClose size={15} /> : <PanelLeftOpen size={15} />}
              </button>
              <span className="path">
                {viewer.hit.rootName}/{viewer.hit.path}
                {dirty && <span className="dirty"> ●</span>}
              </span>
              <span className="hints">
                Esc back · {isMac ? "⌘⌥" : "Ctrl+Alt+"}↓↑ next result · {modLabel}S save · {modLabel}B definition · {altLabel}F7 usages ·{" "}
                {modLabel}L line · {modLabel}F find
              </span>
            </header>
            <main className="viewer">
              {sidebarOpen && (contextRows.length > 0 || (agentView && run)) && (
                <aside className="context">
                  <button className="context-query" onClick={backToPalette} title="Back to search (Esc)">
                    <ArrowLeft size={14} />
                    <span>{list ? list.title : query || "Search"}</span>
                    {!list && <span className="context-tab">{tabs.find((t) => t.tab === tab)?.label}</span>}
                  </button>
                  <div className="context-body">
                    {agentView && run && <AgentAnswer run={run} onOpen={openLocation} />}
                    {contextRows.map((row, i) =>
                      row.type === "agent" ? null : (
                        <RowView
                          key={i}
                          row={row}
                          selected={row.type === "hit" && sameHit(row.hit, viewer.hit)}
                          onSelect={() => row.type === "hit" && openFromContext(row.hit)}
                          onOpen={() => {}}
                        />
                      ),
                    )}
                  </div>
                </aside>
              )}
              <Code
                path={viewer.hit.path}
                content={viewer.content}
                line={viewer.hit.line}
                column={viewer.hit.column}
                editable
                onDirty={(d) => (setDirty(d), setDiscardArmed(false))}
                onSave={save}
                onDefinition={goToDefinition}
                onUsages={findUsages}
                onEscape={backToPalette}
              />
            </main>
            {footer}
          </div>
        )
      )}
    </div>
  );
}

async function search(tab: Tab, q: string, regex: boolean, matchCase: boolean, signal: AbortSignal): Promise<Row[]> {
  if (tab === "all") {
    const sections = await api.searchAll(q, signal);
    const rows: Row[] = sections.flatMap((s): Row[] => [
      { type: "header", label: tabs.find((t) => t.tab === s.tab)!.label },
      ...s.hits.map((hit): Row => ({ type: "hit", hit })),
    ]);
    return [...rows, { type: "agent", query: q }];
  }
  const hits = await api.searchTab(tab as Exclude<Tab, "all" | "agent">, q, regex, matchCase, signal);
  return hits.map((hit) => ({ type: "hit", hit }));
}

function sameHit(a: Hit, b: Hit): boolean {
  return a.root === b.root && a.path === b.path && a.line === b.line;
}

function sourcesSummary(run: Run): string {
  const steps = run.steps.filter((s) => "tool" in s);
  const places = new Set(steps.flatMap((s) => ("hits" in s ? s.hits.map((h) => `${h.root}/${h.path}:${h.line}`) : [])));
  return `${steps.length} steps · ${places.size} places`;
}

function placeholder(tab: Tab): string {
  switch (tab) {
    case "types":
      return "Classes, enums, interfaces, proto messages…";
    case "symbols":
      return "Functions, methods, fields… (Parent.name works too)";
    case "files":
      return "File name or path, :line to jump";
    case "text":
      return "Text in files";
    case "agent":
      return "Ask about the code and press Enter";
    default:
      return "Search everywhere";
  }
}

function statusLine(status: Status | null, progress: Progress | null): string {
  if (progress) return `Indexing ${progress.root}: ${progress.done}/${progress.total}`;
  if (!status) return "";
  if (status.indexing) return "Indexing…";
  const files = status.roots.reduce((n, r) => n + r.files, 0);
  const symbols = status.roots.reduce((n, r) => n + r.symbols, 0);
  return `${status.roots.length} roots · ${files.toLocaleString()} files · ${symbols.toLocaleString()} symbols`;
}

function RowView(props: { row: Row; selected: boolean; onSelect: () => void; onOpen: () => void }) {
  const { row } = props;
  if (row.type === "header") return <div className="row-header">{row.label}</div>;
  if (row.type === "note") return <div className="row-note">{row.text}</div>;
  const cls = props.selected ? "row selected" : "row";
  if (row.type === "agent") {
    return (
      <div className={cls} onClick={props.onSelect} onDoubleClick={props.onOpen}>
        <div className="row-main">
          <span className="kind agent">?</span>
          <span className="title">Ask agent: {row.query}</span>
        </div>
      </div>
    );
  }
  const { hit } = row;
  return (
    <div className={cls} onClick={props.onSelect} onDoubleClick={props.onOpen} ref={(el) => props.selected && el?.scrollIntoView({ block: "nearest" })}>
      <div className="row-main">
        <span className={`kind k-${hit.kind ?? hit.tab}`}>{kindLabel(hit.kind ?? hit.tab)}</span>
        <span className="title">
          <Highlighted text={hit.title} positions={hit.titleMatches} />
        </span>
        {hit.container && hit.tab !== "files" && hit.tab !== "text" && <span className="container">{hit.container}</span>}
        <span className="location">{location(hit)}</span>
      </div>
      {hit.snippet && <SnippetLine hit={hit} />}
    </div>
  );
}

// location is shown at the right of a row: where the hit is, without
// repeating the title.
function location(hit: Hit): string {
  const line = hit.line > 0 ? `:${hit.line}` : "";
  const file = hit.path.slice(hit.path.lastIndexOf("/") + 1);
  switch (hit.tab) {
    case "files":
      return `${hit.container ?? ""} · ${hit.rootName}`;
    case "text":
      return `${hit.path}${line} · ${hit.rootName}`;
    default:
      return `${file}${line} · ${hit.rootName}`;
  }
}

function Highlighted(props: { text: string; positions?: number[] }) {
  const marks = new Set(props.positions ?? []);
  return (
    <>
      {Array.from(props.text).map((ch, i) => (marks.has(i) ? <mark key={i}>{ch}</mark> : ch))}
    </>
  );
}

// SnippetLine shows the matched line of a hit, with matches highlighted.
function SnippetLine({ hit }: { hit: Hit }) {
  const sn = hit.snippet!;
  const idx = Math.max(0, Math.min(hit.line - sn.startLine, sn.lines.length - 1));
  const line = sn.lines[idx] ?? "";
  const indent = line.length - line.trimStart().length;
  const matches = (sn.matches ?? []).filter((m) => m.line === idx);
  const parts: JSX.Element[] = [];
  let pos = indent;
  matches.forEach((m, i) => {
    const start = Math.max(m.start, pos);
    if (start > pos) parts.push(<span key={`t${i}`}>{line.slice(pos, start)}</span>);
    parts.push(<mark key={`m${i}`}>{line.slice(start, m.end)}</mark>);
    pos = Math.max(pos, m.end);
  });
  parts.push(<span key="rest">{line.slice(pos)}</span>);
  return <div className="snippet">{parts}</div>;
}

const fileCache = new Map<string, string>();

function Preview({ hit }: { hit: Hit }) {
  const [content, setContent] = useState<string | null>(null);
  const key = `${hit.root}/${hit.path}`;
  useEffect(() => {
    const cached = fileCache.get(key);
    if (cached !== undefined) return setContent(cached);
    setContent(null);
    let live = true;
    const timer = setTimeout(() => {
      api.readFile(hit.root, hit.path).then((c) => {
        fileCache.set(key, c);
        if (live) setContent(c);
      }, () => live && setContent(""));
    }, 30);
    return () => ((live = false), clearTimeout(timer));
  }, [key]);
  if (content === null) return null;
  return <Code path={hit.path} content={content} line={hit.line || 1} column={hit.column} editable={false} />;
}

function kindLabel(kind: string): string {
  const labels: Record<string, string> = {
    class: "C", interface: "I", enum: "E", struct: "S", union: "U", object: "O", message: "M", service: "Sv",
    type: "T", namespace: "N", function: "ƒ", method: "m", rpc: "R", field: "f", property: "p",
    constant: "c", variable: "v", enum_value: "e", files: "F", text: "T",
  };
  return labels[kind] ?? kind[0];
}
