import type { ScrollBoxRenderable } from "@opentui/core"
import { useEffect, type RefObject } from "react"
import { defaultText, isBoolFlag, paramLabel, paramTypeHint, primaryOpt, shortOpt, usageLine } from "../form.ts"
import { useTheme } from "../theme.ts"
import type { CommandIndex, Entry, Param } from "../types.ts"

interface Props {
  entry: Entry | undefined
  index: CommandIndex
  scrollRef: RefObject<ScrollBoxRenderable | null>
  favorite: boolean
  lastCmd?: string
}

function Label({ children }: { children: string }) {
  const t = useTheme()
  return (
    <box marginTop={1}>
      <text fg={t.faint}>{children}</text>
    </box>
  )
}

function ParamRow({ p }: { p: Param }) {
  const t = useTheme()
  const short = p.kind === "option" ? shortOpt(p) : null
  const name = p.kind === "option" ? primaryOpt(p) : paramLabel(p)
  const meta: string[] = []
  const def = defaultText(p)
  if (p.required) meta.push("required")
  if (def) meta.push(`default: ${def}`)
  if (p.envvar) meta.push(`env: ${p.envvar}`)
  if (p.multiple) meta.push("repeatable")

  return (
    <box flexDirection="column" marginBottom={1}>
      <text>
        <b>
          <span fg={p.kind === "option" ? t.accent2 : t.group}>{name}</span>
        </b>
        {short ? <span fg={t.dim}>{`, ${short}`}</span> : null}
        {p.secondary[0] ? <span fg={t.dim}>{` / ${p.secondary[0]}`}</span> : null}
        {!isBoolFlag(p) ? <span fg={t.faint}>{`  ${paramTypeHint(p)}`}</span> : null}
        {p.required ? <span fg={t.err}> *</span> : null}
      </text>
      {p.help ? (
        <box paddingLeft={2}>
          <text fg={t.text}>{p.help}</text>
        </box>
      ) : null}
      {meta.length ? (
        <box paddingLeft={2}>
          <text fg={t.faint}>{`[${meta.join(" · ")}]`}</text>
        </box>
      ) : null}
    </box>
  )
}

export function Details({ entry, index, scrollRef, favorite, lastCmd }: Props) {
  const t = useTheme()

  useEffect(() => {
    scrollRef.current?.scrollTo(0)
  }, [entry?.id, scrollRef])

  if (!entry) {
    return (
      <box border borderStyle="rounded" borderColor={t.border} flexGrow={1} padding={1} title=" Details " titleColor={t.dim}>
        <text fg={t.dim}>Nothing selected.</text>
      </box>
    )
  }

  const args = entry.params.filter((p) => p.kind === "argument")
  const opts = entry.params.filter((p) => p.kind === "option")
  const children = entry.childIds.map((id) => index.byId.get(id)!).filter(Boolean)
  const nameWidth = Math.min(26, Math.max(0, ...children.map((c) => c.name.length)))
  const subGroups = children.filter((c) => c.isGroup).length

  return (
    <box
      border
      borderStyle="rounded"
      borderColor={t.border}
      flexGrow={1}
      flexDirection="column"
      title={entry.isGroup ? " Group " : " Command "}
      titleColor={t.dim}
    >
      <scrollbox
        ref={scrollRef}
        flexGrow={1}
        paddingLeft={2}
        paddingRight={2}
        paddingTop={1}
        scrollY
        verticalScrollbarOptions={{ visible: false }}
      >
        <text>
          <span fg={t.faint}>klaviyo </span>
          {entry.path.map((seg, i) => (
            <b key={i}>
              <span fg={i === entry.path.length - 1 ? (entry.isGroup ? t.group : t.command) : t.accent}>
                {i === 0 ? seg : ` ${seg}`}
              </span>
            </b>
          ))}
          {favorite ? <span fg={t.warn}> ★</span> : null}
        </text>

        <box marginTop={1}>
          <text fg={t.dim}>
            {entry.isGroup
              ? `${children.length} subcommand${children.length === 1 ? "" : "s"}${subGroups ? ` · ${subGroups} group${subGroups === 1 ? "" : "s"}` : ""}`
              : `${args.length} argument${args.length === 1 ? "" : "s"} · ${opts.length} option${opts.length === 1 ? "" : "s"}`}
            {entry.deprecated ? "  ⚠ deprecated" : ""}
          </text>
        </box>

        {entry.help ? (
          <box marginTop={1}>
            <text fg={t.text}>{entry.help}</text>
          </box>
        ) : (
          <box marginTop={1}>
            <text fg={t.faint}>No description.</text>
          </box>
        )}

        {lastCmd ? (
          <>
            <Label>LAST RUN</Label>
            <text fg={t.dim}>{lastCmd}</text>
          </>
        ) : null}

        <Label>USAGE</Label>
        <text fg={t.accent2}>{usageLine(entry)}</text>

        {entry.isGroup ? (
          <>
            <Label>SUBCOMMANDS</Label>
            {children.map((c) => (
              <box key={c.id} flexDirection="row">
                <box width={nameWidth + 4}>
                  <text wrapMode="none" fg={c.isGroup ? t.group : t.command}>{`${c.isGroup ? "▸" : "›"} ${c.name}`}</text>
                </box>
                <box flexGrow={1}>
                  <text fg={t.dim}>{c.short}</text>
                </box>
              </box>
            ))}
          </>
        ) : null}

        {args.length ? (
          <>
            <Label>ARGUMENTS</Label>
            {args.map((p) => (
              <ParamRow key={p.name} p={p} />
            ))}
          </>
        ) : null}

        {opts.length ? (
          <>
            <Label>OPTIONS</Label>
            {opts.map((p) => (
              <ParamRow key={p.name} p={p} />
            ))}
          </>
        ) : null}
      </scrollbox>
    </box>
  )
}
