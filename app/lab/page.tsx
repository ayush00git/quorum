import type { Metadata } from "next";
import { LabApp } from "@/components/lab/lab-app";
import { SiteHeader } from "@/components/ui/site-header";

export const metadata: Metadata = {
  title: "Lab",
  description: "Watch Claude hunt for linearizability violations in a simulated Raft cluster.",
};

export default function LabPage() {
  return (
    <div className="flex h-dvh flex-col">
      <SiteHeader />
      <LabApp />
    </div>
  );
}
