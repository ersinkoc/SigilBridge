import { Activity, Clock3 } from "lucide-react";

import { EmptyState } from "../components/common/State";
import { useEventStore } from "../lib/sse";

export function EventsRoute() {
  const events = useEventStore((state) => state.events);
  const connectionState = useEventStore((state) => state.connectionState);

  return (
    <div className="page">
      <div className="page-title">
        <div className="page-intro">
          <h2>Events</h2>
          <p>Admin control-plane events for reloads, pool probes, provider metadata, audit updates, and budget signals.</p>
        </div>
        <span className={connectionState === "open" ? "status-pill ok" : connectionState === "error" ? "status-pill bad" : "status-pill"}>
          {connectionLabel(connectionState)}
        </span>
      </div>
      <div className="event-list">
        {events.length === 0 ? (
          <EmptyState label="No admin events received in this browser session." />
        ) : (
          events.map((event, index) => (
            <div key={`${event.receivedAt}-${index}`} className="event-row">
              <span className="event-icon">
                <Activity size={16} />
              </span>
              <div className="event-main">
                <div className="event-heading">
                  <strong>{event.type}</strong>
                  <span>
                    <Clock3 size={14} />
                    {formatTimestamp(event.receivedAt)}
                  </span>
                </div>
                <span>{eventSummary(event.data)}</span>
                <code>{eventPayload(event.data)}</code>
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  );
}

function connectionLabel(state: string) {
  if (state === "open") {
    return "Connected";
  }
  if (state === "error") {
    return "Reconnecting";
  }
  if (state === "connecting") {
    return "Connecting";
  }
  return "Disconnected";
}

function formatTimestamp(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString(undefined, { dateStyle: "medium", timeStyle: "medium" });
}

function eventSummary(data: unknown) {
  if (data === null || data === undefined) {
    return "No payload";
  }
  if (typeof data === "string") {
    return data;
  }
  if (typeof data !== "object") {
    return String(data);
  }
  const record = data as Record<string, unknown>;
  const parts = [
    typeof record.pool === "string" ? `pool ${record.pool}` : "",
    typeof record.checked === "number" && typeof record.passed === "number" ? `${record.passed}/${record.checked} passed` : "",
    typeof record.ok === "boolean" ? (record.ok ? "ok" : "failed") : ""
  ].filter(Boolean);
  return parts.length > 0 ? parts.join(" - ") : `${Object.keys(record).length} payload fields`;
}

function eventPayload(data: unknown) {
  if (typeof data === "string") {
    return data;
  }
  return JSON.stringify(data ?? null);
}
