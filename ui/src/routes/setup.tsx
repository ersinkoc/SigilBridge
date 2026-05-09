import { Link } from "react-router-dom";
import { ArrowRight, CheckCircle2, Clipboard, KeyRound, Layers3, MessageSquareText, PlugZap, Route, ShieldCheck, Terminal } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import type { ComponentType } from "react";

import { ErrorState, Skeleton } from "../components/common/State";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { api } from "../lib/api";
import type { CredentialsResponse, EndpointInfoResponse, KeyDTO, PoolDTO, ProviderCatalogDTO } from "../types/api";

type SetupStep = {
  id: string;
  title: string;
  detail: string;
  ready: boolean;
  action: string;
  to: string;
  icon: ComponentType<{ size?: number }>;
  outcome: string;
  secondaryAction?: string;
  secondaryTo?: string;
  secondaryIcon?: ComponentType<{ size?: number }>;
};

export function SetupRoute() {
  const endpoints = useQuery({ queryKey: ["endpoints"], queryFn: () => api<EndpointInfoResponse>("/admin/v1/endpoints") });
  const keys = useQuery({ queryKey: ["keys"], queryFn: () => api<KeyDTO[]>("/admin/v1/keys") });
  const credentials = useQuery({ queryKey: ["credentials"], queryFn: () => api<CredentialsResponse>("/admin/v1/credentials") });
  const pools = useQuery({ queryKey: ["pools"], queryFn: () => api<PoolDTO[]>("/admin/v1/pools") });
  const catalog = useQuery({ queryKey: ["provider-catalog"], queryFn: () => api<ProviderCatalogDTO>("/admin/v1/provider-catalog") });
  const loading = endpoints.isLoading || keys.isLoading || credentials.isLoading || pools.isLoading || catalog.isLoading;
  const error = endpoints.error || keys.error || credentials.error || pools.error || catalog.error;
  const bridgeKeys = keys.data ?? [];
  const apiKeys = credentials.data?.api_keys ?? [];
  const cliAgents = credentials.data?.cli?.agents ?? [];
  const providers = catalog.data?.providers ?? [];
  const configuredProviders = providers.filter((provider) => provider.configured || provider.available).length;
  const routedPools = (pools.data ?? []).filter((pool) => (pool.upstreams ?? []).length > 0);
  const readySteps = setupSteps({
    bridgeKeyReady: bridgeKeys.some((key) => !key.revoked_at),
    credentialReady: apiKeys.length > 0 || cliAgents.length > 0,
    catalogReady: providers.length > 0,
    poolsReady: routedPools.length > 0
  });
  const readyCount = readySteps.filter((step) => step.ready).length;
  const nextStep = readySteps.find((step) => !step.ready) ?? readySteps[readySteps.length - 1];
  const firstIncompleteIndex = readySteps.findIndex((step) => !step.ready);
  const activeIndex = firstIncompleteIndex === -1 ? readySteps.length - 1 : firstIncompleteIndex;
  const NextIcon = nextStep.icon;

  return (
    <div className="page setup-page">
      <div className="page-title">
        <div className="page-intro">
          <h2>Setup</h2>
          <p>One guided operating path: issue a client key, add provider auth, choose models, bind pools, then test the bridge.</p>
        </div>
        <Link to={nextStep.to}>
          <Button variant="primary" icon={<NextIcon size={16} />}>
            {nextStep.action}
          </Button>
        </Link>
      </div>
      {loading ? (
        <Skeleton />
      ) : error ? (
        <ErrorState label={(error as Error).message} />
      ) : (
        <>
          <div className="setup-command-row">
            <Card className="setup-score-card">
              <span>{readyCount}/{readySteps.length}</span>
              <div>
                <strong>{readyCount === readySteps.length ? "Bridge path is ready" : "Bridge path needs work"}</strong>
                <p>{readyCount === readySteps.length ? "You can send requests through the dashboard tester or any OpenAI/Anthropic-compatible client." : `Next: ${nextStep.title}`}</p>
              </div>
            </Card>
            <EndpointMiniPanel endpoints={endpoints.data} />
          </div>
          <SetupWizard steps={readySteps} activeIndex={activeIndex} readyCount={readyCount} />
          <div className="setup-inventory-grid">
            <InventoryCard title="Client keys" value={bridgeKeys.filter((key) => !key.revoked_at).length} detail={`${bridgeKeys.length} total bridge keys`} icon={KeyRound} to="/keys" />
            <InventoryCard title="Provider auth" value={apiKeys.length + cliAgents.length} detail={`${apiKeys.length} API keys, ${cliAgents.length} CLI agents`} icon={PlugZap} to="/credentials" />
            <InventoryCard title="Catalog" value={providers.length} detail={`${configuredProviders} configured or available providers`} icon={Layers3} to="/models" />
            <InventoryCard title="Pools" value={routedPools.length} detail={`${pools.data?.length ?? 0} pools in config`} icon={Route} to="/pools" />
          </div>
        </>
      )}
    </div>
  );
}

