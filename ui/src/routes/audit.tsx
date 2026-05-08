import { useMemo, useState, type FormEvent } from "react";
import { Download, Search } from "lucide-react";
import { useQuery } from "@tanstack/react-query";

import { AuditTable } from "../components/audit/AuditTable";
import { ErrorState, Skeleton } from "../components/common/State";
import { Card } from "../components/ui/Card";
import { api } from "../lib/api";
import type { AuditResponse } from "../types/api";

type AuditFilters = {
  requestId: string;
  keyId: string;
  pool: string;
  upstreamId: string;
  status: string;
  from: string;
  to: string;
  limit: string;
};

const initialFilters: AuditFilters = { requestId: "", keyId: "", pool: "", upstreamId: "", status: "", from: "", to: "", limit: "100" };

export function AuditRoute() {
  const [draft, setDraft] = useState(initialFilters);
  const [filters, setFilters] = useState(initialFilters);
  const [cursor, setCursor] = useState("");
  const [history, setHistory] = useState<string[]>([]);
  const queryPath = useMemo(() => auditPath(filters, cursor), [filters, cursor]);
  const audit = useQuery({ queryKey: ["audit", queryPath], queryFn: () => api<AuditResponse>(queryPath) });
  const rows = audit.data?.items ?? [];
  const warnings = rows.filter((row) => String(row.status ?? "") !== "ok").length;
  const denied = rows.filter((row) => String(row.status ?? "").toLowerCase() === "denied").length;
  const errors = rows.filter((row) => String(row.status ?? "").toLowerCase() === "error").length;
  const nextCursor = audit.data?.next_cursor ?? "";

  function applyFilters(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setFilters(draft);
    setCursor("");
    setHistory([]);
  }

  function nextPage() {
    if (!nextCursor) {
      return;
    }
    setHistory((current) => [...current, cursor]);
    setCursor(nextCursor);
  }

  function previousPage() {
    setHistory((current) => {
      if (current.length === 0) {
        setCursor("");
        return current;
      }
      const nextHistory = current.slice(0, -1);
      setCursor(current[current.length - 1] ?? "");
      return nextHistory;
    });
  }

  return (
    <div className="page">
      <div className="page-intro">
        <h2>Audit</h2>
        <p>Review request, credential, routing, and admin actions once events begin flowing.</p>
      </div>
      <form className="filter-bar" onSubmit={applyFilters}>
        <label>
          Request
          <input className="input" value={draft.requestId} onChange={(event) => setDraft({ ...draft, requestId: event.target.value })} placeholder="req_..." />
        </label>
        <label>
          Key
          <input className="input" value={draft.keyId} onChange={(event) => setDraft({ ...draft, keyId: event.target.value })} placeholder="key_..." />
        </label>
        <label>
          Pool
          <input className="input" value={draft.pool} onChange={(event) => setDraft({ ...draft, pool: event.target.value })} placeholder="default" />
        </label>
        <label>
          Upstream
          <input className="input" value={draft.upstreamId} onChange={(event) => setDraft({ ...draft, upstreamId: event.target.value })} placeholder="openai-main" />
        </label>
        <label>
          Status
          <select className="input" value={draft.status} onChange={(event) => setDraft({ ...draft, status: event.target.value })}>
            <option value="">Any</option>
            <option value="ok">OK</option>
            <option value="error">Error</option>
            <option value="denied">Denied</option>
          </select>
        </label>
        <label>
          From
          <input className="input" type="date" value={draft.from} onChange={(event) => setDraft({ ...draft, from: event.target.value })} />
        </label>
        <label>
          To
          <input className="input" type="date" value={draft.to} onChange={(event) => setDraft({ ...draft, to: event.target.value })} />
        </label>
        <label>
          Limit
          <input className="input" type="number" min="1" max="500" value={draft.limit} onChange={(event) => setDraft({ ...draft, limit: event.target.value })} />
        </label>
        <div className="filter-actions">
          <button className="btn btn-primary" type="submit">
            <Search size={16} />
            Apply
          </button>
          <button className="btn" type="button" onClick={() => exportCSV(rows)} disabled={rows.length === 0}>
            <Download size={16} />
            CSV
          </button>
        </div>
      </form>
      <div className="summary-strip">
        <Card className="summary-item">
          <span>Events</span>
          <strong>{rows.length}</strong>
        </Card>
        <Card className="summary-item">
          <span>Warnings</span>
          <strong>{warnings}</strong>
        </Card>
        <Card className="summary-item">
          <span>Errors</span>
          <strong>{errors}</strong>
        </Card>
        <Card className="summary-item">
          <span>Denied</span>
          <strong>{denied}</strong>
        </Card>
      </div>
      {audit.isLoading ? <Skeleton /> : audit.isError ? <ErrorState label={(audit.error as Error).message} /> : <AuditTable rows={rows} />}
      <div className="pagination">
        <button className="btn" type="button" onClick={previousPage} disabled={history.length === 0}>
          Previous
        </button>
        <button className="btn" type="button" onClick={nextPage} disabled={!nextCursor}>
          Next
        </button>
      </div>
    </div>
  );
}

function auditPath(filters: AuditFilters, cursor: string) {
  const params = new URLSearchParams();
  if (filters.requestId.trim()) {
    params.set("request_id", filters.requestId.trim());
  }
  if (filters.keyId.trim()) {
    params.set("key_id", filters.keyId.trim());
  }
  if (filters.pool.trim()) {
    params.set("pool", filters.pool.trim());
  }
  if (filters.upstreamId.trim()) {
    params.set("upstream_id", filters.upstreamId.trim());
  }
  if (filters.status.trim()) {
    params.set("status", filters.status.trim());
  }
  if (filters.from) {
    params.set("from", filters.from);
  }
  if (filters.to) {
    params.set("to", filters.to);
  }
  if (filters.limit.trim()) {
    params.set("limit", filters.limit.trim());
  }
  if (cursor) {
    params.set("cursor", cursor);
  }
  const query = params.toString();
  return query ? `/admin/v1/audit?${query}` : "/admin/v1/audit";
}

function exportCSV(rows: Array<Record<string, unknown>>) {
  const headers = [
    "request_id",
    "time",
    "bridge_key_id",
    "ingress_format",
    "pool_name",
    "upstream_id",
    "upstream_provider",
    "model_alias",
    "upstream_model",
    "status",
    "latency_ms",
    "ttfb_ms",
    "input_tokens",
    "output_tokens",
    "stop_reason",
    "cost_cents"
  ];
  const lines = [headers.join(","), ...rows.map((row) => headers.map((header) => csvCell(row[header])).join(","))];
  const blob = new Blob([lines.join("\n")], { type: "text/csv;charset=utf-8" });
  const href = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = href;
  link.download = "sigilbridge-audit.csv";
  link.click();
  URL.revokeObjectURL(href);
}

function csvCell(value: unknown) {
  const text = String(value ?? "");
  return `"${text.replaceAll('"', '""')}"`;
}
