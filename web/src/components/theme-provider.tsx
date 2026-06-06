import * as React from "react"

const COLOR_SCHEME_QUERY = "(prefers-color-scheme: dark)"

// Follows the OS color scheme only — no toggle UI by design.
export function ThemeProvider({ children }: { children: React.ReactNode }) {
  React.useEffect(() => {
    const mediaQuery = window.matchMedia(COLOR_SCHEME_QUERY)
    const apply = () => {
      document.documentElement.classList.toggle("dark", mediaQuery.matches)
    }
    apply()
    mediaQuery.addEventListener("change", apply)
    return () => mediaQuery.removeEventListener("change", apply)
  }, [])

  return <>{children}</>
}
