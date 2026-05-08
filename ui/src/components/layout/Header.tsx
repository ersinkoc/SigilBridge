import { Command, LogOut, Menu, RefreshCw } from "lucide-react";
import { useMemo } from "react";
import { useLocation } from "react-router-dom";

import { api } from "../../lib/api";
import { LanguageSwitcher } from "./LanguageSwitcher";
import { ThemeSwitcher } from "./ThemeSwitcher";
import { Button } from "../ui/Button";

const pages = [
  { path: "/", label: "Dashboard", detail: "Operational readiness, endpoints, and live request testing." },
  { path: "/setup", label: "Setup", detail: "Bring keys, credentials, catalog, and pools into a usable state." },
  { path: "/keys", label: "Keys", detail: "Client keys, scope policy, budgets, and rate limits." },
  { path: "/models", label: "Models", detail: "Provider catalog, model inventory, and availability." },
  { path: "/pools", label: "Pools", detail: "Routing aliases, upstream priority, weights, and credentials." },
  { path: "/credentials", label: "Credentials", detail: "API keys, OAuth, browser sessions, and local CLI agents." },
  { path: "/audit", label: "Audit", detail: "Request history, captured payloads, cost, and latency evidence." },
  { path: "/budgets", label: "Budgets", detail: "Usage controls and spend guardrails." },
  { path: "/health", label: "Health", detail: "Upstream readiness, cooldowns, and operational status." },
  { path: "/events", label: "Events", detail: "Realtime admin events from the local control plane." },
  { path: "/settings", label: "Settings", detail: "Advanced configuration editors and provider metadata." }
];

export function Header({ onMenu, onOpenCommand }: { onMenu: () => void; onOpenCommand: () => void }) {
  const location = useLocation();
  const page = useMemo(() => {
    const exact = pages.find((item) => item.path === location.pathname);
    if (exact) {
      return exact;
    }
    return [...pages].sort((left, right) => right.path.length - left.path.length).find((item) => item.path !== "/" && location.pathname.startsWith(item.path)) ?? pages[0];
  }, [location.pathname]);

  async function logout() {
    await api("/admin/v1/auth/logout", { method: "POST" }).catch(() => undefined);
    window.location.assign(window.location.pathname.startsWith("/admin/ui") ? "/admin/ui/login" : "/login");
  }

  return (
    <header className="header">
      <div className="header-main">
        <Button aria-label="Open navigation" className="mobile-only" icon={<Menu size={18} />} onClick={onMenu} />
        <div>
          <div className="header-kicker">
            <span className="status-dot" />
            <span>Admin console</span>
            <span className="header-page-chip">{page.label}</span>
          </div>
          <h1>SigilBridge</h1>
          <p>{page.detail}</p>
        </div>
      </div>
      <div className="header-actions">
        <Button title="Search" aria-label="Open command palette" icon={<Command size={16} />} onClick={onOpenCommand} />
        <Button title="Reload" aria-label="Reload" icon={<RefreshCw size={16} />} onClick={() => window.location.reload()} />
        <Button title="Sign out" aria-label="Sign out" icon={<LogOut size={16} />} onClick={() => void logout()} />
        <LanguageSwitcher />
        <ThemeSwitcher />
      </div>
    </header>
  );
}
