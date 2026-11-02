import type { CommandIndex, Entry, RawNode, TreeCache } from "./types.ts"

export interface CliInstall { bin: string }

export function locateCli(): CliInstall | null {
  const bin = process.env.KLAVIYO_TUI_BIN
  return bin ? { bin } : null
}

/** Ask the launching binary, not a potentially different version on PATH. */
export async function dumpTree(install: CliInstall): Promise<TreeCache> {
  const proc = Bun.spawn([install.bin, "tui", "catalog"], { stdout: "pipe", stderr: "pipe" })
  const [out, err, code] = await Promise.all([
    new Response(proc.stdout).text(), new Response(proc.stderr).text(), proc.exited,
  ])
  if (code !== 0) throw new Error(`catalog exited ${code}: ${err.slice(-600)}`)
  return { tree: JSON.parse(out) as RawNode, rev: null, createdAt: Date.now() }
}

export function diffIndex(prev: CommandIndex, next: CommandIndex) {
  const leaves = (idx: CommandIndex) => new Set(idx.entries.filter((e) => !e.isGroup).map((e) => e.id))
  const a = leaves(prev), b = leaves(next)
  return { added: [...b].filter((id) => !a.has(id)), removed: [...a].filter((id) => !b.has(id)) }
}

export function buildIndex(tree: RawNode): CommandIndex {
  const entries: Entry[] = []
  const byId = new Map<string, Entry>()
  let groups = 0, commands = 0
  const visit = (node: RawNode): Entry => {
    const id = node.path.join(" ")
    const entry: Entry = {
      id, path: node.path, name: node.name, aliases: node.aliases ?? [], isGroup: node.isGroup,
      help: node.help, short: node.short, deprecated: node.deprecated,
      params: node.params.filter((p) => !p.hidden), childIds: [],
      pathText: id.toLowerCase(), helpText: `${node.short}\n${node.help}`.toLowerCase(),
    }
    byId.set(id, entry)
    if (node.path.length) {
      entries.push(entry)
      if (node.isGroup) groups++; else commands++
    }
    for (const child of node.children) {
      if (!child.hidden) entry.childIds.push(visit(child).id)
    }
    return entry
  }
  const root = visit(tree)
  return { root, entries, byId, groups, commands }
}
