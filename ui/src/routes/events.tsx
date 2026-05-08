import { EmptyState } from "../components/common/State";
import { useEventStore } from "../lib/sse";

export function EventsRoute() {
  const events = useEventStore((state) => state.events);

  return (
    <div className="page">
      <div className="page-intro">
        <h2>Events</h2>
        <p>Live admin stream for reloads, health changes, audit entries, and budget signals.</p>
      </div>
      <div className="event-list">
        {events.length === 0 ? (
          <EmptyState label="No live events yet" />
        ) : (
          events.map((event, index) => (
            <div key={`${event.receivedAt}-${index}`} className="event-row">
              <strong>{event.type}</strong>
              <span>{event.receivedAt}</span>
            </div>
          ))
        )}
      </div>
    </div>
  );
}
