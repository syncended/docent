import { useEffect, useRef, useState, type MutableRefObject } from "react";
import { Events, Window } from "@wailsio/runtime";
import { Copy, FolderPlus, FolderTree, Layers, LoaderCircle, Minus, Settings as SettingsIcon, Square, X } from "lucide-react";
import * as api from "./api";
import type { Hit, Progress, Status } from "./api";
import { Code, type At } from "./Code";
import { Tree } from "./Tree";
import { crumbs, RowView, sameHit } from "./Rows";
import { definitionOrUsages, usages, type Where } from "./navigate";
import { empty as noHistory, jump, step } from "./history";
import { diskNotice, statusLine } from "./App";
import { languageName } from "./languages";
import { isMac, label, matches } from "./keys";

// FileTab is a file open in a tab. content is the file as read from disk;
// draft is unsaved text the tab was opened with. rev remounts the editor
// (new draft, discarded edits); reveal scrolls to hit's line again. onDisk
// is set when the file changed on disk while the tab had unsaved edits, or
// when it was deleted (DESIGN.md, Editing).
type FileTab = {
  id: number;
  hit: Hit;
  content: string;
  draft?: string;
  dirty: boolean;
  rev: number;
  reveal: number;
  onDisk?: "changed" | "deleted";
};

// Results is the results panel: definitions or usages to choose from.
type Results = { title: string; hits: Hit[]; selected: number };

// ProjectView is what the window keeps of a project while another one is
// active: its tabs and the tree (DESIGN.md, Overview window).
type ProjectView = { tabs: FileTab[]; active: number; treeOpen: boolean };

// Prompt asks about unsaved edits before closing the window or switching
// to another project, or whether to overwrite a file that changed on disk.
type EditsPrompt = { kind: "close" } | { kind: "switch"; project: string };
type Prompt = EditsPrompt | { kind: "overwrite"; id: number };

let nextTab = 1;

const fileName = (path: string) => path.slice(path.lastIndexOf("/") + 1);

