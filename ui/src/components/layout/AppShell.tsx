import { X } from "lucide-react";
import { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Outlet, useLocation } from "react-router-dom";

import { ErrorBoundary } from "../common/ErrorBoundary";
import { connectEvents, invalidateForEvent, markEventsDisconnected } from "../../lib/sse";
import { Button } from "../ui/Button";
import { CommandPalette } from "./CommandPalette";
import { Header } from "./Header";
import { Sidebar } from "./Sidebar";

export function AppShell() {
  const [open, setOpen] = useState(false);
  const [commandOpen, setCommandOpen] = useState(false);
  const queryClient = useQueryClient();
  const location = useLocation();

  useEffect(() => {
    if (typeof EventSource === "undefined") {
      return;
    }
    const source = connectEvents("/admin/v1/events/stream", (event) => invalidateForEvent(queryClient, event));
    return () => {
      source.close();
      markEventsDisconnected();
    };
  }, [queryClient]);

  useEffect(() => {
    setOpen(false);
  }, [location.pathname]);

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      const target = event.target as HTMLElement | null;
      const isTyping = target?.tagName === "INPUT" || target?.tagName === "TEXTAREA" || target?.tagName === "SELECT" || target?.isContentEditable;
      if (event.key === "Escape") {
        setOpen(false);
      }
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "k" && !isTyping) {
        event.preventDefault();
        setCommandOpen(true);
      }
    }
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, []);

  return (
    <div className="app-shell">
      <a className="skip-link" href="#main-content">
        Skip to main content
      </a>
      <div className={`drawer ${open ? "open" : ""}`}>
        <Button aria-label="Close navigation" className="mobile-only drawer-close" icon={<X size={18} />} onClick={() => setOpen(false)} />
        <Sidebar />
      </div>
      <button className={`drawer-backdrop ${open ? "open" : ""}`} aria-label="Close navigation overlay" onClick={() => setOpen(false)} />
      <main className="main">
        <Header onMenu={() => setOpen(true)} onOpenCommand={() => setCommandOpen(true)} />
        <section id="main-content" className="content" tabIndex={-1}>
          <ErrorBoundary resetKey={location.pathname}>
            <Outlet />
          </ErrorBoundary>
        </section>
      </main>
      <CommandPalette open={commandOpen} onClose={() => setCommandOpen(false)} />
    </div>
  );
}
