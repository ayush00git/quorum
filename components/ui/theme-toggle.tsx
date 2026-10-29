"use client";

import { useSyncExternalStore } from "react";
import { applyTheme, currentTheme, type Theme } from "@/lib/theme";

const listeners = new Set<() => void>();

function subscribe(cb: () => void) {
  listeners.add(cb);
  const mq = window.matchMedia("(prefers-color-scheme: light)");
  mq.addEventListener("change", cb);
  return () => {
    listeners.delete(cb);
    mq.removeEventListener("change", cb);
  };
}

export function ThemeToggle() {
  const theme = useSyncExternalStore<Theme | null>(subscribe, currentTheme, () => null);
  const next: Theme = theme === "light" ? "dark" : "light";

  return (
    <button
      type="button"
      className="inline-flex size-8 items-center justify-center rounded border border-border text-muted transition-colors duration-150 hover:border-border-strong hover:text-fg"
      aria-label={`Switch to ${next} theme`}
      title={`Switch to ${next} theme`}
      onClick={() => {
        applyTheme(next);
        listeners.forEach((l) => l());
      }}
    >
      <svg viewBox="0 0 16 16" className="size-3.5" aria-hidden="true">
        <circle cx="8" cy="8" r="6" className="fill-none stroke-current" strokeWidth="1.25" />
        <path d="M8 2 A6 6 0 0 1 8 14 Z" className="fill-current" />
      </svg>
    </button>
  );
}
