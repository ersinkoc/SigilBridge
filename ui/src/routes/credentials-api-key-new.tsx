import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowRight, Check, KeyRound, Link2, RefreshCw, Save, Search } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { toast } from "sonner";

import { ErrorState, Skeleton } from "../components/common/State";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { Input } from "../components/ui/Input";
import { api } from "../lib/api";
import type { APIKeyCredentialRequest, CatalogModelDTO, CatalogProviderDTO, ProviderCatalogDTO } from "../types/api";

type WizardStep = "provider" | "secret" | "routing";

type SaveResult = {
  id?: string;
  provider?: string;
  pool?: string;
  upstream?: string;
};

function visibleProviders(providers: CatalogProviderDTO[], query: string) {
  const needle = query.trim().toLowerCase();
  const filtered = needle
    ? providers.filter((item) => `${item.id} ${item.name ?? ""} ${item.provider ?? ""} ${(item.env ?? []).join(" ")} ${item.base_url ?? ""}`.toLowerCase().includes(needle))
    : providers;
  return [...filtered].sort((a, b) => (a.name ?? a.id).localeCompare(b.name ?? b.id));
}

function providerName(item: CatalogProviderDTO) {
  return item.name || item.id;
}

export function CredentialsApiKeyNewRoute() {
  const queryClient = useQueryClient();
  const catalog = useQuery({ queryKey: ["provider-catalog"], queryFn: () => api<ProviderCatalogDTO>("/admin/v1/provider-catalog") });
  const catalogProviders = useMemo(() => (catalog.data?.providers ?? []).filter((item) => item.category === "api_key"), [catalog.data?.providers]);
  const [step, setStep] = useState<WizardStep>("provider");
  const [search, setSearch] = useState("");
  const providers = useMemo(() => visibleProviders(catalogProviders, search), [catalogProviders, search]);
  const [selectedID, setSelectedID] = useState("");
  const selected = providers.find((item) => item.id === selectedID) ?? catalogProviders.find((item) => item.id === selectedID) ?? providers[0];
  const [credentialName, setCredentialName] = useState("");
  const [apiKey, setAPIKey] = useState("");
  const [attachRoute, setAttachRoute] = useState(true);
  const [poolAlias, setPoolAlias] = useState("");
  const [upstreamID, setUpstreamID] = useState("");
  const [endpoint, setEndpoint] = useState("");
  const [model, setModel] = useState("");
  const [modelQuery, setModelQuery] = useState("");
  const [saved, setSaved] = useState<SaveResult | null>(null);

  useEffect(() => {
    if (!selected) {
      return;
    }
    setCredentialName((current) => current || selected.id);
    setPoolAlias((current) => current || safeAlias(selected.id));
    setUpstreamID((current) => current || `${selected.provider || selected.id}-${safeAlias(selected.id)}`);
    setEndpoint((current) => current || selected.base_url || "");
    setModel((current) => current || selected.top_models?.[0]?.id || "");
  }, [selected]);

  const models = useMemo(() => filterModels(selected?.top_models ?? [], modelQuery), [selected?.top_models, modelQuery]);
  const visibleModels = models.slice(0, 80);

  const save = useMutation({
    mutationFn: () => {
      if (!selected) {
        throw new Error("Select a provider first");
      }
      const body: APIKeyCredentialRequest = {
        provider: selected.provider || selected.id,
        name: credentialName.trim() || selected.id,
        api_key: apiKey
      };
      if (attachRoute) {
        body.pool = poolAlias.trim() || safeAlias(selected.id);
        body.upstream_id = upstreamID.trim() || `${selected.provider || selected.id}-${safeAlias(selected.id)}`;
        body.base_url = endpoint.trim() || selected.base_url || undefined;
        body.model = model.trim() || undefined;
      }
      return api<SaveResult>("/admin/v1/credentials/api-key", { method: "POST", body: JSON.stringify(body) });
    },
    onSuccess: (result) => {
      toast.success(attachRoute ? "API key stored and route updated" : "API key stored in the encrypted vault");
      setSaved(result);
      setAPIKey("");
      void queryClient.invalidateQueries({ queryKey: ["credentials"] });
      void queryClient.invalidateQueries({ queryKey: ["provider-catalog"] });
      void queryClient.invalidateQueries({ queryKey: ["pools"] });
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : "API key save failed")
  });

  function selectProvider(item: CatalogProviderDTO) {
    setSelectedID(item.id);
    setCredentialName(item.id);
    setPoolAlias(safeAlias(item.id));
    setUpstreamID(`${item.provider || item.id}-${safeAlias(item.id)}`);
    setEndpoint(item.base_url || "");
    setModel(item.top_models?.[0]?.id || "");
    setStep("secret");
    setSaved(null);
  }

  return (
    <div className="page">
      <div className="page-title">
        <div className="page-intro">
          <h2>API key setup</h2>
          <p>Store a provider secret and optionally attach it to a routing pool in one guided pass.</p>
        </div>
        <Button icon={<RefreshCw size={16} />} onClick={() => void catalog.refetch()} disabled={catalog.isFetching}>
          Sync catalog
        </Button>
      </div>
      <WizardRail current={step} hasProvider={Boolean(selected)} hasSecret={Boolean(apiKey.trim()) || Boolean(saved)} />
      {catalog.isLoading ? (
        <Skeleton />
      ) : catalog.isError ? (
        <ErrorState label={(catalog.error as Error).message} />
      ) : !selected ? (
        <Card className="empty-panel">
          <strong>No API-key providers found</strong>
          <span>Sync the provider catalog or create a custom pool endpoint manually.</span>
        </Card>
      ) : (
        <div className="wizard-layout">
          <Card className="wizard-panel">
            {step === "provider" ? (
              <ProviderStep providers={providers} selected={selected} search={search} setSearch={setSearch} selectProvider={selectProvider} source={catalog.data?.source ?? "built-in"} />
            ) : null}
            {step === "secret" ? (
              <SecretStep selected={selected} credentialName={credentialName} setCredentialName={setCredentialName} apiKey={apiKey} setAPIKey={setAPIKey} onBack={() => setStep("provider")} onNext={() => setStep("routing")} />
            ) : null}
            {step === "routing" ? (
              <RoutingStep
                selected={selected}
                attachRoute={attachRoute}
                setAttachRoute={setAttachRoute}
                poolAlias={poolAlias}
                setPoolAlias={setPoolAlias}
                upstreamID={upstreamID}
                setUpstreamID={setUpstreamID}
                endpoint={endpoint}
                setEndpoint={setEndpoint}
                model={model}
                setModel={setModel}
                modelQuery={modelQuery}
                setModelQuery={setModelQuery}
                visibleModels={visibleModels}
                totalModels={models.length}
                onBack={() => setStep("secret")}
                onSave={() => save.mutate()}
                saving={save.isPending}
                canSave={Boolean(apiKey.trim()) || Boolean(saved)}
              />
            ) : null}
          </Card>
          <Card className="wizard-summary">
            <div className="panel-heading">
              <div>
                <h2>Connection summary</h2>
                <p>What this wizard will write.</p>
              </div>
              <KeyRound size={20} />
            </div>
            <SummaryRow label="Provider" value={providerName(selected)} />
            <SummaryRow label="Adapter" value={selected.provider || selected.id} />
            <SummaryRow label="Credential" value={`vault://apikey/${selected.provider || selected.id}/${credentialName || selected.id}`} />
            <SummaryRow label="Endpoint" value={endpoint || selected.base_url || "Provider default"} />
            <SummaryRow label="Model" value={model || "Not selected"} />
            <SummaryRow label="Pool" value={attachRoute ? poolAlias || safeAlias(selected.id) : "Not attached"} />
            {saved ? (
              <div className="success-panel">
                <Check size={18} />
                <div>
                  <strong>Saved</strong>
                  <span>{saved.pool ? `Attached to ${saved.pool}` : saved.id}</span>
                </div>
              </div>
            ) : null}
            {saved?.pool ? (
              <Link to={`/pools/${saved.pool}`}>
                <Button variant="primary" icon={<ArrowRight size={16} />}>Open pool</Button>
              </Link>
            ) : null}
          </Card>
        </div>
      )}
    </div>
  );
}

