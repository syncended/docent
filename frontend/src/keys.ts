// Shortcuts: matching the bindings from the config against key events and
// rendering them (shortcuts.go; DESIGN.md, Settings).
//
// A binding is an accelerator such as "CmdOrCtrl+Alt+Down": modifiers
// CmdOrCtrl, Ctrl, Alt and Shift, then a key: a character, Space, Enter,
// Tab, Up, Down, Left, Right, Backspace, Delete, Home, End or F1 to F24.
// Keys are matched by physical position (KeyboardEvent.code), so bindings
// work in any keyboard layout (DESIGN.md, Features).

import type { Shortcuts } from "./api";

export type { Shortcuts };

export const isMac = /Mac/.test(navigator.platform);

// cmd reports whether the platform's command modifier is held.
export function cmd(e: { metaKey: boolean; ctrlKey: boolean }): boolean {
  return isMac ? e.metaKey : e.ctrlKey;
}

// altLabel is the option or alt modifier in labels of fixed keys.
export const altLabel = isMac ? "⌥" : "Alt+";

// capturing is set while the settings screen records a binding, so the
// windows' own shortcuts stay out of the way.
export const capturing = { active: false };

type KeyEvent = { code: string; metaKey: boolean; ctrlKey: boolean; altKey: boolean; shiftKey: boolean };

type Parsed = { cmd: boolean; ctrl: boolean; alt: boolean; shift: boolean; key: string };

// parse splits a binding; key is lower case. Null for anything malformed.
function parse(binding: string | undefined): Parsed | null {
  if (!binding) return null;
  const parts = binding.split("+");
  const p: Parsed = { cmd: false, ctrl: false, alt: false, shift: false, key: parts[parts.length - 1].toLowerCase() };
  if (!p.key) return null;
  for (const m of parts.slice(0, -1)) {
    const mod = m.toLowerCase();
    if (mod === "cmdorctrl" || mod === "cmd") p.cmd = true;
    else if (mod === "ctrl") p.ctrl = true;
    else if (mod === "alt" || mod === "option") p.alt = true;
    else if (mod === "shift") p.shift = true;
    else return null;
  }
  return p;
}

// codes maps the keys a binding names, besides letters, digits and
// function keys, to KeyboardEvent.code.
const codes: Record<string, string> = {
  space: "Space",
  enter: "Enter",
  tab: "Tab",
  up: "ArrowUp",
  down: "ArrowDown",
  left: "ArrowLeft",
  right: "ArrowRight",
  backspace: "Backspace",
  delete: "Delete",
  home: "Home",
  end: "End",
  ",": "Comma",
  ".": "Period",
  "/": "Slash",
  ";": "Semicolon",
  "'": "Quote",
  "[": "BracketLeft",
  "]": "BracketRight",
  "\\": "Backslash",
  "-": "Minus",
  "=": "Equal",
  "`": "Backquote",
};

function codeOf(key: string): string {
  if (codes[key]) return codes[key];
  if (/^[a-z]$/.test(key)) return "Key" + key.toUpperCase();
  if (/^[0-9]$/.test(key)) return "Digit" + key;
  return key.toUpperCase();
}

// keyOf is the key a binding names for a KeyboardEvent.code; null for
// modifiers and keys bindings cannot name.
function keyOf(code: string): string | null {
  const letter = /^Key([A-Z])$/.exec(code);
  if (letter) return letter[1];
  const digit = /^Digit([0-9])$/.exec(code);
  if (digit) return digit[1];
  if (/^F([1-9]|1[0-9]|2[0-4])$/.test(code)) return code;
  if (code === "NumpadEnter") return "Enter";
  for (const [key, c] of Object.entries(codes)) {
    if (c === code) return key.length === 1 ? key : key[0].toUpperCase() + key.slice(1);
  }
  return null;
}

// matches reports whether e presses the binding: the modifiers exactly and
// the key by physical position. Outside macOS Ctrl means CmdOrCtrl.
export function matches(binding: string | undefined, e: KeyEvent): boolean {
  const p = parse(binding);
  if (!p) return false;
  const meta = isMac ? p.cmd : false;
  const ctrl = isMac ? p.ctrl : p.cmd || p.ctrl;
  if (e.metaKey !== meta || e.ctrlKey !== ctrl || e.altKey !== p.alt || e.shiftKey !== p.shift) return false;
  return (e.code === "NumpadEnter" ? "Enter" : e.code) === codeOf(p.key);
}

// fromEvent renders the keys held in e as a binding; null if they make
// none: a modifier alone, or a key without Cmd, Ctrl or Alt that is not a
// function key (DESIGN.md, Settings).
export function fromEvent(e: KeyEvent): string | null {
  const key = keyOf(e.code);
  if (!key) return null;
  const mods: string[] = [];
  if (isMac ? e.metaKey : e.ctrlKey) mods.push("CmdOrCtrl");
  if (isMac && e.ctrlKey) mods.push("Ctrl");
  if (e.altKey) mods.push("Alt");
  if (e.shiftKey) mods.push("Shift");
  if (!mods.some((m) => m !== "Shift") && !/^F\d+$/.test(key)) return null;
  return [...mods, key].join("+");
}

// canonical renders a binding so that two meaning the same keys on this
// platform compare equal, as shortcuts.go does.
export function canonical(binding: string | undefined): string {
  const p = parse(binding);
  if (!p) return binding ?? "";
  const mods = isMac ? [p.cmd && "cmd", p.ctrl && "ctrl"] : [(p.cmd || p.ctrl) && "ctrl"];
  return [...mods, p.alt && "alt", p.shift && "shift"].filter(Boolean).concat(p.key).join("+");
}

// keyLabels are the keys shown as symbols on macOS; elsewhere keys are
// shown by name.
const keyLabels: Record<string, string> = isMac ? { up: "↑", down: "↓", left: "←", right: "→", enter: "↩", backspace: "⌫", delete: "⌦" } : {};

function keyLabel(key: string): string {
  if (keyLabels[key]) return keyLabels[key];
  if (key.length === 1 || /^f\d+$/.test(key)) return key.toUpperCase();
  return key[0].toUpperCase() + key.slice(1);
}

// label renders a binding for people: "⌘⇧Space" on macOS, "Ctrl+Shift+Space"
// elsewhere. A malformed binding is shown as it is.
export function label(binding: string | undefined): string {
  const p = parse(binding);
  if (!p) return binding ?? "";
  const key = keyLabel(p.key);
  if (isMac) return [p.ctrl && "⌃", p.alt && "⌥", p.shift && "⇧", p.cmd && "⌘"].filter(Boolean).join("") + key;
  return [(p.cmd || p.ctrl) && "Ctrl", p.alt && "Alt", p.shift && "Shift"].filter(Boolean).concat(key).join("+");
}

const cmNames: Record<string, string> = {
  space: " ",
  enter: "Enter",
  tab: "Tab",
  up: "ArrowUp",
  down: "ArrowDown",
  left: "ArrowLeft",
  right: "ArrowRight",
  backspace: "Backspace",
  delete: "Delete",
  home: "Home",
  end: "End",
};

// cmKey renders a binding as a CodeMirror key name, such as "Mod-Alt-ArrowDown".
export function cmKey(binding: string): string {
  const p = parse(binding);
  if (!p) return binding;
  const key = cmNames[p.key] ?? (/^f\d+$/.test(p.key) ? p.key.toUpperCase() : p.key);
  return [p.cmd && "Mod", p.ctrl && "Ctrl", p.alt && "Alt", p.shift && "Shift"].filter(Boolean).concat(key).join("-");
}
