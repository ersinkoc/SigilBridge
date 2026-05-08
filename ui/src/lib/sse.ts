import { create } from "zustand";
import type { QueryClient } from "@tanstack/react-query";

export type AdminEvent = {
  type: string;
  data: unknown;
  receivedAt: string;
};

type EventState = {
  events: AdminEvent[];
  push: (event: AdminEvent) => void;
};

export const useEventStore = create<EventState>((set) => ({
  events: [],
  push: (event) => set((state) => ({ events: [event, ...state.events].slice(0, 100) }))
}));

export function connectEvents(path = "/admin/v1/events/stream", onEvent?: (event: AdminEvent) => void) {
  const source = new EventSource(path, { withCredentials: true });
  const push = (event: AdminEvent) => {
    useEventStore.getState().push(event);
    onEvent?.(event);
  };
  source.onmessage = (message) => {
    push({ type: "message", data: message.data, receivedAt: new Date().toISOString() });
  };
  for (const type of ["reload", "health", "audit", "budget"]) {
    source.addEventListener(type, (message) => {
      push({ type, data: (message as MessageEvent).data, receivedAt: new Date().toISOString() });
    });
  }
  return source;
}

export function invalidateForEvent(queryClient: QueryClient, event: AdminEvent) {
  if (event.type === "reload") {
    void queryClient.invalidateQueries({ queryKey: ["pools"] });
    void queryClient.invalidateQueries({ queryKey: ["credentials"] });
    void queryClient.invalidateQueries({ queryKey: ["health"] });
    return;
  }
  if (event.type === "health") {
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
