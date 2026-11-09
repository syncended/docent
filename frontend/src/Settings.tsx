import { useEffect, useRef, useState } from "react";
import { FolderPlus, Plus, RotateCcw, Trash2, X } from "lucide-react";
import * as api from "./api";
import { AgentSettings, type AgentDraft } from "./AgentSettings";
import { canonical, capturing, fromEvent, label, matches, type Shortcuts } from "./keys";

// RootDraft keeps the include list as typed, so separators survive editing.
type RootDraft = { path: string; include: string; exclude: string[] | null };

// ProjectDraft is a project as edited; key follows it through renames.
type ProjectDraft = { key: number; name: string; roots: RootDraft[]; newPath: string };

let nextKey = 1;

// rootStats describes what the index holds for a root: files, symbols and
// when it was indexed; "" for a root that is not indexed.
function rootStats(r: api.RootInfo | undefined): string {
  if (!r) return "";
  return `${r.files.toLocaleString()} files · ${r.symbols.toLocaleString()} symbols · indexed ${ago(r.indexedAt)}`;
}

// ago renders a time as "just now", "5 min ago", "3 h ago" or "2 days ago".
function ago(at: string): string {
  const s = (Date.now() - new Date(at).getTime()) / 1000;
  if (!(s >= 0)) return "";
  if (s < 60) return "just now";
  if (s < 3600) return `${Math.round(s / 60)} min ago`;
  if (s < 86400) return `${Math.round(s / 3600)} h ago`;
  const d = Math.round(s / 86400);
  return d === 1 ? "yesterday" : `${d} days ago`;
}

// Section is a settings tab (DESIGN.md, Settings).
export type Section = "Projects" | "Agent" | "Palette" | "Shortcuts";

const sections: Section[] = ["Projects", "Agent", "Palette", "Shortcuts"];

// lastSection is the tab settings open at, until Docent restarts.
let lastSection: Section = "Projects";

const draft = (p: api.Project): ProjectDraft => ({
  key: nextKey++,
  name: p.name,
  roots: p.roots.map((r) => ({ path: r.path, include: (r.include ?? []).join(", "), exclude: r.exclude })),
  newPath: "",
});

const screens: { id: api.Palette["screen"]; label: string }[] = [
  { id: "focus", label: "Screen with the focused window" },
  { id: "mouse", label: "Screen with the pointer" },
  { id: "primary", label: "Primary screen" },
];

