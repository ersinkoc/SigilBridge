import { useEffect, useState, type FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Save } from "lucide-react";
import { toast } from "sonner";

import { ErrorState, Skeleton } from "../components/common/State";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { Input } from "../components/ui/Input";
import { api } from "../lib/api";
import type { CredentialsResponse, OAuthProviderDTO } from "../types/api";

type EditableProvider = {
  id: string;
  displayName: string;
  clientID: string;
  authURL: string;
  tokenURL: string;
  deviceAuthURL: string;
  revokeURL: string;
  defaultScopes: string;
};

export function SettingsOAuthProvidersRoute() {
  const queryClient = useQueryClient();
  const credentials = useQuery({ queryKey: ["credentials"], queryFn: () => api<CredentialsResponse>("/admin/v1/credentials") });
  const rawProviders = useQuery({ queryKey: ["oauth-providers-raw"], queryFn: () => api<{ path?: string; body: string }>("/admin/v1/credentials/oauth/providers") });
  const [raw, setRaw] = useState("");
  const [drafts, setDrafts] = useState<EditableProvider[]>([]);
  const providers = credentials.data?.oauth_providers ?? [];

  useEffect(() => {
    setDrafts(providers.map(editableFromProvider));
  }, [credentials.data?.oauth_providers]);

  useEffect(() => {
    if (rawProviders.data?.body) {
      setRaw(rawProviders.data.body);
    }
  }, [rawProviders.data?.body]);

  const save = useMutation({
    mutationFn: (body: string) =>
      api("/admin/v1/credentials/oauth/providers", {
        method: "PUT",
        body: JSON.stringify({ body })
      }),
    onSuccess: (_result, body) => {
      setRaw(body);
      toast.success("OAuth provider metadata saved");
      void queryClient.invalidateQueries({ queryKey: ["credentials"] });
      void queryClient.invalidateQueries({ queryKey: ["oauth-providers-raw"] });
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : "OAuth provider save failed")
  });

  function submitStructured(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    save.mutate(providersToYAML(drafts));
  }

  function submitRaw(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    save.mutate(raw);
  }

  function updateDraft(id: string, patch: Partial<EditableProvider>) {
    setDrafts((current) => current.map((item) => (item.id === id ? { ...item, ...patch } : item)));
  }

  return (
    <div className="page">
      <div className="page-intro">
        <h2>OAuth providers</h2>
        <p>Configure provider endpoints and OAuth client ids used by subscription login.</p>
      </div>
      {credentials.isLoading ? (
        <Skeleton />
      ) : credentials.isError ? (
        <ErrorState label={(credentials.error as Error).message} />
      ) : drafts.length === 0 ? (
        <Card className="empty-panel">
          <strong>No OAuth providers found</strong>
          <span>Create oauth_providers.yaml or restart the bridge after adding provider metadata.</span>
        </Card>
      ) : (
        <form className="form-grid" onSubmit={submitStructured}>
          <div className="oauth-provider-list">
            {drafts.map((provider) => {
              const missing = missingFields(provider);
              const ready = missing.length === 0;
              return (
                <Card className="oauth-provider-card" key={provider.id}>
                  <div className="panel-heading">
                    <div>
                      <h2>{provider.displayName || provider.id}</h2>
                      <p className="mono-cell">{provider.id}</p>
                    </div>
                    <span className={ready ? "status-pill ok" : "status-pill bad"}>{ready ? "Ready for login" : "Needs metadata"}</span>
                  </div>
                  <div className="oauth-status-row">
                    <span>Browser: {provider.authURL && provider.tokenURL ? "Ready" : "Not ready"}</span>
                    <span>Device: {provider.deviceAuthURL && provider.tokenURL ? "Ready" : "Not ready"}</span>
                    <span>{missing.length ? `Missing: ${missing.join(", ")}` : "All required fields configured"}</span>
                  </div>
                  <div className="oauth-provider-fields">
                    <label>
                      Display name
                      <Input value={provider.displayName} onChange={(event) => updateDraft(provider.id, { displayName: event.target.value })} />
                    </label>
                    <label>
                      OAuth client id
                      <Input value={provider.clientID} onChange={(event) => updateDraft(provider.id, { clientID: event.target.value })} />
                    </label>
                    <label>
                      Authorization URL
                      <Input value={provider.authURL} onChange={(event) => updateDraft(provider.id, { authURL: event.target.value })} />
                    </label>
                    <label>
                      Token URL
                      <Input value={provider.tokenURL} onChange={(event) => updateDraft(provider.id, { tokenURL: event.target.value })} />
                    </label>
                    <label>
                      Device code URL
                      <Input value={provider.deviceAuthURL} onChange={(event) => updateDraft(provider.id, { deviceAuthURL: event.target.value })} />
                    </label>
                    <label>
                      Revoke URL
                      <Input value={provider.revokeURL} onChange={(event) => updateDraft(provider.id, { revokeURL: event.target.value })} />
                    </label>
                    <label className="wide-field">
                      Default scopes
                      <textarea
                        className="input text-area scopes-editor"
                        value={provider.defaultScopes}
                        onChange={(event) => updateDraft(provider.id, { defaultScopes: event.target.value })}
                      />
                    </label>
                  </div>
                </Card>
              );
            })}
          </div>
          <div className="actions-row">
            <Button variant="primary" icon={<Save size={16} />} disabled={save.isPending}>
              Save provider registry
            </Button>
          </div>
        </form>
      )}
      {rawProviders.isLoading ? (
        <Skeleton />
      ) : rawProviders.isError ? (
        <ErrorState label={(rawProviders.error as Error).message} />
      ) : (
        <details className="advanced-panel">
          <summary>Advanced YAML editor</summary>
          <form className="form-grid" onSubmit={submitRaw}>
            <Card className="form-grid">
              <div className="panel-heading">
                <div>
                  <h2>oauth_providers.yaml</h2>
                  <p className="mono-cell">{rawProviders.data?.path}</p>
                </div>
              </div>
              <textarea className="input text-area raw-editor" value={raw} onChange={(event) => setRaw(event.target.value)} />
              <div className="actions-row">
                <Button icon={<Save size={16} />} disabled={save.isPending}>
                  Save YAML
                </Button>
              </div>
            </Card>
          </form>
        </details>
      )}
    </div>
  );
}

