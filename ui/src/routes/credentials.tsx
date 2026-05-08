import { Link } from "react-router-dom";
import { Activity, Bot, Clock, Globe2, KeyRound, Link2, Plus, RefreshCw, Route, ShieldAlert, Trash2, XCircle } from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import type { ReactNode } from "react";
import { useState } from "react";

import { ConfirmDialog } from "../components/common/ConfirmDialog";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { CredentialTabs } from "../components/credentials/CredentialTabs";
import { ErrorState, Skeleton } from "../components/common/State";
import { api } from "../lib/api";
import type { CLIStatusDTO, CredentialsResponse } from "../types/api";

type CredentialConfirmAction = {
  kind: "delete" | "revoke";
  id: string;
  label: string;
  description: string;
};

export function CredentialsRoute() {
  const queryClient = useQueryClient();
  const [probeResult, setProbeResult] = useState<Record<string, unknown> | null>(null);
  const [confirmAction, setConfirmAction] = useState<CredentialConfirmAction | null>(null);
  const credentials = useQuery({ queryKey: ["credentials"], queryFn: () => api<CredentialsResponse>("/admin/v1/credentials") });
  const cliDetect = useQuery({ queryKey: ["credentials-cli-detect"], queryFn: () => api<CLIStatusDTO>("/admin/v1/credentials/cli/detect") });
  const apiKeys = credentials.data?.api_keys ?? [];
  const sessions = credentials.data?.sessions ?? [];
  const providers = credentials.data?.oauth_providers ?? [];
  const readyProviders = providers.filter((provider) => provider.usable).length;
  const cliAgents = credentials.data?.cli?.agents ?? [];
  const detectedTools = cliDetect.data?.agents ?? [];
  const refresh = useMutation({
    mutationFn: (id: string) => api("/admin/v1/credentials/oauth/refresh", { method: "POST", body: JSON.stringify({ id }) }),
    onSuccess: () => {
      toast.success("Credential refreshed");
      void queryClient.invalidateQueries({ queryKey: ["credentials"] });
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : "Refresh failed")
  });
  const revoke = useMutation({
    mutationFn: (id: string) => api("/admin/v1/credentials/oauth/revoke", { method: "POST", body: JSON.stringify({ id }) }),
    onSuccess: () => {
      toast.success("Credential revoked");
      setConfirmAction(null);
      void queryClient.invalidateQueries({ queryKey: ["credentials"] });
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : "Revoke failed")
  });
  const remove = useMutation({
    mutationFn: (id: string) => api(`/admin/v1/credentials?id=${encodeURIComponent(id)}`, { method: "DELETE" }),
    onSuccess: () => {
      toast.success("Credential deleted");
      setConfirmAction(null);
      void queryClient.invalidateQueries({ queryKey: ["credentials"] });
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : "Delete failed")
  });
  const probe = useMutation({
    mutationFn: (pool: string) => api<Record<string, unknown>>(`/admin/v1/pools/${encodeURIComponent(pool)}/probe`, { method: "POST", body: "{}" }),
    onSuccess: (result) => {
      setProbeResult(result);
      const checked = Number(result.checked ?? 0);
      const passed = Number(result.passed ?? 0);
      toast[Boolean(result.ok) ? "success" : "error"](`${String(result.pool ?? "Pool")} probe: ${passed}/${checked} passed`);
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : "Probe failed")
  });
  const confirmBusy = revoke.isPending || remove.isPending;

  function requestDelete(id: string, label: string, type: "API key" | "session") {
    setConfirmAction({
      kind: "delete",
      id,
      label: `Delete ${type}`,
      description:
        type === "API key"
          ? `Delete ${label}? Pools using this vault reference will fail until another credential is attached.`
          : `Delete ${label}? This removes the local fallback credential from the vault. Revoke first if the upstream provider should invalidate the remote session too.`
    });
  }

  function requestRevoke(id: string, label: string) {
    setConfirmAction({
      kind: "revoke",
      id,
      label: "Revoke session credential",
      description: `Revoke ${label}? This asks the provider to invalidate the session token and the fallback will stop working immediately.`
    });
  }

  function confirmCredentialAction() {
    if (!confirmAction) {
      return;
    }
    if (confirmAction.kind === "revoke") {
      revoke.mutate(confirmAction.id);
      return;
    }
    remove.mutate(confirmAction.id);
  }

  return (
    <div className="page">
      <div className="page-intro">
        <h2>Credentials</h2>
        <p>Encrypted secrets and local auth sources only. Models and routing are managed separately.</p>
      </div>
      <CredentialTabs />
      <SetupGrid providers={providers.length} readyProviders={readyProviders} detectedTools={detectedTools} configuredCLI={cliAgents.length} />
      {credentials.isLoading ? (
        <Skeleton />
      ) : credentials.isError ? (
        <ErrorState label={(credentials.error as Error).message} />
      ) : (
        <>
          <div className="summary-strip">
            <Card className="summary-item">
              <span>API keys</span>
              <strong>{apiKeys.length}</strong>
            </Card>
            <Card className="summary-item">
              <span>OAuth providers</span>
              <strong>{readyProviders}/{providers.length}</strong>
            </Card>
            <Card className="summary-item">
              <span>CLI upstreams</span>
              <strong>{cliAgents.length}</strong>
            </Card>
            <Card className="summary-item">
              <span>Browser sessions</span>
              <strong>{sessions.length}</strong>
            </Card>
          </div>
          {apiKeys.length === 0 && sessions.length === 0 ? (
            <Card className="empty-panel">
              <strong>No stored API keys or session fallbacks</strong>
              <span>Add an API key here, then bind it to models and pools from the routing screens.</span>
            </Card>
          ) : null}
          {apiKeys.length > 0 ? (
            <CredentialRegistry
              title="API key registry"
              rows={apiKeys}
              remove={(id, label) => requestDelete(id, label, "API key")}
              deleting={remove.isPending}
              probePool={(pool) => probe.mutate(pool)}
              probing={probe.isPending}
              probeResult={probeResult}
            />
          ) : null}
          {sessions.length > 0 ? (
            <SessionRegistry
              rows={sessions}
              refresh={(id) => refresh.mutate(id)}
              revoke={(id, label) => requestRevoke(id, label)}
              remove={(id, label) => requestDelete(id, label, "session")}
              busy={refresh.isPending || revoke.isPending || remove.isPending}
            />
          ) : null}
        </>
      )}
      <ConfirmDialog
        open={Boolean(confirmAction)}
        title={confirmAction?.label ?? "Confirm credential action"}
        description={confirmAction?.description}
        confirmLabel={confirmAction?.kind === "revoke" ? "Revoke credential" : "Delete credential"}
        busy={confirmBusy}
        onCancel={() => setConfirmAction(null)}
        onConfirm={confirmCredentialAction}
      />
    </div>
  );
}

