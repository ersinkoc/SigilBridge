import { expect, type Page, test } from "@playwright/test";

async function mockAdminAPI(page: Page) {
  let sessionImported = false;
  let apiKeyStored = false;
  let cliEnabled = false;
  let keySettingsSaved = false;
  let savedPoolUpstreams: Array<Record<string, unknown>> = [];

  await page.route("**/admin/v1/auth/login", (route) => route.fulfill({ json: { ok: true } }));
  await page.route("**/admin/v1/events/stream", (route) => route.fulfill({ status: 200, contentType: "text/event-stream", body: "" }));
  await page.route("**/admin/v1/keys/key_e2e", async (route) => {
    if (route.request().method() === "PATCH") {
      const body = (await route.request().postDataJSON()) as Record<string, unknown>;
      keySettingsSaved = Boolean(body.budgets && body.scopes && body.rate_limits);
      await route.fulfill({
        json: {
          id: "key_e2e",
          name: body.name ?? "e2e",
          hash: "sha256:test",
          budgets: body.budgets ?? { daily_cents: 0, monthly_cents: 0, hard_cap: true },
          scopes: body.scopes ?? {},
          rate_limits: body.rate_limits ?? {}
        }
      });
      return;
    }
    await route.fulfill({
      json: {
        id: "key_e2e",
        name: "e2e",
        hash: "sha256:test",
        created_at: "2026-05-07T00:00:00Z",
        budgets: { daily_cents: 100, monthly_cents: 1000, hard_cap: true },
        scopes: { allowed_pools: ["mock-chat"], allowed_models: [], ip_allowlist: [] },
        rate_limits: { rpm: 60, tpm: 1000 }
      }
    });
  });
  await page.route("**/admin/v1/keys", async (route) => {
    if (route.request().method() === "POST") {
      await route.fulfill({ status: 201, json: { id: "key_e2e", name: "e2e", hash: "sha256:test", plaintext: "sb_test_e2esecret" } });
      return;
    }
    await route.fulfill({ json: [{ id: "key_e2e", name: "e2e", created_at: "2026-05-07T00:00:00Z" }] });
  });
  await page.route("**/admin/v1/pools", async (route) => {
    if (route.request().method() === "POST") {
      const body = (await route.request().postDataJSON()) as Record<string, unknown>;
      savedPoolUpstreams = (body.upstreams as Array<Record<string, unknown>> | undefined) ?? [];
      await route.fulfill({ json: { id: body.id, strategy: body.strategy, upstreams: savedPoolUpstreams } });
      return;
    }
    await route.fulfill({ json: [{ id: "mock-chat", strategy: "priority", upstreams: savedPoolUpstreams }] });
  });
  await page.route("**/admin/v1/endpoints", (route) =>
    route.fulfill({
      json: {
        openai_base: "http://127.0.0.1:8187/v1",
        openai_chat: "http://127.0.0.1:8187/v1/chat/completions",
        openai_models: "http://127.0.0.1:8187/v1/models",
        anthropic_base: "http://127.0.0.1:8187",
        anthropic_messages: "http://127.0.0.1:8187/v1/messages"
      }
    })
  );
  await page.route("**/admin/v1/chat/test", async (route) => {
    const body = (await route.request().postDataJSON()) as Record<string, unknown>;
    await route.fulfill({
      json: {
        id: "chat_e2e",
        model: body.model,
        upstream_provider: "mock",
        upstream_model: "mock-chat",
        content: "dashboard tester ok",
        stop_reason: "end_turn",
        latency_ms: 12,
        input_tokens: 3,
        output_tokens: 4
      }
    });
  });
  await page.route("**/admin/v1/credentials/session", async (route) => {
    sessionImported = true;
    await route.fulfill({ status: 201, json: { ok: true, id: "vault://claude_web/e2e", provider: "claude_web" } });
  });
  await page.route("**/admin/v1/credentials/api-key", async (route) => {
    apiKeyStored = true;
    await route.fulfill({ status: 201, json: { ok: true, id: "vault://apikey/openai_api/e2e", provider: "openai_api", pool: "openai" } });
  });
  await page.route("**/admin/v1/provider-catalog", async (route) => {
    await route.fulfill({
      json: {
        source: "test",
        providers: [
          {
            id: "openai",
            name: "OpenAI",
            provider: "openai_api",
            category: "api_key",
            base_url: "https://api.openai.com",
            model_count: 1,
            top_models: [{ id: "gpt-test", name: "gpt-test" }]
          },
          { id: "codex_cli", name: "Codex CLI", provider: "codex_cli", category: "cli_acp", available: true }
        ]
      }
    });
  });
  await page.route("**/admin/v1/credentials/cli/detect", (route) =>
    route.fulfill({ json: { agents: [{ provider: "codex_cli", command: "codex", path: "C:\\Tools\\codex.exe", available: true, configured: cliEnabled }] } })
  );
  await page.route("**/admin/v1/credentials/cli/enable", async (route) => {
    cliEnabled = true;
    await route.fulfill({ json: { ok: true, provider: "codex_cli", pool: "codex", upstream: "codex_cli-local" } });
  });
  await page.route("**/admin/v1/credentials", async (route) => {
    await route.fulfill({
      json: {
        oauth_providers: [],
        api_keys: apiKeyStored ? [{ id: "vault://apikey/openai_api/e2e", provider: "openai_api", created_at: "2026-05-07T00:00:00Z" }] : [],
        sessions: sessionImported ? [{ id: "vault://claude_web/e2e", provider: "claude_web", created_at: "2026-05-07T00:00:00Z" }] : [],
        cli: { enabled: true, agents: cliEnabled ? [{ provider: "codex_cli", command: "codex", pool: "codex", upstream: "codex_cli-local", available: true }] : [] }
      }
    });
  });
  await page.route("**/admin/v1/credentials/cli", (route) =>
    route.fulfill({ json: { enabled: true, agents: cliEnabled ? [{ provider: "codex_cli", command: "codex", pool: "codex", upstream: "codex_cli-local", available: true }] : [] } })
  );
  await page.route("**/admin/v1/health", (route) => route.fulfill({ json: { upstreams: [], cooldowns: [] } }));
  await page.route("**/admin/v1/audit**", (route) =>
    route.fulfill({ json: { items: [{ request_id: "req_1", status: "ok", pool_name: "mock-chat", cost_cents: 0 }], next_cursor: "" } })
  );
  await page.route("**/admin/v1/budgets", (route) => route.fulfill({ json: { keys: 1, daily_cents: 100, monthly_cents: 1000, daily_used_cents: 10, monthly_used_cents: 25 } }));
  await page.route("**/admin/v1/usage", (route) => route.fulfill({ json: { items: [{ key_id: "key_e2e", daily_cents: 10, monthly_cents: 25, daily_budget_cents: 100, monthly_budget_cents: 1000, hard_cap: true }] } }));
  await page.exposeFunction("e2eState", () => ({ keySettingsSaved, savedPoolUpstreams }));
}

