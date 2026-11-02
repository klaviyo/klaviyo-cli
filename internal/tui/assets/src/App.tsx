import type { ScrollBoxRenderable } from "@opentui/core"
import { useKeyboard, useRenderer, useTerminalDimensions } from "@opentui/react"
import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { Header, HelpOverlay, KeyBar, Splash } from "./components/Chrome.tsx"
import { Details } from "./components/Details.tsx"
import { ResultList } from "./components/ResultList.tsx"
import { RunForm } from "./components/RunForm.tsx"
import { formatCommand, type FieldValues } from "./form.ts"
import { parseArgs, resolveLaunch } from "./launch.ts"
import { buildRows, firstItem, stepSelection, type Row } from "./rows.ts"
import { recordRun, saveState, toggleFavorite, type PersistedState } from "./state.ts"
import { ThemeContext, themes } from "./theme.ts"
import { buildIndex, diffIndex, dumpTree, type CliInstall } from "./tree.ts"
import type { CommandIndex, Entry } from "./types.ts"

export interface Session {
  install: CliInstall | null
  index: CommandIndex | null
  rev: string | null
  state: PersistedState
  query: string
  scope: string[]
  selId: string | null
  toast: string | null
  /** CLI args to interpret once the index is ready. */
  launchArgs: string[] | null
  commonArgs?: string[]
}

export type Result =
  | { type: "run"; argv: string[] }
  | { type: "quit" }

type Modal = { kind: "run"; entry: Entry; initial?: FieldValues; initialArgv?: string[] } | { kind: "help" } | null

async function copyText(text: string, osc52: () => boolean): Promise<boolean> {
  if (Bun.which("pbcopy")) {
    try {
      const proc = Bun.spawn(["pbcopy"], { stdin: "pipe" })
      proc.stdin.write(text)
      await proc.stdin.end()
      if (await proc.exited === 0) return true
    } catch {
      /* fall through to OSC 52 */
    }
  }
  return osc52()
}

