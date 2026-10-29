"use client";

import { useState } from "react";
import type { WorkspaceTab } from "./experiment-workspace";

const TABS: { id: WorkspaceTab; label: string }[] = [
  { id: "agent", label: "Agent" },
  { id: "timeline", label: "Timeline" },
  { id: "cluster", label: "Cluster" },
];

/**
 * Desktop: control rail, workspace, agent log in three columns.
 * Mobile: controls in a collapsible top sheet and the rest as tabs.
 */
export function LabShell({
  rail,
  railSummary,
  workspace,
  log,
  logTitle,
}: {
  rail: React.ReactNode;
  railSummary: React.ReactNode;
  workspace: (tab: WorkspaceTab) => React.ReactNode;
  log: React.ReactNode;
  logTitle: React.ReactNode;
}) {
  const [tab, setTab] = useState<WorkspaceTab>("agent");
  const [sheet, setSheet] = useState(false);

  return (
    <div className="flex min-h-0 flex-1 flex-col lg:grid lg:grid-cols-[264px_minmax(0,1fr)_minmax(340px,420px)]">
      <div className="border-b border-border lg:hidden">
        <button
          type="button"
          onClick={() => setSheet((s) => !s)}
          aria-expanded={sheet}
          aria-controls="lab-controls"
          className="flex h-10 w-full items-center gap-2 px-4 text-left"
        >
          <span className="label text-fg">Controls</span>
          <span className="min-w-0 flex-1 truncate font-mono text-2xs text-muted">{railSummary}</span>
          <svg viewBox="0 0 8 8" className={`size-2 transition-transform duration-150 ${sheet ? "rotate-180" : ""}`} aria-hidden="true">
            <path d="M1 2.5l3 3 3-3" className="fill-none stroke-current" />
          </svg>
        </button>
        <div role="tablist" aria-label="Lab views" className="grid grid-cols-3 border-t border-border">
          {TABS.map((t) => (
            <button
              key={t.id}
              type="button"
              role="tab"
              aria-selected={tab === t.id}
              onClick={() => setTab(t.id)}
              className={`h-9 border-b-2 font-mono text-2xs uppercase tracking-wider transition-colors duration-150 ${
                tab === t.id ? "border-fg text-fg" : "border-transparent text-subtle"
              }`}
            >
              {t.label}
            </button>
          ))}
        </div>
      </div>

      <aside
        id="lab-controls"
        aria-label="Controls"
        className={`${sheet ? "block" : "hidden"} max-h-[70dvh] overflow-y-auto border-b border-border lg:block lg:max-h-none lg:border-r lg:border-b-0`}
      >
        {rail}
      </aside>

      <main className={`${tab === "agent" ? "hidden" : "flex"} min-h-0 flex-1 flex-col lg:flex`}>{workspace(tab)}</main>

      <section
        aria-label="Agent log"
        className={`${tab === "agent" ? "flex" : "hidden"} min-h-0 flex-1 flex-col border-border lg:flex lg:border-l`}
      >
        <header className="hidden h-9 shrink-0 items-center gap-2 border-b border-border px-3 lg:flex">{logTitle}</header>
        {log}
      </section>
    </div>
  );
}
