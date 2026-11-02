import type { Entry } from "./types.ts"

export interface Match {
  entry: Entry
  score: number
  /** indices into entry.id that matched (for highlighting) */
  positions: number[]
}

const isBoundary = (ch: string | undefined) => ch === undefined || ch === " " || ch === "-" || ch === "_" || ch === "/"

interface TokenHit {
  score: number
  positions: number[]
}

/**
 * Subsequence match that must start on a word boundary and stay reasonably tight,
 * so short tokens like "ssh" don't match scattered letters in long paths.
 */
function subsequence(token: string, text: string): TokenHit | null {
  const maxSpan = token.length * 3 + 4
  let best: TokenHit | null = null
  for (let start = 0; start < text.length; start++) {
    if (text[start] !== token[0] || !isBoundary(text[start - 1])) continue
    const positions: number[] = []
    let score = 0
    let ti = 0
    let prev = -2
    for (let i = start; i < text.length && ti < token.length; i++) {
      if (text[i] !== token[ti]) continue
      positions.push(i)
      score += 1
      if (i === prev + 1) score += 3
      if (isBoundary(text[i - 1])) score += 4
      prev = i
      ti++
    }
    if (ti < token.length) break
    if (positions[positions.length - 1]! - start + 1 > maxSpan) continue
    if (!best || score > best.score) best = { score, positions }
  }
  return best
}

function matchToken(token: string, entry: Entry): TokenHit | null {
  const text = entry.pathText
  const idx = text.indexOf(token)
  if (idx >= 0) {
    const positions = Array.from({ length: token.length }, (_, i) => idx + i)
    let score = 40 + token.length * 4
    if (isBoundary(text[idx - 1])) score += 20
    // bonus when it hits the final path segment (the command name itself)
    const lastSeg = text.lastIndexOf(" ") + 1
    if (idx >= lastSeg) score += 12
    if (idx === lastSeg && token.length === text.length - lastSeg) score += 25
    if (idx === 0) score += 15
    if (token.length === text.length) score += 30
    return { score, positions }
  }
  const sub = subsequence(token, text)
  if (sub && token.length >= 3) return { score: sub.score, positions: sub.positions }
  if (token.length >= 3 && entry.helpText.includes(token)) return { score: 6, positions: [] }
  return null
}

export function search(query: string, pool: Entry[], limit = 400): Match[] {
  const tokens = query.toLowerCase().split(/\s+/).filter(Boolean)
  if (tokens.length === 0) return pool.slice(0, limit).map((entry) => ({ entry, score: 0, positions: [] }))

  const out: Match[] = []
  for (const entry of pool) {
    let total = 0
    const positions = new Set<number>()
    let ok = true
    for (const token of tokens) {
      const hit = matchToken(token, entry)
      if (!hit) {
        ok = false
        break
      }
      total += hit.score
      for (const p of hit.positions) positions.add(p)
    }
    if (!ok) continue
    total -= entry.path.length * 2
    if (!entry.isGroup) total += 3
    if (entry.deprecated) total -= 15
    out.push({ entry, score: total, positions: [...positions].sort((a, b) => a - b) })
  }
  out.sort((a, b) => b.score - a.score || a.entry.id.localeCompare(b.entry.id))
  return out.slice(0, limit)
}
