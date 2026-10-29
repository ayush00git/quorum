/** SVG pattern defs for diagonal hatching (indeterminate, paused, partitions). */
export function HatchDefs({ id, className = "stroke-fg", gap = 4 }: { id: string; className?: string; gap?: number }) {
  return (
    <defs>
      <pattern id={id} width={gap} height={gap} patternUnits="userSpaceOnUse" patternTransform="rotate(45)">
        <line x1="0" y1="0" x2="0" y2={gap} className={className} strokeWidth="1" />
      </pattern>
    </defs>
  );
}
