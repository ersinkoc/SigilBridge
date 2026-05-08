import { Link } from "react-router-dom";
import { KeyRound, ShieldCheck } from "lucide-react";

import type { KeyDTO } from "../../types/api";
import { EmptyState } from "../common/State";
import { Button } from "../ui/Button";
import { Card } from "../ui/Card";

export function KeyTable({ keys }: { keys: KeyDTO[] }) {
  if (keys.length === 0) {
    return <EmptyState />;
  }
  return (
    <div className="bridge-key-grid">
      {keys.map((key) => {
        const allowedPools = listValue(key.scopes?.allowed_pools);
        const allowedModels = listValue(key.scopes?.allowed_models);
        const ipAllowlist = listValue(key.scopes?.ip_allowlist);
        return (
          <Card className="bridge-key-card" key={key.id}>
            <div className="credential-card-header">
              <div>
                <strong>{key.name || key.id}</strong>
                <span>{key.id}</span>
              </div>
              <span className={key.revoked_at ? "status-pill bad" : "status-pill ok"}>{key.revoked_at ? "Revoked" : "Active"}</span>
            </div>
            <div className="credential-id-line">
              <KeyRound size={15} />
              <code>{key.hash || "Hash unavailable"}</code>
            </div>
            <div className="key-scope-grid">
              <ScopeCell label="Pools" values={allowedPools} empty="All pools" />
              <ScopeCell label="Models" values={allowedModels} empty="All models" />
              <ScopeCell label="IP allowlist" values={ipAllowlist} empty="Any IP" />
            </div>
            <div className="key-timing-grid">
              <div>
                <span>Created</span>
                <strong>{formatDate(key.created_at) || "-"}</strong>
              </div>
              <div>
                <span>Last used</span>
                <strong>{formatDate(key.last_used_at) || "Never"}</strong>
              </div>
            </div>
            <div className="credential-card-footer">
              <span>Budgets, rate limits, and scopes are edited in key detail.</span>
              <Link to={`/keys/${key.id}`}>
                <Button icon={<ShieldCheck size={14} />}>Manage</Button>
              </Link>
            </div>
          </Card>
        );
      })}
    </div>
  );
}

function ScopeCell({ label, values, empty }: { label: string; values: string[]; empty: string }) {
  return (
    <div className="key-scope-cell">
      <span>{label}</span>
      <strong>{values.length > 0 ? values.join(", ") : empty}</strong>
    </div>
  );
}

function formatDate(value?: string) {
  if (!value) {
    return "";
  }
  return new Date(value).toLocaleString();
}

function listValue(value: unknown) {
  return Array.isArray(value) ? value.map((item) => String(item)).filter(Boolean) : [];
}