test.beforeEach(async ({ page }) => {
  await mockAdminAPI(page);
});

test("renders the dashboard and toggles theme", async ({ page }) => {
  await page.goto("/admin/ui/");
  await expect(page.getByRole("heading", { name: "SigilBridge" })).toBeVisible();
  await expect(page.getByRole("navigation").getByRole("link", { name: "Keys", exact: true })).toBeVisible();
  await expect(page.getByText("http://127.0.0.1:8187/v1/chat/completions")).toBeVisible();
  await expect(page.getByText("http://127.0.0.1:8187/v1/messages")).toBeVisible();
  await page.getByRole("button", { name: "Open command palette" }).click();
  await page.getByPlaceholder("Search screens and actions").fill("audit");
  await page.getByRole("button", { name: /Audit Request history/ }).click();
  await expect(page).toHaveURL(/\/admin\/ui\/audit$/);
  await page.goto("/admin/ui/");
  await page.getByRole("button", { name: "Open command palette" }).waitFor();
  await page.locator("body").focus();
  await page.keyboard.press("Control+K");
  await page.getByPlaceholder("Search screens and actions").fill("budget");
  await page.keyboard.press("Enter");
  await expect(page).toHaveURL(/\/admin\/ui\/budgets$/);
  await page.goto("/admin/ui/");
  await page.getByLabel("Dark").click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await page.getByRole("button", { name: "Send" }).click();
  await expect(page.getByText("dashboard tester ok", { exact: true })).toBeVisible();
  await page.goto("/admin/ui/setup");
  await expect(page.getByRole("heading", { name: "Guided setup" })).toBeVisible();
  await expect(page.getByText("40% complete")).toBeVisible();
  await expect(page.getByText("Current step")).toBeVisible();
  await expect(page.getByRole("heading", { name: "Add provider authentication" })).toBeVisible();
});

