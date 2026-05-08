import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ThemeSwitcher } from "../../src/components/layout/ThemeSwitcher";

describe("ThemeSwitcher", () => {
  it("sets the document theme", () => {
    render(<ThemeSwitcher />);
    fireEvent.click(screen.getByLabelText("Dark"));
    expect(document.documentElement.dataset.theme).toBe("dark");
  });
});
