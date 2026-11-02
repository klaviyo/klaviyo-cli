import type { Param, RawNode } from "../src/types.ts"

export const param = (over: Partial<Param> & Pick<Param, "name">): Param => ({
  kind: "option", opts: [`--${over.name}`], secondary: [], type: "string",
  choices: null, required: false, multiple: false, nargs: 1, default: "",
  is_flag: false, count: false, hidden: false, envvar: null, help: "", metavar: null, ...over,
})
export const node = (path: string[], over: Partial<RawNode> = {}): RawNode => ({
  name: path.at(-1) ?? "klaviyo", path, isGroup: false, help: "", short: "",
  hidden: false, deprecated: false, params: [], children: [], ...over,
})
const globals = [param({ name: "account", opts: ["--account", "-a"] }), param({ name: "verbose", is_flag: true })]
export const fixtureTree = node([], {
  isGroup: true, params: globals,
  children: [
    node(["profiles"], { isGroup: true, params: globals, children: [
      node(["profiles", "get"], { short: "Retrieve a customer profile", params: [
        param({ name: "id", kind: "argument", required: true }), ...globals,
        param({ name: "include" }), param({ name: "data", multiple: true, opts: ["--data", "-d"] }),
      ] }),
      node(["profiles", "list"], { short: "List profiles", params: globals }),
    ] }),
    node(["auth"], { isGroup: true, params: globals, children: [node(["auth", "list"], { aliases: ["ls"], params: globals })] }),
    node(["api"], { params: [param({ name: "method", kind: "argument" }), param({ name: "path", kind: "argument", required: true }), ...globals] }),
  ],
})
