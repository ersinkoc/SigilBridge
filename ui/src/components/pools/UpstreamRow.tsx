export function UpstreamRow({ upstream }: { upstream: Record<string, unknown> }) {
  return (
    <div className="upstream-row">
      <strong>{String(upstream.id ?? "upstream")}</strong>
      <span>{String(upstream.provider ?? "provider")}</span>
    </div>
  );
}
