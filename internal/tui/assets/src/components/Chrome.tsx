import { useTerminalDimensions } from "@opentui/react"
import { useEffect, useState } from "react"
import { useTheme } from "../theme.ts"

const SPINNER = ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"]

export function useSpinner(active: boolean) {
  const [frame, setFrame] = useState(0)
  useEffect(() => {
    if (!active) return
    const id = setInterval(() => setFrame((f) => (f + 1) % SPINNER.length), 80)
    return () => clearInterval(id)
  }, [active])
  return SPINNER[frame]!
}

export function Header(props: { commands: number; groups: number; refreshing: boolean; rev: string | null }) {
  const t = useTheme()
  const spin = useSpinner(props.refreshing)
  return (
    <box height={3} flexDirection="row" paddingLeft={1} paddingRight={1} justifyContent="space-between" alignItems="flex-end">
      <ascii-font text="klaviyo" font="tiny" color={t.gradient} />
      <box flexDirection="column" alignItems="flex-end">
        <text fg={t.dim}>
          <span fg={t.accent}>{props.commands}</span> commands · <span fg={t.group}>{props.groups}</span> groups
        </text>
        <text fg={t.faint}>
          {props.refreshing ? `${spin} re-indexing…` : props.rev ? `@ ${props.rev.slice(0, 7)}` : "indexed"}
        </text>
      </box>
    </box>
  )
}

export function KeyBar({ hints, toast }: { hints: [string, string][]; toast: string | null }) {
  const t = useTheme()
  return (
    <box height={1} flexDirection="row" paddingLeft={1} paddingRight={1} justifyContent="space-between">
      <text wrapMode="none">
        {hints.map(([k, label], i) => (
          <span key={k}>
            {i > 0 ? <span fg={t.faint}>{"  "}</span> : null}
            <b>
              <span fg={t.accent}>{k}</span>
            </b>
            <span fg={t.dim}>{` ${label}`}</span>
          </span>
        ))}
      </text>
      {toast ? (
        <text fg={t.ok}>
          <b>{toast}</b>
        </text>
      ) : null}
    </box>
  )
}

const HELP: [string, [string, string][]][] = [
  [
    "Find",
    [
      ["type", "fuzzy search commands, groups & descriptions (multiple words AND together)"],
      ["↑ ↓ / ^P ^N", "move selection  (PgUp/PgDn jump)"],
      ["esc / ⌫", "clear query → leave group → quit"],
    ],
  ],
  [
    "Explore",
    [
      ["⏎ / ⇥", "on a group: step inside it"],
      ["⏎", "on a command: open the run form"],
      ["⇧↑ ⇧↓ / wheel", "scroll the details pane"],
    ],
  ],
  [
    "Run",
    [
      ["⇥ / ⇧⇥", "next / previous field"],
      ["␣ ← →", "toggle flags, cycle choices"],
      ["⏎", "run — TUI hands the terminal over, then returns"],
      ["^Y", "copy the command line to the clipboard"],
    ],
  ],
  [
    "Everything else",
    [
      ["^F", "favorite / unfavorite"],
      ["^T", "cycle theme"],
      ["^R", "refresh the command catalog"],
      ["? / F1", "this help"],
      ["^C", "quit"],
    ],
  ],
]

export function HelpOverlay() {
  const t = useTheme()
  const { width } = useTerminalDimensions()
  const w = Math.min(88, width - 4)
  return (
    <box
      position="absolute"
      zIndex={30}
      top={3}
      left={Math.floor((width - w) / 2)}
      width={w}
      border
      borderStyle="rounded"
      borderColor={t.borderActive}
      backgroundColor={t.raised}
      title=" Help "
      titleColor={t.accent}
      bottomTitle=" any key to close "
      bottomTitleAlignment="center"
      flexDirection="column"
      padding={1}
    >
      {HELP.map(([section, rows]) => (
        <box key={section} flexDirection="column" marginBottom={1}>
          <text fg={t.group}>
            <b>{section}</b>
          </text>
          {rows.map(([k, d]) => (
            <box key={k} flexDirection="row">
              <box width={16}>
                <text fg={t.accent}>{k}</text>
              </box>
              <text fg={t.text}>{d}</text>
            </box>
          ))}
        </box>
      ))}
    </box>
  )
}

export function Splash({ message, error }: { message: string; error?: string | null }) {
  const t = useTheme()
  const { width, height } = useTerminalDimensions()
  const spin = useSpinner(!error)
  return (
    <box width={width} height={height} alignItems="center" justifyContent="center" flexDirection="column" gap={1}>
      <ascii-font text="klaviyo" font="block" color={t.gradient} />
      {error ? (
        <box flexDirection="column" alignItems="center" maxWidth={width - 8}>
          <text fg={t.err}>
            <b>Could not read the command catalog</b>
          </text>
          <text fg={t.dim}>{error}</text>
          <text fg={t.faint}>press ^C to quit</text>
        </box>
      ) : (
        <>
          <text fg={t.accent}>
            {spin} {message}
          </text>
          <text fg={t.faint}>reading metadata from the launching CLI binary</text>
        </>
      )}
    </box>
  )
}
