import { afterEach, beforeEach, expect, test } from "bun:test"
import { chmodSync, linkSync, mkdirSync, mkdtempSync, readFileSync, rmSync, statSync, symlinkSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"

let home: string
beforeEach(() => { home = mkdtempSync(join(tmpdir(), "klaviyo-state-")) })
afterEach(() => { rmSync(home, { recursive: true, force: true }) })

function save(file?: string) {
  const env: NodeJS.ProcessEnv = { ...process.env, HOME: home }
  delete env.KLAVIYO_TUI_STATE
  delete env.XDG_CONFIG_HOME
  if (file) env.KLAVIYO_TUI_STATE = file
  const result = Bun.spawnSync([process.execPath, "-e", `
    import { loadState, saveState } from ${JSON.stringify(join(import.meta.dir, "../src/state.ts"))}
    process.umask(0)
    saveState({ favorites: [], history: [], lastValues: { cmd: { secret: "private-value" } }, theme: "klaviyo" })
    console.log(JSON.stringify(loadState()))
  `], { env })
  expect(result.exitCode).toBe(0)
  return JSON.parse(result.stdout.toString())
}

const mode = (path: string) => statSync(path).mode & 0o777

test("malformed persisted fields cannot break browser navigation", () => {
  const file = join(home, "state.json")
  writeFileSync(file, JSON.stringify({ favorites: null, history: [null, { cmd: "missing id" }], lastValues: { bad: 7, ok: { id: "value" } }, theme: false }))
  const result = Bun.spawnSync([process.execPath, "-e", `
    import { loadState } from ${JSON.stringify(join(import.meta.dir, "../src/state.ts"))}
    console.log(JSON.stringify(loadState()))
  `], { env: { ...process.env, KLAVIYO_TUI_STATE: file } })
  expect(result.exitCode).toBe(0)
  expect(JSON.parse(result.stdout.toString())).toEqual({ favorites: [], history: [], lastValues: { ok: { id: "value" } }, theme: "klaviyo" })
})

test("new default directories and state are private even with permissive umask", () => {
  expect(save().lastValues.cmd.secret).toBe("private-value")
  expect(mode(join(home, ".config"))).toBe(0o700)
  expect(mode(join(home, ".config/klaviyo-tui"))).toBe(0o700)
  expect(mode(join(home, ".config/klaviyo-tui/state.json"))).toBe(0o600)
})

test("existing app directory and file are restricted without changing .config", () => {
  const directory = join(home, ".config/klaviyo-tui")
  mkdirSync(directory, { recursive: true })
  chmodSync(join(home, ".config"), 0o755)
  chmodSync(directory, 0o755)
  const file = join(directory, "state.json")
  writeFileSync(file, "old data".repeat(100))
  chmodSync(file, 0o666)
  expect(save().lastValues.cmd.secret).toBe("private-value")
  expect(mode(directory)).toBe(0o700)
  expect(mode(file)).toBe(0o600)
  expect(mode(join(home, ".config"))).toBe(0o755)
})

test("override preserves a shared parent but restricts its existing state file", () => {
  const directory = join(home, "shared")
  mkdirSync(directory)
  chmodSync(directory, 0o755)
  const file = join(directory, "state.json")
  writeFileSync(file, "old")
  chmodSync(file, 0o644)
  expect(save(file).lastValues.cmd.secret).toBe("private-value")
  expect(mode(directory)).toBe(0o755)
  expect(mode(file)).toBe(0o600)
})

test("new override directories and files are private", () => {
  const file = join(home, "custom/nested/state.json")
  save(file)
  expect(mode(join(home, "custom"))).toBe(0o700)
  expect(mode(join(home, "custom/nested"))).toBe(0o700)
  expect(mode(file)).toBe(0o600)
})

test("symlink and hardlink state targets are not modified", () => {
  const target = join(home, "target")
  writeFileSync(target, "unchanged")
  chmodSync(target, 0o644)
  const symlink = join(home, "symlink.json")
  symlinkSync(target, symlink)
  save(symlink)
  const hardlink = join(home, "hardlink.json")
  linkSync(target, hardlink)
  save(hardlink)
  expect(readFileSync(target, "utf8")).toBe("unchanged")
  expect(mode(target)).toBe(0o644)
})

test("symlink parent directory is not followed for writes or chmod", () => {
  const target = join(home, "target")
  mkdirSync(target)
  chmodSync(target, 0o755)
  writeFileSync(join(target, "state.json"), "unchanged")
  const directory = join(home, "linked")
  symlinkSync(target, directory)
  save(join(directory, "state.json"))
  expect(readFileSync(join(target, "state.json"), "utf8")).toBe("unchanged")
  expect(mode(target)).toBe(0o755)
})
