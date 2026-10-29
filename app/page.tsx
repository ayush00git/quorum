import Link from "next/link";
import { HeroTimeline } from "@/components/landing/hero-timeline";
import { TargetsLadder } from "@/components/landing/targets-ladder";
import { SiteHeader, Wordmark } from "@/components/ui/site-header";

const STEPS = [
  {
    title: "Hypothesize",
    body: "Claude names a failure mode, such as a deposed leader still serving reads, and predicts which fault schedule should expose it.",
    glyph: <path d="M4 16a8 8 0 1 1 12 0v3H4z M7 19v2h6v-2" />,
  },
  {
    title: "Inject faults",
    body: "Partitions, crashes, restarts, GC pauses and latency land at exact virtual times in a deterministic simulator.",
    glyph: <path d="M2 10h6l2-5 3 10 2-5h5" />,
  },
  {
    title: "Check linearizability",
    body: "Porcupine checks every client history. Only the checker can declare a violation; Claude cannot claim one on its own.",
    glyph: <path d="M3 5h14M3 10h9M3 15h5 M12 15l2.5 2.5L19 13" />,
  },
  {
    title: "Shrink and report",
    body: "Delta debugging cuts the schedule down to a minimal reproduction, and Claude writes the causal chain step by step.",
    glyph: <path d="M2 4h16M5 9h10M8 14h4M10 18v0" />,
  },
];

export default function Landing() {
  return (
    <>
      <SiteHeader />
      <main>
        <section className="border-b border-border">
          <div className="mx-auto grid max-w-6xl gap-10 px-4 pt-16 pb-12 sm:px-6 sm:pt-24 lg:grid-cols-12">
            <div className="flex min-w-0 flex-col gap-6 lg:col-span-8">
              <p className="label">Jepsen-style testing · Claude as the adversary</p>
              <h1 className="max-w-3xl text-4xl leading-[1.04] font-semibold tracking-[-0.03em] text-balance sm:text-5xl lg:text-6xl">
                An AI that tries to break your distributed system.
              </h1>
              <p className="max-w-xl text-base leading-7 text-pretty text-muted">
                Claude attacks a simulated five-node Raft cluster with partitions, crashes and pauses. A real
                linearizability checker decides whether it won.
              </p>
              <div className="flex flex-wrap gap-2">
                <Link href="/lab" className="btn btn-primary h-10 px-5 text-sm">
                  Open the lab
                </Link>
                <Link href="/replay" className="btn h-10 px-5 text-sm">
                  Watch a replay
                </Link>
              </div>
            </div>
            <dl className="grid grid-cols-3 gap-px self-end overflow-hidden rounded border border-border bg-border font-mono lg:col-span-4 lg:grid-cols-1">
              {[
                ["5", "Raft nodes"],
                ["7", "fault actions"],
                ["0", "claims without the checker"],
              ].map(([v, k]) => (
                <div key={k} className="flex flex-col gap-1 bg-bg p-3 lg:flex-row lg:items-baseline lg:gap-3">
                  <dt className="sr-only">{k}</dt>
                  <dd className="text-2xl tabular-nums">{v}</dd>
                  <dd className="text-2xs uppercase tracking-wider text-muted">{k}</dd>
                </div>
              ))}
            </dl>
          </div>
          <div className="mx-auto max-w-6xl px-4 pb-16 sm:px-6">
            <HeroTimeline />
          </div>
        </section>

        <section aria-labelledby="how" className="border-b border-border">
          <div className="mx-auto max-w-6xl px-4 py-16 sm:px-6">
            <h2 id="how" className="label mb-8">How it works</h2>
            <ol className="grid gap-px overflow-hidden rounded border border-border bg-border sm:grid-cols-2 lg:grid-cols-4">
              {STEPS.map((s, i) => (
                <li key={s.title} className="flex flex-col gap-4 bg-bg p-5">
                  <div className="flex items-center justify-between">
                    <span className="font-mono text-xs tabular-nums text-subtle">{String(i + 1).padStart(2, "0")}</span>
                    <svg viewBox="0 0 20 22" className="size-5" aria-hidden="true">
                      <g className="fill-none stroke-fg" strokeWidth="1.25" strokeLinecap="round" strokeLinejoin="round">
                        {s.glyph}
                      </g>
                    </svg>
                  </div>
                  <h3 className="text-[15px] font-semibold tracking-tight">{s.title}</h3>
                  <p className="text-[13px] leading-5 text-muted">{s.body}</p>
                </li>
              ))}
            </ol>
          </div>
        </section>

        <section aria-labelledby="targets" className="border-b border-border">
          <div className="mx-auto grid max-w-6xl gap-8 px-4 py-16 sm:px-6 lg:grid-cols-12">
            <div className="flex min-w-0 flex-col gap-3 lg:col-span-4">
              <h2 id="targets" className="label">Targets</h2>
              <p className="text-[13px] leading-5 text-muted">
                Each target is the same Raft implementation with a different planted bug, or none at all. Claude
                only sees the codename. The bug is revealed after the run ends.
              </p>
            </div>
            <div className="min-w-0 lg:col-span-8">
              <TargetsLadder />
            </div>
          </div>
        </section>
      </main>

      <footer className="mx-auto flex max-w-6xl flex-wrap items-center gap-x-6 gap-y-3 px-4 py-8 sm:px-6">
        <Wordmark />
        <p className="font-mono text-2xs text-subtle">deterministic simulation · porcupine checker · claude adversary</p>
        <a
          href="https://github.com/your-org/quorum"
          className="ml-auto font-mono text-xs text-muted underline-offset-2 hover:text-fg hover:underline"
        >
          source
        </a>
      </footer>
    </>
  );
}
