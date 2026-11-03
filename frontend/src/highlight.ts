import { StateEffect, StateField, type Extension } from "@codemirror/state";
import { Decoration, EditorView, ViewPlugin, type DecorationSet, type ViewUpdate } from "@codemirror/view";
import * as api from "./api";

// Highlighting computed by the backend (tree-sitter), for languages without a
// good CodeMirror grammar. Spans move with edits and are refreshed shortly
// after typing stops.

const extensions = new Set(["kt", "kts", "proto"]);

export function serverHighlighted(path: string): boolean {
  return extensions.has(path.slice(path.lastIndexOf(".") + 1).toLowerCase());
}

const setSpans = StateEffect.define<DecorationSet>();

const spans = StateField.define<DecorationSet>({
  create: () => Decoration.none,
  update(value, tr) {
    for (const e of tr.effects) if (e.is(setSpans)) return e.value;
    return value.map(tr.changes);
  },
  provide: (f) => EditorView.decorations.from(f),
});

const marks = new Map<string, Decoration>();
const mark = (cls: string) => {
  let m = marks.get(cls);
  if (!m) marks.set(cls, (m = Decoration.mark({ class: `tok-${cls}` })));
  return m;
};

export function serverHighlight(path: string): Extension {
  const plugin = ViewPlugin.fromClass(
    class {
      timer = 0;
      version = 0;
      constructor(readonly view: EditorView) {
        this.request(0);
      }
      update(u: ViewUpdate) {
        if (u.docChanged) this.request(300);
      }
      request(delay: number) {
        clearTimeout(this.timer);
        const version = ++this.version;
        this.timer = window.setTimeout(async () => {
          const doc = this.view.state.doc;
          try {
            const result = await api.highlight(path, doc.toString());
            // Drop results for text that has changed since.
            if (version !== this.version || this.view.state.doc !== doc) return;
            const len = doc.length;
            const ranges = result.filter((s) => s.to <= len && s.from < s.to).map((s) => mark(s.class).range(s.from, s.to));
            this.view.dispatch({ effects: setSpans.of(Decoration.set(ranges, true)) });
          } catch {
            // Keep the previous highlighting.
          }
        }, delay);
      }
      destroy() {
        clearTimeout(this.timer);
      }
    },
  );
  return [spans, plugin];
}