function WizardRail({ current, hasProvider, hasSecret }: { current: WizardStep; hasProvider: boolean; hasSecret: boolean }) {
  const steps: Array<{ id: WizardStep; label: string; detail: string; ok: boolean }> = [
    { id: "provider", label: "Provider", detail: "Catalog source", ok: hasProvider },
    { id: "secret", label: "Secret", detail: "Vault credential", ok: hasSecret },
    { id: "routing", label: "Routing", detail: "Optional pool binding", ok: current === "routing" }
  ];
  return (
    <Card className="wizard-rail">
      {steps.map((item, index) => (
        <div className={item.id === current ? "wizard-step active" : "wizard-step"} key={item.id}>
          <span className={item.ok ? "status-dot" : "status-dot pending"} />
          <div>
            <strong>{index + 1}. {item.label}</strong>
            <span>{item.detail}</span>
          </div>
        </div>
      ))}
    </Card>
  );
}

function ProviderStep({
  providers,
  selected,
  search,
  setSearch,
  selectProvider,
  source
}: {
  providers: CatalogProviderDTO[];
  selected: CatalogProviderDTO;
  search: string;
  setSearch: (value: string) => void;
  selectProvider: (provider: CatalogProviderDTO) => void;
  source: string;
}) {
  return (
    <>
      <div className="panel-heading">
        <div>
          <h2>Select provider</h2>
          <p>Provider metadata comes from the synced model catalog, then remains editable in Pools.</p>
        </div>
        <span className="status-pill">{source}</span>
      </div>
      <div className="input-icon">
        <Search size={16} />
        <Input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="OpenAI, Anthropic, Groq, Kimi, MiniMax..." />
      </div>
      <div className="provider-pick-list wizard-provider-list">
        {providers.map((item) => (
          <button className={item.id === selected.id ? "provider-pick active" : "provider-pick"} key={item.id} type="button" onClick={() => selectProvider(item)}>
            <strong>{providerName(item)}</strong>
            <span>{item.provider || item.id}</span>
            <em>{item.base_url || (item.env ?? []).join(", ") || "API key"}</em>
          </button>
        ))}
      </div>
    </>
  );
}

