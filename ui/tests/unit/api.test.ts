import { afterEach, describe, expect, it, vi } from "vitest";

import { api, ApiClientError } from "../../src/lib/api";

describe("api", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("returns JSON responses", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response(JSON.stringify({ ok: true }), { status: 200, headers: { "Content-Type": "application/json" } }))
    );
    await expect(api<{ ok: boolean }>("/admin/v1/health")).resolves.toEqual({ ok: true });
  });

  it("throws typed errors", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response(JSON.stringify({ error: "nope" }), { status: 500, headers: { "Content-Type": "application/json" } }))
    );
    await expect(api("/bad")).rejects.toBeInstanceOf(ApiClientError);
  });
});
