import { useEffect, useRef } from "react";
import { EditorState, Prec, type Extension } from "@codemirror/state";
import { EditorView, keymap } from "@codemirror/view";
import { basicSetup } from "codemirror";
import { gotoLine } from "@codemirror/search";
import { oneDark } from "@codemirror/theme-one-dark";
import { languageFor } from "./languages";
import { serverHighlight, serverHighlighted } from "./highlight";
import { cmd } from "./keys";

export interface CodeProps {
  path: string;
  content: string;
  // line is 1-based; column is a 0-based offset in the line.
  line: number;
  column?: number;
  editable: boolean;
  onDirty?: (dirty: boolean) => void;
  onSave?: (text: string) => void;
  onDefinition?: (word: string) => void;
  onUsages?: (word: string) => void;
  onEscape?: () => void;
}

// Code shows a file with CodeMirror: read-only for previews, editable in the viewer.
export function Code(props: CodeProps) {
  const host = useRef<HTMLDivElement>(null);
  const view = useRef<EditorView>();
  // Key bindings are created once; they read the latest props from here.
  const latest = useRef(props);
  latest.current = props;

  useEffect(() => {
    view.current = new EditorView({ parent: host.current! });
    return () => view.current?.destroy();
  }, []);

  useEffect(() => {
    const v = view.current!;
    v.setState(EditorState.create({ doc: props.content, extensions: extensions(props.path, props.editable, props.content, latest) }));
    reveal(v, props.line, props.column ?? 0);
    if (props.editable) v.focus();
  }, [props.path, props.content, props.editable]);

  useEffect(() => {
    reveal(view.current!, props.line, props.column ?? 0);
  }, [props.line, props.column]);

  return <div className="code" ref={host} />;
}

function extensions(path: string, editable: boolean, original: string, latest: { current: CodeProps }): Extension[] {
  const word = (v: EditorView, pos = v.state.selection.main.head) => {
    const range = v.state.wordAt(pos);
    return range ? v.state.sliceDoc(range.from, range.to) : "";
  };
  const withWord = (f?: (w: string) => void) => (v: EditorView) => {
    const w = word(v);
    if (w && f) f(w);
    return true;
  };
  return [
    Prec.high(
      keymap.of([
        { key: "Mod-s", run: (v) => (latest.current.onSave?.(v.state.doc.toString()), true) },
        { key: "Mod-b", run: (v) => withWord(latest.current.onDefinition)(v) },
        { key: "Alt-F7", run: (v) => withWord(latest.current.onUsages)(v) },
        { key: "Mod-l", run: gotoLine },
      ]),
    ),
    basicSetup,
    keymap.of([{ key: "Escape", run: () => (latest.current.onEscape?.(), true) }]),
    oneDark,
    // Match the window background instead of oneDark's.
    EditorView.theme({ "&, .cm-gutters": { backgroundColor: "#18181b" } }, { dark: true }),
    serverHighlighted(path) ? serverHighlight(path) : languageFor(path),
    EditorState.readOnly.of(!editable),
    EditorView.domEventHandlers({
      mousedown: (e, v) => {
        if (!cmd(e)) return false;
        const pos = v.posAtCoords({ x: e.clientX, y: e.clientY });
        const w = pos == null ? "" : word(v, pos);
        if (w) latest.current.onDefinition?.(w);
        return !!w;
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
