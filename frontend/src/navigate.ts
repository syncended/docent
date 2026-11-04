// Go to definition and find usages, shared by the viewer and the overview
// window (DESIGN.md, Viewer).

import * as api from "./api";
import type { Hit } from "./api";
import type { At } from "./Code";

// Where is what a navigation found: one place to open, a list to choose
// from, or a note that nothing was found.
export type Where = { open: Hit } | { list: { title: string; hits: Hit[] } } | { note: string };

// definitionOrUsages goes to the definition of word, clicked at a place of
// a file; on the definition itself it finds usages instead, as IntelliJ
// IDEA does.
export async function definitionOrUsages(word: string, file: { root: string; path: string }, at: At): Promise<Where> {
  const hits = await api.definitions(word);
  if (hits.some((h) => h.root === file.root && h.path === file.path && h.line === at.line)) return usages(word);
  if (hits.length === 0) return { note: `No definition found for ${word}` };
  if (hits.length === 1) return { open: hits[0] };
  return { list: { title: `Definitions of ${word}`, hits } };
}

export async function usages(word: string): Promise<Where> {
  const hits = await api.usages(word);
  if (hits.length === 0) return { note: `No usages found for ${word}` };
  return { list: { title: `Usages of ${word}`, hits } };
}
