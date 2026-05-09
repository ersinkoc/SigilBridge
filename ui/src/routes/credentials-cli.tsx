import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Activity, Plus, RefreshCw, Search, Terminal } from "lucide-react";
import { useMemo, useState } from "react";
import { toast } from "sonner";

import { ErrorState, Skeleton } from "../components/common/State";
import { Card } from "../components/ui/Card";
import { Button } from "../components/ui/Button";
import { Input } from "../components/ui/Input";
import { api } from "../lib/api";
import type { CLIAgentDTO, CLIStatusDTO } from "../types/api";

export function CredentialsCliRoute() {
  const queryClient = useQueryClient();
  const cli = useQuery({ queryKey: ["credentials-cli"], queryFn: () => api<CLIStatusDTO>("/admin/v1/credentials/cli") });
  const detect = useQuery({ queryKey: ["credentials-cli-detect"], queryFn: () => api<CLIStatusDTO>("/admin/v1/credentials/cli/detect") });
  const [query, setQuery] = useState("");
  const [source, setSource] = useState("all");
  const [status, setStatus] = useState("all");
  const [probeResult, setProbeResult] = useState<Record<string, unknown> | null>(null);
  const agents = cli.data?.agents ?? [];
  const discovered = detect.data?.agents ?? [];
  const filteredDiscovered = useMemo(() => filterAgents(discovered, query, source, status), [discovered, query, source, status]);
  const registryCount = discovered.filter((agent) => agent.source === "ACP registry").length;
  const runnableCount = discovered.filter((agent) => agent.available).length;
  const enable = useMutation({
    mutationFn: (agent: CLIAgentDTO) =>
      api("/admin/v1/credentials/cli/enable", {
        method: "POST",
        body: JSON.stringify({ provider: agent.provider, command: agent.command, protocol: agent.protocol, framing: agent.framing, args: agent.args })
      }),
    onSuccess: () => {
      toast.success("CLI upstream enabled");
      void queryClient.invalidateQueries({ queryKey: ["credentials"] });
      void queryClient.invalidateQueries({ queryKey: ["credentials-cli"] });
      void queryClient.invalidateQueries({ queryKey: ["credentials-cli-detect"] });
      void queryClient.invalidateQueries({ queryKey: ["pools"] });
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : "CLI enable failed")
  });
  const probe = useMutation({
    mutationFn: (pool: string) => api<Record<string, unknown>>(`/admin/v1/pools/${encodeURIComponent(pool)}/probe`, { method: "POST", body: "{}" }),
    onSuccess: (result) => {
      setProbeResult(result);
      toast[Boolean(result.ok) ? "success" : "error"](`${String(result.pool ?? "Pool")} probe: ${Number(result.passed ?? 0)}/${Number(result.checked ?? 0)} passed`);
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : "Probe failed")
  });

  return (
    <div className="page">
      <div className="page-title">
        <div className="page-intro">
          <h2>CLI and ACP agents</h2>
          <p>Discover local headless tools and ACP Registry agents that can be attached as routing upstreams.</p>
        </div>
        <Button icon={<RefreshCw size={16} />} onClick={() => void detect.refetch()} disabled={detect.isFetching}>
          Scan machine
        </Button>
      </div>
      {cli.isLoading ? (
        <Skeleton />
      ) : cli.isError ? (
        <ErrorState label={(cli.error as Error).message} />
      ) : (
        <>
          <Card className="detail-panel">
            <div>
              <span>CLI subsystem</span>
              <strong>{cli.data?.enabled ? "Enabled" : "Disabled"}</strong>
            </div>
            <div>
              <span>Configured upstreams</span>
              <strong>{agents.length}</strong>
            </div>
            <div>
              <span>Detected candidates</span>
              <strong>{discovered.length}</strong>
            </div>
            <div>
              <span>ACP registry candidates</span>
              <strong>{registryCount}</strong>
            </div>
            <div>
              <span>Runnable now</span>
              <strong>{runnableCount}</strong>
            </div>
          </Card>
          {detect.isLoading ? (
            <Skeleton />
          ) : detect.isError ? (
            <ErrorState label={(detect.error as Error).message} />
          ) : (
            <>
              <Card className="filter-bar">
                <label>
                  Search agents
                  <div className="input-icon">
                    <Search size={16} />
                    <Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Claude, Codex, Qwen, Copilot, opencode..." />
                  </div>
                </label>
                <label>
                  Source
                  <select className="input" value={source} onChange={(event) => setSource(event.target.value)}>
                    <option value="all">All sources</option>
                    <option value="built-in">Built-in</option>
                    <option value="ACP registry">ACP registry</option>
                  </select>
                </label>
                <label>
                  Status
                  <select className="input" value={status} onChange={(event) => setStatus(event.target.value)}>
                    <option value="all">All statuses</option>
                    <option value="runnable">Runnable</option>
                    <option value="missing">Missing</option>
                    <option value="enabled">Enabled</option>
                  </select>
                </label>
              </Card>
              {filteredDiscovered.length === 0 ? (
                <Card className="empty-panel">
                  <strong>No matching agents</strong>
                  <span>Adjust the search or filters to see detected CLI and ACP candidates.</span>
                </Card>
              ) : (
                <div className="agent-grid">
                  {filteredDiscovered.map((agent) => (
                    <Card className="agent-card" key={String(agent.provider)}>
                      <div className="panel-heading">
                        <div>
                          <h2>{agent.name || agent.provider}</h2>
                          <p className="mono-cell">{agent.provider}</p>
                        </div>
                        <span className={agent.available ? "status-pill ok" : "status-pill bad"}>{agent.available ? "Runnable" : "Missing"}</span>
                      </div>
                      <div className="agent-meta">
                        <span>{agent.source || "built-in"}</span>
                        {agent.version ? <span>v{agent.version}</span> : null}
                        <span>{agent.protocol || "legacy"} / {agent.framing || "content-length"}</span>
                      </div>
                      <div className="agent-command">
                        <Terminal size={15} />
                        <code>{agent.command} {agent.args?.join(" ")}</code>
                      </div>
                      <AgentDiagnostics agent={agent} mode="discovered" />
                      <Button icon={<Plus size={14} />} onClick={() => enable.mutate(agent)} disabled={!agent.available || agent.configured || enable.isPending}>
                        {agent.configured ? "Enabled" : "Enable"}
                      </Button>
                    </Card>
                  ))}
                </div>
              )}
            </>
          )}
          {agents.length === 0 ? (
            <Card className="empty-panel">
              <strong>No CLI upstreams configured</strong>
              <span>Enable an available built-in or ACP Registry agent above, then route it from Pools.</span>
            </Card>
          ) : (
            <div className="agent-grid compact">
              {agents.map((agent) => (
                <Card className="agent-card" key={`${agent.pool}:${agent.upstream}`}>
                  <div className="panel-heading">
                    <div>
                      <h2>{agent.name || agent.provider}</h2>
                      <p>{agent.pool} / {agent.upstream}</p>
                    </div>
                    <span className={agent.available ? "status-pill ok" : "status-pill bad"}>{agent.available ? "Available" : "Missing"}</span>
                  </div>
                  <div className="agent-command">
                    <Terminal size={15} />
                    <code>{agent.command} {agent.args?.join(" ")}</code>
                  </div>
                  <AgentDiagnostics agent={agent} mode="configured" />
                  {probeResult && probeResult.pool === agent.pool ? <AgentProbeResult result={probeResult} /> : null}
                  {agent.pool ? (
                    <Button icon={<Activity size={14} />} onClick={() => probe.mutate(String(agent.pool))} disabled={probe.isPending}>
                      Probe pool
                    </Button>
                  ) : null}
                </Card>
              ))}
            </div>
          )}
        </>
      )}
    </div>
  );
}

