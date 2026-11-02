import type { Row } from "../rows.ts"
import { useTheme } from "../theme.ts"

const truncate = (s: string, max: number) => (max <= 0 ? "" : s.length <= max ? s : `${s.slice(0, Math.max(0, max - 1))}…`)

interface Props {
  rows: Row[]
  selected: number
  top: number
  height: number
  width: number
  onSelect: (index: number) => void
  onActivate: (index: number) => void
  onScroll: (delta: number) => void
  title: string
}

export function ResultList({ rows, selected, top, height, width, onSelect, onActivate, onScroll, title }: Props) {
  const t = useTheme()
  const visible = rows.slice(top, top + height)
  const inner = width - 4
  const longest = Math.max(0, ...visible.map((r) => (r.kind === "item" ? r.entry.id.length : 0)))
  const col = Math.max(8, Math.min(longest, Math.floor((inner - 4) * 0.5)))

  return (
    <box
      border
      borderStyle="rounded"
      borderColor={t.border}
      title={title}
      titleColor={t.dim}
      flexGrow={1}
      flexDirection="column"
      paddingLeft={1}
      paddingRight={1}
      onMouseScroll={(e) => {
        const dir = e.scroll?.direction
        if (dir === "up") onScroll(-2)
        else if (dir === "down") onScroll(2)
      }}
    >
      {visible.length === 0 ? (
        <text fg={t.dim}>No matching commands. Try fewer words, or press esc to clear.</text>
      ) : (
        visible.map((row, i) => {
          const index = top + i
          if (row.kind === "header") {
            return (
              <box key={`h-${index}`} height={1}>
                <text fg={t.faint}>{truncate(`── ${row.label} `.padEnd(inner, "─"), inner)}</text>
              </box>
            )
          }
          const { entry, positions, fav, note } = row
          const isSel = index === selected
          const icon = entry.isGroup ? "▸" : "›"
          const marker = fav ? "★" : " "
          const id = truncate(entry.id, inner - 4)
          const posSet = new Set(positions)
          const nameFg = isSel ? t.selFg : entry.isGroup ? t.group : t.command
          const pad = Math.max(0, col - id.length)
          const rest = inner - 4 - id.length - pad - 2
          const sub = truncate(note ?? entry.short, rest)

          // Group consecutive matched/unmatched characters into spans.
          const chunks: { text: string; hit: boolean }[] = []
          for (let c = 0; c < id.length; c++) {
            const hit = posSet.has(c)
            const last = chunks[chunks.length - 1]
            if (last && last.hit === hit) last.text += id[c]
            else chunks.push({ text: id[c]!, hit })
          }

          return (
            <box
              key={`r-${entry.id}-${index}`}
              height={1}
              backgroundColor={isSel ? t.selBg : undefined}
              onMouseDown={() => (isSel ? onActivate(index) : onSelect(index))}
            >
              <text wrapMode="none" fg={nameFg}>
                <span fg={fav ? t.warn : t.faint}>{marker}</span>
                <span fg={isSel ? t.accent : t.dim}>{` ${icon} `}</span>
                {chunks.map((c, ci) =>
                  c.hit ? (
                    <b key={ci}>
                      <span fg={t.match}>{c.text}</span>
                    </b>
                  ) : (
                    <span key={ci} fg={nameFg}>
                      {c.text}
                    </span>
                  ),
                )}
                {entry.deprecated ? <span fg={t.warn}> ⚠</span> : null}
                {sub ? <span fg={isSel ? t.dim : t.faint}>{`${" ".repeat(pad)}  ${sub}`}</span> : null}
              </text>
            </box>
          )
        })
      )}
    </box>
  )
}
