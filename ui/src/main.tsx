import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import React from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import { Toaster } from "sonner";

import App from "./App";
import "./lib/i18n";
import { initTheme } from "./lib/theme";
import "./styles/app.css";

initTheme();

const queryClient = new QueryClient();

function basename() {
  return window.location.pathname.startsWith("/admin/ui") ? "/admin/ui" : "/";
}

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter basename={basename()}>
        <App />
        <Toaster position="top-right" richColors />
      </BrowserRouter>
    </QueryClientProvider>
  </React.StrictMode>
);
