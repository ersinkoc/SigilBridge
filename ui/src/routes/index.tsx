import { useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { AlertTriangle, ArrowRight, Bot, Check, Clipboard, ClipboardList, HeartPulse, KeyRound, PlugZap, Route, Send, User } from "lucide-react";
import { Link } from "react-router-dom";

import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { api } from "../lib/api";
import type { AuditResponse, BudgetResponse, ChatTestResponse, CredentialsResponse, EndpointInfoResponse, HealthResponse, KeyDTO, PoolDTO, UsageResponse } from "../types/api";

const quickActions = [
  { to: "/keys/new", label: "Create client key", icon: KeyRound },
  { to: "/credentials", label: "Connect provider", icon: PlugZap },
  { to: "/models", label: "Review models", icon: ClipboardList },
  { to: "/pools", label: "Wire routing", icon: Route }
];

export function HomeRoute() {
  const keys = useQuery({ queryKey: ["keys"], queryFn: () => api<KeyDTO[]>("/admin/v1/keys") });
  const pools = useQuery({ queryKey: ["pools"], queryFn: () => api<PoolDTO[]>("/admin/v1/pools") });
  const credentials = useQuery({ queryKey: ["credentials"], queryFn: () => api<CredentialsResponse>("/admin/v1/credentials") });
  const health = useQuery({ queryKey: ["health"], queryFn: () => api<HealthResponse>("/admin/v1/health") });
  const endpoints = useQuery({ queryKey: ["endpoints"], queryFn: () => api<EndpointInfoResponse>("/admin/v1/endpoints") });
  const audit = useQuery({ queryKey: ["audit"], queryFn: () => api<AuditResponse>("/admin/v1/audit") });
  const budgets = useQuery({ queryKey: ["budgets"], queryFn: () => api<BudgetResponse>("/admin/v1/budgets") });
  const usage = useQuery({ queryKey: ["usage"], queryFn: () => api<UsageResponse>("/admin/v1/usage") });
  const keyRows = keys.data ?? [];
  const poolRows = pools.data ?? [];
  const apiKeys = credentials.data?.api_keys ?? [];
  const sessions = credentials.data?.sessions ?? [];
  const oauthProviders = credentials.data?.oauth_providers ?? [];
  const readyOAuth = oauthProviders.filter((provider) => provider.usable).length;
  const cliAgents = credentials.data?.cli?.agents ?? [];
  const auditRows = audit.data?.items ?? [];
  const spendCents = usage.data?.items?.reduce((total, item) => total + Number(item.total_cents ?? item.monthly_cents ?? item.cents ?? 0), 0) ?? 0;
  const configuredUpstreams = poolRows.reduce((total, pool) => total + (pool.upstreams?.length ?? 0), 0);
  const healthRows = health.data?.upstreams ?? [];
  const unhealthyUpstreams = healthRows.filter((item) => {
    const state = String(item.state ?? "").toLowerCase();
    return state && state !== "healthy" && state !== "configured";
  }).length;
  const connectedCredentials = apiKeys.length + sessions.length + cliAgents.length;
  const readinessRows = [
    {
      name: "Client access",
      meta: keyRows.length ? `${keyRows.length} bridge keys can call the API` : "Create a bridge key before external clients can call the API",
      status: keyRows.length ? "Ready" : "Missing",
      ok: keyRows.length > 0,
      to: "/keys/new"
    },
    {
      name: "Provider credentials",
      meta: `${apiKeys.length} API keys, ${readyOAuth}/${oauthProviders.length} OAuth providers ready, ${sessions.length} sessions, ${cliAgents.length} CLI upstreams`,
      status: connectedCredentials + readyOAuth > 0 ? "Connected" : "Missing",
      ok: connectedCredentials + readyOAuth > 0,
      to: "/credentials"
    },
    {
      name: "Routing pools",
      meta: `${poolRows.length} pools with ${configuredUpstreams} upstreams`,
      status: configuredUpstreams > 0 ? "Routable" : "Empty",
      ok: configuredUpstreams > 0,
      to: "/pools"
    },
    {
      name: "Upstream health",
      meta: healthRows.length ? `${healthRows.length - unhealthyUpstreams}/${healthRows.length} upstreams ready` : "No health records yet",
      status: unhealthyUpstreams > 0 ? "Attention" : healthRows.length ? "Ready" : "Pending",
      ok: unhealthyUpstreams === 0,
      to: "/health"
    }
  ];
  const readyCount = readinessRows.filter((row) => row.ok).length;
  const nextStep = readinessRows.find((row) => !row.ok) ?? readinessRows[readinessRows.length - 1];
  const metrics = [
    { label: "Keys", value: String(keyRows.length), detail: keyRows.length ? "client access" : "none" },
    { label: "Credentials", value: String(connectedCredentials), detail: `${apiKeys.length} API - ${cliAgents.length} CLI` },
    { label: "Routes", value: String(configuredUpstreams), detail: `${poolRows.length} pools` },
    { label: "Spend", value: formatDollars(spendCents), detail: `${budgets.data?.keys ?? 0} budgets` }
  ];

  return (
    <div className="page dashboard">
      <div className="operations-board">
        <Card className="readiness-card">
          <div className="readiness-score">
            <span>{readyCount}/{readinessRows.length}</span>
            <div>
              <h2>Operational readiness</h2>
              <p>{nextStep.ok ? "The local bridge is wired. Run a chat test or inspect audit details." : `Next: ${nextStep.name.toLowerCase()}`}</p>
            </div>
          </div>
          <div className="readiness-list">
            {readinessRows.map((row) => (
              <Link key={row.name} to={row.to} className="readiness-row">
                <span className={row.ok ? "status-dot" : "status-dot bad"} />
                <div>
                  <strong>{row.name}</strong>
                  <span>{row.meta}</span>
                </div>
                <em>{row.status}</em>
              </Link>
            ))}
          </div>
        </Card>

        <div className="command-stack">
          <Card className="next-action">
            <div>
              <span className={nextStep.ok ? "status-pill ok" : "status-pill bad"}>{nextStep.ok ? "Ready" : "Action needed"}</span>
              <h2>{nextStep.ok ? "Bridge is routable" : nextStep.name}</h2>
              <p>{nextStep.meta}</p>
            </div>
            <Link to={nextStep.to}>
              <Button variant="primary" icon={<ArrowRight size={16} />}>
                Open
              </Button>
            </Link>
          </Card>
          <div className="metric-strip">
            {metrics.map((metric) => (
              <Card key={metric.label} className="metric-card compact">
                <span className="metric-label">{metric.label}</span>
                <strong className="metric-value">{metric.value}</strong>
                <span className="metric-chip">{metric.detail}</span>
              </Card>
            ))}
          </div>
        </div>
      </div>

      <EndpointPanel endpoints={endpoints.data} />

      <div className="workbench-grid">
        <ChatTester pools={pools.data ?? []} />
        <Card className="overview-panel">
          <div className="panel-heading">
            <div>
              <h2>Control surface</h2>
              <p>Common operations for bringing a provider online and checking traffic.</p>
            </div>
            <HeartPulse size={20} />
          </div>
          <div className="quick-actions">
            {quickActions.map((action) => {
              const Icon = action.icon;
              return (
                <Link key={action.to} to={action.to} className="quick-action">
                  <Icon size={18} />
                  <span>{action.label}</span>
                  <ArrowRight size={16} />
                </Link>
              );
            })}
          </div>
          <div className="recent-strip">
            <div>
              <span>Recent audit rows</span>
              <strong>{auditRows.length}</strong>
            </div>
            <div>
              <span>Health records</span>
              <strong>{healthRows.length}</strong>
            </div>
          </div>
        </Card>
      </div>
    </div>
  );
}

function EndpointPanel({ endpoints }: { endpoints?: EndpointInfoResponse }) {
  const openAIChat = endpoints?.openai_chat ?? "";
  const openAIModels = endpoints?.openai_models ?? "";
  const openAIBase = endpoints?.openai_base ?? "";
  const anthropicMessages = endpoints?.anthropic_messages ?? "";
  const anthropicBase = endpoints?.anthropic_base ?? "";
  return (
    <Card className="endpoint-panel">
      <div className="panel-heading">
        <div>
          <h2>Client endpoints</h2>
          <p>Use a SigilBridge key as the bearer token. These URLs are ready to paste into OpenAI- and Anthropic-compatible clients.</p>
        </div>
        <ClipboardList size={20} />
      </div>
      <div className="endpoint-grid">
        <EndpointGroup
          title="OpenAI-compatible"
          rows={[
            ["Base URL", openAIBase],
            ["Chat completions", openAIChat],
            ["Models", openAIModels]
          ]}
        />
        <EndpointGroup
          title="Anthropic-compatible"
          rows={[
            ["Base URL", anthropicBase],
            ["Messages", anthropicMessages]
          ]}
        />
      </div>
    </Card>
  );
}

function EndpointGroup({ title, rows }: { title: string; rows: Array<[string, string]> }) {
  return (
    <div className="endpoint-group">
      <strong>{title}</strong>
      {rows.map(([label, value]) => (
        <EndpointRow key={label} label={label} value={value || "Loading"} />
      ))}
    </div>
  );
}

function EndpointRow({ label, value }: { label: string; value: string }) {
  const [copied, setCopied] = useState(false);

  async function copy() {
    if (!value || value === "Loading") {
      return;
    }
    await navigator.clipboard.writeText(value);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1200);
  }

  return (
    <div className="endpoint-row">
      <span>{label}</span>
      <code title={value}>{value}</code>
      <Button icon={copied ? <Check size={14} /> : <Clipboard size={14} />} onClick={() => void copy()} disabled={value === "Loading"}>
        {copied ? "Copied" : "Copy"}
      </Button>
    </div>
  );
}

