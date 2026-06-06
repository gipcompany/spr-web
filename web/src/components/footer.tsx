import type { VersionInfo } from "@/lib/types"

interface FooterProps {
  version?: VersionInfo
  fetchedAt?: string
}

export function Footer({ version, fetchedAt }: FooterProps) {
  return (
    <footer className="flex items-center justify-between border-t px-4 py-1 text-muted-foreground text-xs">
      <span>
        {fetchedAt &&
          `Stack fetched at ${new Date(fetchedAt).toLocaleTimeString()}`}
      </span>
      <span>
        {version
          ? `spr-web ${version.version} · spr ${version.spr}`
          : "spr-web"}
      </span>
    </footer>
  )
}
