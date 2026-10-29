import type { Metadata } from "next";
import { ReplayApp } from "@/components/replay/replay-app";
import { SiteHeader } from "@/components/ui/site-header";

type Params = { params: Promise<{ id: string }> };

export async function generateMetadata({ params }: Params): Promise<Metadata> {
  const { id } = await params;
  return { title: `Replay ${id}` };
}

export default async function ReplayPage({ params }: Params) {
  const { id } = await params;
  return (
    <div className="flex h-dvh flex-col">
      <SiteHeader />
      <ReplayApp id={id} />
    </div>
  );
}