// section, if given, is the tab to open at; otherwise the last one used.
export function Settings(props: { section?: Section; onClose: () => void; onSaved: (message: string) => void }) {
  const [loaded, setLoaded] = useState<api.Settings | null>(null);
  // indexed is what the index holds per root path, for the active project.
  const [indexed, setIndexed] = useState<api.RootInfo[]>([]);
  const [section, setSection] = useState<Section>(props.section ?? lastSection);
  lastSection = section;
  const [projects, setProjects] = useState<ProjectDraft[]>([]);
  const [activeKey, setActiveKey] = useState(0);
  const [agent, setAgent] = useState<AgentDraft>({ agent: { provider: "anthropic", providers: {} }, keys: {}, clearKeys: new Set() });
  const [catalog, setCatalog] = useState<{ providers: api.Provider[]; levels: api.Level[]; defaultLevel: api.Level } | null>(null);
  const [screen, setScreen] = useState<api.Palette["screen"]>("focus");
  const [shortcuts, setShortcuts] = useState<Shortcuts>({});
  // recording is the command whose binding the next key press sets.
  const [recording, setRecording] = useState<string | null>(null);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const container = useRef<HTMLDivElement>(null);

  useEffect(() => {
    api.getStatus().then((s) => setIndexed(s.roots), () => {});
    api.getProviders().then(setCatalog, (e) => setError(e.message));
    api.getSettings().then(
      (s) => {
        setLoaded(s);
        // The active project is shown first (DESIGN.md, Settings).
        const drafts = [...s.projects].sort((a, b) => Number(b.name === s.active) - Number(a.name === s.active)).map(draft);
        setProjects(drafts);
        setActiveKey(drafts[0]?.key ?? 0);
        setAgent({ agent: { provider: s.agent.provider || "anthropic", providers: s.agent.providers ?? {} }, keys: {}, clearKeys: new Set() });
        setScreen(s.palette?.screen || "focus");
        setShortcuts(s.shortcuts);
      },
      (e) => setError(e.message),
    );
    container.current?.focus();
  }, []);

  // While a binding is recorded, the windows' own shortcuts stay out of the way.
  useEffect(() => {
    capturing.active = recording !== null;
    return () => {
      capturing.active = false;
    };
  }, [recording]);

  const commands = loaded?.commands ?? [];
  // conflicts are the commands sharing a binding with another, by id.
  const conflicts = new Map<string, string>();
  for (const a of commands) {
    for (const b of commands) {
      if (a.id < b.id && canonical(shortcuts[a.id]) === canonical(shortcuts[b.id])) {
        conflicts.set(a.id, b.label);
        conflicts.set(b.id, a.label);
      }
    }
  }
  const changed = commands.filter((c) => canonical(shortcuts[c.id]) !== canonical(c.default));
  // groups are the headings of the shortcuts tab, in the order of commands.
  const groups = [...new Set(commands.map((c) => c.group))];

  // onRecord sets the recorded command's binding from the keys pressed;
  // Esc keeps the old one, a modifier alone keeps recording.
  const onRecord = (id: string) => (e: React.KeyboardEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (e.key === "Escape") return setRecording(null);
    const binding = fromEvent(e);
    if (!binding) return;
    setShortcuts({ ...shortcuts, [id]: binding });
    setRecording(null);
  };

  const update = (key: number, f: (p: ProjectDraft) => ProjectDraft) => setProjects((ps) => ps.map((p) => (p.key === key ? f(p) : p)));

  const addRoot = (key: number, path: string) => {
    path = path.trim();
    update(key, (p) => ({
      ...p,
      roots: path && !p.roots.some((r) => r.path === path) ? [...p.roots, { path, include: "", exclude: null }] : p.roots,
      newPath: "",
    }));
  };

  const pick = async (key: number) => {
    try {
      const path = await api.pickFolder();
      if (path) addRoot(key, path);
    } catch (e) {
      setError((e as Error).message);
    }
  };

  const addProject = () => {
    const names = new Set(projects.map((p) => p.name));
    let name = "New project";
    for (let n = 2; names.has(name); n++) name = `New project ${n}`;
    const p = draft({ name, roots: [] });
    setProjects([...projects, p]);
    if (projects.length === 0) setActiveKey(p.key);
  };

  // Removing the active project makes the first remaining one active.
  const removeProject = (key: number) => {
    const rest = projects.filter((p) => p.key !== key);
    setProjects(rest);
    if (key === activeKey) setActiveKey(rest[0]?.key ?? 0);
  };

  const save = async () => {
    if (!loaded || saving || conflicts.size > 0) return;
    setSaving(true);
    setError("");
    try {
      await api.putSettings({
        ...loaded,
        active: projects.find((p) => p.key === activeKey)?.name ?? "",
        projects: projects.map((p) => ({
          name: p.name,
          roots: p.roots.map((r) => ({
            path: r.path,
            include: r.include.split(/[,\n]/).map((s) => s.trim()).filter(Boolean),
            exclude: r.exclude,
          })),
        })),
        agent: agent.agent,
        palette: { screen },
        shortcuts,
        apiKeys: Object.fromEntries(Object.entries(agent.keys).filter(([, k]) => k.trim())),
        clearKeys: [...agent.clearKeys],
      });
      props.onSaved("Settings saved · reindexing");
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setSaving(false);
    }
  };

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Escape") {
      e.preventDefault();
      props.onClose();
    } else if (matches(shortcuts.save, e)) {
      e.preventDefault();
      save();
    }
  };

  return (
    <div className="settings" ref={container} tabIndex={-1} onKeyDown={onKeyDown}>
      <header className="drag settings-header">
        <span>Settings</span>
        <nav className="settings-tabs">
          {sections.map((s) => (
            <button key={s} className={s === section ? "tab active" : "tab"} onClick={() => setSection(s)}>
              {s}
            </button>
          ))}
        </nav>
        <button className="icon-button" onClick={props.onClose} title="Close">
          <X size={16} />
        </button>
      </header>
      <div className="settings-body">
        <section hidden={section !== "Projects"}>
          <p className="hint">
            Folders Docent indexes, grouped into projects. Search, the agent and the overview window see the active project; {label(shortcuts.projects)} switches.
            Limit large repositories to the subdirectories you need.
          </p>
          {projects.map((p) => (
            <div className="project-block" key={p.key}>
              <div className="project-head">
                <input
                  className="field project-name"
                  placeholder="Project name"
                  value={p.name}
                  onChange={(e) => update(p.key, (x) => ({ ...x, name: e.target.value }))}
                />
                {p.key === activeKey && <span className="project-active">active</span>}
                <button className="icon-button" title="Remove project" onClick={() => removeProject(p.key)}>
                  <Trash2 size={15} />
                </button>
              </div>
              {p.roots.map((r, i) => (
                <div className="root-row" key={r.path}>
                  <div className="root-main">
                    <div className="root-path">{r.path}</div>
                    {rootStats(indexed.find((x) => x.path === r.path)) && <div className="root-stats">{rootStats(indexed.find((x) => x.path === r.path))}</div>}
                  </div>
                  <input
                    className="field"
                    placeholder="Subdirectories, comma-separated (empty = everything)"
                    value={r.include}
                    onChange={(e) => update(p.key, (x) => ({ ...x, roots: x.roots.map((y, j) => (j === i ? { ...y, include: e.target.value } : y)) }))}
                  />
                  <button className="icon-button" title="Remove folder" onClick={() => update(p.key, (x) => ({ ...x, roots: x.roots.filter((_, j) => j !== i) }))}>
                    <Trash2 size={15} />
                  </button>
                </div>
              ))}
              <div className="root-add">
                <button className="pill" onClick={() => pick(p.key)}>
                  <FolderPlus size={14} /> Add folder…
                </button>
                <input
                  className="field"
                  placeholder="or type a path and press Enter"
                  value={p.newPath}
                  onChange={(e) => update(p.key, (x) => ({ ...x, newPath: e.target.value }))}
                  onKeyDown={(e) => e.key === "Enter" && (e.preventDefault(), addRoot(p.key, p.newPath))}
                />
              </div>
            </div>
          ))}
          <button className="pill" onClick={addProject}>
            <Plus size={14} /> New project
          </button>
        </section>

        <section hidden={section !== "Palette"}>
          <p className="hint">Where the palette opens.</p>
          <div className="pills">
            {screens.map((s) => (
              <button key={s.id} className={s.id === screen ? "pill active" : "pill"} onClick={() => setScreen(s.id)}>
                {s.label}
              </button>
            ))}
          </div>
        </section>

        <section hidden={section !== "Shortcuts"}>
          <p className="hint">Click a shortcut and press the keys; Esc keeps it. A shortcut needs Cmd, Ctrl or Alt, unless it is a function key.</p>
          {groups.map((g) => (
            <div className="shortcut-group" key={g}>
              <h4>{g}</h4>
              <div className="shortcut-grid">
                {commands
                  .filter((c) => c.group === g)
                  .map((c) => (
                    <div key={c.id} className={conflicts.has(c.id) ? "shortcut conflict" : "shortcut"}>
                      <span className="shortcut-label">
                        {c.label}
                        {conflicts.has(c.id) && <span className="error">also {conflicts.get(c.id)}</span>}
                      </span>
                      {canonical(shortcuts[c.id]) !== canonical(c.default) && (
                        <button className="icon-button" title={`Reset to ${label(c.default)}`} onClick={() => setShortcuts({ ...shortcuts, [c.id]: c.default })}>
                          <RotateCcw size={13} />
                        </button>
                      )}
                      <button
                        className={recording === c.id ? "key recording" : "key"}
                        title="Click, then press the keys"
                        onClick={() => setRecording(c.id)}
                        onKeyDown={recording === c.id ? onRecord(c.id) : undefined}
                        onBlur={() => setRecording((r) => (r === c.id ? null : r))}
                      >
                        <kbd>{recording === c.id ? "Press keys…" : label(shortcuts[c.id])}</kbd>
                      </button>
                    </div>
                  ))}
              </div>
            </div>
          ))}
          {changed.length > 0 && (
            <button className="link" onClick={() => setShortcuts(Object.fromEntries(commands.map((c) => [c.id, c.default])))}>
              Reset all to defaults
            </button>
          )}
        </section>

        <section hidden={section !== "Agent"}>
          {catalog && loaded && (
            <AgentSettings
              providers={catalog.providers}
              levels={catalog.levels}
              defaultLevel={catalog.defaultLevel}
              stored={loaded.keys ?? []}
              draft={agent}
              onChange={setAgent}
            />
          )}
        </section>
      </div>
      <footer className="settings-footer">
        <span className="hint">{error ? <span className="error">{error}</span> : loaded?.configPath}</span>
        <div className="actions">
          <button className="pill" onClick={props.onClose}>
            Cancel
          </button>
          <button className="pill active" onClick={save} disabled={saving || conflicts.size > 0}>
            {saving ? "Saving…" : `Save ${label(shortcuts.save)}`}
          </button>
        </div>
      </footer>
    </div>
  );
}
