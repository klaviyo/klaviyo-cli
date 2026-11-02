import { useKeyboard, useTerminalDimensions } from "@opentui/react"
import { useMemo, useState } from "react"
import {
  EXTRA_KEY,
  LITERAL_EXTRA_KEY,
  buildArgv,
  defaultText,
  emptyValues,
  formatCommand,
  isBoolFlag,
  missingRequired,
  orderedParams,
  paramLabel,
  paramTypeHint,
  primaryOpt,
  type FieldValues,
} from "../form.ts"
import { useTheme } from "../theme.ts"
import type { Entry, Param } from "../types.ts"

interface Props {
  entry: Entry
  initial?: FieldValues
  initialArgv?: string[]
  onRun: (argv: string[], values: FieldValues) => void
  onCancel: () => void
  onCopy: (cmd: string) => void
}

type Field = { kind: "param"; param: Param } | { kind: "extra"; key: string; label: string }

function cycleFlag(p: Param, cur: string): string {
  const states = ["", "on", "off"]
  return states[(states.indexOf(cur) + 1) % states.length]!
}

function cycleChoice(p: Param, cur: string, dir: 1 | -1): string {
  const states = ["", ...(p.choices ?? [])]
  const i = states.indexOf(cur)
  return states[(i + dir + states.length) % states.length]!
}

