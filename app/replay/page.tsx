import type { Metadata } from "next";
import { RunIndex } from "@/components/replay/run-index";
import { SiteHeader } from "@/components/ui/site-header";

export const metadata: Metadata = {
  title: "Replays",
  description: "Recorded Quorum runs, replayed with deterministic re-execution.",
};

export default function ReplayIndexPage() {
  return (
    <>
      <SiteHeader />
      <main className="mx-auto flex max-w-3xl flex-col gap-6 px-4 py-16 sm:px-6">
        <div className="flex flex-col gap-2">
          <h1 className="label">Recorded runs</h1>
          <p className="max-w-xl text-[13px] leading-5 text-muted">
            Replays play Claude&apos;s messages back at reading pace and re-execute every experiment, checking each
            history hash against the recording.
          </p>
        </div>
        <RunIndex />
      </main>
    </>
  );
}
