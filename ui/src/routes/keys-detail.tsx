import { useEffect, useState, type FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Ban, RotateCcw, Save, Trash2 } from "lucide-react";
import { useNavigate, useParams } from "react-router-dom";
import { toast } from "sonner";

import { ConfirmDialog } from "../components/common/ConfirmDialog";
import { ErrorState, Skeleton } from "../components/common/State";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { api } from "../lib/api";
import type { KeyDTO } from "../types/api";

export function KeysDetailRoute() {
  const { id } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [deleteOpen, setDeleteOpen] = useState(false);
  const key = useQuery({ queryKey: ["keys", id], queryFn: () => api<KeyDTO>(`/admin/v1/keys/${id}`), enabled: Boolean(id) });
  const [form, setForm] = useState({
    name: "",
    dailyCents: "0",
    monthlyCents: "0",
    hardCap: true,
    allowedPools: "",
    allowedModels: "",
    ipAllowlist: "",
    rpm: "0",
    tpm: "0"
  });
  useEffect(() => {
    if (!key.data) {
      return;
    }
    setForm({
      name: key.data.name ?? "",
      dailyCents: String(numberValue(key.data.budgets?.daily_cents)),
      monthlyCents: String(numberValue(key.data.budgets?.monthly_cents)),
      hardCap: key.data.budgets?.hard_cap !== false,
      allowedPools: listValue(key.data.scopes?.allowed_pools),
      allowedModels: listValue(key.data.scopes?.allowed_models),
      ipAllowlist: listValue(key.data.scopes?.ip_allowlist),
      rpm: String(numberValue(key.data.rate_limits?.rpm)),
      tpm: String(numberValue(key.data.rate_limits?.tpm))
    });
  }, [key.data]);
  const patchKey = useMutation({
    mutationFn: (revoked: boolean) => api<KeyDTO>(`/admin/v1/keys/${id}`, { method: "PATCH", body: JSON.stringify({ revoked }) }),
    onSuccess: (updated) => {
      queryClient.setQueryData(["keys", id], updated);
      void queryClient.invalidateQueries({ queryKey: ["keys"] });
      toast.success(updated.revoked_at ? "Key revoked" : "Key restored");
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : "Key update failed")
  });
  const deleteKey = useMutation({
    mutationFn: () => api(`/admin/v1/keys/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["keys"] });
      toast.success("Key deleted");
      navigate("/keys");
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : "Delete key failed")
  });
  const saveKey = useMutation({
    mutationFn: () =>
      api<KeyDTO>(`/admin/v1/keys/${id}`, {
        method: "PATCH",
        body: JSON.stringify({
          name: form.name,
          budgets: {
            daily_cents: Number(form.dailyCents || 0),
            monthly_cents: Number(form.monthlyCents || 0),
            hard_cap: form.hardCap
          },
          scopes: {
            allowed_pools: splitList(form.allowedPools),
            allowed_models: splitList(form.allowedModels),
            ip_allowlist: splitList(form.ipAllowlist)
          },
          rate_limits: {
            rpm: Number(form.rpm || 0),
            tpm: Number(form.tpm || 0)
          }
        })
      }),
    onSuccess: (updated) => {
      queryClient.setQueryData(["keys", id], updated);
      void queryClient.invalidateQueries({ queryKey: ["keys"] });
      toast.success("Key settings saved");
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : "Key settings update failed")
  });

  function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    saveKey.mutate();
  }

  return (
    <div className="page">
      <h2>Key detail</h2>
      {key.isLoading ? (
        <Skeleton />
      ) : key.isError ? (
        <ErrorState label={(key.error as Error).message} />
      ) : (
        <>
          <Card className="detail-panel">
            <div>
              <span>ID</span>
              <strong>{key.data?.id}</strong>
            </div>
            <div>
              <span>Name</span>
              <strong>{key.data?.name || "-"}</strong>
            </div>
            <div>
              <span>Created</span>
              <strong>{formatDate(key.data?.created_at) || "-"}</strong>
            </div>
            <div>
              <span>Last used</span>
              <strong>{formatDate(key.data?.last_used_at) || "Never"}</strong>
            </div>
            <div>
              <span>Status</span>
              <strong>{key.data?.revoked_at ? "Revoked" : "Active"}</strong>
            </div>
          </Card>
          <form className="settings-grid" onSubmit={save}>
            <Card className="form-grid">
              <h3>Budgets</h3>
              <label>
                Name
                <input className="input" value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} />
              </label>
              <label>
                Daily cents
                <input className="input" type="number" min="0" value={form.dailyCents} onChange={(event) => setForm({ ...form, dailyCents: event.target.value })} />
              </label>
              <label>
                Monthly cents
                <input className="input" type="number" min="0" value={form.monthlyCents} onChange={(event) => setForm({ ...form, monthlyCents: event.target.value })} />
              </label>
              <label className="check-row">
                <input type="checkbox" checked={form.hardCap} onChange={(event) => setForm({ ...form, hardCap: event.target.checked })} />
                Hard cap
              </label>
            </Card>
            <Card className="form-grid">
              <h3>Scopes</h3>
              <label>
                Allowed pools
                <input className="input" value={form.allowedPools} onChange={(event) => setForm({ ...form, allowedPools: event.target.value })} placeholder="default, fallback" />
              </label>
              <label>
                Allowed models
                <input className="input" value={form.allowedModels} onChange={(event) => setForm({ ...form, allowedModels: event.target.value })} placeholder="gpt-4o, claude-sonnet" />
              </label>
              <label>
                IP allowlist
                <input className="input" value={form.ipAllowlist} onChange={(event) => setForm({ ...form, ipAllowlist: event.target.value })} placeholder="127.0.0.1/32" />
              </label>
            </Card>
            <Card className="form-grid">
              <h3>Rate limits</h3>
              <label>
                RPM
                <input className="input" type="number" min="0" value={form.rpm} onChange={(event) => setForm({ ...form, rpm: event.target.value })} />
              </label>
              <label>
                TPM
                <input className="input" type="number" min="0" value={form.tpm} onChange={(event) => setForm({ ...form, tpm: event.target.value })} />
              </label>
              <Button icon={<Save size={16} />} disabled={saveKey.isPending}>
                Save
              </Button>
            </Card>
          </form>
          <div className="actions-row">
            {key.data?.revoked_at ? (
              <Button icon={<RotateCcw size={16} />} onClick={() => patchKey.mutate(false)} disabled={patchKey.isPending}>
                Restore
              </Button>
            ) : (
              <Button icon={<Ban size={16} />} onClick={() => patchKey.mutate(true)} disabled={patchKey.isPending}>
                Revoke
              </Button>
            )}
            <Button icon={<Trash2 size={16} />} variant="danger" onClick={() => setDeleteOpen(true)} disabled={deleteKey.isPending}>
              Delete
            </Button>
          </div>
          <Card className="code-panel">
            <pre>{JSON.stringify(key.data, null, 2)}</pre>
          </Card>
          <ConfirmDialog
            open={deleteOpen}
            title="Delete bridge key"
            description={`Delete ${key.data?.name || key.data?.id || "this key"} permanently? Clients using this key will fail authentication immediately. Revoke is safer if you may need an audit trail before removal.`}
            confirmLabel="Delete key"
            busy={deleteKey.isPending}
            onCancel={() => setDeleteOpen(false)}
            onConfirm={() => deleteKey.mutate()}
          />
        </>
      )}
    </div>
  );
}

function formatDate(value?: string) {
  if (!value) {
    return "";
  }
  return new Date(value).toLocaleString();
}

function numberValue(value: unknown) {
  return typeof value === "number" ? value : Number(value ?? 0);
}

function listValue(value: unknown) {
  if (!Array.isArray(value)) {
    return "";
  }
  return value.map((item) => String(item)).join(", ");
}

function splitList(value: string) {
  return value
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
}