function SecretStep({
  selected,
  credentialName,
  setCredentialName,
  apiKey,
  setAPIKey,
  onBack,
  onNext
}: {
  selected: CatalogProviderDTO;
  credentialName: string;
  setCredentialName: (value: string) => void;
  apiKey: string;
  setAPIKey: (value: string) => void;
  onBack: () => void;
  onNext: () => void;
}) {
  return (
    <>
      <div className="panel-heading">
        <div>
          <h2>Store secret</h2>
          <p>The key is written to the encrypted vault. It is never displayed back by the UI.</p>
        </div>
        <span className={selected.configured ? "status-pill ok" : "status-pill"}>{selected.configured ? "Existing provider" : "New provider"}</span>
      </div>
      <div className="credential-form-grid">
        <label>
          Credential name
          <Input value={credentialName} onChange={(event) => setCredentialName(event.target.value)} placeholder="main" />
        </label>
        <label>
          API key
          <Input type="password" value={apiKey} onChange={(event) => setAPIKey(event.target.value)} placeholder={(selected.env ?? [])[0] || "API key"} autoComplete="off" />
        </label>
      </div>
      <Card className="info-panel">
        <strong>{providerName(selected)}</strong>
        <span>{selected.base_url || "Provider default endpoint"} - {selected.top_models?.length ?? selected.model_count ?? 0} catalog models</span>
      </Card>
      <div className="actions-row">
        <Button onClick={onBack}>Back</Button>
        <Button variant="primary" icon={<ArrowRight size={16} />} disabled={!apiKey.trim()} onClick={onNext}>Continue</Button>
      </div>
    </>
  );
}

