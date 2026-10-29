import { Command } from "@/components/ui/panel";

export function RecordHint() {
  return (
    <div className="flex w-full max-w-lg flex-col gap-2">
      <p>Record a run headlessly from the repository root. It needs ANTHROPIC_API_KEY in the environment:</p>
      <Command>go run ./cmd/record -target target-c -seed 42</Command>
      <p>
        The run is written to <code className="font-mono text-fg">public/runs/target-c-42.json</code>. Add it to{" "}
        <code className="font-mono text-fg">public/runs/index.json</code> to list it here.
      </p>
    </div>
  );
}
