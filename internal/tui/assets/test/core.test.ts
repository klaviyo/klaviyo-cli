import { expect, test } from "bun:test"
import { search } from "../src/fuzzy.ts"
import { EXTRA_KEY, LITERAL_EXTRA_KEY, buildArgv, emptyValues, formatCommand, shellQuote, shellSplit } from "../src/form.ts"
import { parseArgs, resolveLaunch } from "../src/launch.ts"
import { buildRows, firstItem, stepSelection } from "../src/rows.ts"
import { buildIndex, diffIndex } from "../src/tree.ts"
import { fixtureTree } from "./fixture.ts"

const index = buildIndex(fixtureTree)
const get = index.byId.get("profiles get")!

test("catalog indexes commands, groups and descriptions", () => {
  expect(index.commands).toBe(4)
  expect(index.groups).toBe(2)
  expect(search("customer", index.entries)[0]?.entry.id).toBe("profiles get")
  expect(search("prf get", index.entries)[0]?.entry.id).toBe("profiles get")
  expect(search("profiles auth", index.entries)).toHaveLength(0)
  expect(diffIndex(index, buildIndex({ ...fixtureTree, children: [] })).removed).toHaveLength(4)
})

test("bare/group launches browse; exact commands prefill without executing", () => {
  expect(resolveLaunch(index, [])).toEqual({ scope: [], query: "" })
  expect(resolveLaunch(index, ["profiles"])).toEqual({ scope: ["profiles"], query: "" })
  const launch = resolveLaunch(index, ["profiles", "get", "id1", "--include=lists"])
  expect(launch.run?.values).toMatchObject({ id: "id1", include: "lists" })
  expect(resolveLaunch(index, ["auth", "ls"]).run?.entry.id).toBe("auth list")
})

test("global flags before and between command words retain their exact argv", () => {
  const argv = ["--account", "my account", "profiles", "--verbose=false", "get", "id", "--include=", "--", "-literal", ""]
  const launch = resolveLaunch(index, argv)
  expect(launch.run?.argv).toEqual(argv)
  expect(launch.run?.values).toMatchObject({ account: "my account", verbose: "off", id: "id", include: "" })
  expect(resolveLaunch(index, ["--account", "prod", "profiles"]).commonArgs).toEqual(["--account", "prod"])
  expect(resolveLaunch(index, ["-ademo", "profiles", "get", "id"]).run?.values.account).toBe("demo")
  expect(resolveLaunch(index, ["--account", "prod", "profiles", "ge"]).commonArgs).toEqual(["--account", "prod"])
})

test("optional method before required path is correctly allocated", () => {
  const api = index.byId.get("api")!
  expect(parseArgs(api, ["/api/metrics/"])).toMatchObject({ method: "", path: "/api/metrics/" })
  expect(parseArgs(api, ["GET", "/api/metrics/"])).toMatchObject({ method: "GET", path: "/api/metrics/" })
})

test("unknown option tails and literal arguments stay on separate sides of -- after edits", () => {
  const values = parseArgs(get, ["original", "--unknown", "a b", "--other=ok", "--", "--literal", ""])
  expect(values[EXTRA_KEY]).toBe("--unknown 'a b' --other=ok")
  expect(shellSplit(values[LITERAL_EXTRA_KEY]!)).toEqual(["--literal", ""])
  expect(buildArgv(get, { ...values, id: "-edited" })).toEqual([
    "profiles", "get", "--unknown", "a b", "--other=ok", "--", "-edited", "--literal", "",
  ])
})

test("repeatable flags, boolean false and shell quoting round-trip", () => {
  const values = parseArgs(get, ["id", "-d", "a b", "--data=c", "--verbose=false"])
  expect(buildArgv(get, values)).toEqual(["profiles", "get", "--verbose=false", "--data", "a b", "--data", "c", "id"])
  const tokens = ["plain", "has space", "it's", "", "$(no shell)"]
  expect(shellSplit(tokens.map(shellQuote).join(" "))).toEqual(tokens)
  expect(formatCommand(["profiles", "get", "my id"])).toBe("klaviyo profiles get 'my id'")
  expect(buildArgv(get, emptyValues(get))).toEqual(["profiles", "get"])
})

test("favorites/history pin entries; scoped search excludes other groups; selection skips headings", () => {
  const base = { index, query: "", scope: [], favorites: [get.id], history: [{ id: "auth list", cmd: "klaviyo auth list --account demo", at: 1, exit: 0 }] }
  const rows = buildRows(base)
  expect(rows[0]?.kind).toBe("header")
  expect(rows[firstItem(rows)]).toMatchObject({ kind: "item", entry: { id: get.id } })
  expect(rows[stepSelection(rows, firstItem(rows), 1)]?.kind).toBe("item")
  expect(rows.find((r) => r.kind === "item" && r.entry.id === "auth list")).toMatchObject({ note: "--account demo" })
  expect(buildRows({ ...base, scope: ["profiles"], query: "list" }).every((r) => r.kind === "item" && r.entry.id.startsWith("profiles "))).toBe(true)
})
