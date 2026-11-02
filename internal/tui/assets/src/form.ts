import type { Entry, Param } from "./types.ts"

export type FieldValues = Record<string, string>

/** Forwarded tokens before --, and literal positional extras after it. */
export const EXTRA_KEY = "__extra__"
export const LITERAL_EXTRA_KEY = "__literal_extra__"

export function shellQuote(s: string): string {
  if (s === "") return "''"
  if (/^[A-Za-z0-9_@%+=:,./~-]+$/.test(s)) return s
  return `'${s.replace(/'/g, `'\\''`)}'`
}

export function shellSplit(input: string): string[] {
  const out: string[] = []
  let cur = ""
  let quote: "'" | '"' | null = null
  let has = false
  for (let i = 0; i < input.length; i++) {
    const ch = input[i]!
    if (quote) {
      if (ch === quote) quote = null
      else if (ch === "\\" && quote === '"' && i + 1 < input.length) cur += input[++i]
      else cur += ch
    } else if (ch === "'" || ch === '"') {
      quote = ch
      has = true
    } else if (ch === "\\" && i + 1 < input.length) {
      cur += input[++i]
      has = true
    } else if (/\s/.test(ch)) {
      if (cur || has) out.push(cur)
      cur = ""
      has = false
    } else {
      cur += ch
    }
  }
  if (cur || has) out.push(cur)
  return out
}

export const isBoolFlag = (p: Param) => p.kind === "option" && (p.is_flag || p.count)

export function primaryOpt(p: Param): string {
  return p.opts.find((o) => o.startsWith("--")) ?? p.opts[0] ?? p.name
}

export function shortOpt(p: Param): string | null {
  return p.opts.find((o) => /^-[^-]/.test(o)) ?? null
}

export function paramLabel(p: Param): string {
  if (p.kind === "argument") return (p.metavar ?? p.name).replace(/[<>\[\].]/g, "").toUpperCase()
  return primaryOpt(p)
}

export function paramTypeHint(p: Param): string {
  if (isBoolFlag(p)) return p.secondary.length ? `${primaryOpt(p)}/${p.secondary[0]}` : "flag"
  if (p.choices?.length) return p.choices.join("|")
  const t = p.type.toLowerCase()
  const multi = p.multiple || p.nargs > 1 || p.nargs < 0
  return multi ? `${t}…` : t
}

export function defaultText(p: Param): string {
  const d = p.default
  if (d === null || d === undefined || d === false || d === "") return ""
  if (Array.isArray(d)) return d.length ? d.join(" ") : ""
  if (d === true) return "true"
  return String(d)
}

/** Required params first so the first Tab-stops are the ones you must fill in. */
export function orderedParams(entry: Entry): Param[] {
  const args = entry.params.filter((p) => p.kind === "argument")
  const opts = entry.params.filter((p) => p.kind === "option")
  const rank = (p: Param) => (p.required ? 0 : 1)
  return [...[...args].sort((a, b) => rank(a) - rank(b)), ...[...opts].sort((a, b) => rank(a) - rank(b))]
}

export function emptyValues(entry: Entry): FieldValues {
  const v: FieldValues = {}
  for (const p of entry.params) v[p.name] = ""
  return v
}

function optionTokens(p: Param, raw: string): string[] {
  const flag = primaryOpt(p)
  if (isBoolFlag(p)) {
    if (raw === "on") return [flag]
    if (raw === "off" && p.secondary[0]) return [p.secondary[0]]
    if (raw === "off") return [`${flag}=false`]
    return []
  }
  if (raw.trim() === "") return []
  if (p.multiple) {
    const parts = p.nargs > 1 || p.nargs < 0 ? [shellSplit(raw)] : shellSplit(raw).map((t) => [t])
    return parts.flatMap((group) => [flag, ...group])
  }
  if (p.nargs > 1 || p.nargs < 0) return [flag, ...shellSplit(raw)]
  return [flag, raw]
}

function argumentTokens(p: Param, raw: string): string[] {
  if (raw.trim() === "") return []
  if (p.nargs === 1 && !p.multiple) return [raw]
  return shellSplit(raw)
}

export function buildArgv(entry: Entry, values: FieldValues, extra = values[EXTRA_KEY] ?? ""): string[] {
  const options = entry.params
    .filter((p) => p.kind === "option")
    .flatMap((p) => optionTokens(p, values[p.name] ?? ""))
  const args = entry.params
    .filter((p) => p.kind === "argument")
    .flatMap((p) => argumentTokens(p, values[p.name] ?? ""))
  const forwarded = shellSplit(extra)
  const boundary = forwarded.indexOf("--")
  const literals = [
    ...(boundary < 0 ? [] : forwarded.splice(boundary).slice(1)),
    ...shellSplit(values[LITERAL_EXTRA_KEY] ?? ""),
  ]
  const sep = args.some((a) => a.startsWith("-")) || literals.length ? ["--"] : []
  return [...entry.path, ...options, ...forwarded, ...sep, ...args, ...literals]
}

export const formatCommand = (argv: string[]) => ["klaviyo", ...argv.map(shellQuote)].join(" ")

export function missingRequired(entry: Entry, values: FieldValues): Param[] {
  return entry.params.filter((p) => p.required && (values[p.name] ?? "") === "")
}

/** Derive a synopsis from the command's argument and flag metadata. */
export function usageLine(entry: Entry): string {
  const base = ["klaviyo", ...entry.path].join(" ")
  if (entry.isGroup) return `${base} COMMAND [ARGS]...`
  const hasOpts = entry.params.some((p) => p.kind === "option")
  const args = entry.params
    .filter((p) => p.kind === "argument")
    .map((p) => {
      const l = paramLabel(p) + (p.nargs < 0 || p.multiple ? "..." : "")
      return p.required ? l : `[${l}]`
    })
  return [base, hasOpts ? "[OPTIONS]" : "", ...args].filter(Boolean).join(" ")
}