export function RunForm({ entry, initial, initialArgv, onRun, onCancel, onCopy }: Props) {
  const t = useTheme()
  const { width, height } = useTerminalDimensions()
  const params = useMemo(() => orderedParams(entry), [entry])
  const fields = useMemo<Field[]>(
    () => [
      ...params.map((param): Field => ({ kind: "param", param })),
      { kind: "extra", key: EXTRA_KEY, label: "extra options" },
      { kind: "extra", key: LITERAL_EXTRA_KEY, label: "literal args" },
    ],
    [params],
  )
  const [values, setValues] = useState<FieldValues>(() => ({ ...emptyValues(entry), ...(initial ?? {}) }))
  const [focus, setFocus] = useState(() => {
    const firstEmptyRequired = params.findIndex((p) => p.required && !(initial?.[p.name] ?? ""))
    return firstEmptyRequired >= 0 ? firstEmptyRequired : 0
  })

  // Keep exact token order and unsupported flag shapes until the user edits.
  const unchanged = JSON.stringify(values) === JSON.stringify({ ...emptyValues(entry), ...(initial ?? {}) })
  const argv = initialArgv && unchanged ? initialArgv : buildArgv(entry, values)
  const cmd = formatCommand(argv)
  const missing = missingRequired(entry, values)

  const set = (key: string, v: string) => setValues((prev) => ({ ...prev, [key]: v }))
  const current = fields[focus]!
  const currentParam = current.kind === "param" ? current.param : null

  const move = (d: number) => setFocus((f) => (f + d + fields.length) % fields.length)

  useKeyboard((key) => {
    if (key.name === "escape") return onCancel()
    if (key.name === "return" || key.name === "enter") return onRun(argv, values)
    if (key.name === "tab") return move(key.shift ? -1 : 1)
    if (key.name === "down") return move(1)
    if (key.name === "up") return move(-1)
    if (key.ctrl && key.name === "y") return onCopy(cmd)
    if (key.ctrl && key.name === "l") return setValues(emptyValues(entry))

    if (!currentParam) return
    if (isBoolFlag(currentParam)) {
      if (key.name === "space" || key.name === "right" || key.name === "left") {
        set(currentParam.name, cycleFlag(currentParam, values[currentParam.name] ?? ""))
      } else if (key.name === "backspace" || key.name === "delete") {
        set(currentParam.name, "")
      }
    } else if (currentParam.choices?.length) {
      if (key.name === "space" || key.name === "right") {
        set(currentParam.name, cycleChoice(currentParam, values[currentParam.name] ?? "", 1))
      } else if (key.name === "left") {
        set(currentParam.name, cycleChoice(currentParam, values[currentParam.name] ?? "", -1))
      } else if (key.name === "backspace" || key.name === "delete") {
        set(currentParam.name, "")
      }
    }
  })

  const modalWidth = Math.min(110, width - 4)
  // border 2 + scroll hints 2 + help 2 + command (margin 1 + box 4) + status 1
  const chromeRows = 12
  const maxFieldRows = Math.max(3, height - 2 - chromeRows)
  const visibleCount = Math.min(fields.length, maxFieldRows)
  const top = Math.min(Math.max(0, focus - Math.floor(visibleCount / 2)), Math.max(0, fields.length - visibleCount))
  const windowed = fields.slice(top, top + visibleCount)

  const labelWidth = Math.min(
    26,
    Math.max(13, ...params.map((p) => paramLabel(p).length + (p.required ? 2 : 0))),
  )
  const valueWidth = Math.max(10, modalWidth - 4 - labelWidth - 6 - 22)

  return (
    <box
      position="absolute"
      zIndex={20}
      left={Math.floor((width - modalWidth) / 2)}
      top={1}
      width={modalWidth}
      height={visibleCount + chromeRows}
      border
      borderStyle="rounded"
      borderColor={t.borderActive}
      backgroundColor={t.raised}
      title={` Run · ${entry.path.join(" ")} `}
      titleColor={t.accent}
      bottomTitle=" ⏎ run  ⇥ next  ⇧⇥ prev  ␣/←/→ toggle  ^Y copy  ^L clear  esc back "
      bottomTitleAlignment="center"
      flexDirection="column"
      paddingLeft={1}
      paddingRight={1}
    >
      <box height={1}>
        <text fg={t.faint}>
          {top > 0 ? `↑ ${top} more` : ""}
        </text>
      </box>
      {windowed.map((field, i) => {
        const idx = top + i
        const sel = idx === focus
        if (field.kind === "extra") {
          const extra = values[field.key] ?? ""
          return (
            <box key={field.key} height={1} flexDirection="row" backgroundColor={sel ? t.selBg : undefined}>
              <text fg={sel ? t.accent : t.faint}>{sel ? "▸ " : "  "}</text>
              <box width={labelWidth + 2}>
                <text fg={t.dim}>{field.label}</text>
              </box>
              {sel ? (
                <input
                  flexGrow={1}
                  focused
                  value={extra}
                  placeholder={field.key === EXTRA_KEY ? "before --, shell-quoted" : "after --, shell-quoted"}
                  textColor={t.text}
                  backgroundColor={t.selBg}
                  focusedBackgroundColor={t.selBg}
                  cursorColor={t.accent}
                  onInput={(v: string) => set(field.key, v)}
                />
              ) : (
                <text fg={extra ? t.text : t.faint}>{extra || "—"}</text>
              )}
            </box>
          )
        }
        const p = field.param
        const raw = values[p.name] ?? ""
        const label = paramLabel(p)
        const star = p.required ? " *" : ""
        const bg = sel ? t.selBg : undefined
        const missingNow = p.required && raw === ""
        return (
          <box key={p.name} height={1} flexDirection="row" backgroundColor={bg}>
            <text fg={sel ? t.accent : t.faint}>{sel ? "▸ " : "  "}</text>
            <box width={labelWidth + 2}>
              <text wrapMode="none">
                <span fg={p.kind === "option" ? t.accent2 : t.group}>{label}</span>
                <span fg={t.err}>{star}</span>
              </text>
            </box>
            {isBoolFlag(p) ? (
              <text fg={raw === "on" ? t.ok : raw === "off" ? t.warn : t.dim}>
                {raw === "on" ? "[x] " : raw === "off" ? "[–] " : "[ ] "}
                {raw === "on" ? primaryOpt(p) : raw === "off" ? p.secondary[0] ?? `${primaryOpt(p)}=false` : "unset"}
              </text>
            ) : p.choices?.length ? (
              <text fg={raw ? t.text : t.faint}>{`‹ ${raw || "—"} ›`}</text>
            ) : sel ? (
              <input
                width={valueWidth}
                focused
                value={raw}
                placeholder={defaultText(p) ? `default: ${defaultText(p)}` : paramTypeHint(p)}
                textColor={t.text}
                backgroundColor={t.selBg}
                focusedBackgroundColor={t.selBg}
                cursorColor={t.accent}
                onInput={(v: string) => set(p.name, v)}
              />
            ) : (
              <text wrapMode="none" fg={raw ? t.text : missingNow ? t.err : t.faint}>
                {raw || (defaultText(p) ? `default: ${defaultText(p)}` : missingNow ? "required" : "—")}
              </text>
            )}
            <box flexGrow={1} />
            <text wrapMode="none" fg={t.faint}>
              {` ${paramTypeHint(p)}`.slice(0, 20)}
            </text>
          </box>
        )
      })}
      <box height={1}>
        <text fg={t.faint}>{top + visibleCount < fields.length ? `↓ ${fields.length - top - visibleCount} more` : ""}</text>
      </box>

      <box height={2} flexDirection="column">
        <text fg={t.dim}>
          {currentParam
            ? currentParam.help ||
              (currentParam.kind === "argument"
                ? currentParam.nargs < 0 || currentParam.multiple
                  ? "Positional argument — separate several values with spaces."
                  : "Positional argument."
                : "No description.")
            : current.kind === "extra" && current.key === EXTRA_KEY
              ? "Forwarded options and their values, before --. Quote values containing spaces."
              : "Literal positional arguments, after --. Leading dashes are not options; quote spaces."}
        </text>
      </box>

      <box marginTop={1} height={4} flexDirection="column" border borderStyle="single" borderColor={t.border} paddingLeft={1}>
        <text fg={t.command}>{`$ ${cmd}`}</text>
      </box>
      <box height={1}>
        {missing.length ? (
          <text fg={t.warn}>{`⚠ required: ${missing.map(paramLabel).join(", ")}`}</text>
        ) : (
          <text fg={t.ok}>✓ ready</text>
        )}
      </box>
    </box>
  )
}
