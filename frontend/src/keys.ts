// Platform modifiers: shortcuts are written with Cmd, which is Ctrl outside
// macOS (DESIGN.md, Features).

export const isMac = /Mac/.test(navigator.platform);

// cmd reports whether the platform's command modifier is held.
export function cmd(e: { metaKey: boolean; ctrlKey: boolean }): boolean {
  return isMac ? e.metaKey : e.ctrlKey;
}

// hotkeyLabel renders an accelerator such as "CmdOrCtrl+Shift+Space".
export function hotkeyLabel(accel: string): string {
  if (!isMac) return accel.replace("CmdOrCtrl", "Ctrl");
  return accel.replace("CmdOrCtrl", "⌘").replace("Shift", "⇧").replaceAll("+", "");
}

// Labels for the command and option modifiers in hints.
export const modLabel = isMac ? "⌘" : "Ctrl+";
export const altLabel = isMac ? "⌥" : "Alt+";
