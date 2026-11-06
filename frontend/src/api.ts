// Functions over the shell's /api endpoints (api.go).

export type Tab = "all" | "types" | "symbols" | "files" | "text" | "agent";

export interface Range {
  line: number;
  start: number;
  end: number;
}

export interface Snippet {
  startLine: number;
  lines: string[];
  matches?: Range[];
}

export interface Hit {
  tab: Tab;
  root: string;
  rootName: string;
  path: string;
  line: number;
  column: number;
  title: string;
  titleMatches?: number[];
  kind?: string;
  container?: string;
  snippet?: Snippet;
}

export interface Section {
  tab: Tab;
  hits: Hit[];
}

export interface RootInfo {
  id: string;
  name: string;
  path: string;
  files: number;
  symbols: number;
  indexedAt: string;
}

// Shortcuts maps command ids to bindings (shortcuts.go; keys.ts).
export type Shortcuts = Record<string, string>;

// Command is a keyboard command with its default binding.
export interface Command {
  id: string;
  label: string;
  default: string;
}

export interface Status {
  configPath: string;
  shortcuts: Shortcuts;
  indexing: boolean;
  // project is the active project; projects lists them all.
  project: string;
  projects: string[];
  roots: RootInfo[];
}

export interface Progress {
  root: string;
  done: number;
  total: number;
  // listed is set once a root indexed for the first time has its files
  // listed: the tree can show them before indexing ends.
  listed?: boolean;
}

async function request<T>(method: string, path: string, params: Record<string, string>, body?: string, signal?: AbortSignal): Promise<T> {
  const res = await fetch(`${path}?${new URLSearchParams(params)}`, { method, body, signal });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error ?? res.statusText);
  return data as T;
}

export const getStatus = () => request<Status>("GET", "/api/status", {});

export const searchAll = (q: string, signal?: AbortSignal) =>
  request<{ sections: Section[] }>("GET", "/api/search", { tab: "all", q }, undefined, signal).then((r) => r.sections ?? []);

export const searchTab = (tab: Exclude<Tab, "all" | "agent">, q: string, regex: boolean, matchCase: boolean, signal?: AbortSignal) =>
  request<{ hits: Hit[] }>("GET", "/api/search", { tab, q, regex: regex ? "1" : "0", case: matchCase ? "1" : "0" }, undefined, signal).then(
    (r) => r.hits ?? [],
  );

export const readFile = (root: string, path: string) =>
  request<{ content: string }>("GET", "/api/file", { root, path }).then((r) => r.content);

export interface Entry {
  name: string;
  dir: boolean;
}

// listDir returns the indexed files and folders directly inside a folder of a root.
export const listDir = (root: string, path: string) =>
  request<Entry[] | null>("GET", "/api/dir", { root, path }).then((r) => r ?? []);

export const writeFile = (root: string, path: string, content: string) =>
  request<{ ok: boolean }>("PUT", "/api/file", { root, path }, content);

export const definitions = (ident: string) =>
  request<{ hits: Hit[] }>("GET", "/api/definitions", { ident }).then((r) => r.hits ?? []);

export const usages = (ident: string) =>
  request<{ hits: Hit[] }>("GET", "/api/usages", { ident }).then((r) => r.hits ?? []);

export type Layout = "compact" | "results" | "viewer" | "viewer-side";

export const setLayout = (name: Layout) => request<{ ok: boolean }>("POST", "/api/layout", { name });

export interface Root {
  path: string;
  include: string[] | null;
  exclude: string[] | null;
}

export interface Project {
  name: string;
  roots: Root[];
}

export interface Agent {
  provider: "" | "anthropic" | "openai";
  model: string;
  baseUrl: string;
}

export interface Palette {
  screen: "" | "focus" | "mouse" | "primary";
}

export interface Settings {
  configPath: string;
  active: string;
  projects: Project[];
  agent: Agent;
  palette: Palette;
  shortcuts: Shortcuts;
  commands: Command[];
  hasKey: boolean;
  apiKey?: string;
  clearKey?: boolean;
}

export const getSettings = () => request<Settings>("GET", "/api/settings", {});

export const putSettings = (s: Settings) => request<Settings>("PUT", "/api/settings", {}, JSON.stringify(s));

// pickFolder asks for a folder with the system dialog attached to the
// calling window; "" if cancelled.
export const pickFolder = (window: "palette" | "overview" = "palette") =>
  request<{ path: string }>("POST", "/api/pick-folder", { window }).then((r) => r.path);

// addFolder is the first run's "Add folder…": a folder picked with the system
// dialog goes into the active project, or a new one if there is none.
export const addFolder = (window: "palette" | "overview" = "palette") => request<{ ok: boolean }>("POST", "/api/add-folder", { window });

// ProjectSwitch is sent to both windows when the active project changes.
export interface ProjectSwitch {
  from: string;
  to: string;
}

// switchProject makes a project active. switched is false while the overview
// window asks about its unsaved edits first; it switches once they are saved
// or discarded.
export const switchProject = (name: string) => request<{ switched: boolean }>("POST", "/api/project/switch", { name });

// newProject creates a project from a folder picked with the system dialog
// and switches to it; name is "" if cancelled.
export const newProject = (window: "palette" | "overview" = "palette") =>
  request<{ name: string; switched: boolean }>("POST", "/api/project/new", { window });

// showProjects summons the palette with the project list.
export const showProjects = () => request<{ ok: boolean }>("POST", "/api/projects/show", {});

// setOverviewDirty tells the shell whether the overview window has unsaved edits.
export const setOverviewDirty = (dirty: boolean) => request<{ ok: boolean }>("POST", "/api/overview/dirty", { dirty: dirty ? "1" : "0" });

export interface AgentEvent {
  kind: "turn" | "thinking" | "call" | "step" | "text" | "done" | "error";
  tool?: string;
  input?: string;
  text?: string;
  hits?: Hit[];
}

// Turn is an earlier question of an agent thread and the answer it got.
export interface Turn {
  question: string;
  answer: string;
}

// askAgent starts a run; thread holds the earlier turns the question follows up on.
export const askAgent = (thread: Turn[], question: string) =>
  request<{ run: number }>("POST", "/api/agent/ask", {}, JSON.stringify({ thread, question })).then((r) => r.run);

export const stopAgent = () => request<{ ok: boolean }>("POST", "/api/agent/stop", {});

export interface Span {
  from: number;
  to: number;
  class: string;
}

export const highlight = (path: string, text: string) =>
  request<{ spans: Span[] }>("POST", "/api/highlight", { path }, text).then((r) => r.spans ?? []);

// openInOverview opens a file as a tab in the overview window, with unsaved
// text if given, and hides the palette.
export const openInOverview = (hit: Hit, content?: string) =>
  request<{ ok: boolean }>("POST", "/api/overview/open", {}, JSON.stringify({ hit, content }));

export const hideOverview = () => request<{ ok: boolean }>("POST", "/api/overview/hide", {});

export const openSettings = () => request<{ ok: boolean }>("POST", "/api/settings/open", {});

// present moves the palette, shown off screen, into place once the
// frontend has painted (main.go, showSettings).
export const present = () => request<{ ok: boolean }>("POST", "/api/present", {});

export const hide = () => request<{ ok: boolean }>("POST", "/api/hide", {});

export const reindex = () => request<{ ok: boolean }>("POST", "/api/reindex", {});