export function App({ session, onResult }: { session: Session; onResult: (r: Result) => void }) {
  const renderer = useRenderer()
  const { width, height } = useTerminalDimensions()

  const [index, setIndex] = useState<CommandIndex | null>(session.index)
  const [rev, setRev] = useState<string | null>(session.rev)
  const [refreshing, setRefreshing] = useState(!session.index)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [persisted, setPersisted] = useState(session.state)
  const [query, setQuery] = useState(session.query)
  const [scope, setScope] = useState(session.scope)
  const [modal, setModal] = useState<Modal>(null)
  const [toast, setToast] = useState<string | null>(session.toast)
  const [sel, setSel] = useState(-1)
  const [inputNonce, setInputNonce] = useState(0)
  const scrollRef = useRef<ScrollBoxRenderable | null>(null)
  const topRef = useRef(0)
  const indexRef = useRef(index)
  indexRef.current = index
  const toastTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  const theme = themes.find((th) => th.name === persisted.theme) ?? themes[0]!

  const flash = useCallback((msg: string, ms = 2200) => {
    setToast(msg)
    if (toastTimer.current) clearTimeout(toastTimer.current)
    toastTimer.current = setTimeout(() => setToast(null), ms)
  }, [])

  // keep the cross-restart session in sync
  useEffect(() => {
    Object.assign(session, { index, rev, state: persisted, query, scope, toast: null })
  }, [session, index, rev, persisted, query, scope])

  useEffect(() => {
    saveState(persisted)
  }, [persisted])

  useEffect(() => {
    if (session.toast) flash(session.toast, 4000)
  }, [])

  const refresh = useCallback(
    async (install: CliInstall) => {
      setRefreshing(true)
      try {
        const cache = await dumpTree(install)
        const next = buildIndex(cache.tree)
        const prev = indexRef.current
        setIndex(next)
        setRev(cache.rev)
        setLoadError(null)
        const d = prev ? diffIndex(prev, next) : { added: [], removed: [] }
        const parts = [d.added.length ? `+${d.added.length} new` : "", d.removed.length ? `−${d.removed.length} removed` : ""]
        flash(parts.some(Boolean) ? `index refreshed · ${parts.filter(Boolean).join(" ")}` : "index up to date", 4000)
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err)
        setLoadError(msg)
        flash("re-index failed")
      } finally {
        setRefreshing(false)
      }
    },
    [flash],
  )

  useEffect(() => {
    if (session.index) return
    const install = session.install
    if (!install) {
      setLoadError("Launch the browser with `klaviyo --tui`.")
      setRefreshing(false)
      return
    }
    void refresh(install)
  }, [])

  const rows = useMemo<Row[]>(
    () => (index ? buildRows({ index, query, scope, favorites: persisted.favorites, history: persisted.history }) : []),
    [index, query, scope, persisted.favorites, persisted.history],
  )

  // Restore the previously selected entry after returning from a run.
  const restored = useRef(false)
  useEffect(() => {
    if (restored.current || !index || !session.selId) return
    restored.current = true
    const i = rows.findIndex((r) => r.kind === "item" && r.entry.id === session.selId)
    if (i >= 0) setSel(i)
  }, [index])

  // Apply launch arguments once, opening a group or a prefilled form.
  const launched = useRef(false)
  useEffect(() => {
    if (!index || !session.launchArgs || launched.current) return
    launched.current = true
    const launch = resolveLaunch(index, session.launchArgs)
    session.commonArgs = launch.commonArgs
    session.launchArgs = null
    setScope(launch.scope)
    setQuery(launch.query)
    const nextRows = buildRows({ index, query: launch.query, scope: launch.scope, favorites: persisted.favorites, history: persisted.history })
    const at = launch.select ? nextRows.findIndex((r) => r.kind === "item" && r.entry.id === launch.select) : -1
    setSel(at)
    if (launch.run) setModal({ kind: "run", entry: launch.run.entry, initial: launch.run.values, initialArgv: launch.run.argv })
    if (launch.note) flash(launch.note, 5000)
  }, [index])

  const selected = rows[sel]?.kind === "item" ? sel : firstItem(rows)
  const selectedRow = rows[selected]
  const selectedEntry = selectedRow?.kind === "item" ? selectedRow.entry : undefined
  useEffect(() => {
    session.selId = selectedEntry?.id ?? null
  }, [session, selectedEntry?.id])

  const showDetails = width >= 100
  const listWidth = showDetails ? Math.floor(width * 0.48) : width
  const listHeight = Math.max(3, height - 3 - 3 - 1 - 2)

  if (selected < topRef.current) topRef.current = selected
  if (selected >= topRef.current + listHeight) topRef.current = selected - listHeight + 1
  // Pull a leading section header into view when the first item is selected.
  if (selected <= firstItem(rows)) topRef.current = 0
  topRef.current = Math.max(0, Math.min(topRef.current, Math.max(0, rows.length - listHeight)))

  const changeQuery = (q: string) => {
    // "?" on an empty query opens help instead of being typed; remount the input to discard the character.
    if (q === "?" && query === "") {
      setModal({ kind: "help" })
      setInputNonce((n) => n + 1)
      return
    }
    setQuery(q)
    setSel(-1)
    topRef.current = 0
  }

  const goScope = (path: string[]) => {
    setScope(path)
    setQuery("")
    setSel(-1)
    topRef.current = 0
  }

  const move = (delta: number) => setSel(stepSelection(rows, selected, delta))

  const openRun = (entry: Entry) => setModal({ kind: "run", entry,
    ...(session.commonArgs?.length ? { initial: parseArgs(entry, session.commonArgs), initialArgv: [...entry.path, ...session.commonArgs] } : {}),
  })

  const activate = (i: number) => {
    const row = rows[i]
    if (!row || row.kind !== "item") return
    if (row.entry.isGroup) goScope(row.entry.path)
    else openRun(row.entry)
  }

  const copy = async (text: string) => {
    const ok = await copyText(text, () => renderer.copyToClipboardOSC52(text))
    flash(ok ? "copied to clipboard" : "clipboard unavailable")
  }

  const toggleFav = () => {
    if (!selectedEntry || !index) return
    const next = toggleFavorite(persisted, selectedEntry.id)
    setPersisted(next)
    const nextRows = buildRows({ index, query, scope, favorites: next.favorites, history: next.history })
    const i = nextRows.findIndex((r) => r.kind === "item" && r.entry.id === selectedEntry.id)
    setSel(i >= 0 ? i : -1)
    flash(next.favorites.includes(selectedEntry.id) ? "★ favorited" : "unfavorited")
  }

  const runCommand = (entry: Entry, argv: string[], values: FieldValues) => {
    const next = recordRun(persisted, { id: entry.id, cmd: formatCommand(argv), values })
    saveState(next)
    session.state = next
    onResult({ type: "run", argv })
  }

  useKeyboard((key) => {
    if (modal?.kind === "help") {
      setModal(null)
      return
    }
    if (modal || !index) return

    if (key.ctrl && key.name === "c") return
    if (key.name === "escape") {
      if (query) changeQuery("")
      else if (scope.length) goScope(scope.slice(0, -1))
      else onResult({ type: "quit" })
      return
    }
    if (key.name === "backspace" && query === "" && scope.length) return goScope(scope.slice(0, -1))
    if (key.name === "down" && key.shift) return void scrollRef.current?.scrollBy(3)
    if (key.name === "up" && key.shift) return void scrollRef.current?.scrollBy(-3)
    if (key.name === "down" || (key.ctrl && (key.name === "n" || key.name === "j"))) return move(1)
    if (key.name === "up" || (key.ctrl && (key.name === "p" || key.name === "k"))) return move(-1)
    if (key.name === "pagedown") return move(listHeight - 1)
    if (key.name === "pageup") return move(-(listHeight - 1))
    if (key.name === "return" || key.name === "enter") return activate(selected)
    if (key.name === "tab" && selectedEntry?.isGroup) return goScope(selectedEntry.path)
    if (key.name === "f1") return setModal({ kind: "help" })
    if (key.ctrl && key.name === "f") return toggleFav()
    if (key.ctrl && key.name === "y" && selectedEntry) return void copy(`klaviyo ${selectedEntry.id}`)
    if (key.ctrl && key.name === "t") {
      const i = themes.findIndex((th) => th.name === theme.name)
      const nextTheme = themes[(i + 1) % themes.length]!
      setPersisted({ ...persisted, theme: nextTheme.name })
      return flash(`theme: ${nextTheme.name}`)
    }
    if (key.ctrl && key.name === "r" && session.install && !refreshing) {
      flash("refreshing command catalog…", 4000)
      return void refresh(session.install)
    }
  })

  if (!index) {
    return (
      <ThemeContext.Provider value={theme}>
        <box width={width} height={height} backgroundColor={theme.bg}>
          <Splash message="Reading Klaviyo commands…" error={loadError} />
        </box>
      </ThemeContext.Provider>
    )
  }

  const lastCmd = selectedEntry
    ? persisted.history.find((h) => h.id === selectedEntry.id)?.cmd
    : undefined
  const count = rows.filter((r) => r.kind === "item").length
  const scopeLabel = scope.length ? scope.join(" › ") : ""
  const title = query
    ? ` ${count} match${count === 1 ? "" : "es"}${scope.length ? ` in ${scope.join(" ")}` : ""} `
    : scope.length
      ? ` ${scope.join(" › ")} · ${count} `
      : " Browse "

  const hints: [string, string][] =
    modal?.kind === "run"
      ? []
      : [
          ["↑↓", "move"],
          ["⏎", selectedEntry?.isGroup ? "open group" : "run…"],
          ...(selectedEntry?.isGroup ? ([["⇥", "enter"]] as [string, string][]) : []),
          ["esc", query ? "clear" : scope.length ? "up" : "quit"],
          ["^F", "★"],
          ["^Y", "copy"],
          ["^T", "theme"],
          ["?", "help"],
        ]

  return (
    <ThemeContext.Provider value={theme}>
      <box width={width} height={height} flexDirection="column" backgroundColor={theme.bg}>
        <Header commands={index.commands} groups={index.groups} refreshing={refreshing} rev={rev} />

        <box
          height={3}
          border
          borderStyle="rounded"
          borderColor={modal ? theme.border : theme.borderActive}
          flexDirection="row"
          paddingLeft={1}
          paddingRight={1}
        >
          <text wrapMode="none">
            <span fg={theme.accent}>❯ </span>
            {scopeLabel ? <span fg={theme.group}>{`${scopeLabel} › `}</span> : null}
          </text>
          <input
            key={inputNonce}
            flexGrow={1}
            focused={!modal}
            value={query}
            placeholder={scope.length ? "filter within this group…" : `search ${index.commands} commands — try “profiles”, “lists”, “campaigns”…`}
            textColor={theme.text}
            placeholderColor={theme.faint}
            cursorColor={theme.accent}
            backgroundColor={theme.bg}
            focusedBackgroundColor={theme.bg}
            onInput={changeQuery}
          />
        </box>

        <box flexGrow={1} flexDirection="row">
          <box width={listWidth} flexDirection="column">
            <ResultList
              rows={rows}
              selected={selected}
              top={topRef.current}
              height={listHeight}
              width={listWidth}
              title={title}
              onSelect={setSel}
              onActivate={activate}
              onScroll={move}
            />
          </box>
          {showDetails ? (
            <Details
              entry={selectedEntry}
              index={index}
              scrollRef={scrollRef}
              favorite={!!selectedEntry && persisted.favorites.includes(selectedEntry.id)}
              lastCmd={lastCmd}
            />
          ) : null}
        </box>

        <KeyBar hints={hints} toast={toast} />

        {modal?.kind === "run" ? (
          <RunForm
            key={modal.entry.id}
            entry={modal.entry}
            initial={modal.initial ?? persisted.lastValues[modal.entry.id]}
            initialArgv={modal.initialArgv}
            onCancel={() => setModal(null)}
            onCopy={(cmd) => void copy(cmd)}
            onRun={(argv, values) => runCommand(modal.entry, argv, values)}
          />
        ) : null}
        {modal?.kind === "help" ? <HelpOverlay /> : null}
      </box>
    </ThemeContext.Provider>
  )
}