test("covers key, pool, credential, audit, and login workflows", async ({ page }) => {
  await page.goto("/admin/ui/login");
  await page.getByLabel("Admin token").fill("admin_test");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page).toHaveURL(/\/admin\/ui\/$/);

  await page.goto("/admin/ui/keys/new");
  await page.getByLabel("Name").fill("e2e");
  await page.getByRole("button", { name: "Create" }).click();
  await expect(page.getByText("sb_test_e2esecret")).toBeVisible();

  await page.goto("/admin/ui/keys/key_e2e");
  await page.getByLabel("Daily cents").fill("250");
  await page.getByLabel("Allowed pools").fill("mock-chat, fallback");
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText("Key settings saved")).toBeVisible();
  await page.getByRole("button", { name: "Revoke" }).click();
  await expect(page.getByRole("dialog", { name: "Revoke bridge key" })).toBeVisible();
  await page.getByRole("button", { name: "Cancel" }).click();
  await expect(page.getByRole("dialog", { name: "Revoke bridge key" })).toBeHidden();
  await page.getByRole("button", { name: "Delete" }).click();
  await expect(page.getByRole("dialog", { name: "Delete bridge key" })).toBeVisible();
  await page.getByRole("button", { name: "Cancel" }).click();
  await expect(page.getByRole("dialog", { name: "Delete bridge key" })).toBeHidden();

  await page.goto("/admin/ui/pools/e2e-pool");
  await page.getByLabel("Pool alias").fill("e2e-pool");
  await page.getByRole("button", { name: "Add" }).click();
  await page.getByLabel("Compatible adapter").selectOption("openai_api");
  await page.getByRole("slider", { name: "Weight" }).fill("80");
  await page.getByRole("button", { name: "Save pool" }).click();
  await expect(page.getByText("Pool saved")).toBeVisible();

  await page.goto("/admin/ui/credentials/api-key/new");
  await page.getByRole("button", { name: /OpenAI/ }).click();
  await page.getByLabel("API key").fill("sk-test");
  await page.getByRole("button", { name: "Continue" }).click();
  await page.getByRole("button", { name: "Save and attach" }).click();
  await expect(page.getByText("API key stored and route updated")).toBeVisible();
  await page.goto("/admin/ui/credentials");
  await expect(page.getByText("vault://apikey/openai_api/e2e")).toBeVisible();
  await page.getByRole("button", { name: "Delete" }).click();
  await expect(page.getByRole("dialog", { name: "Delete API key" })).toBeVisible();
  await page.getByRole("button", { name: "Cancel" }).click();
  await expect(page.getByRole("dialog", { name: "Delete API key" })).toBeHidden();

  await page.goto("/admin/ui/credentials/sessions/new");
  await page.getByLabel("Name").fill("e2e");
  await page.getByLabel("User agent").fill("Playwright");
  await page.getByLabel("Cookies JSON").fill('{"session":"s1"}');
  await page.getByRole("button", { name: "Store session" }).click();
  await expect(page.getByText("vault://claude_web/e2e")).toBeVisible();

  await page.goto("/admin/ui/credentials/cli");
  await page.getByRole("button", { name: "Enable" }).click();
  await expect(page.getByText("CLI upstream enabled")).toBeVisible();

  await page.goto("/admin/ui/audit");
  await page.getByLabel("Pool").fill("mock-chat");
  await page.getByRole("button", { name: "Apply" }).click();
  await expect(page.getByText("req_1")).toBeVisible();

  const state = await page.evaluate(async () => (window as unknown as { e2eState: () => Promise<{ keySettingsSaved: boolean; savedPoolUpstreams: Array<Record<string, unknown>> }> }).e2eState());
  expect(state.keySettingsSaved).toBe(true);
  expect(state.savedPoolUpstreams).toHaveLength(1);
});
