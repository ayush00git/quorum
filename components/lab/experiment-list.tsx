import { VerdictGlyph } from "@/components/ui/verdict-glyph";
import { formatMs } from "@/lib/format";
import type { ExperimentSummary } from "@/lib/types";

export function ExperimentList({
  experiments,
  selectedId,
  onSelect,
}: {
  experiments: readonly ExperimentSummary[];
  selectedId: string | null;
  onSelect: (id: string) => void;
}) {
  return (
    <table className="w-full border-collapse font-mono text-xs tabular-nums">
      <thead className="sticky top-0 bg-bg">
        <tr className="text-left text-2xs uppercase tracking-wider text-subtle">
          <th scope="col" className="w-8 py-1.5 pl-3 font-normal">#</th>
          <th scope="col" className="py-1.5 font-normal">id</th>
          <th scope="col" className="py-1.5 text-right font-normal">ops</th>
          <th scope="col" className="py-1.5 text-right font-normal">faults</th>
          <th scope="col" className="py-1.5 pr-3 text-right font-normal">dur</th>
        </tr>
      </thead>
      <tbody>
        {experiments.map((e, i) => {
          const selected = e.id === selectedId;
          return (
            <tr
              key={e.id}
              className={`border-t border-border transition-colors duration-150 ${selected ? "bg-surface" : "hover:bg-surface"}`}
            >
              <td className={`border-l-2 py-0 pl-2.5 text-subtle ${selected ? "border-l-fg" : "border-l-transparent"}`}>
                {String(i + 1).padStart(2, "0")}
              </td>
              <td className="py-0">
                <button
                  type="button"
                  onClick={() => onSelect(e.id)}
                  aria-pressed={selected}
                  aria-label={`Experiment ${e.id}, verdict ${e.verdict}${e.shrunkFrom ? `, shrunk from ${e.shrunkFrom}` : ""}`}
                  className="flex h-8 w-full items-center gap-2 text-left focus-visible:outline-offset-[-1px]"
                >
                  <VerdictGlyph verdict={e.verdict} />
                  <span className={selected ? "text-fg" : "text-muted"}>{e.id}</span>
                  {e.shrunkFrom ? (
                    <span className="rounded-sm border border-border px-1 text-[10px] uppercase tracking-wider text-subtle">
                      min
                    </span>
                  ) : null}
                </button>
              </td>
              <td className="text-right text-muted">{e.ops}</td>
              <td className="text-right text-muted">{e.faults}</td>
              <td className="pr-3 text-right text-muted">{formatMs(e.durationMs)}</td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}
