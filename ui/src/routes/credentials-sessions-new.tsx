import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Save } from "lucide-react";
import { type FormEvent, useState } from "react";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";

import { Card } from "../components/ui/Card";
import { Button } from "../components/ui/Button";
import { Input } from "../components/ui/Input";
import { api } from "../lib/api";

function parseCookies(raw: string) {
  const parsed = JSON.parse(raw) as unknown;
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    throw new Error("Cookies must be a JSON object");
  }
  return Object.fromEntries(Object.entries(parsed).map(([key, value]) => [key, String(value)]));
}

export function CredentialsSessionsNewRoute() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [provider, setProvider] = useState("claude_web");
  const [name, setName] = useState("default");
  const [userAgent, setUserAgent] = useState("");
  const [organizationID, setOrganizationID] = useState("");
  const [cookies, setCookies] = useState('{\n  "session": ""\n}');

  const create = useMutation({
    mutationFn: () =>
      api("/admin/v1/credentials/session", {
        method: "POST",
        body: JSON.stringify({
          provider,
          name,
          user_agent: userAgent,
          organization_id: organizationID,
          cookies: parseCookies(cookies)
        })
      }),
    onSuccess: () => {
      toast.success("Session credential stored");
      void queryClient.invalidateQueries({ queryKey: ["credentials"] });
      navigate("/credentials");
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : "Session import failed")
  });

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    create.mutate();
  }

  return (
    <div className="page narrow">
      <div className="page-intro">
        <h2>Session fallback</h2>
        <p>Manual cookie import for providers where API keys, OAuth metadata, and local CLI auth are not usable.</p>
      </div>
      <Card className="warning-panel">
        <strong>Use this sparingly</strong>
        <span>Browser sessions are brittle, provider-specific, and should not be the normal way to run SigilBridge.</span>
      </Card>
      <Card>
        <form className="form-grid" onSubmit={submit}>
          <label>
            Provider
            <select className="input" value={provider} onChange={(event) => setProvider(event.target.value)}>
              <option value="claude_web">Claude web</option>
              <option value="chatgpt_web">ChatGPT web</option>
            </select>
          </label>
          <label>
            Name
            <Input value={name} onChange={(event) => setName(event.target.value)} />
          </label>
          <label>
            User agent
            <Input value={userAgent} onChange={(event) => setUserAgent(event.target.value)} />
          </label>
          <label>
            Organization ID
            <Input value={organizationID} onChange={(event) => setOrganizationID(event.target.value)} />
          </label>
          <label>
            Cookies JSON
            <textarea className="input text-area" value={cookies} onChange={(event) => setCookies(event.target.value)} />
          </label>
          <Button type="submit" variant="primary" icon={<Save size={16} />} disabled={create.isPending}>
            {create.isPending ? "Saving" : "Store session"}
          </Button>
        </form>
      </Card>
    </div>
  );
}
