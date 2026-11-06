import { useCallback, useEffect, useRef, useState } from "react";
import { Events } from "@wailsio/runtime";
import { AppWindow, ArrowLeft, CaseSensitive, ChevronDown, ChevronRight, File, FolderTree, PanelLeftClose, PanelLeftOpen, FolderPlus, Layers, LoaderCircle, Regex, Search, Settings as SettingsIcon, Shapes, Sparkles, SquareFunction, TextSearch, type LucideIcon } from "lucide-react";
import * as api from "./api";
import type { Hit, Progress, Status, Tab } from "./api";
import { Code, type At } from "./Code";
import { definitionOrUsages, usages, type Where } from "./navigate";
import { Settings } from "./Settings";
import { Tree } from "./Tree";
import { crumbs, Highlighted, RowView, sameHit, type Row } from "./Rows";
import { AgentAnswer, applyEvent, followUp, newRun, stepLabel, type Run } from "./Agent";
import { capturing, label, matches } from "./keys";

// shortcut switches to the tab when typed into an empty query.
const tabs: { tab: Tab; label: string; icon: LucideIcon; shortcut?: string }[] = [
  { tab: "all", label: "All", icon: Search },
  { tab: "types", label: "Types", icon: Shapes, shortcut: "#" },
  { tab: "symbols", label: "Symbols", icon: SquareFunction, shortcut: "@" },
  { tab: "files", label: "Files", icon: File, shortcut: "/" },
  { tab: "text", label: "Text", icon: TextSearch },
  { tab: "agent", label: "Agent", icon: Sparkles, shortcut: "?" },
];

// A list that replaces search results, e.g. usages opened from the viewer.
type List = { title: string; hits: Hit[] };

type Viewer = { hit: Hit; content: string };

// Picker is the project list (DESIGN.md, Switching projects); its filter is
// typed into the search bar in place of the query.
type Picker = { filter: string; selected: number };

