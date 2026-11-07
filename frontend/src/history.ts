// Navigation history: the places a window has visited, for back and
// forward (DESIGN.md, Viewer). A place is a hit: a file and a position.

import type { Hit } from "./api";

export type History = { places: Hit[]; at: number };

export const empty: History = { places: [], at: -1 };

const sameFile = (a: Hit, b: Hit) => a.root === b.root && a.path === b.path;
const samePlace = (a: Hit, b: Hit) => sameFile(a, b) && a.line === b.line;

// jump records a jump from where the window is to a place. The origin
// replaces the last place when that is the same file (the cursor moved on
// since), and is added otherwise; then the destination. Places ahead of the
// current one are dropped, as in a browser.
export function jump(h: History, from: Hit | null, to: Hit): History {
  const places = h.places.slice(0, h.at + 1);
  if (from) {
    if (places.length > 0 && sameFile(places[places.length - 1], from)) places[places.length - 1] = from;
    else places.push(from);
  }
  if (places.length === 0 || !samePlace(places[places.length - 1], to)) places.push(to);
  return { places, at: places.length - 1 };
}

// step moves back (-1) or forward (1) and returns the place to open, or
// null at either end. The place left behind is updated to where the cursor
// is now, so that coming back finds it.
export function step(h: History, dir: -1 | 1, current: Hit | null): { history: History; place: Hit } | null {
  const at = h.at + dir;
  if (at < 0 || at >= h.places.length) return null;
  const places = h.places.slice();
  if (current && h.at >= 0 && sameFile(places[h.at], current)) places[h.at] = current;
  return { history: { places, at }, place: places[at] };
}
