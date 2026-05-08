import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Activity, Play, RefreshCw } from "lucide-react";
import { useMemo, useState } from "react";
import { toast } from "sonner";

import { ErrorState, Skeleton } from "../components/common/State";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { api } from "../lib/api";
import type { HealthResponse } from "../types/api";

type PoolProbeResult = {
  ok?: boolean;
  pool?: string;
  checked?: number;
  passed?: number;
  upstreams?: Array<Record<string, unknown>>;
};

export function HealthRoute() {
  const queryClient = useQueryClient();
  const [probeResult, setProbeResult] = useState<PoolProbeResult | null>(null);
  const health = useQuery({ queryKey: ["health"], queryFn: () => api<HealthResponse>("/admin/v1/health") });
  const rows =
    health.data?.upstreams?.map((upstream) => ({
      name: String(upstream.id ?? ""),
      pool: String(upstream.pool ?? ""),
      state: String(upstream.state ?? "configured"),
      detail: `${String(upstream.provider ?? "unknown")} in ${String(upstream.pool ?? "default")}`,
      latency: `p95 ${Number(upstream.latency_p95_ms ?? 0).toFixed(0)} ms`,
      inFlight: Number(upstream.in_flight ?? 0),
      lastError: String(upstream.last_error ?? "")
    })) ?? [];
  const pools = useMemo(() => [...new Set(rows.map((row) => row.pool))].filter(Boolean).sort(), [rows]);
  const probe = useMutation({
    mutationFn: (pool: string) => api<PoolProbeResult>(`/admin/v1/pools/${encodeURIComponent(pool)}/probe`, { method: "POST", body: "{}" }),
    onSuccess: (result) => {
      setProbeResult(result);
      void queryClient.invalidateQueries({ queryKey: ["health"] });
      toast[result.ok ? "success" : "error"](`${result.pool ?? "Pool"} probe: ${result.passed ?? 0}/${result.checked ?? 0} passed`);
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : "Probe failed")
  });

  return (
    <div className="page">
      <div className="page-title">
        <div className="page-intro">
          <h2>Health</h2>
          <p>Run real upstream probes and inspect provider readiness.</p>
        </div>
        <Button icon={<RefreshCw size={16} />} onClick={() => void health.refetch()} disabled={health.isFetching}>
          Refresh
        </Button>
      </div>
      <Card className="probe-panel">
        <div className="panel-heading">
          <div>
            <h2>Provider validation</h2>
            <p>Each probe sends a small live request to every upstream in the selected pool.</p>
          </div>
          <Activity size={20} />
        </div>
        <div className="probe-actions">
          {pools.length === 0 ? (
            <span className="muted">No pools available to probe.</span>
          ) : (
            pools.map((pool) => (
              <Button key={pool} icon={<Play size={14} />} onClick={() => probe.mutate(pool)} disabled={probe.isPending}>
                Probe {pool}
              </Button>
            ))
          )}
        </div>
        {probeResult ? <ProbeResult result={probeResult} /> : null}
      </Card>
      <Card className="overview-panel">
        {health.isLoading ? (
          <Skeleton />
        ) : health.isError ? (
          <ErrorState label={(health.error as Error).message} />
        ) : (
          <div className="lane-list">
            {rows.length === 0 ? (
              <div className="lane-row">
                <span className="status-dot" />
                <div>
                  <strong>No upstreams configured</strong>
                  <span>Create a pool to begin health tracking.</span>
                </div>
                <em>Waiting</em>
              </div>
            ) : (
              rows.map((row) => (
                <div key={row.name} className="lane-row">
                  <span className="status-dot" />
                  <div>
                    <strong>{row.name}</strong>
                    <span>
                      {row.detail} - {row.latency} - {row.inFlight} in flight{row.lastError ? ` - ${row.lastError}` : ""}
                    </span>
                  </div>
                  <em>{row.state}</em>
                </div>
              ))
            )}
          </div>
        )}
      </Card>
    </div>
  );
}

function ProbeResult({ result }: { result: PoolProbeResult }) {
  const upstreams = result.upstreams ?? [];
  return (
    <div className="probe-result">
      <div className="probe-summary">
        <span className={result.ok ? "status-pill ok" : "status-pill bad"}>{result.ok ? "Passed" : "Failed"}</span>
        <strong>{result.pool}</strong>
        <span>
          {result.passed ?? 0}/{result.checked ?? 0} upstreams passed
        </span>
      </div>
      <div className="probe-list">
        {upstreams.map((upstream) => {
          const ok = Boolean(upstream.ok);
          return (
            <div className="probe-row" key={String(upstream.id)}>
              <span className={ok ? "status-pill ok" : "status-pill bad"}>{ok ? "OK" : "Error"}</span>
              <div>
                <strong>{String(upstream.id ?? "")}</strong>
                <span className="mono-cell">{String(upstream.provider ?? "")} / {String(upstream.upstream_model ?? upstream.model ?? "")}</span>
                <span>{ok ? String(upstream.content ?? "") : String(upstream.error ?? "")}</span>
              </div>
              <em>{Number(upstream.latency_ms ?? 0).toFixed(0)} ms</em>
            </div>
          );
        })}
      </div>
    </div>
  );
}