function AgentDiagnostics({ agent, mode }: { agent: CLIAgentDTO; mode: "discovered" | "configured" }) {
  const commandStatus = agent.available ? agent.path ? "Resolved on PATH" : "Resolved at runtime" : "Command missing";
  const authStatus = agent.auth_status || (agent.available ? "Native CLI auth must be valid for the service user." : "Install the command and complete its native login.");
  const nextStep = agent.configured ? "Probe the pool to verify non-interactive auth and routing." : agent.available ? "Enable, then probe before using this pool in production." : "Install or fix PATH, then scan again.";
  return (
    <div className="agent-diagnostics">
      <div>
        <span>Command</span>
        <strong>{commandStatus}</strong>
      </div>
      <div>
        <span>Auth</span>
        <strong>{authStatus}</strong>
      </div>
      <div>
        <span>Next</span>
        <strong>{mode === "configured" ? nextStep : agent.error || nextStep}</strong>
      </div>
    </div>
  );
}

function AgentProbeResult({ result }: { result: Record<string, unknown> }) {
  const details = Array.isArray(result.results) ? (result.results as Array<Record<string, unknown>>) : [];
  return (
    <div className="credential-probe-result">
      <div className="probe-summary">
        <span className={Boolean(result.ok) ? "status-pill ok" : "status-pill bad"}>{Boolean(result.ok) ? "Probe passed" : "Probe failed"}</span>
        <strong>{String(result.pool ?? "Pool")}</strong>
        <span>{Number(result.passed ?? 0)}/{Number(result.checked ?? 0)} upstreams</span>
      </div>
      {details.slice(0, 3).map((item) => (
        <div className="credential-probe-row" key={String(item.id ?? item.upstream_id ?? item.provider)}>
          <span className={Boolean(item.ok) ? "status-dot" : "status-dot bad"} />
          <strong>{String(item.id ?? item.upstream_id ?? item.provider ?? "upstream")}</strong>
          <span>{String(item.error ?? item.model ?? item.provider ?? "")}</span>
        </div>
      ))}
    </div>
  );
}

function filterAgents(agents: CLIAgentDTO[], query: string, source: string, status: string) {
  const needle = query.trim().toLowerCase();
  return agents.filter((agent) => {
    if (source !== "all" && (agent.source || "built-in") !== source) {
      return false;
    }
    if (status === "runnable" && !agent.available) {
      return false;
    }
    if (status === "missing" && agent.available) {
      return false;
    }
    if (status === "enabled" && !agent.configured) {
      return false;
    }
    if (!needle) {
      return true;
    }
    return `${agent.provider ?? ""} ${agent.name ?? ""} ${agent.command ?? ""} ${agent.source ?? ""} ${(agent.args ?? []).join(" ")}`.toLowerCase().includes(needle);
  });
}
