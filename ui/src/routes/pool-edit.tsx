import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { toast } from "sonner";

import { PoolEditor } from "../components/pools/PoolEditor";
import { api } from "../lib/api";
import type { PoolDTO } from "../types/api";

export function PoolEditRoute() {
  const { id } = useParams();
  const navigate = useNavigate();
  const [pool, setPool] = useState<PoolDTO>({ id: id ?? "new", strategy: "priority", upstreams: [] });
  const queryClient = useQueryClient();
  const pools = useQuery({ queryKey: ["pools"], queryFn: () => api<PoolDTO[]>("/admin/v1/pools"), enabled: Boolean(id && id !== "new") });
  useEffect(() => {
    const existing = pools.data?.find((candidate) => candidate.id === id);
    if (existing) {
      setPool(existing);
    }
  }, [id, pools.data]);
  const savePool = useMutation({
    mutationFn: () => api<PoolDTO>("/admin/v1/pools", { method: "POST", body: JSON.stringify(pool) }),
    onSuccess: (saved) => {
      setPool(saved);
      void queryClient.invalidateQueries({ queryKey: ["pools"] });
      toast.success("Pool saved");
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : "Save pool failed")
  });
  const deletePool = useMutation({
    mutationFn: () => api(`/admin/v1/pools/${pool.id}`, { method: "DELETE" }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["pools"] });
      toast.success("Pool deleted");
      navigate("/pools");
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : "Delete pool failed")
  });

  return (
    <div className="page">
      <div className="page-intro">
        <h2>Pool editor</h2>
        <p>Define the routing policy and upstream list for this provider pool.</p>
      </div>
      <PoolEditor
        pool={pool}
        onChange={setPool}
        onSave={() => savePool.mutate()}
        onDelete={pool.id === "new" ? undefined : () => confirmDelete(() => deletePool.mutate())}
        saving={savePool.isPending}
        deleting={deletePool.isPending}
      />
    </div>
  );
}

function confirmDelete(action: () => void) {
  if (window.confirm("Delete this pool?")) {
    action();
  }
}