function SessionRegistry({
  rows,
  refresh,
  revoke,
  remove,
  busy
}: {
  rows: CredentialsResponse["sessions"];
  refresh: (id: string) => void;
  revoke: (id: string, label: string) => void;
  remove: (id: string, label: string) => void;
  busy: boolean;
}) {
  return (
    <div className="credential-registry">
      <div className="panel-heading">
        <div>
          <h2>Session fallback registry</h2>
          <p>Legacy browser sessions are isolated from API keys, OAuth metadata, and pool routing.</p>
        </div>
        <ShieldAlert size={20} />
      </div>
      <div className="credential-card-grid">
        {(rows ?? []).map((row) => {
          const expired = isExpired(row.expires_at);
          return (
            <Card className="credential-card" key={String(row.id)}>
              <div className="credential-card-header">
                <div>
                  <strong>{credentialShortName(row.id)}</strong>
                  <span>{row.provider ?? "session provider"}</span>
                </div>
                <span className={expired ? "status-pill bad" : "status-pill"}>{expired ? "Expired" : row.expires_at ? "Expires" : "No expiry"}</span>
              </div>
              <div className="credential-id-line">
                <ShieldAlert size={15} />
                <code>{row.id}</code>
              </div>
              <div className="session-meta-grid">
                <SummaryRow icon={<Clock size={15} />} label="Created" value={formatDate(row.created_at)} />
                <SummaryRow icon={<RefreshCw size={15} />} label="Refreshed" value={formatDate(row.last_refreshed_at)} />
                <SummaryRow icon={<XCircle size={15} />} label="Expires" value={formatDate(row.expires_at)} />
              </div>
              <div className="credential-card-footer">
                <span>Use Pools to bind this fallback only when no supported auth path exists.</span>
                <div className="actions-row">
                  <Button icon={<RefreshCw size={14} />} onClick={() => refresh(row.id)} disabled={busy}>
                    Refresh
                  </Button>
                  <Button icon={<XCircle size={14} />} onClick={() => revoke(row.id, credentialShortName(row.id))} disabled={busy}>
                    Revoke
                  </Button>
                  <Button icon={<Trash2 size={14} />} variant="danger" onClick={() => remove(row.id, credentialShortName(row.id))} disabled={busy}>
                    Delete
                  </Button>
                </div>
              </div>
            </Card>
          );
        })}
      </div>
    </div>
  );
}

