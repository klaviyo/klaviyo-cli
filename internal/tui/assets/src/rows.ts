import { search } from "./fuzzy.ts"
import type { HistoryItem } from "./state.ts"
import type { CommandIndex, Entry } from "./types.ts"

export type Row =
  | { kind: "header"; label: string }
  | { kind: "item"; entry: Entry; positions: number[]; fav: boolean; note?: string }

export interface RowInput {
  index: CommandIndex
  query: string
  scope: string[]
  favorites: string[]
  history: HistoryItem[]
}

const MAX_RECENT = 6

export function buildRows({ index, query, scope, favorites, history }: RowInput): Row[] {
  const favSet = new Set(favorites)
  const scopeId = scope.join(" ")
  const scopeEntry = index.byId.get(scopeId) ?? index.root
  const item = (entry: Entry, extra: Partial<Extract<Row, { kind: "item" }>> = {}): Row => ({
    kind: "item",
    entry,
    positions: [],
    fav: favSet.has(entry.id),
    ...extra,
  })

  if (query.trim()) {
    const pool = scopeId ? index.entries.filter((e) => e.id.startsWith(`${scopeId} `)) : index.entries
    return search(query, pool).map((m) => item(m.entry, { positions: m.positions }))
  }

  const children = scopeEntry.childIds.map((id) => index.byId.get(id)!).filter(Boolean)
  if (scopeId) return children.map((e) => item(e))

  const rows: Row[] = []
  const favs = favorites.map((id) => index.byId.get(id)).filter((e): e is Entry => !!e)
  if (favs.length) {
    rows.push({ kind: "header", label: "★ Favorites" })
    for (const e of favs) rows.push(item(e))
  }
  const seen = new Set<string>()
  const recents: Row[] = []
  for (const h of history) {
    const entry = index.byId.get(h.id)
    if (!entry || seen.has(h.id)) continue
    seen.add(h.id)
    const args = h.cmd.replace(/^klaviyo /, "").slice(entry.id.length).trim()
    recents.push(item(entry, args ? { note: args } : {}))
    if (recents.length >= MAX_RECENT) break
  }
  if (recents.length) {
    rows.push({ kind: "header", label: "↺ Recent" })
    rows.push(...recents)
  }
  rows.push({ kind: "header", label: `Commands · ${children.length}` })
  rows.push(...children.map((e) => item(e)))
  return rows
}

export const firstItem = (rows: Row[]) => Math.max(0, rows.findIndex((r) => r.kind === "item"))

export function stepSelection(rows: Row[], from: number, delta: number): number {
  const items = rows.map((r, i) => (r.kind === "item" ? i : -1)).filter((i) => i >= 0)
  if (items.length === 0) return 0
  const pos = Math.max(0, items.indexOf(from))
  const raw = pos + delta
  const next = Math.abs(delta) === 1 ? (raw + items.length) % items.length : Math.min(items.length - 1, Math.max(0, raw))
  return items[next]!
}
