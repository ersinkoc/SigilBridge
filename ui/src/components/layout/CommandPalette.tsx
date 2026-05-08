import { Activity, BadgeDollarSign, ClipboardList, Gauge, HeartPulse, KeyRound, Layers3, PlugZap, Route, Search, Settings, WandSparkles, X } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";

import { Button } from "../ui/Button";

const commands = [
  { to: "/", label: "Dashboard", detail: "Readiness, endpoints, and live chat test", icon: Gauge },
  { to: "/setup", label: "Setup", detail: "Configuration progress and client endpoints", icon: WandSparkles },
  { to: "/keys", label: "Keys", detail: "Client access and policy", icon: KeyRound },
  { to: "/keys/new", label: "Create client key", detail: "Issue a scoped bridge key", icon: KeyRound },
  { to: "/models", label: "Models", detail: "Provider catalog and available models", icon: Layers3 },
  { to: "/pools", label: "Pools", detail: "Routing aliases and upstream order", icon: Route },
  { to: "/credentials", label: "Credentials", detail: "API keys, OAuth, sessions, and CLI agents", icon: PlugZap },
  { to: "/credentials/api-key/new", label: "Add API key", detail: "Store and attach provider credentials", icon: PlugZap },
  { to: "/credentials/oauth/new", label: "Connect OAuth", detail: "Browser and device authorization", icon: PlugZap },
  { to: "/credentials/cli", label: "CLI agents", detail: "Local agent discovery and enablement", icon: Activity },
  { to: "/audit", label: "Audit", detail: "Request history and captured content", icon: ClipboardList },
  { to: "/budgets", label: "Budgets", detail: "Spend limits and usage", icon: BadgeDollarSign },
  { to: "/health", label: "Health", detail: "Upstream readiness and cooldowns", icon: HeartPulse },
  { to: "/events", label: "Events", detail: "Realtime admin event stream", icon: Activity },
  { to: "/settings", label: "Settings", detail: "Raw pools and OAuth provider metadata", icon: Settings }
];

export function CommandPalette({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [query, setQuery] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);
  const navigate = useNavigate();
  const results = useMemo(() => {
    const value = query.trim().toLowerCase();
    if (!value) {
      return commands;
    }
    return commands.filter((command) => `${command.label} ${command.detail}`.toLowerCase().includes(value));
  }, [query]);

  useEffect(() => {
    if (!open) {
      setQuery("");
      return;
    }
    window.setTimeout(() => inputRef.current?.focus(), 0);
  }, [open]);

  useEffect(() => {
    if (!open) {
      return;
    }
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        onClose();
      }
    }
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [onClose, open]);

  if (!open) {
    return null;
  }

  function openCommand(to: string) {
    navigate(to);
    onClose();
  }

  return (
    <div className="command-backdrop" role="presentation" onMouseDown={onClose}>
      <div className="command-dialog" role="dialog" aria-modal="true" aria-label="Command palette" onMouseDown={(event) => event.stopPropagation()}>
        <div className="command-search">
          <Search size={18} />
          <input ref={inputRef} value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search screens and actions" />
          <Button aria-label="Close command palette" variant="ghost" icon={<X size={16} />} onClick={onClose} />
        </div>
        <div className="command-list">
          {results.map((command) => {
            const Icon = command.icon;
            return (
              <button key={command.to} type="button" className="command-item" onClick={() => openCommand(command.to)}>
                <span>
                  <Icon size={17} />
                </span>
                <div>
                  <strong>{command.label}</strong>
                  <em>{command.detail}</em>
                </div>
              </button>
            );
          })}
          {results.length === 0 ? <div className="command-empty">No matching action</div> : null}
        </div>
      </div>
    </div>
  );
}
