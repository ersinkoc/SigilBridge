import { useMutation, useQueryClient } from "@tanstack/react-query";
import { KeyRound, Plus, ShieldCheck } from "lucide-react";
import { type FormEvent, useState } from "react";
import { toast } from "sonner";

import { SecretReveal } from "../components/keys/SecretReveal";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { Input } from "../components/ui/Input";
import { api } from "../lib/api";
import type { CreateKeyResponse } from "../types/api";

export function KeysNewRoute() {
  const [prefix, setPrefix] = useState("test");
  const [name, setName] = useState("");
  const [allowedPools, setAllowedPools] = useState("");
  const [allowedModels, setAllowedModels] = useState("");
  const [ipAllowlist, setIPAllowlist] = useState("");
  const [dailyCents, setDailyCents] = useState("0");
  const [monthlyCents, setMonthlyCents] = useState("0");
  const [rpm, setRPM] = useState("0");
  const [tpm, setTPM] = useState("0");
  const [hardCap, setHardCap] = useState(true);
  const [secret, setSecret] = useState("");
  const queryClient = useQueryClient();
  const createKey = useMutation({
    mutationFn: () =>
      api<CreateKeyResponse>("/admin/v1/keys", {
        method: "POST",
        body: JSON.stringify({
          prefix,
          metadata: name ? { name } : {},
          scopes: {
            allowed_pools: splitList(allowedPools),
            allowed_models: splitList(allowedModels),
            ip_allowlist: splitList(ipAllowlist)
          },
          budgets: {
            daily_cents: Number(dailyCents || 0),
            monthly_cents: Number(monthlyCents || 0),
            hard_cap: hardCap
          },
          rate_limits: {
            rpm: Number(rpm || 0),
            tpm: Number(tpm || 0)
          }
        })
      }),
    onSuccess: (result) => {
      setSecret(result.plaintext);
      void queryClient.invalidateQueries({ queryKey: ["keys"] });
      toast.success("Bridge key created");
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : "Create key failed")
  });

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    createKey.mutate();
  }

  return (
    <div className="page">
      <div className="page-intro">
        <h2>Create bridge key</h2>
        <p>Provision a client-facing key with its operational guardrails in one pass.</p>
      </div>
      <form className="key-create-layout" onSubmit={submit}>
        <Card className="form-grid">
          <div className="panel-heading">
            <div>
              <h2>Identity</h2>
              <p>Name the client and choose whether this is a test or live credential.</p>
            </div>
            <KeyRound size={20} />
          </div>
          <label>
            Prefix
            <select className="input" value={prefix} onChange={(event) => setPrefix(event.target.value)}>
              <option value="test">test</option>
              <option value="live">live</option>
            </select>
          </label>
          <label>
            Name
            <Input value={name} placeholder="frontend-local" onChange={(event) => setName(event.target.value)} />
          </label>
        </Card>
        <Card className="form-grid">
          <div className="panel-heading">
            <div>
              <h2>Scopes</h2>
              <p>Leave fields empty to allow all matching pools, models, or client IPs.</p>
            </div>
            <ShieldCheck size={20} />
          </div>
          <label>
            Allowed pools
            <Input value={allowedPools} placeholder="coding, default" onChange={(event) => setAllowedPools(event.target.value)} />
          </label>
          <label>
            Allowed models
            <Input value={allowedModels} placeholder="claude-sonnet, gpt-4o" onChange={(event) => setAllowedModels(event.target.value)} />
          </label>
          <label>
            IP allowlist
            <Input value={ipAllowlist} placeholder="127.0.0.1/32" onChange={(event) => setIPAllowlist(event.target.value)} />
          </label>
        </Card>
        <Card className="form-grid">
          <div className="panel-heading">
            <div>
              <h2>Limits</h2>
              <p>Zero means unlimited for that guardrail.</p>
            </div>
          </div>
          <div className="limit-grid">
            <label>
              Daily cents
              <Input type="number" min="0" value={dailyCents} onChange={(event) => setDailyCents(event.target.value)} />
            </label>
            <label>
              Monthly cents
              <Input type="number" min="0" value={monthlyCents} onChange={(event) => setMonthlyCents(event.target.value)} />
            </label>
            <label>
              RPM
              <Input type="number" min="0" value={rpm} onChange={(event) => setRPM(event.target.value)} />
            </label>
            <label>
              TPM
              <Input type="number" min="0" value={tpm} onChange={(event) => setTPM(event.target.value)} />
            </label>
          </div>
          <label className="check-row">
            <input type="checkbox" checked={hardCap} onChange={(event) => setHardCap(event.target.checked)} />
            Hard cap budgets
          </label>
          <Button type="submit" variant="primary" icon={<Plus size={16} />} disabled={createKey.isPending}>
            {createKey.isPending ? "Creating" : "Create bridge key"}
          </Button>
        </Card>
      </form>
      <SecretReveal plaintext={secret} />
    </div>
  );
}

function splitList(value: string) {
  return value
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
}
