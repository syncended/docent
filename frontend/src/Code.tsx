import { useEffect, useRef, useState, type MutableRefObject } from "react";
import { EditorState, Prec, type Extension } from "@codemirror/state";
import { EditorView, keymap } from "@codemirror/view";
import { basicSetup } from "codemirror";
import { gotoLine, search } from "@codemirror/search";
import { oneDark } from "@codemirror/theme-one-dark";
import { languageFor } from "./languages";
import { serverHighlight, serverHighlighted } from "./highlight";
import { altLabel, cmd, modLabel } from "./keys";
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
  // focus moves keyboard focus to the editor; it changes on every request.
  focus?: number;
  // reveal scrolls to line again; it changes on every request.
  reveal?: number;
  // text is set to a function returning the current text.
  text?: MutableRefObject<(() => string) | null>;
}

type Menu = { x: number; y: number; word: string; at: At };

// Code shows a file with CodeMirror: read-only for previews, editable in the viewer.
export function Code(props: CodeProps) {
  const host = useRef<HTMLDivElement>(null);
  const view = useRef<EditorView>();
  // Key bindings are created once; they read the latest props from here.
  const latest = useRef(props);
  latest.current = props;
  const [menu, setMenu] = useState<Menu | null>(null);

  useEffect(() => {
    view.current = new EditorView({ parent: host.current! });
    if (props.text) props.text.current = () => view.current!.state.doc.toString();
    return () => view.current?.destroy();
  }, []);

  useEffect(() => {
    const v = view.current!;
    const original = props.original ?? props.content;
    v.setState(EditorState.create({ doc: props.content, extensions: extensions(props.path, props.editable, original, latest, setMenu) }));
    reveal(v, props.line, props.column ?? 0);
    if (props.editable) v.focus();
  }, [props.path, props.content, props.original, props.editable]);

  useEffect(() => {
    reveal(view.current!, props.line, props.column ?? 0);
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
            Go to definition<span>{modLabel}B</span>
          </button>
          <button onClick={choose(() => latest.current.onUsages?.(menu.word))}>
            Find usages<span>{altLabel}F7</span>
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

function extensions(path: string, editable: boolean, original: string, latest: { current: CodeProps }, setMenu: (m: Menu | null) => void): Extension[] {
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
  return [
    Prec.high(
      keymap.of([
        { key: "Mod-s", run: (v) => (latest.current.onSave?.(v.state.doc.toString()), true) },
        { key: "Mod-b", run: definition },
        { key: "Alt-F7", run: usages },
        { key: "Mod-l", run: gotoLine },
      ]),
    ),
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
      if (u.docChanged) latest.current.onDirty?.(u.state.doc.toString() !== original);
    }),
  ];
}

function reveal(v: EditorView, line: number, column: number) {
  const doc = v.state.doc;
  const l = doc.line(Math.min(Math.max(line, 1), doc.lines));
  const pos = l.from + Math.min(column, l.length);
  v.dispatch({ selection: { anchor: pos }, effects: EditorView.scrollIntoView(pos, { y: "center" }) });
}
