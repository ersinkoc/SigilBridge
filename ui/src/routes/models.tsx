import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { DatabaseZap, RefreshCw, Search } from "lucide-react";

import { ErrorState, Skeleton } from "../components/common/State";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { Input } from "../components/ui/Input";
import { api } from "../lib/api";
import type { CatalogProviderDTO, ProviderCatalogDTO } from "../types/api";

export function ModelsRoute() {
  const catalog = useQuery({ queryKey: ["provider-catalog"], queryFn: () => api<ProviderCatalogDTO>("/admin/v1/provider-catalog") });
  const [providerQuery, setProviderQuery] = useState("");
  const [modelQuery, setModelQuery] = useState("");
  const [category, setCategory] = useState("all");
  const providers = useMemo(() => filterProviders(catalog.data?.providers ?? [], providerQuery, category), [catalog.data?.providers, providerQuery, category]);
  const [selectedID, setSelectedID] = useState("");
  const selected = providers.find((provider) => provider.id === selectedID) ?? providers[0];
  const models = filterModels(selected, modelQuery);
  const visibleModels = models.slice(0, 80);
  const totalModels = (catalog.data?.providers ?? []).reduce((total, provider) => total + (provider.model_count ?? provider.top_models?.length ?? 0), 0);
  const syncedAt = catalog.dataUpdatedAt ? new Date(catalog.dataUpdatedAt).toLocaleTimeString() : "Not synced yet";

  return (
    <div className="page">
      <div className="page-title">
        <div className="page-intro">
          <h2>Model catalog</h2>
          <p>Inspect provider endpoints, compatibility adapters, and available model ids before wiring pools.</p>
        </div>
        <Button icon={<RefreshCw size={16} />} onClick={() => void catalog.refetch()} disabled={catalog.isFetching}>
          Sync catalog
        </Button>
      </div>
      {catalog.isLoading ? (
        <Skeleton />
      ) : catalog.isError ? (
        <ErrorState label={(catalog.error as Error).message} />
      ) : (
        <>
          <Card className="sync-status-panel">
            <div>
              <span>Source</span>
              <strong>{catalog.data?.source ?? "built-in"}</strong>
            </div>
            <div>
              <span>Last sync</span>
              <strong>{syncedAt}</strong>
            </div>
            <div>
              <span>Providers</span>
              <strong>{catalog.data?.providers?.length ?? 0}</strong>
            </div>
            <div>
              <span>Models</span>
              <strong>{totalModels.toLocaleString()}</strong>
            </div>
          </Card>
          <div className="model-catalog-layout">
            <Card className="provider-picker">
              <div className="panel-heading">
                <div>
                  <h2>Providers</h2>
                  <p>{catalog.data?.source ?? "built-in"}</p>
                </div>
                <DatabaseZap size={20} />
              </div>
              <div className="input-icon">
                <Search size={16} />
                <Input value={providerQuery} onChange={(event) => setProviderQuery(event.target.value)} placeholder="Search providers" />
              </div>
              <select className="input" value={category} onChange={(event) => setCategory(event.target.value)}>
                <option value="all">All provider types</option>
                <option value="api_key">API-key providers</option>
                <option value="cli_acp">CLI / ACP agents</option>
                <option value="other">Other providers</option>
              </select>
              <span className="field-help">{providers.length} providers shown</span>
              <div className="provider-pick-list">
                {providers.map((provider) => (
                  <button className={provider.id === selected?.id ? "provider-pick active" : "provider-pick"} key={provider.id} type="button" onClick={() => setSelectedID(provider.id)}>
                    <strong>{provider.name || provider.id}</strong>
                    <span>{provider.provider || provider.id}</span>
                    <em>{provider.category || "provider"} - {provider.model_count ?? provider.top_models?.length ?? 0} models</em>
                  </button>
                ))}
              </div>
            </Card>
            <Card className="model-catalog-panel">
              {selected ? (
                <>
                  <div className="panel-heading">
                    <div>
                      <h2>{selected.name || selected.id}</h2>
                      <p className="mono-cell">{selected.base_url || "Provider default endpoint"}</p>
                    </div>
                    <span className="status-pill">{selected.provider || selected.id}</span>
                  </div>
                  <div className="detail-panel">
                    <div>
                      <span>Compatible adapter</span>
                      <strong>{selected.provider || selected.id}</strong>
                    </div>
                    <div>
                      <span>Provider type</span>
                      <strong>{selected.category || "provider"}</strong>
                    </div>
                    <div>
                      <span>Environment</span>
                      <strong>{(selected.env ?? []).join(", ") || "-"}</strong>
                    </div>
                    <div>
                      <span>Models</span>
                      <strong>{models.length}</strong>
                    </div>
                  </div>
                  <div className="input-icon">
                    <Search size={16} />
                    <Input value={modelQuery} onChange={(event) => setModelQuery(event.target.value)} placeholder="Search models" />
                  </div>
                  <table className="table">
                    <thead>
                      <tr>
                        <th>Model ID</th>
                        <th>Name</th>
                        <th>Context</th>
                        <th>Output</th>
                        <th>Updated</th>
                      </tr>
                    </thead>
                    <tbody>
                      {visibleModels.map((model) => (
                        <tr key={model.id}>
                          <td className="mono-cell">{model.id}</td>
                          <td>{model.name || "-"}</td>
                          <td>{model.context ? model.context.toLocaleString() : "-"}</td>
                          <td>{model.output ? model.output.toLocaleString() : "-"}</td>
                          <td>{model.updated_at || "-"}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                  {models.length > visibleModels.length ? <span className="field-help">Showing {visibleModels.length.toLocaleString()} of {models.length.toLocaleString()} matching models. Use search to narrow the list.</span> : null}
                </>
              ) : (
                <div className="empty-panel">
                  <strong>No provider selected</strong>
                  <span>Sync the catalog or select a provider.</span>
                </div>
              )}
            </Card>
          </div>
        </>
      )}
    </div>
  );
}

function filterProviders(providers: CatalogProviderDTO[], query: string, category: string) {
  const needle = query.trim().toLowerCase();
  const filtered = providers.filter((provider) => {
    if (category === "api_key" && provider.category !== "api_key") {
      return false;
    }
    if (category === "cli_acp" && provider.category !== "cli_acp") {
      return false;
    }
    if (category === "other" && (provider.category === "api_key" || provider.category === "cli_acp")) {
      return false;
    }
    if (!needle) {
      return true;
    }
    return `${provider.id} ${provider.name ?? ""} ${provider.provider ?? ""} ${provider.base_url ?? ""} ${provider.category ?? ""}`.toLowerCase().includes(needle);
  });
  return [...filtered].sort((a, b) => (a.name ?? a.id).localeCompare(b.name ?? b.id));
}

function filterModels(provider: CatalogProviderDTO | undefined, query: string) {
  const models = provider?.top_models ?? [];
  const needle = query.trim().toLowerCase();
  return needle ? models.filter((model) => `${model.id} ${model.name ?? ""}`.toLowerCase().includes(needle)) : models;
}
