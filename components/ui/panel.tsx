export function Panel({
  title,
  actions,
  children,
  className = "",
  bodyClassName = "",
}: {
  title: React.ReactNode;
  actions?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
  bodyClassName?: string;
}) {
  return (
    <section className={`flex min-h-0 flex-col border-border ${className}`}>
      <header className="flex h-9 shrink-0 items-center gap-3 border-b border-border px-3">
        <h2 className="label">{title}</h2>
        {actions ? <div className="ml-auto flex items-center gap-2">{actions}</div> : null}
      </header>
      <div className={`min-h-0 flex-1 ${bodyClassName}`}>{children}</div>
    </section>
  );
}

export function EmptyState({
  title,
  children,
  className = "",
}: {
  title: string;
  children?: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={`flex flex-col items-start gap-2 p-4 text-[13px] text-muted ${className}`}>
      <p className="font-medium text-fg">{title}</p>
      {children}
    </div>
  );
}

export function Command({ children }: { children: string }) {
  return (
    <code className="block w-full overflow-x-auto rounded border border-border bg-surface px-3 py-2 font-mono text-xs text-fg">
      <span className="select-none text-subtle">$ </span>
      {children}
    </code>
  );
}

export function Kbd({ children }: { children: React.ReactNode }) {
  return (
    <kbd className="inline-flex h-4 min-w-4 items-center justify-center rounded-sm border border-border px-1 font-mono text-[10px] text-muted">
      {children}
    </kbd>
  );
}
