import { useEffect, useRef, useState, type MutableRefObject } from "react";
import { Compartment, EditorState, Prec, type Extension } from "@codemirror/state";
import { EditorView, keymap } from "@codemirror/view";
import { basicSetup } from "codemirror";
import { gotoLine, openSearchPanel, search } from "@codemirror/search";
import { oneDark } from "@codemirror/theme-one-dark";
import { languageFor } from "./languages";
import { serverHighlight, serverHighlighted } from "./highlight";
import { cmKey, cmd, label, type Shortcuts } from "./keys";
import { findPanel } from "./find";

// Where a symbol was clicked or the cursor is: line is 1-based, column is a
// 0-based offset of the word in the line.
export type At = { line: number; column: number };

export interface CodeProps {
  path: string;
  content: string;
  // original is the saved text when content is an unsaved draft; edits are
  // compared to it. Defaults to content.
  original?: string;
  // line is 1-based; column is a 0-based offset in the line.
  line: number;
  column?: number;
  editable: boolean;
  onDirty?: (dirty: boolean) => void;
  onSave?: (text: string) => void;
  // onDefinition is go to definition (Cmd+B, Cmd+click), at the symbol.
  onDefinition?: (word: string, at: At) => void;
  onUsages?: (word: string) => void;
  onEscape?: () => void;
  // onCursor reports the cursor's 1-based line and column as it moves.
  onCursor?: (line: number, column: number) => void;
  // focus moves keyboard focus to the editor; it changes on every request.
  focus?: number;
  // reveal scrolls to line again; it changes on every request.
  reveal?: number;
  // text is set to a function returning the current text.
  text?: MutableRefObject<(() => string) | null>;
  // shortcuts binds save, go to definition, find usages, go to line and
  // find in file (keys.ts); none are bound without it.
  shortcuts?: Shortcuts;
}

type Menu = { x: number; y: number; word: string; at: At };

// Code shows a file with CodeMirror: read-only for previews, editable in the viewer.
export function Code(props: CodeProps) {
  const host = useRef<HTMLDivElement>(null);
  const view = useRef<EditorView>();
  // Key bindings are created once; they read the latest props from here.
  const latest = useRef(props);
  latest.current = props;
  // keys holds the command bindings, replaced when the shortcuts change.
  const keys = useRef(new Compartment());
  const [menu, setMenu] = useState<Menu | null>(null);
  // shown is the file the editor state was created for; the same file
  // with new content is replaced in place instead.
  const shown = useRef<{ path: string; editable: boolean } | null>(null);

  useEffect(() => {
    view.current = new EditorView({ parent: host.current! });
    // A new view has no state: the file is set up anew, not replaced in
    // place. React mounts twice in development (StrictMode), and replacing
    // the text of the second, bare view left it without highlighting, line
    // numbers and key bindings.
    shown.current = null;
    if (props.text) props.text.current = () => view.current!.state.doc.toString();
    return () => view.current?.destroy();
  }, []);

  useEffect(() => {
    const v = view.current!;
    if (shown.current?.path === props.path && shown.current.editable === props.editable) {
      // The file changed on disk, or was saved: the text is replaced in
      // place, keeping the cursor and the scroll position as far as the
      // new text allows (DESIGN.md, Editing).
      replaceText(v, props.content);
      return;
    }
    shown.current = { path: props.path, editable: props.editable };
    const keymaps = keys.current.of(commandKeymap(props.shortcuts ?? {}, latest));
    v.setState(EditorState.create({ doc: props.content, extensions: extensions(props.path, props.editable, latest, setMenu, keymaps) }));
    reveal(v, props.line, props.column ?? 0);
    reportCursor(v, props);
    if (props.editable) v.focus();
  }, [props.path, props.content, props.original, props.editable]);

  useEffect(() => {
    view.current?.dispatch({ effects: keys.current.reconfigure(commandKeymap(props.shortcuts ?? {}, latest)) });
  }, [props.shortcuts]);

  useEffect(() => {
    reveal(view.current!, props.line, props.column ?? 0);
    reportCursor(view.current!, props);
  }, [props.line, props.column, props.reveal]);

  useEffect(() => {
    if (props.focus) view.current?.focus();
  }, [props.focus]);

  // The context menu closes on any click or key elsewhere.
  useEffect(() => {
    if (!menu) return;
    const close = () => setMenu(null);
    window.addEventListener("mousedown", close);
    window.addEventListener("keydown", close, true);
    window.addEventListener("blur", close);
    return () => (window.removeEventListener("mousedown", close), window.removeEventListener("keydown", close, true), window.removeEventListener("blur", close));
  }, [menu]);

  const choose = (f: () => void) => (e: React.MouseEvent) => (e.stopPropagation(), setMenu(null), f());

  return (
    <>
      <div className="code" ref={host} />
      {menu && (
        <div className="context-menu" style={{ left: menu.x, top: menu.y }} onMouseDown={(e) => e.stopPropagation()}>
          <button onClick={choose(() => latest.current.onDefinition?.(menu.word, menu.at))}>
            Go to definition<span>{label(props.shortcuts?.definition)}</span>
          </button>
          <button onClick={choose(() => latest.current.onUsages?.(menu.word))}>
            Find usages<span>{label(props.shortcuts?.usages)}</span>
          </button>
        </div>
      )}
    </>
  );
}