// Overview is the overview window: the project tree and files in tabs
// (DESIGN.md, Overview window).
export function Overview() {
  const [status, setStatus] = useState<Status | null>(null);
  const [progress, setProgress] = useState<Progress | null>(null);
  const [version, setVersion] = useState(0);
  const [tabs, setTabs] = useState<FileTab[]>([]);
  const [active, setActive] = useState(0);
  const [treeOpen, setTreeOpen] = useState(true);
  const [treeFocus, setTreeFocus] = useState(0);
  const [treeReveal, setTreeReveal] = useState<{
    root: string;
    path: string;
    n: number;
  } | null>(null);
  const [codeFocus, setCodeFocus] = useState(0);
  const [results, setResults] = useState<Results | null>(null);
  // cursors are the cursor positions by tab, for the status bar.
  const [cursors, setCursors] = useState(new Map<number, { line: number; column: number }>());
  const [notice, setNotice] = useState("");
  // discardArmed is the tab whose next close discards its edits.
  const [discardArmed, setDiscardArmed] = useState(0);
  // prompt asks about unsaved edits before closing or switching projects.
  const [prompt, setPrompt] = useState<Prompt | null>(null);
  // views are the other projects' tabs, by project name, until Docent restarts.
  const views = useRef(new Map<string, ProjectView>());
  // history is the places visited, for back and forward (history.ts).
  const history = useRef(noHistory);
  const cursorsRef = useRef(cursors);
  cursorsRef.current = cursors;
  const texts = useRef(new Map<number, MutableRefObject<(() => string) | null>>());
  const panel = useRef<HTMLDivElement>(null);
  const tabRow = useRef<HTMLDivElement>(null);
  const tabsRef = useRef(tabs);
  tabsRef.current = tabs;
  const activeRef = useRef(active);
  activeRef.current = active;
  const viewRef = useRef<ProjectView>({ tabs, active, treeOpen });
  viewRef.current = { tabs, active, treeOpen };
  const statusRef = useRef(status);
  statusRef.current = status;
  const { maximised, fullscreen } = useWindowState();

  const tab = tabs.find((t) => t.id === active) ?? null;
  // sc is the shortcut bindings (shortcuts.go).
  const sc = status?.shortcuts ?? {};
  const dirtyTabs = tabs.filter((t) => t.dirty);
  const anyDirty = dirtyTabs.length > 0;

  // The shell asks about these edits before switching projects.
  useEffect(() => {
    api.setOverviewDirty(anyDirty);
  }, [anyDirty]);

  // The active tab stays in view when the row scrolls (DESIGN.md, Overview window).
  useEffect(() => {
    tabRow.current?.querySelector(".ov-tab.active")?.scrollIntoView({ inline: "nearest", block: "nearest" });
  }, [active, tabs.length]);

  // currentPlace is the active tab at its cursor, for the history.
  const currentPlace = (): Hit | null => {
    const t = tabsRef.current.find((t) => t.id === activeRef.current);
    if (!t) return null;
    const c = cursorsRef.current.get(t.id);
    return c ? { ...t.hit, line: c.line, column: c.column - 1 } : t.hit;
  };

  // openFile opens hit in a tab; record is false when the history itself
  // opens a place.
  const openFile = async (hit: Hit, draft?: string, record = true) => {
    setNotice("");
    if (record) history.current = jump(history.current, currentPlace(), hit);
    const open = tabsRef.current.find((t) => t.hit.root === hit.root && t.hit.path === hit.path);
    if (open) {
      const move = hit.line > 0;
      setTabs((ts) =>
        ts.map((t) =>
          t.id !== open.id
            ? t
            : {
                ...t,
                hit: move ? hit : t.hit,
                reveal: move ? t.reveal + 1 : t.reveal,
                ...(draft !== undefined && {
                  draft,
                  dirty: draft !== t.content,
                  rev: t.rev + 1,
                }),
              },
        ),
      );
      setActive(open.id);
      setCodeFocus((n) => n + 1);
      return;
    }
    try {
      const content = await api.readFile(hit.root, hit.path);
      const id = nextTab++;
      setTabs((ts) => [
        ...ts,
        {
          id,
          hit,
          content,
          draft,
          dirty: draft !== undefined && draft !== content,
          rev: 0,
          reveal: 0,
        },
      ]);
      setActive(id);
      setCodeFocus((n) => n + 1);
    } catch (e) {
      setNotice((e as Error).message);
    }
  };

  const openPath = (root: string, path: string) => {
    const rootName = status?.roots.find((r) => r.id === root)?.name ?? root;
    openFile({
      tab: "files",
      root,
      rootName,
      path,
      line: 0,
      column: 0,
      title: fileName(path),
    });
  };

  // requestClose handles the window's close button: hide, or ask first.
  const requestClose = () => {
    if (tabsRef.current.some((t) => t.dirty)) setPrompt({ kind: "close" });
    else api.hideOverview();
  };

  // switched puts this project's tabs away and brings back the new one's.
  // Renaming the active project in settings also switches, from a name that
  // is gone to one never seen: the tabs then stay, as far as their roots do.
  const switched = async ({ from, to }: api.ProjectSwitch) => {
    history.current = noHistory;
    const next = await api.getStatus();
    const current = viewRef.current;
    const renamed = !views.current.has(to) && !next.projects.includes(from);
    const ids = new Set(next.roots.map((r) => r.id));
    let view: ProjectView = views.current.get(to) ?? { tabs: [], active: 0, treeOpen: true };
    if (renamed) {
      const kept = current.tabs.filter((t) => ids.has(t.hit.root));
      view = { ...current, tabs: kept, active: kept.some((t) => t.id === current.active) ? current.active : (kept[0]?.id ?? 0) };
    } else if (from) {
      views.current.set(from, current);
    }
    views.current.delete(to);
    for (const name of views.current.keys()) if (!next.projects.includes(name)) views.current.delete(name);
    // Files may have changed while the project was away, saved edits included.
    const tabs = await Promise.all(
      view.tabs.map(async (t) => (renamed ? t : { ...t, content: await api.readFile(t.hit.root, t.hit.path).catch(() => t.content), draft: undefined, rev: t.rev + 1 })),
    );
    for (const t of current.tabs) if (!tabs.some((x) => x.id === t.id)) texts.current.delete(t.id);
    setStatus(next);
    setTabs(tabs);
    setActive(view.active);
    setTreeOpen(view.treeOpen);
    setTreeReveal(null);
    setResults(null);
    setDiscardArmed(0);
    setNotice("");
    setCodeFocus((n) => n + 1);
  };

  // filesChanged brings the tabs a change on disk concerns up to date: a
  // tab without edits reloads in place, one with edits is marked, and a
  // deleted file's tab stays, marked (DESIGN.md, Editing).
  const filesChanged = async (c: api.Change) => {
    const hit = tabsRef.current.filter((t) => api.affects(c, t.hit.root, t.hit.path));
    const read = await Promise.all(hit.map((t) => api.readFile(t.hit.root, t.hit.path).then((content) => ({ t, content }), () => ({ t, content: null }))));
    setTabs((ts) =>
      ts.map((t) => {
        const r = read.find((r) => r.t.id === t.id);
        if (!r) return t;
        if (r.content === null) return { ...t, onDisk: "deleted" };
        if (r.content === t.content) return t.onDisk === "deleted" ? { ...t, onDisk: undefined } : t;
        if (t.dirty) return { ...t, onDisk: "changed" };
        return { ...t, content: r.content, draft: undefined, onDisk: undefined };
      }),
    );
  };

  // requestSwitch is the shell asking to switch with unsaved edits here.
  const requestSwitch = async (project: string) => {
    if (tabsRef.current.some((t) => t.dirty)) return setPrompt({ kind: "switch", project });
    // The shell's flag lagged behind: clear it first, or it asks here again.
    try {
      await api.setOverviewDirty(false);
      await api.switchProject(project);
    } catch (e) {
      setNotice((e as Error).message);
    }
  };

  // Event handlers are registered once; they call the latest versions.
  const handlers = useRef({ openFile, requestClose, requestSwitch, switched, filesChanged });
  handlers.current = { openFile, requestClose, requestSwitch, switched, filesChanged };

  useEffect(() => {
    api.getStatus().then(setStatus);
    const offs = [
      Events.On("overview:open", (e) => {
        const { hit, content } = e.data as { hit: Hit; content?: string };
        handlers.current.openFile(hit, content ?? undefined);
      }),
      Events.On("overview:close", () => handlers.current.requestClose()),
      Events.On("overview:switch", (e) => handlers.current.requestSwitch(e.data as string)),
      Events.On("project:switched", (e) => handlers.current.switched(e.data as api.ProjectSwitch)),
      Events.On("files:changed", (e) => handlers.current.filesChanged(e.data as api.Change)),
      Events.On("index:progress", (e) => {
        const p = e.data as Progress;
        setProgress(p);
        if (p.listed) {
          api.getStatus().then(setStatus);
          setVersion((v) => v + 1);
        }
      }),
      Events.On("index:done", () => {
        setProgress(null);
        api.getStatus().then(setStatus);
        setVersion((v) => v + 1);
      }),
      // Shortcuts may have changed.
      Events.On("settings:saved", () => api.getStatus().then(setStatus)),
    ];
    return () => offs.forEach((off) => off());
  }, []);

  const closeTab = (id: number) => {
    const t = tabs.find((t) => t.id === id);
    if (!t) return;
    if (t.dirty && discardArmed !== id) {
      setActive(id);
      setDiscardArmed(id);
      setNotice(`Unsaved changes in ${fileName(t.hit.path)}: ${label(sc.save)} to save, close again to discard`);
      return;
    }
    setDiscardArmed(0);
    setNotice("");
    texts.current.delete(id);
    const i = tabs.indexOf(t);
    const rest = tabs.filter((x) => x.id !== id);
    setTabs(rest);
    if (active === id) {
      setActive(rest[Math.min(i, rest.length - 1)]?.id ?? 0);
      setCodeFocus((n) => n + 1);
    }
  };

  const switchTab = (dir: 1 | -1) => {
    if (tabs.length === 0) return;
    const i = tabs.findIndex((t) => t.id === active);
    setActive(tabs[(i + dir + tabs.length) % tabs.length].id);
    setCodeFocus((n) => n + 1);
  };

  const setDirty = (id: number, dirty: boolean) => {
    setTabs((ts) => ts.map((t) => (t.id === id && t.dirty !== dirty ? { ...t, dirty } : t)));
    if (discardArmed === id) setDiscardArmed(0);
  };

  // save writes a tab's text. A file that changed on disk meanwhile is
  // asked about first; a deleted one is created again (DESIGN.md, Editing).
  const save = async (t: FileTab, text: string) => {
    if (t.onDisk === "changed") return setPrompt({ kind: "overwrite", id: t.id });
    try {
      await api.writeFile(t.hit.root, t.hit.path, text);
      setTabs((ts) => ts.map((x) => (x.id === t.id ? { ...x, content: text, draft: undefined, dirty: false, onDisk: undefined } : x)));
      if (discardArmed === t.id) setDiscardArmed(0);
      setNotice(`Saved ${t.hit.path}`);
    } catch (e) {
      setNotice((e as Error).message);
    }
  };

  // overwrite saves over a file that changed on disk; reload drops the
  // tab's edits for what is on disk.
  const overwrite = async (id: number) => {
    setPrompt(null);
    const t = tabsRef.current.find((t) => t.id === id);
    const text = texts.current.get(id)?.current?.();
    if (t && text !== undefined) await save({ ...t, onDisk: undefined }, text);
  };

  const reload = async (id: number) => {
    setPrompt(null);
    const t = tabsRef.current.find((t) => t.id === id);
    if (!t) return;
    try {
      const content = await api.readFile(t.hit.root, t.hit.path);
      setTabs((ts) => ts.map((x) => (x.id === id ? { ...x, content, draft: undefined, dirty: false, onDisk: undefined, rev: x.rev + 1 } : x)));
      if (discardArmed === id) setDiscardArmed(0);
    } catch (e) {
      setNotice((e as Error).message);
    }
  };

  // afterPrompt closes the window or switches projects, once the edits it
  // asked about are saved or discarded.
  const afterPrompt = async (p: EditsPrompt) => {
    setPrompt(null);
    if (p.kind === "close") return api.hideOverview();
    try {
      await api.setOverviewDirty(false);
      await api.switchProject(p.project);
    } catch (e) {
      setNotice((e as Error).message);
    }
  };

  const saveAll = async (p: EditsPrompt) => {
    try {
      for (const t of dirtyTabs) {
        const text = texts.current.get(t.id)?.current?.();
        if (text !== undefined) await api.writeFile(t.hit.root, t.hit.path, text);
      }
      setTabs((ts) => ts.map((t) => (t.dirty ? { ...t, dirty: false } : t)));
      afterPrompt(p);
    } catch (e) {
      setNotice((e as Error).message);
    }
  };

  // discardAll reloads edited tabs from disk.
  const discardAll = async (p: EditsPrompt) => {
    const reloaded = await Promise.all(
      tabs.map(async (t) => {
        if (!t.dirty) return t;
        const content = await api.readFile(t.hit.root, t.hit.path).catch(() => t.content);
        return {
          ...t,
          content,
          draft: undefined,
          dirty: false,
          rev: t.rev + 1,
        };
      }),
    );
    setTabs(reloaded);
    setDiscardArmed(0);
    afterPrompt(p);
  };

  const go = (where: Where) => {
    if ("open" in where) openFile(where.open);
    else if ("list" in where) setResults({ ...where.list, selected: -1 });
    else setNotice(where.note);
  };

  // The panel takes focus when it shows new results, for the arrow keys.
  useEffect(() => {
    if (results) panel.current?.focus();
  }, [results?.hits]);

  const openResult = (i: number) => {
    if (!results || !results.hits[i]) return;
    setResults({ ...results, selected: i });
    openFile(results.hits[i]);
  };

  // navigate goes back or forward through the history.
  const navigate = (dir: -1 | 1) => {
    const r = step(history.current, dir, currentPlace());
    if (!r) return;
    history.current = r.history;
    openFile(r.place, undefined, false);
  };

  const stepResult = (dir: 1 | -1) => {
    if (!results || results.hits.length === 0) return;
    const n = results.hits.length;
    openResult(results.selected < 0 ? (dir > 0 ? 0 : n - 1) : (results.selected + dir + n) % n);
  };

  const hideResults = () => {
    setResults(null);
    setCodeFocus((n) => n + 1);
  };

  const onPanelKey = (e: React.KeyboardEvent) => {
    if (!results) return;
    const n = results.hits.length;
    switch (e.key) {
      case "ArrowDown":
        setResults({
          ...results,
          selected: Math.min(results.selected + 1, n - 1),
        });
        break;
      case "ArrowUp":
        setResults({ ...results, selected: Math.max(results.selected - 1, 0) });
        break;
      case "Enter":
        openResult(results.selected);
        break;
      case "Escape":
        hideResults();
        break;
      default:
        return;
    }
    e.preventDefault();
  };

  const hideTree = () => {
    setTreeOpen(false);
    setTreeReveal(null);
    setCodeFocus((n) => n + 1);
  };

  // toggleTree is the shortcut: it shows and focuses the tree, or hides it
  // when it has focus (DESIGN.md, Overview window).
  const toggleTree = () => {
    if (treeOpen && document.activeElement?.closest(".tree")) {
      hideTree();
    } else {
      setTreeOpen(true);
      setTreeFocus((n) => n + 1);
    }
  };

  const revealInTree = (root: string, path: string) => {
    setTreeOpen(true);
    setTreeReveal((r) => ({ root, path, n: (r?.n ?? 0) + 1 }));
    setTreeFocus((n) => n + 1);
  };

  const addFolder = async () => {
    try {
      await api.addFolder("overview");
      setStatus(await api.getStatus());
    } catch (e) {
      setNotice((e as Error).message);
    }
  };

  // Window shortcuts; the global key handler, registered once, calls the
  // latest version.
  const onKey = useRef<(e: KeyboardEvent) => boolean>(() => false);
  onKey.current = (e) => {
    if (matches(sc.nextTab, e) || matches(sc.prevTab, e)) {
      switchTab(matches(sc.nextTab, e) ? 1 : -1);
      return true;
    }
    if (matches(sc.nextResult, e) || matches(sc.prevResult, e)) {
      stepResult(matches(sc.nextResult, e) ? 1 : -1);
      return true;
    }
    if (matches(sc.back, e) || matches(sc.forward, e)) {
      navigate(matches(sc.back, e) ? -1 : 1);
      return true;
    }
    if (matches(sc.tree, e)) {
      toggleTree();
      return true;
    }
    if (matches(sc.close, e)) {
      if (tab) closeTab(tab.id);
      else requestClose();
      return true;
    }
    if (matches(sc.settings, e)) {
      api.openSettings();
      return true;
    }
    if (matches(sc.projects, e)) {
      api.showProjects();
      return true;
    }
    return false;
  };

  useEffect(() => {
    const handle = (e: KeyboardEvent) => {
      if (onKey.current(e)) (e.preventDefault(), e.stopPropagation());
    };
    window.addEventListener("keydown", handle, true);
    return () => window.removeEventListener("keydown", handle, true);
  }, []);

  const noRoots = status !== null && status.roots.length === 0;
  const diskNote = tab ? diskNotice(tab.hit.path, tab.onDisk, sc.save) : "";
  const textRef = (id: number) => {
    let ref = texts.current.get(id);
    if (!ref) texts.current.set(id, (ref = { current: null }));
    return ref;
  };

  const keys = (k: string | undefined) => <kbd>{label(k)}</kbd>;

  return (
    <div className={isMac ? "overview" : maximised ? "overview frameless maximised" : "overview frameless"}>
      <nav className={isMac ? (fullscreen ? "ov-top mac fullscreen" : "ov-top mac") : "ov-top"}>
        {!noRoots && (
          <button
            className={treeOpen ? "tab icon active" : "tab icon"}
            title={`${treeOpen ? "Hide" : "Show"} project tree (${label(sc.tree)})`}
            onClick={() => (treeOpen ? hideTree() : setTreeOpen(true))}
          >
            <FolderTree size={14} strokeWidth={2} />
          </button>
        )}
        <div
          className="ov-tabs"
          ref={tabRow}
          onWheel={(e) => {
            // The wheel scrolls the row sideways.
            if (e.deltaY && !e.deltaX) tabRow.current?.scrollBy({ left: e.deltaY });
          }}
        >
          {tabs.map((t) => (
            <div
              key={t.id}
              className={t.id === active ? "tab ov-tab active" : "tab ov-tab"}
              title={`${t.hit.rootName}/${t.hit.path}`}
              onMouseDown={(e) => e.button === 1 && (e.preventDefault(), closeTab(t.id))}
              onClick={() => (setActive(t.id), setCodeFocus((n) => n + 1))}
            >
              <span>{fileName(t.hit.path)}</span>
              {t.dirty && <span className="dirty">●</span>}
              <button className="ov-tab-close" title={`Close (${label(sc.close)})`} onClick={(e) => (e.stopPropagation(), closeTab(t.id))}>
                <X size={12} />
              </button>
            </div>
          ))}
        </div>
        {status && status.projects.length > 1 && (
          <button className="tab project" title={`Switch project (${label(sc.projects)})`} onClick={() => api.showProjects()}>
            <Layers size={14} strokeWidth={2} />
            {status.project}
          </button>
        )}
        <button className="tab icon" title={`Settings (${label(sc.settings)})`} onClick={() => api.openSettings()}>
          <SettingsIcon size={14} strokeWidth={2} />
        </button>
        {!isMac && <WindowButtons maximised={maximised} />}
      </nav>
      <div className="ov-body">
        {treeOpen && !noRoots && (
          <aside className="card ov-side">
            <div className="ov-side-title">{status && status.projects.length > 1 ? status.project : "Project"}</div>
            <Tree
              roots={status?.roots ?? []}
              file={tab?.hit ?? null}
              reveal={treeReveal}
              version={version}
              focus={treeFocus}
              onOpen={openPath}
              onEscape={() => setCodeFocus((n) => n + 1)}
            />
          </aside>
        )}
        <div className="ov-column">
          <section className="card ov-main">
            {tab && (
              <header className="ov-path">
                <span className="path">
                  {crumbs(tab.hit.rootName, tab.hit.path).map((c, i, all) =>
                    i === all.length - 1 ? (
                      <span key={i}>{c.name}</span>
                    ) : (
                      <span key={i}>
                        <button className="crumb" onClick={() => revealInTree(tab.hit.root, c.path)}>
                          {c.name}
                        </button>
                        <span className="crumb-sep">/</span>
                      </span>
                    ),
                  )}
                </span>
              </header>
            )}
            <main className="ov-code">
              {tabs.map((t) => (
                <div key={`${t.id}:${t.rev}`} className="ov-file" hidden={t.id !== active}>
                  <Code
                    shortcuts={sc}
                    onCursor={(line, column) => setCursors((m) => new Map(m).set(t.id, { line, column }))}
                    path={t.hit.path}
                    content={t.draft ?? t.content}
                    original={t.content}
                    line={t.hit.line}
                    column={t.hit.column}
                    reveal={t.reveal}
                    editable
                    focus={t.id === active ? codeFocus : 0}
                    text={textRef(t.id)}
                    onDirty={(d) => setDirty(t.id, d)}
                    onSave={(text) => save(t, text)}
                    onDefinition={async (word: string, at: At) => go(await definitionOrUsages(word, t.hit, at))}
                    onUsages={async (word: string) => go(await usages(word))}
                  />
                </div>
              ))}
              {!tab && (
                <div className="ov-welcome">
                  <h1>Docent</h1>
                  {noRoots ? (
                    <>
                      <p>Add a folder with code to browse and search it.</p>
                      <div className="ov-welcome-actions">
                        <button className="pill active" onClick={addFolder}>
                          <FolderPlus size={14} /> Add folder…
                        </button>
                        <button className="pill" onClick={() => api.openSettings()}>
                          <SettingsIcon size={14} /> Settings
                        </button>
                      </div>
                    </>
                  ) : (
                    <dl className="ov-welcome-keys">
                      <dt>Search everywhere</dt>
                      <dd>{keys(sc.search)}</dd>
                      <dt>Project tree</dt>
                      <dd>{keys(sc.tree)}</dd>
                      <dt>Go to definition</dt>
                      <dd>
                        {keys(sc.definition)} or <kbd>{isMac ? "⌘click" : "Ctrl+click"}</kbd>
                      </dd>
                      <dt>Find usages</dt>
                      <dd>{keys(sc.usages)}</dd>
                      <dt>Switch project</dt>
                      <dd>{keys(sc.projects)}</dd>
                      <dt>Settings</dt>
                      <dd>{keys(sc.settings)}</dd>
                    </dl>
                  )}
                </div>
              )}
            </main>
            {prompt && prompt.kind === "overwrite" && (
              <div className="ov-prompt">
                <span>{fileName(tabs.find((t) => t.id === prompt.id)?.hit.path ?? "")} changed on disk</span>
                <button className="pill active" onClick={() => overwrite(prompt.id)}>
                  Overwrite
                </button>
                <button className="pill" onClick={() => reload(prompt.id)}>
                  Reload
                </button>
                <button className="pill" onClick={() => setPrompt(null)}>
                  Cancel
                </button>
              </div>
            )}
            {prompt && prompt.kind !== "overwrite" && (
              <div className="ov-prompt">
                <span>
                  Unsaved changes in {dirtyTabs.length === 1 ? fileName(dirtyTabs[0].hit.path) : `${dirtyTabs.length} files`}
                  {prompt.kind === "switch" && ` · switching to ${prompt.project}`}
                </span>
                <button className="pill active" onClick={() => saveAll(prompt)}>
                  Save
                </button>
                <button className="pill" onClick={() => discardAll(prompt)}>
                  Discard
                </button>
                <button className="pill" onClick={() => setPrompt(null)}>
                  Cancel
                </button>
              </div>
            )}
            {(notice || diskNote || statusLine(status, progress) || (tab && cursors.has(tab.id))) && (
              <footer className="status">
                <span>
                  {progress && <LoaderCircle size={11} className="spin" />} {notice || diskNote || statusLine(status, progress)}
                </span>
                {tab && cursors.has(tab.id) && (
                  <span>
                    Ln {cursors.get(tab.id)!.line}, Col {cursors.get(tab.id)!.column} · {languageName(tab.hit.path)}
                  </span>
                )}
              </footer>
            )}
          </section>
          {results && (
            <div className="card ov-results" ref={panel} tabIndex={0} onKeyDown={onPanelKey}>
              <div className="ov-results-title">
                <span title={`Next ${label(sc.nextResult)}, previous ${label(sc.prevResult)}`}>
                  {results.title} · {results.hits.length}
                </span>
                <button className="icon-button" title="Close (Esc)" onClick={hideResults}>
                  <X size={13} />
                </button>
              </div>
              <div className="ov-results-body">
                {results.hits.map((hit, i) => (
                  <RowView
                    key={i}
                    row={{ type: "hit", hit }}
                    selected={i === results.selected || (results.selected < 0 && !!tab && sameHit(hit, tab.hit))}
                    onSelect={() => openResult(i)}
                    onOpen={() => {}}
                  />
                ))}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

// WindowButtons are the minimise, maximise and close buttons of the
// frameless window outside macOS. Close goes through the window's close
// hook, so unsaved edits are still asked about.
function WindowButtons({ maximised }: { maximised: boolean }) {
  return (
    <div className="ov-window-buttons">
      <button title="Minimise" onClick={() => Window.Minimise()}>
        <Minus size={14} />
      </button>
      <button title={maximised ? "Restore" : "Maximise"} onClick={() => Window.ToggleMaximise()}>
        {maximised ? <Copy size={12} /> : <Square size={12} />}
      </button>
      <button className="close" title="Close" onClick={() => Window.Close()}>
        <X size={15} />
      </button>
    </div>
  );
}

// useWindowState reports whether the window is maximised, when it has
// square corners and shows the restore button, and whether it is full
// screen, when macOS hides the traffic lights.
function useWindowState(): { maximised: boolean; fullscreen: boolean } {
  const [maximised, setMaximised] = useState(false);
  const [fullscreen, setFullscreen] = useState(false);
  useEffect(() => {
    const update = () => {
      Window.IsMaximised().then(setMaximised);
      Window.IsFullscreen().then(setFullscreen);
    };
    update();
    window.addEventListener("resize", update);
    return () => window.removeEventListener("resize", update);
  }, []);
  return { maximised, fullscreen };
}
