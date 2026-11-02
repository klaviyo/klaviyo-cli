import { EXTRA_KEY, LITERAL_EXTRA_KEY, emptyValues, isBoolFlag, shellQuote, type FieldValues } from "./form.ts"
import { search } from "./fuzzy.ts"
import type { CommandIndex, Entry, Param } from "./types.ts"

export interface Launch {
  scope: string[]
  query: string
  commonArgs?: string[]
  /** entry to highlight in the list */
  select?: string
  /** open the run form for this command, prefilled from the CLI args */
  run?: { entry: Entry; values: FieldValues; argv: string[] }
  /** shown as a toast when args were only partially understood */
  note?: string
}

/** Turn `["--type", "micro", "host1"]` into run-form values for a command. */
export function parseArgs(entry: Entry, tokens: string[]): FieldValues {
  const values = emptyValues(entry)
  const byFlag = new Map<string, { p: Param; negated: boolean }>()
  for (const p of entry.params) {
    if (p.kind !== "option") continue
    for (const o of p.opts) byFlag.set(o, { p, negated: false })
    for (const o of p.secondary) byFlag.set(o, { p, negated: true })
  }

  const positional: string[] = []
  const extra: string[] = []
  let onlyPositional = false
  let forwarding = false

  for (let i = 0; i < tokens.length; ) {
    const tok = tokens[i++]!
    if (!onlyPositional && tok === "--") {
      onlyPositional = true
      continue
    }
    if (forwarding && !onlyPositional) {
      extra.push(tok)
      continue
    }
    if (onlyPositional || !tok.startsWith("-") || tok === "-") {
      positional.push(tok)
      continue
    }

    const eq = tok.startsWith("--") ? tok.indexOf("=") : -1
    let flag = eq > 0 ? tok.slice(0, eq) : tok
    let inline = eq > 0 ? tok.slice(eq + 1) : undefined
    if (!tok.startsWith("--") && tok.length > 2 && byFlag.has(tok.slice(0, 2))) {
      flag = tok.slice(0, 2)
      inline = tok.slice(2).replace(/^=/, "")
    }
    const hit = byFlag.get(flag)
    if (!hit) {
      // Unknown arity: keep the remaining option/value sequence intact up to --.
      forwarding = true
      extra.push(tok)
      continue
    }
    const { p, negated } = hit
    if (isBoolFlag(p)) {
      values[p.name] = negated || inline === "false" || inline === "0" ? "off" : "on"
      continue
    }
    const take = p.nargs > 1 ? p.nargs : 1
    const vals = inline !== undefined ? [inline] : tokens.slice(i, i + take)
    if (inline === undefined) i += vals.length
    const joined = p.multiple || p.nargs > 1 ? vals.map(shellQuote).join(" ") : (vals[0] ?? "")
    const prev = values[p.name] ?? ""
    values[p.name] = p.multiple && prev ? `${prev} ${joined}` : joined
  }

  const arguments_ = entry.params.filter((x) => x.kind === "argument")
  for (const [at, p] of arguments_.entries()) {
    // Cobra's `api [method] <path>` permits omitting method before a required path.
    if (!p.required && positional.length <= arguments_.slice(at + 1).filter((a) => a.required).length) continue
    if (p.nargs === 1 && !p.multiple) values[p.name] = positional.shift() ?? ""
    else values[p.name] = positional.splice(0).map(shellQuote).join(" ")
  }
  values[EXTRA_KEY] = extra.map(shellQuote).join(" ")
  values[LITERAL_EXTRA_KEY] = positional.map(shellQuote).join(" ")
  return values
}

/**
 * Decide where the TUI should open for `klaviyo --tui <args…>`:
 * walk exact command names as deep as they go, then
 *  - on a command: open its run form with the remaining args prefilled,
 *  - on a group: open it (leftover words become a fuzzy filter inside it).
 */
export function resolveLaunch(index: CommandIndex, args: string[]): Launch {
  let node = index.root
  let i = 0
  const commonArgs: string[] = []
  while (i < args.length && node.isGroup) {
    const tok = args[i]!
    if (tok.startsWith("-")) {
      const flag = tok.startsWith("--") ? tok.split("=")[0]! : tok.slice(0, 2)
      const p = node.params.find((p) => p.opts.includes(flag))
      if (!p || tok === "--") break
      commonArgs.push(tok)
      i++
      const attached = tok.includes("=") || (!tok.startsWith("--") && tok.length > 2)
      if (!isBoolFlag(p) && !attached && i < args.length) commonArgs.push(args[i++]!)
      continue
    }
    const child = node.childIds.map((id) => index.byId.get(id)!).find((c) => c.name === tok || c.aliases?.includes(tok))
    if (!child) break
    node = child
    i++
  }
  const rest = args.slice(i)

  if (!node.isGroup) {
    return {
      scope: node.path.slice(0, -1),
      query: "",
      select: node.id,
      run: { entry: node, values: parseArgs(node, [...commonArgs, ...rest]), argv: [...args] },
    }
  }

  const scope = node.path
  const words = rest.filter((t) => !t.startsWith("-"))
  if (words.length === 0) return { scope, query: "", ...(commonArgs.length ? { commonArgs } : {}) }

  const prefix = scope.join(" ")
  const pool = prefix ? index.entries.filter((e) => e.id.startsWith(`${prefix} `)) : index.entries
  for (let n = words.length; n >= 1; n--) {
    const query = words.slice(0, n).join(" ")
    if (search(query, pool, 1).length > 0) {
      const dropped = words.slice(n)
      return {
        scope,
        query,
        ...(commonArgs.length ? { commonArgs } : {}),
        note: dropped.length ? `no match for “${words.join(" ")}” — showing “${query}”` : undefined,
      }
    }
  }
  return { scope, query: "", ...(commonArgs.length ? { commonArgs } : {}), note: `no match for “${words.join(" ")}”` }
}
