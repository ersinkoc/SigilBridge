import { ArrowDown, ArrowUp, KeyRound, Plus, Search, Trash2 } from "lucide-react";
import { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";

import { api } from "../../lib/api";
import type { CatalogModelDTO, CatalogProviderDTO, CredentialsResponse, PoolDTO, ProviderCatalogDTO } from "../../types/api";
import { Button } from "../ui/Button";
import { Card } from "../ui/Card";
import { Input } from "../ui/Input";
import { WeightSlider } from "./WeightSlider";

type Upstream = Record<string, unknown>;

type CatalogChoice = CatalogProviderDTO & {
  label: string;
  compatibility: string;
};

const strategyOptions = [
  { value: "priority_first", label: "Priority failover" },
  { value: "weighted_round_robin", label: "Weighted round robin" },
  { value: "weighted_random", label: "Weighted random" },
  { value: "round_robin", label: "Round robin" },
  { value: "least_inflight", label: "Least in-flight" },
  { value: "lowest_latency", label: "Lowest latency" },
  { value: "random", label: "Random" }
];

export function PoolEditor({
  pool,
  onChange,
  onSave,
  onDelete,
  saving = false,
  deleting = false
}: {
  pool: PoolDTO;
  onChange: (pool: PoolDTO) => void;
  onSave?: () => void;
  onDelete?: () => void;
  saving?: boolean;
  deleting?: boolean;
}) {
  const catalog = useQuery({ queryKey: ["provider-catalog"], queryFn: () => api<ProviderCatalogDTO>("/admin/v1/provider-catalog") });
  const credentials = useQuery({ queryKey: ["credentials"], queryFn: () => api<CredentialsResponse>("/admin/v1/credentials") });
  const upstreams = pool.upstreams ?? [];
  const choices = useMemo(() => catalogChoices(catalog.data?.providers ?? []), [catalog.data?.providers]);
  const [providerQuery, setProviderQuery] = useState("");
  const [providerType, setProviderType] = useState("all");
  const visibleChoices = useMemo(() => filterChoices(choices, providerQuery, providerType), [choices, providerQuery, providerType]);
  const apiKeys = credentials.data?.api_keys ?? [];
  const readiness = poolReadiness(upstreams, choices, apiKeys);

  function updateUpstream(index: number, patch: Upstream) {
    onChange({ ...pool, upstreams: upstreams.map((upstream, current) => (current === index ? { ...upstream, ...patch } : upstream)) });
  }

  function addUpstream() {
    const first = choices.find((choice) => choice.category === "api_key") ?? choices[0];
    const next: Upstream = {
      id: first ? defaultUpstreamID(first, upstreams.length) : `upstream-${upstreams.length + 1}`,
      catalog_id: first?.id ?? "",
      provider: first?.compatibility ?? "openai_api",
      priority: upstreams.length + 1,
      weight: 100
    };
    if (first?.base_url) {
      next.base_url = first.base_url;
    }
    if (first?.top_models?.[0]?.id) {
      next.model = first.top_models[0].id;
    }
    onChange({ ...pool, upstreams: [...upstreams, next] });
  }

  function removeUpstream(index: number) {
    onChange({ ...pool, upstreams: upstreams.filter((_, current) => current !== index).map((upstream, current) => ({ ...upstream, priority: current + 1 })) });
  }

  function moveUpstream(index: number, direction: -1 | 1) {
    const target = index + direction;
    if (target < 0 || target >= upstreams.length) {
      return;
    }
    const next = [...upstreams];
    [next[index], next[target]] = [next[target], next[index]];
    onChange({ ...pool, upstreams: next.map((upstream, current) => ({ ...upstream, priority: current + 1 })) });
  }

  return (
    <Card className="pool-editor">
      <div className="pool-editor-header">
        <label>
          Pool alias
          <Input value={pool.id} onChange={(event) => onChange({ ...pool, id: event.target.value })} placeholder="coding" />
        </label>
        <label>
          Strategy
          <select className="input" value={pool.strategy ?? "priority_first"} onChange={(event) => onChange({ ...pool, strategy: event.target.value })}>
            {strategyOptions.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        </label>
      </div>
      <div className="pipeline-strip">
        <PipelineStep label="Credentials" value={`${apiKeys.length} stored`} ok={apiKeys.length > 0 || readiness.needsCredential === 0} />
        <PipelineStep label="Models" value={`${catalog.data?.providers?.length ?? 0} providers`} ok={(catalog.data?.providers?.length ?? 0) > 0} />
        <PipelineStep label="Upstreams" value={`${readiness.ready}/${upstreams.length} ready`} ok={upstreams.length > 0 && readiness.incomplete === 0} />
        <PipelineStep label="Probe" value="Run after save" ok={false} muted />
      </div>
      <div className="route-builder-toolbar">
        <label>
          Provider filter
          <div className="input-icon">
            <Search size={16} />
            <Input value={providerQuery} onChange={(event) => setProviderQuery(event.target.value)} placeholder="OpenAI, Claude, Kimi, local..." />
          </div>
        </label>
        <label>
          Source
          <select className="input" value={providerType} onChange={(event) => setProviderType(event.target.value)}>
            <option value="all">All provider types</option>
            <option value="api_key">API-key providers</option>
            <option value="cli_acp">CLI / ACP agents</option>
            <option value="other">Other providers</option>
          </select>
        </label>
        <div className="route-builder-stats">
          <strong>{visibleChoices.length}</strong>
          <span>catalog choices shown</span>
        </div>
      </div>
      <div className="panel-heading">
        <div>
          <h2>Upstream routing</h2>
          <p>{upstreams.length} providers are attached to this model alias.</p>
        </div>
        <Button type="button" icon={<Plus size={16} />} onClick={addUpstream}>
          Add upstream
        </Button>
      </div>
      <div className="upstream-list">
        {upstreams.length === 0 ? (
          <Card className="empty-panel">
            <strong>No upstreams attached</strong>
            <span>Add a provider from the catalog to route this pool.</span>
          </Card>
        ) : null}
        {upstreams.map((upstream, index) => {
          const selectedChoice = matchChoice(choices, upstream);
          const modelOptions = selectedChoice?.top_models ?? [];
          const compatibleCredentials = apiKeys.filter((credential) => !upstream.provider || credential.provider === String(upstream.provider));
          const selectedCredential = apiKeys.find((credential) => credential.id === credentialValue(upstream));
          const issues = upstreamIssues(upstream, selectedChoice, compatibleCredentials.length);
          const selectableChoices = selectedChoice && !visibleChoices.some((choice) => choice.id === selectedChoice.id) ? [selectedChoice, ...visibleChoices] : visibleChoices;
          return (
            <div className="upstream-card" key={`${String(upstream.id ?? "upstream")}-${index}`}>
              <div className="upstream-card-main">
                <div className="upstream-card-title">
                  <strong>{String(upstream.id ?? `upstream-${index + 1}`)}</strong>
                  <span className="mono-cell">{String(upstream.provider ?? "")}</span>
                  <span className={issues.length === 0 ? "status-pill ok" : "status-pill bad"}>{issues.length === 0 ? "Ready" : `Missing ${issues.join(", ")}`}</span>
                </div>
                <div className="upstream-card-grid">
                  <label>
                    Provider catalog
                    <select className="input" value={selectedChoice?.id ?? ""} onChange={(event) => applyCatalogChoice(index, choices.find((choice) => choice.id === event.target.value), upstream, updateUpstream)}>
                      <option value="">Custom provider</option>
                      {selectableChoices.map((choice) => (
                        <option key={choice.id} value={choice.id}>
                          {choice.label}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label>
                    Compatible adapter
                    <select className="input" value={String(upstream.provider ?? "")} onChange={(event) => updateUpstream(index, { provider: event.target.value })}>
                      {adapterOptions(choices, String(upstream.provider ?? "")).map((adapter) => (
                        <option key={adapter} value={adapter}>
                          {adapter}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label>
                    Endpoint
                    <Input value={String(upstream.base_url ?? "")} onChange={(event) => updateUpstream(index, { base_url: event.target.value })} placeholder="Provider endpoint" />
                  </label>
                  <ModelField value={String(upstream.model ?? "")} models={modelOptions} optional={usesLocalModelDefault(upstream, selectedChoice)} onChange={(value) => updateUpstream(index, { model: value })} />
                  <label>
                    Credential
                    <select className="input" value={String(upstream.api_key_ref ?? upstream.credential ?? upstream.credential_id ?? "")} onChange={(event) => updateCredentialRef(index, event.target.value, updateUpstream)}>
                      <option value="">No vault credential selected</option>
                      {compatibleCredentials.map((credential) => (
                        <option key={credential.id} value={credential.id}>
                          {credential.provider} / {credential.id}
                        </option>
                      ))}
                    </select>
                    <CredentialHelper
                      upstream={upstream}
                      choice={selectedChoice}
                      compatibleCredentials={compatibleCredentials}
                      selectedCredential={selectedCredential}
                      applyFirst={() => {
                        const first = compatibleCredentials[0];
                        if (first) {
                          updateCredentialRef(index, first.id, updateUpstream);
                        }
                      }}
                    />
                  </label>
                  <label>
                    Upstream ID
                    <Input value={String(upstream.id ?? "")} onChange={(event) => updateUpstream(index, { id: event.target.value })} />
                  </label>
                  <label>
                    Priority
                    <Input type="number" min="1" value={Number(upstream.priority ?? index + 1)} onChange={(event) => updateUpstream(index, { priority: Number(event.target.value) })} />
                  </label>
                  <label>
                    Weight {Number(upstream.weight ?? 100)}
                    <WeightSlider value={Number(upstream.weight ?? 100)} onChange={(value) => updateUpstream(index, { weight: value })} />
                  </label>
                </div>
                <RoutePreview upstream={upstream} choice={selectedChoice} issues={issues} credential={selectedCredential} />
              </div>
              <div className="upstream-actions">
                <Button type="button" icon={<ArrowUp size={16} />} onClick={() => moveUpstream(index, -1)} disabled={index === 0} />
                <Button type="button" icon={<ArrowDown size={16} />} onClick={() => moveUpstream(index, 1)} disabled={index === upstreams.length - 1} />
                <Button type="button" icon={<Trash2 size={16} />} onClick={() => removeUpstream(index)} />
              </div>
            </div>
          );
        })}
      </div>
      <div className="actions-row">
        <Button variant="primary" onClick={onSave} disabled={saving}>
          {saving ? "Saving" : "Save pool"}
        </Button>
        {onDelete ? (
          <Button onClick={onDelete} disabled={deleting}>
            {deleting ? "Deleting" : "Delete"}
          </Button>
        ) : null}
      </div>
    </Card>
  );
}

function CredentialHelper({
  upstream,
  choice,
  compatibleCredentials,
  selectedCredential,
  applyFirst
}: {
  upstream: Upstream;
  choice?: CatalogChoice;
  compatibleCredentials: NonNullable<CredentialsResponse["api_keys"]>;
  selectedCredential?: NonNullable<CredentialsResponse["api_keys"]>[number];
  applyFirst: () => void;
}) {
  if (!requiresCredential(upstream, choice)) {
    return <span className="field-help">This adapter uses local auth or does not require a stored API key.</span>;
  }
  if (selectedCredential) {
    return <span className="field-help">Using {credentialShortName(selectedCredential.id)}</span>;
  }
  if (compatibleCredentials.length > 0) {
    return (
      <span className="field-help action-help">
        {compatibleCredentials.length} matching credentials available.
        <button type="button" onClick={applyFirst}>Use first</button>
      </span>
    );
  }
  return (
    <span className="field-help action-help">
      No matching API key stored for this adapter.
      <Link to="/credentials/api-key/new">
        <KeyRound size={13} /> Add key
      </Link>
    </span>
  );
}

function RoutePreview({
  upstream,
  choice,
  issues,
  credential
}: {
  upstream: Upstream;
  choice?: CatalogChoice;
  issues: string[];
  credential?: NonNullable<CredentialsResponse["api_keys"]>[number];
}) {
  const model = String(upstream.model ?? "").trim() || (usesLocalModelDefault(upstream, choice) ? "CLI default" : "not selected");
  const rows = [
    ["Adapter", String(upstream.provider ?? choice?.compatibility ?? "custom")],
    ["Endpoint", String(upstream.base_url ?? choice?.base_url ?? "provider default")],
    ["Model", model],
    ["Credential", credential ? credentialShortName(credential.id) : requiresCredential(upstream, choice) ? "not selected" : "local/session auth"]
  ];
  return (
    <div className="route-preview">
      <div className="route-preview-head">
        <span className={issues.length === 0 ? "status-pill ok" : "status-pill bad"}>{issues.length === 0 ? "Route ready" : `Needs ${issues.join(", ")}`}</span>
        <strong>{choice?.name || choice?.id || "Custom route"}</strong>
      </div>
      <div className="route-preview-grid">
        {rows.map(([label, value]) => (
          <div key={label}>
            <span>{label}</span>
            <strong title={value}>{value}</strong>
          </div>
        ))}
      </div>
    </div>
  );
}

function catalogChoices(providers: CatalogProviderDTO[]): CatalogChoice[] {
  return providers
    .map((provider) => ({
      ...provider,
      label: `${provider.name || provider.id} (${provider.provider || provider.id})`,
      compatibility: provider.provider || provider.id
    }))
    .sort((a, b) => a.label.localeCompare(b.label));
}

function filterChoices(choices: CatalogChoice[], query: string, providerType: string) {
  const needle = query.trim().toLowerCase();
  return choices.filter((choice) => {
    if (providerType === "api_key" && choice.category !== "api_key") {
      return false;
    }
    if (providerType === "cli_acp" && choice.category !== "cli_acp") {
      return false;
    }
    if (providerType === "other" && (choice.category === "api_key" || choice.category === "cli_acp")) {
      return false;
    }
    if (!needle) {
      return true;
    }
    return `${choice.id} ${choice.name ?? ""} ${choice.provider ?? ""} ${choice.compatibility} ${choice.base_url ?? ""} ${choice.category ?? ""}`.toLowerCase().includes(needle);
  });
}

function matchChoice(choices: CatalogChoice[], upstream: Upstream) {
  const catalogID = String(upstream.catalog_id ?? "");
  if (catalogID) {
    return choices.find((choice) => choice.id === catalogID);
  }
  const provider = String(upstream.provider ?? "");
  const baseURL = String(upstream.base_url ?? "");
  const model = String(upstream.model ?? "");
  return choices.find((choice) => choice.compatibility === provider && ((choice.base_url || "") === baseURL || choice.top_models?.some((item) => item.id === model)));
}

function applyCatalogChoice(index: number, choice: CatalogChoice | undefined, upstream: Upstream, update: (index: number, patch: Upstream) => void) {
  if (!choice) {
    update(index, { catalog_id: "" });
    return;
  }
  const currentID = String(upstream.id ?? "").trim();
  const previousProvider = String(upstream.provider ?? "").trim();
  const generatedID = !currentID || (Boolean(previousProvider) && currentID.startsWith(`${previousProvider}-`));
  update(index, {
    catalog_id: choice.id,
    provider: choice.compatibility,
    base_url: choice.base_url ?? "",
    model: choice.top_models?.[0]?.id ?? (usesLocalModelDefault(upstream, choice) ? "" : String(upstream.model ?? "")),
    id: generatedID ? defaultUpstreamID(choice, index) : currentID
  });
}

function adapterOptions(choices: CatalogChoice[], current: string) {
  const adapters = new Set(choices.map((choice) => choice.compatibility).filter(Boolean));
  if (current) {
    adapters.add(current);
  }
  for (const fallback of ["openai_api", "anthropic_api", "gemini_api", "groq", "mistral_api", "deepseek_api", "ollama", "claude_code_cli", "codex_cli", "gemini_cli"]) {
    adapters.add(fallback);
  }
  return [...adapters].sort();
}

function updateCredentialRef(index: number, value: string, update: (index: number, patch: Upstream) => void) {
  update(index, { api_key_ref: value, credential: value, credential_id: value, vault_id: value });
}

function defaultUpstreamID(choice: CatalogChoice, index: number) {
  return `${choice.compatibility}-${choice.id}`.replace(/[^a-zA-Z0-9_-]+/g, "-").toLowerCase() || `upstream-${index + 1}`;
}

function ModelField({ value, models, optional = false, onChange }: { value: string; models: CatalogModelDTO[]; optional?: boolean; onChange: (value: string) => void }) {
  const [query, setQuery] = useState("");
  if (models.length === 0) {
    return (
      <label>
        Model
        <Input value={value} onChange={(event) => onChange(event.target.value)} placeholder={optional ? "Optional model override" : "Model id"} />
        {optional ? <span className="field-help">Leave blank to use the CLI agent's configured default model.</span> : null}
      </label>
    );
  }
  const needle = query.trim().toLowerCase();
  const filtered = needle ? models.filter((item) => `${item.id} ${item.name ?? ""}`.toLowerCase().includes(needle)) : models;
  const visible = filtered.slice(0, 200);
  const selectedIsVisible = visible.some((item) => item.id === value);
  const options = value && !selectedIsVisible ? [{ id: value, name: "Selected model" } as CatalogModelDTO, ...visible] : visible;

  return (
    <label className="model-field">
      Model
      {models.length > 24 ? (
        <div className="input-icon compact">
          <Search size={16} />
          <Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder={`Search ${models.length.toLocaleString()} models`} />
        </div>
      ) : null}
      <select className="input" value={value} onChange={(event) => onChange(event.target.value)}>
        {options.map((item) => (
          <option key={item.id} value={item.id}>
            {item.id}
            {item.context ? ` - ${item.context.toLocaleString()} ctx` : ""}
          </option>
        ))}
      </select>
      {models.length > 200 ? <span className="field-help">Showing {options.length.toLocaleString()} of {filtered.length.toLocaleString()} matches.</span> : null}
    </label>
  );
}

function PipelineStep({ label, value, ok, muted = false }: { label: string; value: string; ok: boolean; muted?: boolean }) {
  return (
    <div className={muted ? "pipeline-step muted" : "pipeline-step"}>
      <span className={ok ? "status-dot" : "status-dot bad"} />
      <div>
        <strong>{label}</strong>
        <span>{value}</span>
      </div>
    </div>
  );
}

function poolReadiness(upstreams: Upstream[], choices: CatalogChoice[], apiKeys: NonNullable<CredentialsResponse["api_keys"]>) {
  let ready = 0;
  let needsCredential = 0;
  for (const upstream of upstreams) {
    const choice = matchChoice(choices, upstream);
    const compatibleCredentials = apiKeys.filter((credential) => !upstream.provider || credential.provider === String(upstream.provider));
    const issues = upstreamIssues(upstream, choice, compatibleCredentials.length);
    if (requiresCredential(upstream, choice)) {
      needsCredential++;
    }
    if (issues.length === 0) {
      ready++;
    }
  }
  return { ready, incomplete: upstreams.length - ready, needsCredential };
}

function upstreamIssues(upstream: Upstream, choice: CatalogChoice | undefined, compatibleCredentialCount: number) {
  const issues: string[] = [];
  if (!String(upstream.provider ?? "").trim()) {
    issues.push("adapter");
  }
  if (!usesLocalModelDefault(upstream, choice) && !String(upstream.model ?? "").trim()) {
    issues.push("model");
  }
  if (requiresCredential(upstream, choice) && !credentialValue(upstream)) {
    issues.push(compatibleCredentialCount > 0 ? "credential" : "API key");
  }
  return issues;
}

function credentialValue(upstream: Upstream) {
  return String(upstream.api_key_ref ?? upstream.credential ?? upstream.credential_id ?? upstream.vault_id ?? "").trim();
}

function credentialShortName(id: string) {
  const parts = id.split("/");
  return parts[parts.length - 1] || id;
}

function requiresCredential(upstream: Upstream, choice?: CatalogChoice) {
  const provider = String(upstream.provider ?? "").toLowerCase();
  if (choice?.category === "api_key") {
    return true;
  }
  if (!provider || provider === "ollama" || provider.includes("_cli") || provider.includes("cli_") || provider.includes("acp")) {
    return false;
  }
  return true;
}

function usesLocalModelDefault(upstream: Upstream, choice?: CatalogChoice) {
  const provider = String(upstream.provider ?? choice?.compatibility ?? "").toLowerCase();
  const category = String(choice?.category ?? upstream.category ?? "").toLowerCase();
  return category === "cli_acp" || provider.includes("_cli") || provider.includes("cli_") || provider.includes("acp");
}