function RoutingStep({
  selected,
  attachRoute,
  setAttachRoute,
  poolAlias,
  setPoolAlias,
  upstreamID,
  setUpstreamID,
  endpoint,
  setEndpoint,
  model,
  setModel,
  modelQuery,
  setModelQuery,
  visibleModels,
  totalModels,
  onBack,
  onSave,
  saving,
  canSave
}: {
  selected: CatalogProviderDTO;
  attachRoute: boolean;
  setAttachRoute: (value: boolean) => void;
  poolAlias: string;
  setPoolAlias: (value: string) => void;
  upstreamID: string;
  setUpstreamID: (value: string) => void;
  endpoint: string;
  setEndpoint: (value: string) => void;
  model: string;
  setModel: (value: string) => void;
  modelQuery: string;
  setModelQuery: (value: string) => void;
  visibleModels: CatalogModelDTO[];
  totalModels: number;
  onBack: () => void;
  onSave: () => void;
  saving: boolean;
  canSave: boolean;
}) {
  return (
    <>
      <div className="panel-heading">
        <div>
          <h2>Route binding</h2>
          <p>Keep the credential separate, but create the first pool binding now if you already know the model.</p>
        </div>
        <Link2 size={20} />
      </div>
      <label className="check-row">
        <input type="checkbox" checked={attachRoute} onChange={(event) => setAttachRoute(event.target.checked)} />
        Attach this credential to a pool now
      </label>
      <div className={attachRoute ? "routing-fields" : "routing-fields disabled"}>
        <label>
          Pool alias
          <Input value={poolAlias} onChange={(event) => setPoolAlias(event.target.value)} placeholder={safeAlias(selected.id)} disabled={!attachRoute} />
        </label>
        <label>
          Upstream ID
          <Input value={upstreamID} onChange={(event) => setUpstreamID(event.target.value)} disabled={!attachRoute} />
        </label>
        <label className="wide-field">
          Endpoint
          <Input value={endpoint} onChange={(event) => setEndpoint(event.target.value)} placeholder={selected.base_url || "Provider endpoint"} disabled={!attachRoute} />
        </label>
        <label className="wide-field model-field">
          Model
          <div className="input-icon compact">
            <Search size={16} />
            <Input value={modelQuery} onChange={(event) => setModelQuery(event.target.value)} placeholder={`Search ${totalModels.toLocaleString()} models`} disabled={!attachRoute} />
          </div>
          {visibleModels.length > 0 ? (
            <select className="input" value={model} onChange={(event) => setModel(event.target.value)} disabled={!attachRoute}>
              {model && !visibleModels.some((item) => item.id === model) ? <option value={model}>{model}</option> : null}
              {visibleModels.map((item) => (
                <option key={item.id} value={item.id}>
                  {item.id}
                  {item.context ? ` - ${item.context.toLocaleString()} ctx` : ""}
                </option>
              ))}
            </select>
          ) : (
            <Input value={model} onChange={(event) => setModel(event.target.value)} placeholder="Model id" disabled={!attachRoute} />
          )}
          {totalModels > visibleModels.length ? <span className="field-help">Showing {visibleModels.length.toLocaleString()} of {totalModels.toLocaleString()} matches.</span> : null}
        </label>
      </div>
      <div className="actions-row">
        <Button onClick={onBack}>Back</Button>
        <Button variant="primary" icon={<Save size={16} />} disabled={!canSave || saving} onClick={onSave}>
          {saving ? "Saving" : attachRoute ? "Save and attach" : "Store key only"}
        </Button>
      </div>
    </>
  );
}

function SummaryRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="summary-row">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function filterModels(models: CatalogModelDTO[], query: string) {
  const needle = query.trim().toLowerCase();
  return needle ? models.filter((model) => `${model.id} ${model.name ?? ""}`.toLowerCase().includes(needle)) : models;
}

function safeAlias(value: string) {
  return value.trim().replace(/[^a-zA-Z0-9_-]+/g, "-").replace(/^-+|-+$/g, "").toLowerCase() || "provider";
}
