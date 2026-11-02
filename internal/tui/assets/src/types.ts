export interface Param {
  kind: "option" | "argument"
  name: string
  opts: string[]
  secondary: string[]
  type: string
  choices: string[] | null
  required: boolean
  multiple: boolean
  nargs: number
  default: unknown
  is_flag: boolean
  count: boolean
  hidden: boolean
  envvar: string | null
  help: string
  metavar: string | null
}

export interface RawNode {
  aliases?: string[]
  name: string
  path: string[]
  isGroup: boolean
  help: string
  short: string
  hidden: boolean
  deprecated: boolean
  params: Param[]
  children: RawNode[]
}

export interface Entry {
  aliases?: string[]
  id: string
  path: string[]
  name: string
  isGroup: boolean
  help: string
  short: string
  deprecated: boolean
  params: Param[]
  childIds: string[]
  /** Lower-cased command path for fuzzy matching. */
  pathText: string
  /** lower-cased help text, used as a secondary search field */
  helpText: string
}

export interface CommandIndex {
  root: Entry
  entries: Entry[]
  byId: Map<string, Entry>
  groups: number
  commands: number
}

export interface TreeCache {
  rev: string | null
  createdAt: number
  tree: RawNode
}
