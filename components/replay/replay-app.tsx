"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { EmptyState } from "@/components/ui/panel";
import { getRecordedRun } from "@/lib/api";
import type { RecordedRun } from "@/lib/types";
import { RecordHint } from "./record-hint";
import { ReplayView } from "./replay-view";

type State = { status: "loading" } | { status: "missing" } | { status: "ready"; run: RecordedRun };

export function ReplayApp({ id }: { id: string }) {
  const [state, setState] = useState<State>({ status: "loading" });

  useEffect(() => {
    const ac = new AbortController();
    getRecordedRun(id, ac.signal).then(
      (run) => setState(run ? { status: "ready", run } : { status: "missing" }),
      () => !ac.signal.aborted && setState({ status: "missing" }),
    );
    return () => ac.abort();
  }, [id]);

  if (state.status === "loading") {
    return (
      <p className="label animate-pulse p-6" role="status">
        Loading run {id}
      </p>
    );
  }
  if (state.status === "missing") {
    return (
      <div className="mx-auto w-full max-w-3xl px-4 py-16">
        <EmptyState title={`No recorded run named ${id}`} className="p-0">
          <RecordHint />
          <Link href="/replay" className="mt-2 text-fg underline underline-offset-2">
            See all recorded runs
          </Link>
        </EmptyState>
      </div>
    );
  }
  return <ReplayView key={id} run={state.run} />;
}
