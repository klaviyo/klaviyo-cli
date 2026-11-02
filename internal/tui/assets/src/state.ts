import { chmodSync, closeSync, constants, fchmodSync, fstatSync, ftruncateSync, lstatSync, mkdirSync, openSync, readFileSync, writeFileSync } from "node:fs"
import { homedir } from "node:os"
import { dirname, join } from "node:path"
import type { FieldValues } from "./form.ts"

const STATE_DIR = join(process.env.XDG_CONFIG_HOME ?? join(homedir(), ".config"), "klaviyo-tui")
const STATE_FILE = process.env.KLAVIYO_TUI_STATE ?? join(STATE_DIR, "state.json")

export interface HistoryItem {
  id: string
  cmd: string
  at: number
  exit: number | null
}

export interface PersistedState {
  favorites: string[]
  history: HistoryItem[]
  lastValues: Record<string, FieldValues>
  theme: string
}

const empty = (): PersistedState => ({ favorites: [], history: [], lastValues: {}, theme: "klaviyo" })

export function loadState(): PersistedState {
  try {
    const state = JSON.parse(readFileSync(STATE_FILE, "utf8"))
    if (!state || typeof state !== "object") return empty()
    return {
      favorites: Array.isArray(state.favorites) ? state.favorites.filter((v: unknown) => typeof v === "string") : [],
      history: Array.isArray(state.history) ? state.history.filter((v: HistoryItem) => v && typeof v.id === "string" && typeof v.cmd === "string").slice(0, 100) : [],
      lastValues: state.lastValues && typeof state.lastValues === "object"
        ? Object.fromEntries(Object.entries(state.lastValues).filter((pair): pair is [string, FieldValues] => {
            const values = pair[1]
            return !!values && typeof values === "object" && Object.values(values).every((v) => typeof v === "string")
          }))
        : {},
      theme: typeof state.theme === "string" ? state.theme : "klaviyo",
    }
  } catch {
    return empty()
  }
}

export function saveState(state: PersistedState) {
  let fd: number | undefined
  try {
    const directory = dirname(STATE_FILE)
    mkdirSync(directory, { recursive: true, mode: 0o700 })
    const parent = lstatSync(directory)
    if (!parent.isDirectory()) return
    // Only the dedicated app directory is ours to restrict, not an override's shared parent.
    if (directory === STATE_DIR && parent.uid === process.getuid?.()) chmodSync(directory, 0o700)
    fd = openSync(STATE_FILE, constants.O_WRONLY | constants.O_CREAT | constants.O_NOFOLLOW, 0o600)
    const file = fstatSync(fd)
    if (!file.isFile() || file.nlink !== 1 || file.uid !== process.getuid?.()) return
    // Restrict an existing file before truncating or writing command arguments.
    fchmodSync(fd, 0o600)
    ftruncateSync(fd, 0)
    writeFileSync(fd, JSON.stringify(state, null, 2))
  } catch {
    // persistence is best-effort; never crash the UI over it
  } finally {
    if (fd !== undefined) closeSync(fd)
  }
}

export function toggleFavorite(state: PersistedState, id: string): PersistedState {
  const favorites = state.favorites.includes(id) ? state.favorites.filter((f) => f !== id) : [...state.favorites, id]
  return { ...state, favorites }
}

export function recordRun(
  state: PersistedState,
  item: { id: string; cmd: string; values: FieldValues },
): PersistedState {
  const history = [{ id: item.id, cmd: item.cmd, at: Date.now(), exit: null }, ...state.history].slice(0, 100)
  return { ...state, history, lastValues: { ...state.lastValues, [item.id]: item.values } }
}

export function recordExit(state: PersistedState, exit: number): PersistedState {
  const [first, ...rest] = state.history
  if (!first) return state
  return { ...state, history: [{ ...first, exit }, ...rest] }
}
