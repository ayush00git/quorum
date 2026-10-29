import Link from "next/link";
import { ThemeToggle } from "./theme-toggle";

const NAV = [
  { href: "/lab", label: "Lab" },
  { href: "/replay", label: "Replays" },
];

export function Wordmark() {
  return (
    <Link href="/" className="group inline-flex items-center gap-2" aria-label="Quorum home">
      <svg viewBox="0 0 16 16" className="size-4" aria-hidden="true">
        <circle cx="8" cy="3" r="1.75" className="fill-fg" />
        <circle cx="13" cy="7" r="1.75" className="fill-none stroke-fg" strokeWidth="1" />
        <circle cx="11" cy="13" r="1.75" className="fill-none stroke-fg" strokeWidth="1" />
        <circle cx="5" cy="13" r="1.75" className="fill-none stroke-fg" strokeWidth="1" />
        <circle cx="3" cy="7" r="1.75" className="fill-none stroke-fg" strokeWidth="1" />
      </svg>
      <span className="font-mono text-sm font-medium tracking-tight">quorum</span>
    </Link>
  );
}

export function SiteHeader({ children }: { children?: React.ReactNode }) {
  return (
    <header className="sticky top-0 z-30 border-b border-border bg-bg">
      <div className="flex h-12 items-center gap-6 px-4 sm:px-6">
        <Wordmark />
        <nav aria-label="Primary" className="flex items-center gap-4">
          {NAV.map((n) => (
            <Link
              key={n.href}
              href={n.href}
              className="text-[13px] text-muted transition-colors duration-150 hover:text-fg"
            >
              {n.label}
            </Link>
          ))}
        </nav>
        <div className="ml-auto flex items-center gap-2">
          {children}
          <ThemeToggle />
        </div>
      </div>
    </header>
  );
}
