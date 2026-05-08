import type { QueryClient } from "@tanstack/react-query";
import { describe, expect, it, vi } from "vitest";

import { adminEventTypes, invalidateForEvent, parseEventData } from "../../src/lib/sse";

describe("sse", () => {
  it("parses JSON event payloads and preserves plain text", () => {
    expect(parseEventData('{"pool":"default","checked":2,"passed":1}')).toEqual({ pool: "default", checked: 2, passed: 1 });
    expect(parseEventData("plain event")).toBe("plain event");
    expect(parseEventData("")).toBeNull();
  });

  it("subscribes to backend admin event types", () => {
    expect(adminEventTypes).toContain("pool_probe");
    expect(adminEventTypes).toContain("pool_reloaded");
    expect(adminEventTypes).toContain("oauth_providers_reloaded");
  });

  it("invalidates health after a pool probe event", () => {
    const queryClient = { invalidateQueries: vi.fn() } as unknown as QueryClient;
    invalidateForEvent(queryClient, { type: "pool_probe", data: { pool: "default" }, receivedAt: "2026-05-08T00:00:00Z" });
    expect(queryClient.invalidateQueries).toHaveBeenCalledWith({ queryKey: ["health"] });
  });

  it("invalidates configuration after provider metadata reloads", () => {
    const queryClient = { invalidateQueries: vi.fn() } as unknown as QueryClient;
    invalidateForEvent(queryClient, { type: "oauth_providers_reloaded", data: { ok: true }, receivedAt: "2026-05-08T00:00:00Z" });
    expect(queryClient.invalidateQueries).toHaveBeenCalledWith({ queryKey: ["pools"] });
    expect(queryClient.invalidateQueries).toHaveBeenCalledWith({ queryKey: ["credentials"] });
    expect(queryClient.invalidateQueries).toHaveBeenCalledWith({ queryKey: ["health"] });
  });
});