type ChatLine = {
  role: "user" | "assistant" | "error";
  content: string;
  meta?: string;
};

type ChatInspector = {
  request?: Record<string, unknown>;
  response?: ChatTestResponse;
  error?: string;
};

function ChatTester({ pools }: { pools: PoolDTO[] }) {
  const [model, setModel] = useState("");
  const [message, setMessage] = useState("Reply with one sentence confirming the active upstream.");
  const [lines, setLines] = useState<ChatLine[]>([{ role: "assistant", content: "Chat tester is ready.", meta: "admin route" }]);
  const [inspector, setInspector] = useState<ChatInspector>({});
  const modelOptions = pools.map((pool) => pool.id).filter(Boolean);
  const selectedModel = model || modelOptions[0] || "";
  const send = useMutation({
    mutationFn: (body: { model: string; message: string }) =>
      api<ChatTestResponse>("/admin/v1/chat/test", {
        method: "POST",
        body: JSON.stringify(body)
      }),
    onSuccess: (response) => {
      setInspector((current) => ({ ...current, response, error: undefined }));
      const content = response.content.trim();
      const meta = [response.upstream_provider, response.upstream_model || response.model, response.latency_ms ? `${response.latency_ms}ms` : ""].filter(Boolean).join(" - ");
      if (!content) {
        setLines((current) => [...current, { role: "error", content: "The upstream returned an empty text response.", meta }]);
        return;
      }
      setLines((current) => [...current, { role: "assistant", content, meta }]);
    },
    onError: (error) => {
      const message = error instanceof Error ? error.message : "Chat test failed";
      setInspector((current) => ({ ...current, error: message, response: undefined }));
      setLines((current) => [...current, { role: "error", content: message }]);
    }
  });

  function submit() {
    const text = message.trim();
    if (!text || !selectedModel || send.isPending) {
      return;
    }
    const request = { model: selectedModel, message: text };
    setInspector({ request });
    setLines((current) => [...current, { role: "user", content: text, meta: selectedModel }]);
    setMessage("");
    send.mutate(request);
  }

  return (
    <Card className="chat-tester">
      <div className="panel-heading">
        <div>
          <h2>Live chat test</h2>
          <p>Send a real request through a selected pool and inspect the upstream response immediately.</p>
        </div>
        <Bot size={20} />
      </div>
      <div className="chat-layout">
        <div className="chat-transcript" aria-live="polite">
          {lines.map((line, index) => {
            const Icon = line.role === "user" ? User : line.role === "error" ? AlertTriangle : Bot;
            return (
              <div key={`${line.role}-${index}`} className={`chat-message ${line.role}`}>
                <span className="chat-avatar">
                  <Icon size={15} />
                </span>
                <div>
                  <p>{line.content}</p>
                  {line.meta ? <em>{line.meta}</em> : null}
                </div>
              </div>
            );
          })}
        </div>
        <div className="chat-compose">
          <label>
            Pool alias
            <select className="input" value={selectedModel} onChange={(event) => setModel(event.target.value)} disabled={modelOptions.length === 0}>
              {modelOptions.length === 0 ? <option value="">No pools</option> : null}
              {modelOptions.map((option) => (
                <option key={option} value={option}>
                  {option}
                </option>
              ))}
            </select>
          </label>
          <label>
            Message
            <textarea className="input text-area chat-input" value={message} onChange={(event) => setMessage(event.target.value)} />
          </label>
          <Button variant="primary" icon={<Send size={16} />} onClick={submit} disabled={!message.trim() || !selectedModel || send.isPending}>
            {send.isPending ? "Sending" : "Send"}
          </Button>
          <ChatInspectorPanel inspector={inspector} pending={send.isPending} />
        </div>
      </div>
    </Card>
  );
}

