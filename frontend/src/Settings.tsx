import { useEffect, useRef, useState } from "react";
import { FolderPlus, Trash2, X } from "lucide-react";
import * as api from "./api";
import { cmd, modLabel } from "./keys";

// RootDraft keeps the include list as typed, so separators survive editing.
type RootDraft = { path: string; include: string; exclude: string[] | null };

const screens: { id: api.Palette["screen"]; label: string }[] = [
  { id: "focus", label: "Screen with the focused window" },
  { id: "mouse", label: "Screen with the pointer" },
  { id: "primary", label: "Primary screen" },
];

const providers: { id: api.Agent["provider"]; label: string; model: string; baseUrl: string }[] = [
  { id: "anthropic", label: "Anthropic", model: "claude-opus-5-5", baseUrl: "https://api.anthropic.com" },
  { id: "openai", label: "OpenAI-compatible", model: "model name", baseUrl: "https://api.openai.com/v1" },
];

export function Settings(props: { onClose: () => void; onSaved: (message: string) => void }) {
  const [loaded, setLoaded] = useState<api.Settings | null>(null);
  const [roots, setRoots] = useState<RootDraft[]>([]);
  const [agent, setAgent] = useState<api.Agent>({ provider: "anthropic", model: "", baseUrl: "" });
  const [screen, setScreen] = useState<api.Palette["screen"]>("focus");
  const [apiKey, setApiKey] = useState("");
  const [clearKey, setClearKey] = useState(false);
  const [newPath, setNewPath] = useState("");
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const container = useRef<HTMLDivElement>(null);

  useEffect(() => {
    api.getSettings().then(
      (s) => {
        setLoaded(s);
        setRoots(s.roots.map((r) => ({ path: r.path, include: (r.include ?? []).join(", "), exclude: r.exclude })));
        setAgent({ ...s.agent, provider: s.agent.provider || "anthropic" });
        setScreen(s.palette?.screen || "focus");
      },
      (e) => setError(e.message),
    );
    container.current?.focus();
  }, []);

  const provider = providers.find((p) => p.id === agent.provider) ?? providers[0];
  const keyStored = !!loaded?.hasKey && loaded.agent.provider === agent.provider;

  const addRoot = (path: string) => {
    path = path.trim();
    if (path && !roots.some((r) => r.path === path)) setRoots([...roots, { path, include: "", exclude: null }]);
    setNewPath("");
  };

  const pick = async () => {
    try {
      const path = await api.pickFolder();
      if (path) addRoot(path);
    } catch (e) {
      setError((e as Error).message);
    }
  };

  const save = async () => {
    if (!loaded || saving) return;
    setSaving(true);
    setError("");
    try {
      await api.putSettings({
        ...loaded,
        roots: roots.map((r) => ({
          path: r.path,
          include: r.include.split(/[,\n]/).map((s) => s.trim()).filter(Boolean),
          exclude: r.exclude,
        })),
        agent,
        palette: { screen },
        apiKey,
        clearKey: clearKey && !apiKey,
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
    } else if (cmd(e) && e.key === "s") {
      e.preventDefault();
      save();
    }
  };

  return (
    <div className="settings" ref={container} tabIndex={-1} onKeyDown={onKeyDown}>
      <header className="drag settings-header">
        <span>Settings</span>
        <button className="icon-button" onClick={props.onClose} title="Close">
          <X size={16} />
        </button>
      </header>
      <div className="settings-body">
        <section>
          <h3>Code roots</h3>
          <p className="hint">Folders Docent indexes. Limit large repositories to the subdirectories you need.</p>
          {roots.map((r, i) => (
            <div className="root-row" key={r.path}>
              <div className="root-path">{r.path}</div>
              <input
                className="field"
                placeholder="Subdirectories, comma-separated (empty = everything)"
                value={r.include}
                onChange={(e) => setRoots(roots.map((x, j) => (j === i ? { ...x, include: e.target.value } : x)))}
              />
              <button className="icon-button" title="Remove" onClick={() => setRoots(roots.filter((_, j) => j !== i))}>
                <Trash2 size={15} />
              </button>
            </div>
          ))}
          <div className="root-add">
            <button className="pill" onClick={pick}>
              <FolderPlus size={14} /> Add folder…
            </button>
            <input
              className="field"
              placeholder="or type a path and press Enter"
              value={newPath}
              onChange={(e) => setNewPath(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && (e.preventDefault(), addRoot(newPath))}
            />
          </div>
        </section>

        <section>
          <h3>Palette</h3>
          <p className="hint">Where the palette opens.</p>
          <div className="pills">
            {screens.map((s) => (
              <button key={s.id} className={s.id === screen ? "pill active" : "pill"} onClick={() => setScreen(s.id)}>
                {s.label}
              </button>
            ))}
          </div>
        </section>

        <section>
          <h3>Agent</h3>
          <div className="pills">
            {providers.map((p) => (
              <button key={p.id} className={p.id === agent.provider ? "pill active" : "pill"} onClick={() => setAgent({ ...agent, provider: p.id })}>
                {p.label}
              </button>
            ))}
          </div>
          <label>
            <span>Model</span>
            <input className="field" placeholder={provider.model} value={agent.model} onChange={(e) => setAgent({ ...agent, model: e.target.value })} />
          </label>
          <label>
            <span>Base URL</span>
            <input className="field" placeholder={provider.baseUrl} value={agent.baseUrl} onChange={(e) => setAgent({ ...agent, baseUrl: e.target.value })} />
          </label>
          <label>
            <span>API key</span>
            <input
              className="field"
              type="password"
              autoComplete="off"
              placeholder={keyStored && !clearKey ? "Stored in the system credential store · paste to replace" : "Paste API key"}
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
            />
            {keyStored && !apiKey && (
              <button className="link" onClick={() => setClearKey(!clearKey)}>
                {clearKey ? "Keep key" : "Remove key"}
              </button>
            )}
          </label>
          {clearKey && <p className="hint">The stored key will be removed on save.</p>}
        </section>
      </div>
      <footer className="settings-footer">
        <span className="hint">{error ? <span className="error">{error}</span> : loaded?.configPath}</span>
        <div className="actions">
          <button className="pill" onClick={props.onClose}>
            Cancel
          </button>
          <button className="pill active" onClick={save} disabled={saving}>
            {saving ? "Saving…" : `Save ${modLabel}S`}
          </button>
        </div>
      </footer>
    </div>
  );
}
