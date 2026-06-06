// Minimal ANSI SGR parser: spr colors its terminal output with the basic
// 3/4-bit foreground codes (31 red, 32 green, 34 blue, 0 reset). Anything
// unsupported is stripped so escape bytes never reach the DOM.

export interface AnsiSegment {
  text: string
  // Tailwind class for the active foreground color, if any.
  className?: string
}

const FG_CLASSES: Record<number, string> = {
  30: "text-zinc-500",
  31: "text-red-600 dark:text-red-500",
  32: "text-green-600 dark:text-green-500",
  33: "text-amber-600 dark:text-amber-500",
  34: "text-blue-600 dark:text-blue-500",
  35: "text-fuchsia-600 dark:text-fuchsia-500",
  36: "text-cyan-600 dark:text-cyan-500",
  37: "text-zinc-300",
  90: "text-zinc-500",
  91: "text-red-500",
  92: "text-green-500",
  93: "text-amber-500",
  94: "text-blue-500",
  95: "text-fuchsia-500",
  96: "text-cyan-500",
  97: "text-zinc-100",
}

// biome-ignore lint/suspicious/noControlCharactersInRegex: parsing ANSI escapes requires matching ESC
const ANSI_PATTERN = /\x1b\[([0-9;]*)m|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)/g

export function parseAnsi(line: string): AnsiSegment[] {
  const segments: AnsiSegment[] = []
  let className: string | undefined
  let lastIndex = 0

  for (const match of line.matchAll(ANSI_PATTERN)) {
    if (match.index > lastIndex) {
      segments.push({ text: line.slice(lastIndex, match.index), className })
    }
    lastIndex = match.index + match[0].length
    if (match[1] !== undefined) {
      for (const part of match[1].split(";")) {
        const code = part === "" ? 0 : Number(part)
        if (code === 0) {
          className = undefined
        } else if (FG_CLASSES[code]) {
          className = FG_CLASSES[code]
        }
        // Other SGR codes (bold, background, ...) are ignored.
      }
    }
    // OSC sequences (terminal hyperlinks) are stripped entirely.
  }
  if (lastIndex < line.length) {
    segments.push({ text: line.slice(lastIndex), className })
  }
  return segments
}
