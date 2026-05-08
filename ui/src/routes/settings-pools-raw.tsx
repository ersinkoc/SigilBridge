import { useEffect, useState, type FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Save } from "lucide-react";
import { toast } from "sonner";

import { ConfirmDialog } from "../components/common/ConfirmDialog";
import { ErrorState, Skeleton } from "../components/common/State";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { api } from "../lib/api";
import type { PoolDTO } from "../types/api";

type PoolSavePlan = {
  next: PoolDTO[];
  createCount: number;
  updateCount: number;
  deleteIDs: string[];
};

export function SettingsPoolsRawRoute() {
  const queryClient = useQueryClient();
  const pools = useQuery({ queryKey: ["pools"], queryFn: () => api<PoolDTO[]>("/admin/v1/pools") });
  const [raw, setRaw] = useState("[]");
  const [pendingPlan, setPendingPlan] = useState<PoolSavePlan | null>(null);

  useEffect(() => {
    if (pools.data) {
      setRaw(JSON.stringify(pools.data, null, 2));
    }
  }, [pools.data]);

  const savePools = useMutation({
    mutationFn: async (plan: PoolSavePlan) => {
      const next = plan.next;
      const existing = pools.data ?? [];
      const nextIDs = new Set(next.map((pool) => pool.id));
      for (const pool of next) {
        await api<PoolDTO>("/admin/v1/pools", { method: "POST", body: JSON.stringify(pool) });
      }
      for (const pool of existing) {
        if (!nextIDs.has(pool.id)) {
          await api(`/admin/v1/pools/${pool.id}`, { method: "DELETE" });
        }
      }
      return next;
    },
    onSuccess: (saved) => {
      queryClient.setQueryData(["pools"], saved);
      void queryClient.invalidateQueries({ queryKey: ["pools"] });
      setPendingPlan(null);
      toast.success("Pools saved");
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : "Pool save failed")
  });

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    try {
      setPendingPlan(buildPoolSavePlan(raw, pools.data ?? []));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Raw pools are invalid");
    }
  }

  return (
    <div className="page">
      <div className="page-intro">
        <h2>Raw pools</h2>
        <p>Edit the routing pools payload used by the local control plane.</p>
      </div>
      {pools.isLoading ? (
        <Skeleton />
      ) : pools.isError ? (
        <ErrorState label={(pools.error as Error).message} />
      ) : (
        <form className="form-grid" onSubmit={submit}>
          <Card className="form-grid">
            <textarea className="input text-area raw-editor" value={raw} onChange={(event) => setRaw(event.target.value)} />
            <div className="actions-row">
              <Button variant="primary" icon={<Save size={16} />} disabled={savePools.isPending}>
                Save pools
              </Button>
            </div>
          </Card>
        </form>
      )}
      <ConfirmDialog
        open={Boolean(pendingPlan)}
        title="Save raw pools"
        description={describePlan(pendingPlan)}
        confirmLabel="Apply changes"
        busy={savePools.isPending}
        onCancel={() => setPendingPlan(null)}
        onConfirm={() => pendingPlan && savePools.mutate(pendingPlan)}
      />
    </div>
  );
}

function buildPoolSavePlan(raw: string, existing: PoolDTO[]): PoolSavePlan {
  const next = parsePools(raw);
  const existingIDs = new Set(existing.map((pool) => pool.id));
  const nextIDs = new Set(next.map((pool) => pool.id));
  return {
    next,
    createCount: next.filter((pool) => !existingIDs.has(pool.id)).length,
    updateCount: next.filter((pool) => existingIDs.has(pool.id)).length,
    deleteIDs: existing.filter((pool) => !nextIDs.has(pool.id)).map((pool) => pool.id)
  };
}

function describePlan(plan: PoolSavePlan | null) {
  if (!plan) {
    return "";
  }
  const deleteText =
    plan.deleteIDs.length > 0
      ? ` Deleted aliases: ${plan.deleteIDs.join(", ")}. Existing client keys that route through deleted pools will fail until moved.`
      : "";
  return `Apply raw pool changes? This will save ${plan.updateCount} existing pools, create ${plan.createCount}, and delete ${plan.deleteIDs.length}.${deleteText}`;
}

function parsePools(raw: string): PoolDTO[] {
  const parsed = JSON.parse(raw) as unknown;
  if (!Array.isArray(parsed)) {
    throw new Error("Raw pools must be a JSON array");
  }
  const ids = new Set<string>();
  for (const pool of parsed) {
    if (!isPool(pool)) {
      throw new Error("Every pool must include an id string");
    }
    if (ids.has(pool.id)) {
      throw new Error(`Duplicate pool id: ${pool.id}`);
    }
    ids.add(pool.id);
  }
  return parsed;
}

function isPool(value: unknown): value is PoolDTO {
  return Boolean(value && typeof value === "object" && typeof (value as { id?: unknown }).id === "string");
}
