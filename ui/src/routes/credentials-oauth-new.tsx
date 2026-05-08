import { useMutation, useQuery } from "@tanstack/react-query";
import { Bot, ExternalLink, Globe2, KeyRound, PlugZap, ShieldAlert, Terminal } from "lucide-react";
import { Link } from "react-router-dom";
import { useState } from "react";
import { toast } from "sonner";

import { ErrorState, Skeleton } from "../components/common/State";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { Input } from "../components/ui/Input";
import { api } from "../lib/api";
import type { CredentialsResponse, OAuthProviderDTO } from "../types/api";

const subscriptions = [
  {
    id: "claude_oauth",
    name: "Claude",
    bestPath: "Claude API key or Claude Code CLI",
    detail: "Claude account OAuth requires operator-supplied metadata. Claude Code CLI is usually the practical subscription path on a logged-in workstation."
  },
  {
    id: "copilot_oauth",
    name: "GitHub Copilot",
    bestPath: "GitHub OAuth metadata",
    detail: "Copilot subscription access depends on GitHub OAuth/device metadata owned by the operator."
  },
  {
    id: "gemini_oauth",
    name: "Gemini",
    bestPath: "Gemini API key or Gemini CLI",
    detail: "Use an API key for Gemini API. Use Gemini CLI when the local subscription login is the capacity source."
  },
  {
    id: "cursor_oauth",
    name: "Cursor",
    bestPath: "Cursor CLI/session fallback",
    detail: "Cursor account OAuth metadata is not stable here unless the operator provides real endpoints and client id."
  }
];

function providerByID(providers: OAuthProviderDTO[], id: string) {
  return providers.find((item) => item.id === id);
}

function providerStatus(provider: OAuthProviderDTO | undefined) {
  if (!provider) {
    return { label: "No template", className: "status-pill bad", canLogin: false };
  }
  if (provider.usable) {
    return { label: "Ready", className: "status-pill ok", canLogin: true };
  }
  if (provider.metadata_configured) {
    return { label: provider.configured_client ? "OAuth incomplete" : "Client ID missing", className: "status-pill bad", canLogin: false };
  }
  return { label: "Provider template only", className: "status-pill bad", canLogin: false };
}

export function CredentialsOAuthNewRoute() {
  const credentials = useQuery({ queryKey: ["credentials"], queryFn: () => api<CredentialsResponse>("/admin/v1/credentials") });
  const [names, setNames] = useState<Record<string, string>>({});
  const [authURL, setAuthURL] = useState("");
  const [redirectURI, setRedirectURI] = useState("");
  const bootstrap = useMutation({
    mutationFn: ({ provider, name }: { provider: string; name: string }) =>
      api<{ auth_url: string; redirect_uri?: string; vault_id?: string }>("/admin/v1/credentials/oauth/bootstrap", {
        method: "POST",
        body: JSON.stringify({ provider, name, mode: "browser" })
      }),
    onSuccess: (result) => {
      setAuthURL(result.auth_url);
      setRedirectURI(result.redirect_uri ?? "");
      toast.success("OAuth login started");
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : "OAuth login failed")
  });
  const providers = credentials.data?.oauth_providers ?? [];
  const readyCount = providers.filter((provider) => provider.usable).length;
  const metadataCount = providers.filter((provider) => provider.metadata_configured).length;

  return (
    <div className="page">
      <div className="page-intro">
        <h2>Advanced OAuth</h2>
        <p>Use only when a provider has usable operator-supplied OAuth metadata. Most API-first LLM providers should be added with API keys instead.</p>
      </div>
      {credentials.isLoading ? (
        <Skeleton />
      ) : credentials.isError ? (
        <ErrorState label={(credentials.error as Error).message} />
      ) : (
        <>
          <Card className="oauth-capability-panel">
            <div className="panel-heading">
              <div>
                <h2>Capability check</h2>
                <p>OAuth is only enabled for providers with real endpoints and a real client id. Unsupported subscription access belongs in CLI agents or session fallback.</p>
              </div>
              <ShieldAlert size={20} />
            </div>
            <div className="summary-strip mini">
              <div>
                <span>Templates</span>
                <strong>{providers.length}</strong>
              </div>
              <div>
                <span>Metadata present</span>
                <strong>{metadataCount}</strong>
              </div>
              <div>
                <span>Login ready</span>
                <strong>{readyCount}</strong>
              </div>
            </div>
            <div className="oauth-path-grid">
              <Link to="/credentials/api-key/new">
                <span><KeyRound size={16} /> API keys</span>
                <strong>Default for API-first providers</strong>
              </Link>
              <Link to="/credentials/cli">
                <span><Terminal size={16} /> CLI agents</span>
                <strong>Best for logged-in subscriptions</strong>
              </Link>
              <Link to="/credentials/sessions/new">
                <span><PlugZap size={16} /> Session fallback</span>
                <strong>Last resort only</strong>
              </Link>
            </div>
          </Card>
          <div className="setup-grid">
            {subscriptions.map((item) => {
              const configured = providerByID(providers, item.id);
              const status = providerStatus(configured);
              const name = names[item.id] ?? "main";
              return (
                <Card className="setup-card" key={item.id}>
                  <div className="panel-heading">
                    <div>
                      <h2>{item.name}</h2>
                      <p>{item.detail}</p>
                    </div>
                    <Globe2 size={20} />
                  </div>
                  <div className="oauth-status-stack">
                    <span className={status.className}>{status.label}</span>
                    <span className="status-pill">{item.bestPath}</span>
                  </div>
                  {configured?.missing_fields?.length ? <span className="muted">Missing: {configured.missing_fields.join(", ")}</span> : null}
                  <label>
                    Credential name
                    <Input value={name} onChange={(event) => setNames((current) => ({ ...current, [item.id]: event.target.value }))} />
                  </label>
                  <div className="actions-row">
                    <Button
                      variant="primary"
                      icon={<Globe2 size={16} />}
                      disabled={!status.canLogin || bootstrap.isPending}
                      onClick={() => bootstrap.mutate({ provider: item.id, name })}
                    >
                      Login
                    </Button>
                    {!status.canLogin ? (
                      <Link to="/settings/oauth-providers">
                        <Button>Configure metadata</Button>
                      </Link>
                    ) : null}
                  </div>
                </Card>
              );
            })}
            <Card className="setup-card">
              <div className="panel-heading">
                <div>
                  <h2>ChatGPT</h2>
                  <p>Use Codex CLI when installed and authenticated; ChatGPT browser cookies remain a legacy fallback.</p>
                </div>
                <Bot size={20} />
              </div>
              <div className="actions-row">
                <Link to="/credentials/cli">
                  <Button icon={<Bot size={16} />}>Use Codex CLI</Button>
                </Link>
                <Link to="/credentials/sessions/new">
                  <Button icon={<PlugZap size={16} />}>Import browser session</Button>
                </Link>
              </div>
            </Card>
          </div>
        </>
      )}
      {authURL ? (
        <Card className="secret-reveal">
          <strong>Authorization URL</strong>
          <code>{authURL}</code>
          {redirectURI ? (
            <>
              <strong>Redirect URI</strong>
              <code>{redirectURI}</code>
            </>
          ) : null}
          <a className="btn btn-primary" href={authURL} target="_blank" rel="noreferrer">
            <ExternalLink size={16} /> Open login
          </a>
        </Card>
      ) : null}
    </div>
  );
}
