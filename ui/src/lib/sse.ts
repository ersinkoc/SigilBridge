import { create } from "zustand";
import type { QueryClient } from "@tanstack/react-query";

export type AdminEvent = {
  type: string;
  data: unknown;
  receivedAt: string;
};

export type EventConnectionState = "connecting" | "open" | "error" | "disconnected";

type EventState = {
  events: AdminEvent[];
  connectionState: EventConnectionState;
  push: (event: AdminEvent) => void;
  setConnectionState: (state: EventConnectionState) => void;
};

export const useEventStore = create<EventState>((set) => ({
  events: [],
  connectionState: "disconnected",
  push: (event) => set((state) => ({ events: [event, ...state.events].slice(0, 100) })),
  setConnectionState: (connectionState) => set({ connectionState })
}));

export const adminEventTypes = ["reload", "pool_probe", "pool_reloaded", "oauth_providers_reloaded", "health", "audit", "budget"] as const;

export function connectEvents(path = "/admin/v1/events/stream", onEvent?: (event: AdminEvent) => void) {
  useEventStore.getState().setConnectionState("connecting");
  const source = new EventSource(path, { withCredentials: true });
  const push = (event: AdminEvent) => {
    useEventStore.getState().push(event);
    onEvent?.(event);
  };
  source.onopen = () => {
    useEventStore.getState().setConnectionState("open");
  };
  source.onerror = () => {
    useEventStore.getState().setConnectionState("error");
  };
  source.onmessage = (message) => {
    push({ type: "message", data: parseEventData(message.data), receivedAt: new Date().toISOString() });
  };
  for (const type of adminEventTypes) {
    source.addEventListener(type, (message) => {
      push({ type, data: parseEventData((message as MessageEvent).data), receivedAt: new Date().toISOString() });
    });
  }
  return source;
}

export function markEventsDisconnected() {
  useEventStore.getState().setConnectionState("disconnected");
}

export function parseEventData(raw: string): unknown {
  if (raw === "") {
    return null;
  }
  try {
    return JSON.parse(raw);
  } catch {
    return raw;
  }
}

export function invalidateForEvent(queryClient: QueryClient, event: AdminEvent) {
  if (event.type === "reload" || event.type === "pool_reloaded" || event.type === "oauth_providers_reloaded") {
    void queryClient.invalidateQueries({ queryKey: ["pools"] });
    void queryClient.invalidateQueries({ queryKey: ["credentials"] });
    void queryClient.invalidateQueries({ queryKey: ["health"] });
    return;
  }
  if (event.type === "health" || event.type === "pool_probe") {
    void queryClient.invalidateQueries({ queryKey: ["health"] });
    return;
  }
  if (event.type === "audit") {
    void queryClient.invalidateQueries({ queryKey: ["audit"] });
    return;
  }
  if (event.type === "budget") {
    void queryClient.invalidateQueries({ queryKey: ["budgets"] });
    void queryClient.invalidateQueries({ queryKey: ["usage"] });
  }
}