function SetupWizard({ steps, activeIndex, readyCount }: { steps: SetupStep[]; activeIndex: number; readyCount: number }) {
  const activeStep = steps[activeIndex] ?? steps[steps.length - 1];
  const ActiveIcon = activeStep.icon;
  const progress = Math.round((readyCount / steps.length) * 100);
  return (
    <div className="setup-wizard-layout">
      <Card className="setup-wizard-card">
        <div className="panel-heading">
          <div>
            <h2>Guided setup</h2>
            <p>{readyCount === steps.length ? "All required bridge checks are complete." : `Step ${activeIndex + 1} of ${steps.length}: ${activeStep.title}`}</p>
          </div>
          <span className={readyCount === steps.length ? "status-pill ok" : "status-pill"}>{progress}% complete</span>
        </div>
        <div className="setup-progress" role="progressbar" aria-label="Setup progress" aria-valuemin={0} aria-valuemax={100} aria-valuenow={progress}>
          <span style={{ width: `${progress}%` }} />
        </div>
        <ol className="setup-step-list">
          {steps.map((step, index) => {
            const Icon = step.icon;
            const SecondaryIcon = step.secondaryIcon;
            const isActive = index === activeIndex && !step.ready;
            const state = step.ready ? "complete" : isActive ? "active" : "queued";
            return (
              <li className={`setup-step ${state}`} key={step.id} aria-current={isActive ? "step" : undefined}>
                <span className="setup-step-marker">{step.ready ? <CheckCircle2 size={17} /> : index + 1}</span>
                <div className="setup-step-copy">
                  <strong>{step.title}</strong>
                  <span>{step.detail}</span>
                  <em>{step.outcome}</em>
                </div>
                <span className={step.ready ? "status-pill ok" : isActive ? "status-pill" : "status-pill muted-pill"}>{step.ready ? "Complete" : isActive ? "Current step" : "Queued"}</span>
                <div className="setup-step-actions">
                  <Link to={step.to}>
                    <Button variant={isActive ? "primary" : "secondary"} icon={<Icon size={15} />}>
                      {step.action}
                    </Button>
                  </Link>
                  {step.secondaryAction && step.secondaryTo ? (
                    <Link to={step.secondaryTo}>
                      <Button variant="secondary" icon={SecondaryIcon ? <SecondaryIcon size={15} /> : undefined}>
                        {step.secondaryAction}
                      </Button>
                    </Link>
                  ) : null}
                </div>
              </li>
            );
          })}
        </ol>
      </Card>
      <Card className="setup-next-card">
        <div className="setup-next-icon">
          <ActiveIcon size={22} />
        </div>
        <div>
          <span>Next action</span>
          <h2>{activeStep.title}</h2>
          <p>{activeStep.outcome}</p>
        </div>
        <Link to={activeStep.to}>
          <Button variant="primary" icon={<ArrowRight size={16} />}>
            {activeStep.action}
          </Button>
        </Link>
      </Card>
    </div>
  );
}