type PickerRow = { kind: "project"; name: string; matches: number[] } | { kind: "new" } | { kind: "manage" };

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
  const [picker, setPicker] = useState<Picker | null>(null);
  const [status, setStatus] = useState<Status | null>(null);
  const [progress, setProgress] = useState<Progress | null>(null);
  const [indexVersion, setIndexVersion] = useState(0);
  const [notice, setNotice] = useState("");
  const [viewer, setViewer] = useState<Viewer | null>(null);
  const [dirty, setDirty] = useState(false);
  const [discardArmed, setDiscardArmed] = useState(false);
  const input = useRef<HTMLInputElement>(null);
  // followUpField is the follow-up field under an agent answer, when shown.
  const followUpField = useRef<HTMLInputElement>(null);
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [treeOpen, setTreeOpen] = useState(true);
  const [treeReveal, setTreeReveal] = useState<{ root: string; path: string; n: number } | null>(null);
  const [treeFocus, setTreeFocus] = useState(0);
  const [codeFocus, setCodeFocus] = useState(0);
  const dirtyRef = useRef(false);
  dirtyRef.current = dirty;
  // sc is the shortcut bindings (shortcuts.go); the global key handler,
  // registered once, reads the latest through scRef.
  const sc = status?.shortcuts ?? {};
  const scRef = useRef(sc);
  scRef.current = sc;
  const modeRef = useRef(mode);
  modeRef.current = mode;
  // overOverview is set while the palette is summoned over the overview
  // window: results then open there (DESIGN.md, Summoning).
  const overOverview = useRef(false);
  // settingsReturn is set when settings were opened from outside the palette
  // (the overview window, the tray): closing them hides Docent and leaves
  // the palette as it was. Null when opened from the palette itself.
  const settingsReturn = useRef<"palette" | "viewer" | null>(null);
  // viewerText returns the viewer's current text, unsaved edits included.
  const viewerText = useRef<(() => string) | null>(null);
  // stepResult moves the viewer to the next or previous result; the global
  // key handler, registered once, calls the latest version.
  const stepResult = useRef<(dir: 1 | -1) => void>(() => {});
  // toggleTree handles Cmd+1 in the viewer; called by the global key handler.
  const toggleTree = useRef<() => void>(() => {});
  // toSearch handles the hotkey in the viewer; called by the event handler.
  const toSearch = useRef<() => void>(() => {});
  // closeFile handles Cmd+W: back to search from the viewer, hide otherwise.
  const closeFile = useRef<() => void>(() => {});
  // toOverview handles Cmd+Enter in the viewer.
  const toOverview = useRef<() => boolean>(() => false);
  // openPicker handles Cmd+P; called by the global key and event handlers.
  const openPicker = useRef<() => void>(() => {});

  const focusInput = () => setTimeout(() => (input.current?.focus(), input.current?.select()), 0);

  useEffect(() => {
    api.getStatus().then(setStatus);
    const offs = [
      // Docent reopens as it was hidden: the viewer stays (DESIGN.md, Viewer).
      // The palette is off screen until painted (main.go, showOffScreen).
      Events.On("palette:shown", (e) => {
        afterPaint(api.present);
        overOverview.current = e.data === "overview";
        settingsReturn.current = null;
        setPicker(null);
        if (modeRef.current === "viewer") return setCodeFocus((n) => n + 1);
        setMode("palette");
        focusInput();
      }),
      // Sent instead of palette:shown by the project pill and Cmd+P in the
      // overview window.
      Events.On("palette:projects", (e) => {
        afterPaint(api.present);
        overOverview.current = e.data === "overview";
        settingsReturn.current = null;
        openPicker.current();
      }),
      // The palette keeps its tab and query and searches the new project;
      // an agent answer is not re-run (DESIGN.md, Switching projects).
      Events.On("project:switched", () => {
        setPicker(null);
        setList(null);
        setRun(null);
        runId.current = 0;
        if (modeRef.current === "viewer" && !dirtyRef.current) setMode("palette");
        api.getStatus().then(setStatus);
        setIndexVersion((v) => v + 1);
      }),
      Events.On("palette:search", () => toSearch.current()),
      // Sent instead of palette:shown when settings are opened from outside.
      Events.On("settings:open", () => {
        overOverview.current = false;
        settingsReturn.current = modeRef.current === "settings" ? (settingsReturn.current ?? "palette") : modeRef.current;
        setMode("settings");
        afterPaint(api.present);
      }),
      Events.On("agent:event", (e) => {
        const { run: id, event } = e.data as { run: number; event: api.AgentEvent };
        if (id === runId.current) setRun((r) => r && applyEvent(r, event));
      }),
      Events.On("index:progress", (e) => {
        const p = e.data as Progress;
        setProgress(p);
        if (p.listed) {
          api.getStatus().then(setStatus);
          setIndexVersion((v) => v + 1);
        }
      }),
      Events.On("index:done", () => {
        setProgress(null);
        api.getStatus().then(setStatus);
        setIndexVersion((v) => v + 1);
      }),
    ];
    // Window shortcuts (keys.ts): close goes back to search from the viewer
    // and hides the window elsewhere; the rest are the viewer's commands
    // and settings. Settings recording a binding take every key.
    const onKey = (e: KeyboardEvent) => {
      if (capturing.active) return;
      const sc = scRef.current;
      if (matches(sc.nextResult, e) || matches(sc.prevResult, e)) {
        e.preventDefault();
        stepResult.current(matches(sc.nextResult, e) ? 1 : -1);
      } else if (matches(sc.overview, e)) {
        // Stop it here so the editor does not insert a line.
        if (toOverview.current()) (e.preventDefault(), e.stopPropagation());
      } else if (matches(sc.tree, e)) {
        e.preventDefault();
        toggleTree.current();
      } else if (matches(sc.close, e)) {
        e.preventDefault();
        closeFile.current();
      } else if (matches(sc.settings, e)) {
        e.preventDefault();
        if (!dirtyRef.current) setMode("settings");
      } else if (matches(sc.projects, e)) {
        e.preventDefault();
        openPicker.current();
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

  const projects = status?.projects ?? [];
  const pickerRows: PickerRow[] = picker
    ? [...filterProjects(projects, picker.filter).map((p): PickerRow => ({ kind: "project", ...p })), { kind: "new" }, { kind: "manage" }]
    : [];

  // The window is a Spotlight-like bar until there is something to show below it.
  const noRoots = status !== null && status.roots.length === 0;
  const showContext = !!(sidebarOpen && (contextRows.length > 0 || (agentView && run)));
  // The side column widens the viewer instead of narrowing the code.
  const layout: api.Layout =
    mode === "viewer"
      ? treeOpen || showContext
        ? "viewer-side"
        : "viewer"
      : mode === "settings" || picker || list || shownRows.length > 0 || (tab === "agent" && run)
        ? "results"
        : "compact";
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

  // ask starts a new thread; askFollowUp continues the current one.
  const ask = (question: string) => {
    question = question.trim();
    if (question) startRun(newRun(question));
  };

  const askFollowUp = (question: string) => {
    if (run) startRun(followUp(run, question));
  };

  const startRun = async (next: Run) => {
    setTab("agent");
    setSelected(0);
    setRun(next);
    setSourcesOpen(null);
    try {
      const id = await api.askAgent(next.thread, next.question);
      runId.current = id;
      setRun((r) => r && { ...r, id });
    } catch (e) {
      setRun((r) => r && { ...r, status: "error", error: (e as Error).message });
    }
  };

  // openResult opens a chosen result: as a tab in the overview window when
  // the palette is over it, in the viewer otherwise.
  const openResult = (hit: Hit) => {
    if (overOverview.current) openInOverview(hit);
    else openHit(hit);
  };

  const openInOverview = (hit: Hit, content?: string) => {
    overOverview.current = false;
    api.openInOverview(hit, content).catch((e) => setNotice((e as Error).message));
  };

  const openLocation = (root: string, path: string, line: number) => {
    const rootName = status?.roots.find((r) => r.id === root)?.name ?? root;
    openResult({ tab: "files", root, rootName, path, line, column: 0, title: path.slice(path.lastIndexOf("/") + 1) });
  };

  const activate = (row?: Row) => {
    if (row?.type === "hit") openResult(row.hit);
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
    if (matches(sc.overview, e)) {
      e.preventDefault();
      if (currentHit) openInOverview(currentHit);
      return;
    }
    if (tab === "text" && (matches(sc.regex, e) || matches(sc.matchCase, e))) {
      e.preventDefault();
      if (matches(sc.regex, e)) setRegex((r) => !r);
      else setMatchCase((c) => !c);
      return;
    }
    switch (e.key) {
      case "Tab": {
        e.preventDefault();
        if (!e.shiftKey && followUpField.current) {
          followUpField.current.focus();
          return;
        }
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
  };

  const backToPalette = () => {
    if (dirty && !discardArmed) {
      setDiscardArmed(true);
      setNotice(`Unsaved changes: ${label(sc.save)} to save, press again to discard`);
      return;
    }
    setDirty(false);
    setDiscardArmed(false);
    setNotice("");
    setMode("palette");
    focusInput();
  };

  // The hotkey never discards edits, unlike a second Esc.
  toSearch.current = () => {
    if (dirty) return setNotice(`Unsaved changes: ${label(sc.save)} to save, Esc twice to discard`);
    backToPalette();
  };

  closeFile.current = () => (mode === "viewer" ? backToPalette() : api.hide());

  // moveToOverview moves the open file, unsaved edits included, to a tab in
  // the overview window, and closes it here.
  const moveToOverview = () => {
    if (!viewer) return;
    const text = viewerText.current?.();
    openInOverview(viewer.hit, dirty && text !== undefined ? text : undefined);
    setDirty(false);
    setDiscardArmed(false);
    setNotice("");
    setMode("palette");
  };

  toOverview.current = () => {
    if (mode !== "viewer") return false;
    moveToOverview();
    return true;
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

  // closeSettings returns to where settings were opened from.
  const closeSettings = () => {
    const back = settingsReturn.current;
    settingsReturn.current = null;
    if (back) {
      api.hide();
      setMode(back);
      return;
    }
    setMode("palette");
    focusInput();
  };

  // addFolder is the first-run path: pick a folder and start indexing it
  // without going through settings.
  const addFolder = async () => {
    try {
      await api.addFolder();
      setStatus(await api.getStatus());
      focusInput();
    } catch (e) {
      setNotice((e as Error).message);
    }
  };

  openPicker.current = () => {
    if (mode === "settings") return;
    if (mode === "viewer") {
      if (dirty) return setNotice("Save or discard changes before switching projects");
      setMode("palette");
    }
    setPicker({ filter: "", selected: Math.max(projects.indexOf(status?.project ?? ""), 0) });
    setNotice("");
    focusInput();
    // A project added without switching to it is not in the status yet.
    api.getStatus().then((s) => {
      setStatus(s);
      setPicker((p) => (p && p.filter === "" ? { ...p, selected: Math.max(s.projects.indexOf(s.project), 0) } : p));
    });
  };

  const closePicker = () => {
    setPicker(null);
    focusInput();
  };

  const choose = async (row?: PickerRow) => {
    if (!row) return;
    try {
      if (row.kind === "manage") {
        setPicker(null);
        setMode("settings");
      } else if (row.kind === "new") {
        // The switch itself closes the list (project:switched).
        if (!(await api.newProject()).name) focusInput();
      } else if (row.name === status?.project) {
        closePicker();
      } else {
        await api.switchProject(row.name);
      }
    } catch (e) {
      setNotice((e as Error).message);
    }
  };

  const onPickerKey = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (!picker) return;
    switch (e.key) {
      case "ArrowDown":
        setPicker({ ...picker, selected: Math.min(picker.selected + 1, pickerRows.length - 1) });
        break;
      case "ArrowUp":
        setPicker({ ...picker, selected: Math.max(picker.selected - 1, 0) });
        break;
      case "Enter":
        choose(pickerRows[picker.selected]);
        break;
      case "Escape":
        closePicker();
        break;
      case "Tab":
        break;
      default:
        return;
    }
    e.preventDefault();
  };

  stepResult.current = (dir) => {
    if (mode !== "viewer" || !viewer || contextHits.length === 0) return;
    if (dirty) return setNotice("Save or discard changes before leaving the file");
    const i = contextHits.findIndex((h) => sameHit(h, viewer.hit));
    const next = contextHits[(i + dir + contextHits.length) % contextHits.length];
    if (next) openHit(next);
  };

  // showTree shows or hides the project tree; a hidden tree forgets the
  // folder it was asked to reveal, so it reopens at the open file.
  const showTree = (open: boolean) => {
    setTreeOpen(open);
    if (!open) setTreeReveal(null);
  };

  toggleTree.current = () => {
    if (mode !== "viewer") return;
    if (treeOpen && document.activeElement?.closest(".tree")) {
      showTree(false);
      setCodeFocus((n) => n + 1);
    } else {
      setTreeOpen(true);
      setTreeFocus((n) => n + 1);
    }
  };

  const revealInTree = (path: string) => {
    if (!viewer) return;
    setTreeOpen(true);
    setTreeReveal((r) => ({ root: viewer.hit.root, path, n: (r?.n ?? 0) + 1 }));
    setTreeFocus((n) => n + 1);
  };

  const openFromTree = (path: string) => {
    if (!viewer) return;
    if (dirty) return setNotice("Save or discard changes before leaving the file");
    openLocation(viewer.hit.root, path, 1);
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

  const go = (where: Where) => {
    if ("open" in where) openHit(where.open);
    else if ("list" in where) showList(where.list.title, where.list.hits);
    else setNotice(where.note);
  };

  const goToDefinition = async (word: string, at: At) => {
    if (!viewer) return;
    if (dirty) return setNotice("Save or discard changes before leaving the file");
    go(await definitionOrUsages(word, viewer.hit, at));
  };

  const findUsages = async (word: string) => {
    if (dirty) return setNotice("Save or discard changes before leaving the file");
    go(await usages(word));
  };

  const footer = (
    <footer className="status">
      <span>{notice || statusLine(status, progress)}</span>
    </footer>
  );

  return (
    <div className={`app ${layout}`}>
      {mode === "palette" ? (
        <>
          <div className="search drag">
            <Search className="search-icon" size={20} strokeWidth={2} aria-hidden />
            {picker ? (
              <input
                ref={input}
                className="query"
                value={picker.filter}
                placeholder="Switch to project"
                spellCheck={false}
                onChange={(e) => setPicker({ filter: e.target.value, selected: 0 })}
                onKeyDown={onPickerKey}
              />
            ) : (
              <input
                ref={input}
                className="query"
                value={query}
                placeholder={noRoots ? "Add a folder to start searching" : placeholder(tab)}
                spellCheck={false}
                onChange={(e) => (setList(null), setQuery(e.target.value))}
                onKeyDown={onKeyDown}
              />
            )}
          </div>
          <nav className="tabs">
            {picker ? (
              <span className="tab active">Projects · Esc to go back</span>
            ) : noRoots ? (
              <>
                <button className="tab active" onClick={addFolder}>
                  <FolderPlus size={14} strokeWidth={2} />
                  Add folder…
                </button>
                <button className="tab" onClick={() => setMode("settings")}>
                  <SettingsIcon size={14} strokeWidth={2} />
                  Settings
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
            {tab === "text" && !list && !picker && !noRoots && (
              <>
                <button className={regex ? "tab icon active" : "tab icon"} title={`Regex (${label(sc.regex)})`} onClick={() => (setRegex(!regex), focusInput())}>
                  <Regex size={14} strokeWidth={2} />
                </button>
                <button
                  className={matchCase ? "tab icon active" : "tab icon"}
                  title={`Match case (${label(sc.matchCase)})`}
                  onClick={() => (setMatchCase(!matchCase), focusInput())}
                >
                  <CaseSensitive size={14} strokeWidth={2} />
                </button>
              </>
            )}
            {progress && (
              <span className="tab progress">
                <LoaderCircle size={14} strokeWidth={2} className="spin" />
                {progress.root} {Math.round((100 * progress.done) / Math.max(progress.total, 1))}%
              </span>
            )}
            {projects.length > 1 && !picker && (
              <button
                className={progress ? "tab project after-progress" : "tab project"}
                title={`Switch project (${label(sc.projects)})`}
                onClick={() => openPicker.current()}
              >
                <Layers size={14} strokeWidth={2} />
                {status?.project}
              </button>
            )}
            {!noRoots && (
              <button
                className={progress || projects.length > 1 ? "tab gear after-progress" : "tab gear"}
                title={`Settings (${label(sc.settings)})`}
                onClick={() => setMode("settings")}
              >
                <SettingsIcon size={14} strokeWidth={2} />
              </button>
            )}
          </nav>
          {layout === "results" && picker && (
            <div className="card">
              <main className="body">
                <section className="results">
                  {pickerRows.map((row, i) => (
                    <div
                      key={row.kind === "project" ? `p:${row.name}` : row.kind}
                      className={i === picker.selected ? "row picker-row selected" : "row picker-row"}
                      onMouseEnter={() => setPicker({ ...picker, selected: i })}
                      onClick={() => choose(row)}
                    >
                      <div className="row-main">
                        {row.kind === "project" ? (
                          <>
                            <span className="kind k-project">
                              <Layers size={15} />
                            </span>
                            <span className="title">
                              <Highlighted text={row.name} positions={row.matches} />
                            </span>
                            {row.name === status?.project && <span className="location">active</span>}
                          </>
                        ) : row.kind === "new" ? (
                          <>
                            <span className="kind k-project">
                              <FolderPlus size={15} />
                            </span>
                            <span className="title">New project…</span>
                            <span className="location">from a folder</span>
                          </>
                        ) : (
                          <>
                            <span className="kind k-project">
                              <SettingsIcon size={15} />
                            </span>
                            <span className="title">Manage projects…</span>
                            <span className="location">settings</span>
                          </>
                        )}
                      </div>
                    </div>
                  ))}
                </section>
              </main>
              {footer}
            </div>
          )}
          {layout === "results" && !picker && (
            <div className="card">
              <main className="body">
                <section className="results">
                  {agentView && run && (
                        <AgentAnswer run={run} onOpen={openLocation} onFollowUp={askFollowUp} followUpRef={followUpField} onFollowUpEscape={focusInput} />
                      )}
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
            onClose={closeSettings}
            onSaved={(message) => {
              setNotice(message);
              api.getStatus().then(setStatus);
              closeSettings();
            }}
          />
        </div>
      ) : (
        viewer && (
          <div className="viewer-layout">
            {(treeOpen || showContext) && (
              <div className="side">
                {showContext && (
                  <aside className="card context">
                    <button className="context-query" onClick={backToPalette} title="Back to search (Esc)">
                      <ArrowLeft size={14} />
                      <span>{list ? list.title : query || "Search"}</span>
                      {!list && <span className="context-tab">{tabs.find((t) => t.tab === tab)?.label}</span>}
                    </button>
                    <div className="context-body">
                      {agentView && run && (
                        <AgentAnswer run={run} onOpen={openLocation} onFollowUp={askFollowUp} followUpRef={followUpField} onFollowUpEscape={focusInput} />
                      )}
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
                {treeOpen && (
                  <aside className="card">
                    <Tree
                      roots={[{ id: viewer.hit.root, name: viewer.hit.rootName }]}
                      file={viewer.hit}
                      reveal={treeReveal}
                      version={indexVersion}
                      focus={treeFocus}
                      onOpen={(_, path) => openFromTree(path)}
                      onEscape={() => setCodeFocus((n) => n + 1)}
                    />
                  </aside>
                )}
              </div>
            )}
            <div className="card">
              <header className="drag viewer-header">
                <button className="icon-button" title={sidebarOpen ? "Hide search" : "Show search"} onClick={() => setSidebarOpen(!sidebarOpen)}>
                  {sidebarOpen ? <PanelLeftClose size={15} /> : <PanelLeftOpen size={15} />}
                </button>
                <button className="icon-button" title={`${treeOpen ? "Hide" : "Show"} project tree (${label(sc.tree)})`} onClick={() => showTree(!treeOpen)}>
                  <FolderTree size={15} />
                </button>
                <button className="icon-button" title={`Open in overview window (${label(sc.overview)})`} onClick={moveToOverview}>
                  <AppWindow size={15} />
                </button>
                <span className="path">
                  {crumbs(viewer.hit.rootName, viewer.hit.path).map((c, i, all) =>
                    i === all.length - 1 ? (
                      <span key={i}>{c.name}</span>
                    ) : (
                      <span key={i}>
                        <button className="crumb" onClick={() => revealInTree(c.path)}>
                          {c.name}
                        </button>
                        <span className="crumb-sep">/</span>
                      </span>
                    ),
                  )}
                  {dirty && <span className="dirty"> ●</span>}
                </span>
              </header>
              <main className="viewer">
                <Code
                  shortcuts={sc}
                  path={viewer.hit.path}
                  content={viewer.content}
                  line={viewer.hit.line}
                  column={viewer.hit.column}
                  editable
                  focus={codeFocus}
                  text={viewerText}
                  onDirty={(d) => (setDirty(d), setDiscardArmed(false))}
                  onSave={save}
                  onDefinition={goToDefinition}
                  onUsages={findUsages}
                  onEscape={backToPalette}
                />
              </main>
              {footer}
            </div>
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

// afterPaint calls f once the next frame is painted.
function afterPaint(f: () => void) {
  requestAnimationFrame(() => requestAnimationFrame(() => f()));
}

export function statusLine(status: Status | null, progress: Progress | null): string {
  if (progress) return `Indexing ${progress.root}: ${progress.done}/${progress.total}`;
  if (!status) return "";
  if (status.indexing) return "Indexing…";
  const files = status.roots.reduce((n, r) => n + r.files, 0);
  const symbols = status.roots.reduce((n, r) => n + r.symbols, 0);
  const roots = status.roots.length === 1 ? "1 root" : `${status.roots.length} roots`;
  // The project is named only once there is more than one.
  const project = status.projects.length > 1 ? `${status.project} · ` : "";
  return `${project}${roots} · ${files.toLocaleString()} files · ${symbols.toLocaleString()} symbols`;
}

// filterProjects keeps the projects whose names contain the filter's
// characters in order, ignoring case, with the matched positions; names
// where they are closer together come first.
export function filterProjects(names: string[], filter: string): { name: string; matches: number[] }[] {
  const want = Array.from(filter.toLowerCase().replace(/\s+/g, ""));
  const found: { name: string; matches: number[]; span: number }[] = [];
  for (const name of names) {
    const chars = Array.from(name.toLowerCase());
    const matches: number[] = [];
    for (let i = 0; i < chars.length && matches.length < want.length; i++) {
      if (chars[i] === want[matches.length]) matches.push(i);
    }
    if (matches.length < want.length) continue;
    found.push({ name, matches, span: matches.length ? matches[matches.length - 1] - matches[0] : 0 });
  }
  return found.sort((a, b) => a.span - b.span).map(({ name, matches }) => ({ name, matches }));
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
