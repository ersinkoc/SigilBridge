import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it } from "vitest";

import App from "../../src/App";

describe("App", () => {
  it("renders the English dashboard shell", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
          <App />
        </MemoryRouter>
      </QueryClientProvider>
    );
    expect((await screen.findAllByText("SigilBridge"))[0]).toBeInTheDocument();
    expect(await screen.findByText("Dashboard")).toBeInTheDocument();
  });
});