function EndpointMiniPanel({ endpoints }: { endpoints?: EndpointInfoResponse }) {
  const rows = [
    { label: "OpenAI base", value: endpoints?.openai_base },
    { label: "Chat completions", value: endpoints?.openai_chat },
    { label: "Anthropic base", value: endpoints?.anthropic_base },
    { label: "Messages", value: endpoints?.anthropic_messages }
  ];
  return (
    <Card className="setup-endpoints-card">
      <div className="panel-heading">
        <div>
          <h2>Client endpoints</h2>
          <p>Use these exact URLs in clients after creating a bridge key.</p>
        </div>
        <ShieldCheck size={20} />
      </div>
      <div className="setup-endpoint-list">
        {rows.map((row) => (
          <div className="setup-endpoint-row" key={row.label}>
            <span>{row.label}</span>
            <code title={row.value}>{row.value ?? "-"}</code>
            <Button icon={<Clipboard size={14} />} onClick={() => copy(row.value)} disabled={!row.value}>
              Copy
            </Button>
          </div>
        ))}
      </div>
    </Card>
  );
}

function InventoryCard({
  title,
  value,
  detail,
  icon: Icon,
  to
}: {
  title: string;
  value: number;
  detail: string;
  icon: ComponentType<{ size?: number }>;
  to: string;
}) {
  return (
    <Link to={to}>
      <Card className="inventory-card">
        <div>
          <span>{title}</span>
          <strong>{value.toLocaleString()}</strong>
          <em>{detail}</em>
        </div>
        <Icon size={22} />
      </Card>
    </Link>
  );
}

function setupSteps(input: { bridgeKeyReady: boolean; credentialReady: boolean; catalogReady: boolean; poolsReady: boolean }): SetupStep[] {
  return [
    {
      id: "bridge-key",
      title: "Create a bridge key",
      detail: "This is the client-facing secret used by OpenAI-compatible and Anthropic-compatible callers.",
      ready: input.bridgeKeyReady,
      action: input.bridgeKeyReady ? "Manage keys" : "Create key",
      to: input.bridgeKeyReady ? "/keys" : "/keys/new",
      icon: KeyRound,
      outcome: "External clients need one scoped bridge key before they can call the ingress endpoints."
    },
    {
      id: "provider-auth",
      title: "Add provider authentication",
      detail: "Store API keys, enable local CLI agents, or configure advanced OAuth without mixing those concerns.",
      ready: input.credentialReady,
      action: input.credentialReady ? "Manage credentials" : "Add API key",
      to: input.credentialReady ? "/credentials" : "/credentials/api-key/new",
      icon: PlugZap,
      outcome: "Use an API key provider or enable a local CLI agent before binding a real route.",
      secondaryAction: input.credentialReady ? undefined : "Use CLI agent",
      secondaryTo: input.credentialReady ? undefined : "/credentials/cli",
      secondaryIcon: Terminal
    },
    {
      id: "model-catalog",
      title: "Sync and choose models",
      detail: "Use the models.dev-backed catalog to inspect provider endpoints, adapters, and model IDs.",
      ready: input.catalogReady,
      action: "Open catalog",
      to: "/models",
      icon: Layers3,
      outcome: "Model IDs and provider adapters should be selected from the catalog instead of typed from memory."
    },
    {
      id: "pool-routing",
      title: "Bind routes in pools",
      detail: "Choose endpoint compatibility, provider, credential, base URL, and concrete model per upstream.",
      ready: input.poolsReady,
      action: input.poolsReady ? "Manage pools" : "Configure pools",
      to: "/pools",
      icon: Route,
      outcome: "Pools define the alias clients use and the ordered upstreams SigilBridge may call."
    },
    {
      id: "test",
      title: "Test the bridge",
      detail: "Run the dashboard chat tester against the selected pool before pointing external clients at it.",
      ready: input.bridgeKeyReady && input.credentialReady && input.poolsReady,
      action: "Open tester",
      to: "/",
      icon: MessageSquareText,
      outcome: "The request test confirms authentication, routing, adapter compatibility, and audit capture in one pass."
    }
  ];
}

function copy(value?: string) {
  if (!value) {
    return;
  }
  void navigator.clipboard.writeText(value);
  toast.success("Copied");
}
