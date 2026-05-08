import { Link } from "react-router-dom";
import { Plus } from "lucide-react";
import { useQuery } from "@tanstack/react-query";

import { KeyTable } from "../components/keys/KeyTable";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { ErrorState, Skeleton } from "../components/common/State";
import { api } from "../lib/api";
import type { KeyDTO } from "../types/api";

export function KeysRoute() {
  const keys = useQuery({ queryKey: ["keys"], queryFn: () => api<KeyDTO[]>("/admin/v1/keys") });
  const activeKeys = keys.data?.filter((key) => !key.revoked_at).length ?? 0;
  const revokedKeys = keys.data?.filter((key) => key.revoked_at).length ?? 0;
  const scopedPools = new Set((keys.data ?? []).flatMap((key) => listScope(key.scopes?.allowed_pools))).size;

  return (
    <div className="page">
      <div className="page-title">
        <div className="page-intro">
          <h2>Bridge keys</h2>
          <p>Issue scoped credentials for clients that call the OpenAI-compatible endpoint.</p>
        </div>
        <Link to="/keys/new">
          <Button variant="primary" icon={<Plus size={16} />}>
            Create key
          </Button>
        </Link>
      </div>
      <div className="summary-strip">
        <Card className="summary-item">
          <span>Active keys</span>
          <strong>{activeKeys}</strong>
        </Card>
        <Card className="summary-item">
          <span>Scoped pools</span>
          <strong>{scopedPools}</strong>
        </Card>
        <Card className="summary-item">
          <span>Revoked keys</span>
          <strong>{revokedKeys}</strong>
        </Card>
      </div>
      {keys.isLoading ? <Skeleton /> : keys.isError ? <ErrorState label={(keys.error as Error).message} /> : <KeyTable keys={keys.data ?? []} />}
    </div>
  );
}

function listScope(value: unknown) {
  return Array.isArray(value) ? value.map((item) => String(item)).filter(Boolean) : [];
}