function ChatInspectorPanel({ inspector, pending }: { inspector: ChatInspector; pending: boolean }) {
  const response = inspector.response;
  return (
    <div className="chat-inspector">
      <div className="panel-heading">
        <div>
          <h2>Request inspector</h2>
          <p>{pending ? "Waiting for upstream response." : response ? "Last request completed." : inspector.error ? "Last request failed." : "No request sent yet."}</p>
        </div>
        <span className={inspector.error ? "status-pill bad" : response ? "status-pill ok" : "status-pill"}>{pending ? "Pending" : inspector.error ? "Error" : response ? "OK" : "Idle"}</span>
      </div>
      <div className="inspector-grid">
        <div>
          <span>Pool</span>
          <strong>{String(inspector.request?.model ?? "-")}</strong>
        </div>
        <div>
          <span>Provider</span>
          <strong>{response?.upstream_provider || "-"}</strong>
        </div>
        <div>
          <span>Model</span>
          <strong>{response?.upstream_model || response?.model || "-"}</strong>
        </div>
        <div>
          <span>Latency</span>
          <strong>{response?.latency_ms ? `${response.latency_ms} ms` : "-"}</strong>
        </div>
        <div>
          <span>Tokens</span>
          <strong>{response ? `${response.input_tokens ?? 0} in / ${response.output_tokens ?? 0} out` : "-"}</strong>
        </div>
        <div>
          <span>Stop</span>
          <strong>{response?.stop_reason || "-"}</strong>
        </div>
      </div>
      {inspector.error ? <pre>{inspector.error}</pre> : null}
      {inspector.request ? <pre>{JSON.stringify({ request: inspector.request, response: inspector.response ?? null }, null, 2)}</pre> : null}
    </div>
  );
}

function formatDollars(cents: number) {
  return `$${(cents / 100).toFixed(2)}`;
}