function SummaryRow({ icon, label, value }: { icon: ReactNode; label: string; value: string }) {
  return (
    <div className="session-meta-row">
      {icon}
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function SetupGrid({
  providers,
  readyProviders,
  detectedTools,
  configuredCLI
}: {
  providers: number;
  readyProviders: number;
  detectedTools: NonNullable<CLIStatusDTO["agents"]>;
  configuredCLI: number;
}) {
  const availableTools = detectedTools.filter((agent) => agent.available).length;

  return (
    <div className="setup-grid">
      <Card className="setup-card">
        <div className="panel-heading">
          <div>
            <h2>API keys</h2>
            <p>Store provider secrets in the encrypted vault. No model or route is configured here.</p>
          </div>
          <KeyRound size={20} />
        </div>
        <Link to="/credentials/api-key/new">
          <Button variant="primary" icon={<Plus size={16} />}>Open setup wizard</Button>
        </Link>
      </Card>
      <Card className="setup-card">
        <div className="panel-heading">
          <div>
            <h2>Subscriptions</h2>
            <p>{providers > 0 ? `${readyProviders}/${providers} OAuth templates are usable. Treat this as advanced provider-specific auth, not the default path.` : "No OAuth provider metadata loaded."}</p>
          </div>
          <Globe2 size={20} />
        </div>
        <div className="actions-row">
          <Link to="/credentials/oauth/new">
            <Button icon={<Globe2 size={16} />}>Advanced OAuth</Button>
          </Link>
          <Link to="/settings/oauth-providers">
            <Button>Provider metadata</Button>
          </Link>
        </div>
      </Card>
      <Card className="setup-card wide">
        <div className="panel-heading">
          <div>
            <h2>Local CLI agents</h2>
            <p>{detectedTools.length > 0 ? `${availableTools}/${detectedTools.length} detected candidates are runnable. ${configuredCLI} CLI upstreams are enabled.` : "Scan installed ACP and CLI tools that already hold their own local login."}</p>
          </div>
          <Bot size={20} />
        </div>
        <div className="summary-strip mini">
          <div>
            <span>Runnable</span>
            <strong>{availableTools}</strong>
          </div>
          <div>
            <span>Known agents</span>
            <strong>{detectedTools.length}</strong>
          </div>
          <div>
            <span>Enabled</span>
            <strong>{configuredCLI}</strong>
          </div>
        </div>
        <Link to="/credentials/cli">
          <Button icon={<RefreshCw size={16} />}>Open CLI scan</Button>
        </Link>
      </Card>
      <Card className="setup-card">
        <div className="panel-heading">
          <div>
            <h2>Session fallback</h2>
            <p>Manual cookie import for last-resort local experiments. Prefer API keys or CLI auth.</p>
          </div>
          <ShieldAlert size={20} />
        </div>
        <Link to="/credentials/sessions/new">
          <Button icon={<ShieldAlert size={16} />}>Open fallback</Button>
        </Link>
      </Card>
    </div>
  );
}

function CredentialRegistry({
  title,
  rows,
  remove,
  deleting,
  probePool,
  probing,
  probeResult
}: {
  title: string;
  rows: CredentialsResponse["api_keys"];
  remove: (id: string, label: string) => void;
  deleting: boolean;
  probePool: (pool: string) => void;
  probing: boolean;
  probeResult: Record<string, unknown> | null;
}) {
  return (
    <div className="credential-registry">
      <div className="panel-heading">
        <div>
          <h2>{title}</h2>
          <p>Stored secrets are managed here; model and endpoint bindings remain visible as attachments.</p>
        </div>
        <KeyRound size={20} />
      </div>
      <div className="credential-card-grid">
        {(rows ?? []).map((row) => {
          const shortID = credentialShortName(row.id);
          const attached = row.attachments ?? [];
          return (
            <Card className="credential-card" key={String(row.id)}>
              <div className="credential-card-header">
                <div>
                  <strong>{shortID}</strong>
                  <span>{row.provider ?? "provider"}</span>
                </div>
                <span className={attached.length > 0 ? "status-pill ok" : "status-pill"}>{attached.length > 0 ? `${attached.length} routes` : "Unattached"}</span>
              </div>
              <div className="credential-id-line">
                <KeyRound size={15} />
                <code>{row.id}</code>
              </div>
              <AttachmentList attachments={attached} probePool={probePool} probing={probing} />
              {probeResult && attached.some((attachment) => attachment.pool === probeResult.pool) ? <CredentialProbeResult result={probeResult} /> : null}
              <div className="credential-card-footer">
                <span>{formatDate(row.created_at)}</span>
                <div className="actions-row">
                  <Link to="/pools">
                    <Button icon={<Route size={14} />}>Pools</Button>
                  </Link>
                  <Button icon={<Trash2 size={14} />} variant="danger" onClick={() => remove(row.id, shortID)} disabled={deleting}>
                    Delete
                  </Button>
                </div>
              </div>
            </Card>
          );
        })}
      </div>
    </div>
  );
}

function AttachmentList({
  attachments,
  probePool,
  probing
}: {
  attachments: NonNullable<CredentialsResponse["api_keys"]>[number]["attachments"];
  probePool: (pool: string) => void;
  probing: boolean;
}) {
  if (!attachments || attachments.length === 0) {
    return (
      <div className="attachment-empty">
        <Link2 size={15} />
        <span>Not attached to any pool yet</span>
      </div>
    );
  }
  return (
    <div className="attachment-list">
      {attachments.map((attachment) => (
        <div key={`${attachment.pool}-${attachment.upstream_id}`} className="attachment-row">
          <div>
            <strong>{attachment.pool}</strong>
            <span className="mono-cell">{attachment.upstream_id || attachment.provider}</span>
          </div>
          <span className="mono-cell">{attachment.model || "No model"}</span>
          {attachment.base_url ? <span className="mono-cell attachment-url">{attachment.base_url}</span> : null}
          {attachment.pool ? (
            <Button icon={<Activity size={14} />} onClick={() => probePool(String(attachment.pool))} disabled={probing}>
              Probe
            </Button>
          ) : null}
        </div>
      ))}
    </div>
  );
}

function CredentialProbeResult({ result }: { result: Record<string, unknown> }) {
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

function credentialShortName(id: string) {
  const parts = id.split("/");
  return parts[parts.length - 1] || id;
}

function formatDate(value?: string) {
  if (!value) {
    return "Not recorded";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}

function isExpired(value?: string) {
  if (!value) {
    return false;
  }
  const date = new Date(value);
  return !Number.isNaN(date.getTime()) && date.getTime() < Date.now();
}
