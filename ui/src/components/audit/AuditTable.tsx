import { ChevronDown, ChevronRight } from "lucide-react";
import { Fragment, useState } from "react";

import { EmptyState } from "../common/State";

export function AuditTable({ rows }: { rows: Array<Record<string, unknown>> }) {
  const [expanded, setExpanded] = useState<string>("");
  if (rows.length === 0) {
    return <EmptyState />;
  }
  return (
    <table className="table">
      <thead>
        <tr>
          <th>Details</th>
          <th>Request</th>
          <th>Time</th>
          <th>Key</th>
          <th>Pool</th>
          <th>Provider</th>
          <th>Model</th>
          <th>Status</th>
          <th>Latency</th>
          <th>Tokens</th>
          <th>Cost</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((row, index) => {
          const key = String(row.request_id ?? index);
          const isOpen = expanded === key;
          return (
            <Fragment key={key}>
              <tr>
                <td>
                  <button className="btn btn-ghost icon-btn" type="button" onClick={() => setExpanded(isOpen ? "" : key)} aria-label={isOpen ? "Hide audit details" : "Show audit details"}>
                    {isOpen ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
                  </button>
                </td>
                <td className="mono-cell">{String(row.request_id ?? "")}</td>
                <td>{String(row.time ?? "")}</td>
                <td className="mono-cell">{String(row.bridge_key_id ?? "")}</td>
                <td>{String(row.pool_name ?? "")}</td>
                <td>{String(row.upstream_provider ?? "")}</td>
                <td>{String(row.upstream_model ?? row.model_alias ?? "")}</td>
                <td>
                  <span className={String(row.status ?? "") === "ok" ? "status-pill ok" : "status-pill bad"}>{String(row.status ?? "")}</span>
                </td>
                <td>{formatMs(row.latency_ms)}</td>
                <td>{formatTokens(row)}</td>
                <td>{formatCents(row.cost_cents)}</td>
              </tr>
              {isOpen ? (
                <tr className="audit-detail-row">
                  <td colSpan={11}>
                    <AuditDetails row={row} />
                  </td>
                </tr>
              ) : null}
            </Fragment>
          );
        })}
      </tbody>
    </table>
  );
}

function AuditDetails({ row }: { row: Record<string, unknown> }) {
  const record = objectValue(row.record) ?? row;
  const content = objectValue(row.content) ?? objectValue(record.content);
  const error = objectValue(row.error) ?? objectValue(record.error);
  const prompt = String(content?.prompt ?? "");
  const response = String(content?.response ?? "");
  const promptHash = String(content?.prompt_hash ?? "");
  const responseHash = String(content?.response_hash ?? "");

  return (
    <div className="audit-detail">
      <div className="audit-fields">
        <Field label="Ingress" value={record.ingress_format} />
        <Field label="Model alias" value={record.model_alias ?? row.model_alias} />
        <Field label="Upstream provider" value={record.upstream_provider ?? row.upstream_provider} />
        <Field label="Upstream ID" value={record.upstream_id ?? row.upstream_id} />
        <Field label="Upstream model" value={record.upstream_model ?? row.upstream_model} />
        <Field label="Stream" value={record.stream} />
        <Field label="Stop reason" value={record.stop_reason} />
        <Field label="TTFB" value={formatMs(record.ttfb_ms)} />
        <Field label="Latency" value={formatMs(record.latency_ms ?? row.latency_ms)} />
        <Field label="Input tokens" value={record.input_tokens ?? row.input_tokens} />
        <Field label="Output tokens" value={record.output_tokens ?? row.output_tokens} />
        <Field label="User agent" value={record.user_agent} />
      </div>
      {error ? (
        <div className="audit-block error">
          <strong>Error</strong>
          <pre>{JSON.stringify(error, null, 2)}</pre>
        </div>
      ) : null}
      <div className="audit-content-grid">
        <ContentBlock title="Prompt" text={prompt} hash={promptHash} />
        <ContentBlock title="Response" text={response} hash={responseHash} />
      </div>
      {!prompt && !response && !promptHash && !responseHash ? <p className="muted">No prompt or response content was captured for this record. Set audit.content_mode to truncated or full to store request text.</p> : null}
      <div className="audit-block">
        <strong>Raw record</strong>
        <pre>{JSON.stringify(record, null, 2)}</pre>
      </div>
    </div>
  );
}

function Field({ label, value }: { label: string; value: unknown }) {
  return (
    <div>
      <span>{label}</span>
      <strong>{String(value ?? "")}</strong>
    </div>
  );
}

function ContentBlock({ title, text, hash }: { title: string; text: string; hash: string }) {
  return (
    <div className="audit-block">
      <strong>{title}</strong>
      {text ? <pre>{text}</pre> : hash ? <code>{hash}</code> : <span className="muted">Not captured</span>}
    </div>
  );
}

function objectValue(value: unknown): Record<string, unknown> | undefined {
  return value && typeof value === "object" && !Array.isArray(value) ? (value as Record<string, unknown>) : undefined;
}

function formatCents(value: unknown) {
  const cents = typeof value === "number" ? value : Number(value ?? 0);
  return `$${(cents / 100).toFixed(2)}`;
}

function formatMs(value: unknown) {
  const ms = typeof value === "number" ? value : Number(value ?? 0);
  return ms > 0 ? `${ms.toFixed(0)} ms` : "";
}

function formatTokens(row: Record<string, unknown>) {
  const input = Number(row.input_tokens ?? 0);
  const output = Number(row.output_tokens ?? 0);
  if (!input && !output) {
    return "";
  }
  return `${input} in / ${output} out`;
}
