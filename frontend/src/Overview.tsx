import { useEffect, useRef, useState, type MutableRefObject } from "react";
import { Events, Window } from "@wailsio/runtime";
import { Copy, FolderPlus, FolderTree, LoaderCircle, Minus, Settings as SettingsIcon, Square, X } from "lucide-react";
import * as api from "./api";
import type { Hit, Progress, Status } from "./api";
import { Code, type At } from "./Code";
import { Tree } from "./Tree";
import { crumbs, RowView, sameHit } from "./Rows";
import { definitionOrUsages, usages, type Where } from "./navigate";
import { statusLine } from "./App";
import { altLabel, cmd, hotkeyLabel, isMac, modLabel } from "./keys";

// FileTab is a file open in a tab. content is the file as read from disk;
// draft is unsaved text the tab was opened with. rev remounts the editor
// (new draft, discarded edits); reveal scrolls to hit's line again.
type FileTab = {
  id: number;
  hit: Hit;
  content: string;
  draft?: string;
  dirty: boolean;
  rev: number;
  reveal: number;
};

// Results is the results panel: definitions or usages to choose from.
type Results = { title: string; hits: Hit[]; selected: number };

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
  const [notice, setNotice] = useState("");
  // discardArmed is the tab whose next close discards its edits.
  const [discardArmed, setDiscardArmed] = useState(0);
  // closing shows the prompt about unsaved edits when the window is closed.
  const [closing, setClosing] = useState(false);
  const texts = useRef(new Map<number, MutableRefObject<(() => string) | null>>());
  const panel = useRef<HTMLDivElement>(null);
  const tabsRef = useRef(tabs);
  tabsRef.current = tabs;
  const maximised = useMaximised();

  const tab = tabs.find((t) => t.id === active) ?? null;
  const dirtyTabs = tabs.filter((t) => t.dirty);

  const openFile = async (hit: Hit, draft?: string) => {
    setNotice("");
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
    if (tabsRef.current.some((t) => t.dirty)) setClosing(true);
    else api.hideOverview();
  };

  // Event handlers are registered once; they call the latest versions.
  const handlers = useRef({ openFile, requestClose });
  handlers.current = { openFile, requestClose };

  useEffect(() => {
    api.getStatus().then(setStatus);
    const offs = [
      Events.On("overview:open", (e) => {
        const { hit, content } = e.data as { hit: Hit; content?: string };
        handlers.current.openFile(hit, content ?? undefined);
      }),
      Events.On("overview:close", () => handlers.current.requestClose()),
      Events.On("index:progress", (e) => setProgress(e.data as Progress)),
      Events.On("index:done", () => {
        setProgress(null);
        api.getStatus().then(setStatus);
        setVersion((v) => v + 1);
      }),
    ];
    return () => offs.forEach((off) => off());
  }, []);

  const closeTab = (id: number) => {
    const t = tabs.find((t) => t.id === id);
    if (!t) return;
    if (t.dirty && discardArmed !== id) {
      setActive(id);
      setDiscardArmed(id);
      setNotice(`Unsaved changes in ${fileName(t.hit.path)}: ${modLabel}S to save, close again to discard`);
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

  const save = async (t: FileTab, text: string) => {
    try {
      await api.writeFile(t.hit.root, t.hit.path, text);
      setDirty(t.id, false);
      setNotice(`Saved ${t.hit.path}`);
    } catch (e) {
      setNotice((e as Error).message);
    }
  };

  const saveAllAndClose = async () => {
    try {
      for (const t of dirtyTabs) {
        const text = texts.current.get(t.id)?.current?.();
        if (text !== undefined) await api.writeFile(t.hit.root, t.hit.path, text);
      }
      setTabs((ts) => ts.map((t) => (t.dirty ? { ...t, dirty: false } : t)));
      setClosing(false);
      api.hideOverview();
    } catch (e) {
      setNotice((e as Error).message);
    }
  };

  // discardAndClose reloads edited tabs from disk and hides the window.
  const discardAndClose = async () => {
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
    setClosing(false);
    api.hideOverview();
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

  const toggleTree = () => {
    if (treeOpen && document.activeElement?.closest(".tree")) {
      setTreeOpen(false);
      setTreeReveal(null);
      setCodeFocus((n) => n + 1);
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
      const path = await api.pickFolder("overview");
      if (!path) return;
      const s = await api.getSettings();
      await api.putSettings({
        ...s,
        roots: [...s.roots, { path, include: null, exclude: null }],
      });
      setStatus(await api.getStatus());
    } catch (e) {
      setNotice((e as Error).message);
    }
  };

  // Window shortcuts; the global key handler, registered once, calls the
  // latest version.
  const onKey = useRef<(e: KeyboardEvent) => boolean>(() => false);
  onKey.current = (e) => {
    if (e.ctrlKey && e.key === "Tab") {
      switchTab(e.shiftKey ? -1 : 1);
      return true;
    }
    if (!cmd(e) || e.shiftKey) return false;
    if (e.altKey) {
      if (e.key !== "ArrowDown" && e.key !== "ArrowUp") return false;
      stepResult(e.key === "ArrowDown" ? 1 : -1);
      return true;
    }
    // Physical keys, so shortcuts work in any keyboard layout.
    switch (e.code) {
      case "Digit1":
        toggleTree();
        return true;
      case "KeyW":
        if (tab) closeTab(tab.id);
        return true;
      case "Comma":
        api.openSettings();
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
  const textRef = (id: number) => {
    let ref = texts.current.get(id);
    if (!ref) texts.current.set(id, (ref = { current: null }));
    return ref;
  };

  const keys = (k: string) => <kbd>{k}</kbd>;
  const hotkey = status ? hotkeyLabel(status.hotkey) : "";

  return (
    <div className={isMac ? "overview" : maximised ? "overview frameless maximised" : "overview frameless"}>
      <nav className={isMac ? "ov-top mac" : "ov-top"}>
        {!noRoots && (
          <button
            className={treeOpen ? "tab icon active" : "tab icon"}
            title={`${treeOpen ? "Hide" : "Show"} project tree (${modLabel}1)`}
            onClick={() => (treeOpen ? toggleTree() : setTreeOpen(true))}
          >
            <FolderTree size={14} strokeWidth={2} />
          </button>
        )}
        <div className="ov-tabs">
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
              <button className="ov-tab-close" title={`Close (${modLabel}W)`} onClick={(e) => (e.stopPropagation(), closeTab(t.id))}>
                <X size={12} />
              </button>
            </div>
          ))}
        </div>
        <button className="tab icon" title={`Settings (${modLabel},)`} onClick={() => api.openSettings()}>
          <SettingsIcon size={14} strokeWidth={2} />
        </button>
        {!isMac && <WindowButtons maximised={maximised} />}
      </nav>
      <div className="ov-body">
        {treeOpen && !noRoots && (
          <aside className="card ov-side">
            <div className="ov-side-title">Project</div>
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
                <span className="hints">
                  {modLabel}B definition · {altLabel}F7 usages · {modLabel}L line · {modLabel}F find
                </span>
              </header>
            )}
            <main className="ov-code">
              {tabs.map((t) => (
                <div key={`${t.id}:${t.rev}`} className="ov-file" hidden={t.id !== active}>
                  <Code
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
                      <dd>{keys(hotkey)}</dd>
                      <dt>Project tree</dt>
                      <dd>{keys(`${modLabel}1`)}</dd>
                      <dt>Go to definition</dt>
                      <dd>
                        {keys(`${modLabel}B`)} or {keys(`${modLabel}click`)}
                      </dd>
                      <dt>Find usages</dt>
                      <dd>{keys(`${altLabel}F7`)}</dd>
                      <dt>Settings</dt>
                      <dd>{keys(`${modLabel},`)}</dd>
                    </dl>
                  )}
                </div>
              )}
            </main>
            {closing && (
              <div className="ov-prompt">
                <span>Unsaved changes in {dirtyTabs.length === 1 ? fileName(dirtyTabs[0].hit.path) : `${dirtyTabs.length} files`}</span>
                <button className="pill active" onClick={saveAllAndClose}>
                  Save
                </button>
                <button className="pill" onClick={discardAndClose}>
                  Discard
                </button>
                <button className="pill" onClick={() => setClosing(false)}>
                  Cancel
                </button>
              </div>
            )}
            <footer className="status">
              <span>
                {progress && <LoaderCircle size={11} className="spin" />} {notice || statusLine(status, progress)}
              </span>
              <span>{hotkey && `${hotkey} search`}</span>
            </footer>
          </section>
          {results && (
            <div className="card ov-results" ref={panel} tabIndex={0} onKeyDown={onPanelKey}>
              <div className="ov-results-title">
                <span>
                  {results.title} · {results.hits.length} · {isMac ? "⌘⌥" : "Ctrl+Alt+"}↓↑
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

// useMaximised reports whether the window is maximised: it then has square
// corners and shows the restore button.
function useMaximised(): boolean {
  const [maximised, setMaximised] = useState(false);
  useEffect(() => {
    const update = () => Window.IsMaximised().then(setMaximised);
    update();
    window.addEventListener("resize", update);
    return () => window.removeEventListener("resize", update);
  }, []);
  return maximised;
}