function editableFromProvider(provider: OAuthProviderDTO): EditableProvider {
  return {
    id: provider.id,
    displayName: provider.display_name ?? provider.id,
    clientID: provider.client_id ?? "",
    authURL: provider.auth_url ?? "",
    tokenURL: provider.token_url ?? "",
    deviceAuthURL: provider.device_auth_url ?? "",
    revokeURL: provider.revoke_url ?? "",
    defaultScopes: (provider.default_scopes ?? []).join("\n")
  };
}

function missingFields(provider: EditableProvider) {
  const missing = [];
  if (!provider.authURL.trim()) {
    missing.push("auth_url");
  }
  if (!provider.tokenURL.trim()) {
    missing.push("token_url");
  }
  if (!provider.clientID.trim()) {
    missing.push("client_id");
  }
  return missing;
}

function providersToYAML(providers: EditableProvider[]) {
  const lines = ["providers:"];
  for (const provider of providers) {
    lines.push(`  - id: ${yamlString(provider.id)}`);
    lines.push(`    display_name: ${yamlString(provider.displayName)}`);
    lines.push(`    client_id: ${yamlString(provider.clientID)}`);
    lines.push(`    auth_url: ${yamlString(provider.authURL)}`);
    lines.push(`    token_url: ${yamlString(provider.tokenURL)}`);
    lines.push(`    device_auth_url: ${yamlString(provider.deviceAuthURL)}`);
    lines.push(`    revoke_url: ${yamlString(provider.revokeURL)}`);
    const scopes = scopeLines(provider.defaultScopes);
    if (scopes.length === 0) {
      lines.push("    default_scopes: []");
    } else {
      lines.push("    default_scopes:");
      for (const scope of scopes) {
        lines.push(`      - ${yamlString(scope)}`);
      }
    }
  }
  return `${lines.join("\n")}\n`;
}

function scopeLines(value: string) {
  return value
    .split(/[\n,]/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function yamlString(value: string) {
  return JSON.stringify(value.trim());
}
