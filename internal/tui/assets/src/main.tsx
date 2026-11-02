#!/usr/bin/env bun
import { createCliRenderer } from "@opentui/core"
import { createRoot } from "@opentui/react"
import { App, type Result, type Session } from "./App.tsx"
import { formatCommand } from "./form.ts"
import { loadState, recordExit, saveState } from "./state.ts"
import { locateCli } from "./tree.ts"

const ESC = "\x1b"

async function runTui(session: Session): Promise<Result> {
  const renderer = await createCliRenderer({ exitOnCtrlC: true, targetFps: 30, backgroundColor: "#0d1117" })
  return new Promise<Result>((resolve) => {
    let pending: Result | null = null
    // Wait for terminal restoration bytes to flush before attaching the child.
    renderer.on("destroy", () => void renderer.closed.then(() => resolve(pending ?? { type: "quit" })))
    const onResult = (r: Result) => {
      pending = r
      renderer.destroy()
    }
    createRoot(renderer).render(<App session={session} onResult={onResult} />)
  })
}

function waitForKey(): Promise<string> {
  return new Promise((resolve) => {
    const stdin = process.stdin
    stdin.setRawMode?.(true)
    stdin.resume()
    stdin.once("data", (buf) => {
      stdin.setRawMode?.(false)
      stdin.pause()
      resolve(buf.toString())
    })
  })
}

async function runCli(argv: string[]): Promise<number> {
  const dim = (s: string) => `${ESC}[2m${s}${ESC}[0m`
  process.stdout.write(`\n${ESC}[1;32m$${ESC}[0m ${formatCommand(argv)}\n${dim("─".repeat(Math.min(80, process.stdout.columns ?? 80)))}\n`)
  // Ctrl+C in the terminal reaches both processes; only the child should react.
  const swallow = () => {}
  process.on("SIGINT", swallow)
  try {
    const install = locateCli()
    if (!install) throw new Error("Launch with klaviyo --tui")
    const proc = Bun.spawn([install.bin, ...argv], { stdin: "inherit", stdout: "inherit", stderr: "inherit" })
    return await proc.exited
  } finally {
    process.off("SIGINT", swallow)
  }
}

async function main() {
  const rawArgs = process.argv.slice(2)
  if (!process.stdin.isTTY || !process.stdout.isTTY) {
    console.error("klaviyo --tui needs an interactive terminal.")
    process.exit(1)
  }

  const launchArgs = rawArgs

  const session: Session = {
    install: locateCli(),
    index: null,
    rev: null,
    state: loadState(),
    query: "",
    scope: [],
    selId: null,
    toast: null,
    launchArgs: launchArgs.length ? launchArgs : null,
  }

  for (;;) {
    const result = await runTui(session)
    if (result.type === "quit") break

    const code = await runCli(result.argv)
    session.state = recordExit(session.state, code)
    saveState(session.state)
    const status = code === 0 ? `${ESC}[32m✓ exited 0${ESC}[0m` : `${ESC}[31m✗ exited ${code}${ESC}[0m`
    process.stdout.write(`\n${status}  ${ESC}[2m— ⏎ back to browser · q quit${ESC}[0m `)
    const key = await waitForKey()
    process.stdout.write("\n")
    if (key === "q" || key === "\x03") break
    session.toast = code === 0 ? "last command exited 0" : `last command exited ${code}`
  }
  process.exit(0)
}

await main()
