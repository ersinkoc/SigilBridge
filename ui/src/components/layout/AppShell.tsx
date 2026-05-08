import { X } from "lucide-react";
import { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Outlet } from "react-router-dom";

import { connectEvents, invalidateForEvent } from "../../lib/sse";
import { Button } from "../ui/Button";
import { Header } from "./Header";
import { Sidebar } from "./Sidebar";

export function AppShell() {
  const [open, setOpen] = useState(false);
  const queryClient = useQueryClient();

  useEffect(() => {
    if (typeof EventSource === "undefined") {
      return;
    }
    const source = connectEvents("/admin/v1/events/stream", (event) => invalidateForEvent(queryClient, event));
    return () => source.close();
  }, [queryClient]);

  return (
    <div className="app-shell">
      <div className={`drawer ${open ? "open" : ""}`}>
        <Button aria-label="Close navigation" className="mobile-only drawer-close" icon={<X size={18} />} onClick={() => setOpen(false)} />
        <Sidebar />
      </div>
      <button className={`drawer-backdrop ${open ? "open" : ""}`} aria-label="Close navigation overlay" onClick={() => setOpen(false)} />
      <main className="main">
        <Header onMenu={() => setOpen(true)} />
        <section className="content">
          <Outlet />
        </section>
      </main>
    </div>
  );
}
