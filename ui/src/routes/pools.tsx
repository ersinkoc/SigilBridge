import { Link } from "react-router-dom";
import { Activity, Plus, Route, Search, SlidersHorizontal } from "lucide-react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { toast } from "sonner";

import { EmptyState } from "../components/common/State";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { Input } from "../components/ui/Input";
import { api } from "../lib/api";
import type { PoolDTO } from "../types/api";

export function PoolsRoute() {
  const pools = useQuery({ queryKey: ["pools"], queryFn: () => api<PoolDTO[]>("/admin/v1/pools") });
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState("all");
  const poolRows = pools.data ?? [];
  const visiblePools = useMemo(() => filterPools(poolRows, query, status), [poolRows, query, status]);
  const upstreamCount = poolRows.reduce((total, pool) => total + (pool.upstreams?.length ?? 0), 0);
  const incompleteCount = poolRows.reduce((total, pool) => total + poolReadiness(pool).incomplete, 0);
  const probe = useMutation({
    mutationFn: (id: string) => api<Record<string, unknown>>(`/admin/v1/pools/${encodeURIComponent(id)}/probe`, { method: "POST", body: "{}" }),
    onSuccess: (result) => {
      const checked = Number(result.checked ?? 0);
      const passed = Number(result.passed ?? 0);
      const ok = Boolean(result.ok);
      toast[ok ? "success" : "error"](`Probe ${ok ? "passed" : "failed"}: ${passed}/${checked} upstreams`);
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : "Probe failed")
  });

  return (
    <div className="page">
      <div className="page-title">
        <div className="page-intro">
          <h2>Pools</h2>
          <p>Group upstream providers by routing strategy, budget posture, and failover behavior.</p>
        </div>
        <Link to="/pools/new">
          <Button variant="primary" icon={<Plus size={16} />}>
            Create pool
          </Button>
        </Link>
      </div>
      <div className="overview-grid compact">
        <Card className="overview-panel">
          <div className="panel-heading">
            <div>
              <h2>Routing inventory</h2>
              <p>{poolRows.length} model aliases configured across {upstreamCount} upstreams.</p>
            </div>
            <Route size={20} />
          </div>
        </Card>
        <Card className="overview-panel">
          <div className="panel-heading">
            <div>
              <h2>Configuration model</h2>
              <p>{incompleteCount === 0 ? "Every upstream has the required provider, model, and credential wiring." : `${incompleteCount} routing entries need provider, model, or credential wiring.`}</p>
            </div>
            <SlidersHorizontal size={20} />
          </div>
        </Card>
      </div>
      <Card className="filter-bar">
        <label>
          Search pools
          <div className="input-icon">
            <Search size={16} />
            <Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Pool, provider, model, endpoint..." />
          </div>
        </label>
        <label>
          Status
          <select className="input" value={status} onChange={(event) => setStatus(event.target.value)}>
            <option value="all">All pools</option>
            <option value="ready">Ready</option>
            <option value="incomplete">Incomplete</option>
            <option value="empty">Empty</option>
          </select>
        </label>
        <div className="filter-actions">
          <span className="muted">{visiblePools.length} of {poolRows.length} shown</span>
        </div>
      </Card>
      {poolRows.length === 0 ? (
        <EmptyState label={pools.isLoading ? "Loading pools" : "No pools configured"} />
      ) : visiblePools.length === 0 ? (
        <EmptyState label="No pools match the current filters" />
      ) : (
        <div className="pool-list compact">
          {visiblePools.map((pool) => {
            const readiness = poolReadiness(pool);
            const upstreams = pool.upstreams ?? [];
            return (
              <Card className="pool-row-card" key={pool.id}>
                <div className="pool-row-main">
                  <div className="pool-identity">
                    <Link to={`/pools/${pool.id}`}>
                      <strong>{pool.id}</strong>
                    </Link>
                    <span>{pool.strategy || "priority_first"}</span>
                  </div>
                  <div className="pool-route-summary">
                    {upstreams.length === 0 ? (
                      <span className="muted">No upstreams</span>
                    ) : (
                      upstreams.slice(0, 3).map((upstream, index) => {
                        const issues = upstreamIssues(upstream);
                        return (
                          <span className={issues.length === 0 ? "route-chip ready" : "route-chip"} key={`${String(upstream.id ?? "upstream")}-${index}`}>
                            <strong>{String(upstream.id ?? "upstream")}</strong>
                            <em>{String(upstream.model ?? upstream.provider ?? "unwired")}</em>
                          </span>
                        );
                      })
                    )}
                    {upstreams.length > 3 ? <span className="route-chip muted">+{upstreams.length - 3}</span> : null}
                  </div>
                  <div className="pool-card-actions">
                    <span className={readiness.incomplete === 0 ? "status-pill ok" : "status-pill bad"}>
                      {poolStatusLabel(readiness)}
                    </span>
                    <Button icon={<Activity size={16} />} onClick={() => probe.mutate(pool.id)} disabled={probe.isPending}>
                      Probe
                    </Button>
                    <Link to={`/pools/${pool.id}`}>
                      <Button icon={<SlidersHorizontal size={16} />}>Edit</Button>
                    </Link>
                  </div>
                </div>
              </Card>
            );
          })}
        </div>
      )}
    </div>
  );
}

function poolReadiness(pool: PoolDTO) {
  const upstreams = pool.upstreams ?? [];
  if (upstreams.length === 0) {
    return { total: 0, incomplete: 1 };
  }
  return {
    total: upstreams.length,
    incomplete: upstreams.filter((upstream) => upstreamIssues(upstream).length > 0).length
  };
}

function poolStatusLabel(readiness: { total: number; incomplete: number }) {
  if (readiness.total === 0) {
    return "Empty";
  }
  if (readiness.incomplete === 0) {
    return "Ready";
  }
  return `${readiness.incomplete} incomplete`;
}

function upstreamIssues(upstream: Record<string, unknown>) {
  const issues: string[] = [];
  if (!String(upstream.provider ?? "").trim()) {
    issues.push("adapter");
  }
  if (!String(upstream.model ?? "").trim()) {
    issues.push("model");
  }
  if (requiresCredential(upstream) && !credentialValue(upstream)) {
    issues.push("credential");
  }
  return issues;
}

function credentialValue(upstream: Record<string, unknown>) {
  return String(upstream.api_key_ref ?? upstream.credential ?? upstream.credential_id ?? upstream.vault_id ?? "").trim();
}

function requiresCredential(upstream: Record<string, unknown>) {
  const provider = String(upstream.provider ?? "").toLowerCase();
  if (!provider || provider === "ollama" || provider.includes("_cli") || provider.includes("cli_") || provider.includes("acp")) {
    return false;
  }
  return true;
}

function filterPools(pools: PoolDTO[], query: string, status: string) {
  const needle = query.trim().toLowerCase();
  return pools.filter((pool) => {
    const readiness = poolReadiness(pool);
    if (status === "ready" && (readiness.total === 0 || readiness.incomplete > 0)) {
      return false;
    }
    if (status === "incomplete" && (readiness.total === 0 || readiness.incomplete === 0)) {
      return false;
    }
    if (status === "empty" && readiness.total > 0) {
      return false;
    }
    if (!needle) {
      return true;
    }
    const haystack = [
      pool.id,
      pool.strategy,
      ...(pool.upstreams ?? []).flatMap((upstream) => [upstream.id, upstream.provider, upstream.model, upstream.base_url, credentialValue(upstream)])
    ]
      .filter(Boolean)
      .join(" ")
      .toLowerCase();
    return haystack.includes(needle);
  });
}
