import { Link } from "react-router-dom";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Braces, RefreshCw, ShieldCheck } from "lucide-react";
import { toast } from "sonner";

import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { api } from "../lib/api";
import type { ReloadResult } from "../types/api";

export function SettingsRoute() {
  const queryClient = useQueryClient();
  const reload = useMutation({
    mutationFn: () => api<ReloadResult>("/admin/v1/reload", { method: "POST", body: "{}" }),
    onSuccess: () => {
      toast.success("Configuration reloaded");
      void queryClient.invalidateQueries();
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : "Reload failed")
  });

  return (
    <div className="page">
      <div className="page-title">
        <div className="page-intro">
          <h2>Settings</h2>
          <p>Inspect raw configuration surfaces that back the local control plane.</p>
        </div>
        <Button icon={<RefreshCw size={16} />} onClick={() => reload.mutate()} disabled={reload.isPending}>
          {reload.isPending ? "Reloading" : "Reload"}
        </Button>
      </div>
      <div className="settings-grid">
        <Card className="settings-tile">
          <ShieldCheck size={20} />
          <div>
            <strong>OAuth providers</strong>
            <span>Provider bootstrap metadata and adapter defaults.</span>
          </div>
          <Link to="/settings/oauth-providers">
            <Button>Open</Button>
          </Link>
        </Card>
        <Card className="settings-tile">
          <Braces size={20} />
          <div>
            <strong>Raw pools</strong>
            <span>Routing pools and upstream definitions.</span>
          </div>
          <Link to="/settings/pools-raw">
            <Button>Open</Button>
          </Link>
        </Card>
      </div>
    </div>
  );
}
