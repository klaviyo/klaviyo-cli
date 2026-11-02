import { createContext, useContext } from "react"

export interface Theme {
  name: string
  bg: string
  panel: string
  raised: string
  border: string
  borderActive: string
  text: string
  dim: string
  faint: string
  accent: string
  accent2: string
  group: string
  command: string
  ok: string
  warn: string
  err: string
  selBg: string
  selFg: string
  match: string
  gradient: string[]
}

export const themes: Theme[] = [
  {
    name: "klaviyo",
    bg: "#0d1117",
    panel: "#0d1117",
    raised: "#161b22",
    border: "#30363d",
    borderActive: "#2dd4a7",
    text: "#e6edf3",
    dim: "#8b949e",
    faint: "#565f6b",
    accent: "#2dd4a7",
    accent2: "#7aa2f7",
    group: "#e0af68",
    command: "#9ece6a",
    ok: "#9ece6a",
    warn: "#e0af68",
    err: "#f7768e",
    selBg: "#1b3a33",
    selFg: "#ffffff",
    match: "#ffcb6b",
    gradient: ["#2dd4a7", "#34d399", "#4ade80", "#a3e635", "#facc15"],
  },
  {
    name: "midnight",
    bg: "#0f1020",
    panel: "#0f1020",
    raised: "#1a1b33",
    border: "#34365a",
    borderActive: "#bb9af7",
    text: "#c0caf5",
    dim: "#7982a9",
    faint: "#4c5278",
    accent: "#bb9af7",
    accent2: "#7dcfff",
    group: "#ff9e64",
    command: "#7dcfff",
    ok: "#9ece6a",
    warn: "#e0af68",
    err: "#f7768e",
    selBg: "#2a2750",
    selFg: "#ffffff",
    match: "#ff9e64",
    gradient: ["#7dcfff", "#7aa2f7", "#bb9af7", "#f7768e", "#ff9e64"],
  },
  {
    name: "sunset",
    bg: "#1a1216",
    panel: "#1a1216",
    raised: "#271a20",
    border: "#4a2f3a",
    borderActive: "#ff7a59",
    text: "#f5e6e0",
    dim: "#a58b8b",
    faint: "#6b5258",
    accent: "#ff7a59",
    accent2: "#ffc15e",
    group: "#ffc15e",
    command: "#ff9fb2",
    ok: "#a9d97c",
    warn: "#ffc15e",
    err: "#ff5c7a",
    selBg: "#45242c",
    selFg: "#ffffff",
    match: "#ffe08a",
    gradient: ["#ff5c7a", "#ff7a59", "#ff9a52", "#ffc15e", "#ffe08a"],
  },
  {
    name: "paper",
    bg: "#faf8f3",
    panel: "#faf8f3",
    raised: "#efeadf",
    border: "#cfc8b8",
    borderActive: "#0b7a5e",
    text: "#2b2b2b",
    dim: "#6b665b",
    faint: "#9c968a",
    accent: "#0b7a5e",
    accent2: "#2457c5",
    group: "#a8590a",
    command: "#2f6f1f",
    ok: "#2f6f1f",
    warn: "#a8590a",
    err: "#b3261e",
    selBg: "#d5ece3",
    selFg: "#111111",
    match: "#c2410c",
    gradient: ["#0b7a5e", "#1f8f5f", "#2457c5", "#6d3fc0", "#b3261e"],
  },
]

export const ThemeContext = createContext<Theme>(themes[0]!)
export const useTheme = () => useContext(ThemeContext)