// wordAt returns the word at pos and where it starts.
function wordAt(v: EditorView, pos = v.state.selection.main.head): { word: string; at: At } | null {
  const range = v.state.wordAt(pos);
  if (!range) return null;
  const line = v.state.doc.lineAt(range.from);
  return { word: v.state.sliceDoc(range.from, range.to), at: { line: line.number, column: range.from - line.from } };
}

// commandKeymap binds the editor commands to the configured shortcuts.
function commandKeymap(shortcuts: Shortcuts, latest: { current: CodeProps }): Extension {
  const definition = (v: EditorView) => {
    const w = wordAt(v);
    if (w) latest.current.onDefinition?.(w.word, w.at);
    return true;
  };
  const usages = (v: EditorView) => {
    const w = wordAt(v);
    if (w) latest.current.onUsages?.(w.word);
    return true;
  };
  const bindings = [
    { key: shortcuts.save, run: (v: EditorView) => (latest.current.onSave?.(v.state.doc.toString()), true) },
    { key: shortcuts.definition, run: definition },
    { key: shortcuts.usages, run: usages },
    { key: shortcuts.line, run: gotoLine },
    { key: shortcuts.find, run: openSearchPanel },
  ]
    .filter((b) => b.key)
    .map((b) => ({ key: cmKey(b.key!), run: b.run }));
  // basicSetup binds Mod-f to find as well; it is swallowed when find is
  // bound elsewhere, so that a binding changed in settings stops working.
  if (!bindings.some((b) => b.key === "Mod-f")) bindings.push({ key: "Mod-f", run: () => true });
  return Prec.high(keymap.of(bindings));
}

function extensions(path: string, editable: boolean, latest: { current: CodeProps }, setMenu: (m: Menu | null) => void, keymaps: Extension): Extension[] {
  return [
    keymaps,
    basicSetup,
    search({ top: true, createPanel: findPanel }),
    keymap.of([{ key: "Escape", run: () => (latest.current.onEscape?.(), true) }]),
    oneDark,
    // Match the window background instead of oneDark's.
    EditorView.theme({ "&, .cm-gutters": { backgroundColor: "#18181b" } }, { dark: true }),
    serverHighlighted(path) ? serverHighlight(path) : languageFor(path),
    EditorState.readOnly.of(!editable),
    EditorView.domEventHandlers({
      mousedown: (e, v) => {
        if (!cmd(e) || e.button !== 0) return false;
        const pos = v.posAtCoords({ x: e.clientX, y: e.clientY });
        const w = pos == null ? null : wordAt(v, pos);
        if (w) latest.current.onDefinition?.(w.word, w.at);
        return !!w;
      },
      // Right-click on a symbol offers go to definition and find usages.
      contextmenu: (e, v) => {
        if (!latest.current.onDefinition) return false;
        const pos = v.posAtCoords({ x: e.clientX, y: e.clientY });
        const w = pos == null ? null : wordAt(v, pos);
        if (!w) return false;
        e.preventDefault();
        v.dispatch({ selection: { anchor: pos! } });
        setMenu({ x: e.clientX, y: e.clientY, ...w });
        return true;
      },
    }),
    EditorView.updateListener.of((u) => {
      if (u.docChanged) latest.current.onDirty?.(u.state.doc.toString() !== (latest.current.original ?? latest.current.content));
      if (u.selectionSet || u.docChanged) reportCursor(u.view, latest.current);
    }),
  ];
}

function reportCursor(v: EditorView, props: CodeProps) {
  if (!props.onCursor) return;
  const head = v.state.selection.main.head;
  const l = v.state.doc.lineAt(head);
  props.onCursor(l.number, head - l.from + 1);
}

// replaceText puts text in place of the editor's, keeping the cursor at its
// line and column and the view where it was scrolled to.
function replaceText(v: EditorView, text: string) {
  const doc = v.state.doc;
  if (doc.toString() === text) return;
  const head = v.state.selection.main.head;
  const l = doc.lineAt(head);
  const line = l.number;
  const column = head - l.from;
  const top = v.scrollDOM.scrollTop;
  v.dispatch({ changes: { from: 0, to: doc.length, insert: text } });
  const next = v.state.doc;
  const nl = next.line(Math.min(line, next.lines));
  v.dispatch({ selection: { anchor: nl.from + Math.min(column, nl.length) } });
  requestAnimationFrame(() => (v.scrollDOM.scrollTop = top));
}

function reveal(v: EditorView, line: number, column: number) {
  const doc = v.state.doc;
  const l = doc.line(Math.min(Math.max(line, 1), doc.lines));
  const pos = l.from + Math.min(column, l.length);
  v.dispatch({ selection: { anchor: pos }, effects: EditorView.scrollIntoView(pos, { y: "center" }) });
}
