export function Sparkline({ points }: { points: number[] }) {
  const max = Math.max(...points, 1);
  const d = points
    .map((point, index) => {
      const x = (index / Math.max(points.length - 1, 1)) * 100;
      const y = 28 - (point / max) * 24;
      return `${index === 0 ? "M" : "L"} ${x.toFixed(2)} ${y.toFixed(2)}`;
    })
    .join(" ");
  return (
    <svg className="sparkline" viewBox="0 0 100 32" role="img" aria-label="Trend">
      <path d={d} fill="none" stroke="currentColor" strokeWidth="3" />
    </svg>
  );
}
