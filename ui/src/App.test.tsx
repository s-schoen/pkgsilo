import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { App } from "@/App.tsx";

describe("App", () => {
  it("shows the product name", () => {
    render(<App />);

    expect(screen.getByRole("heading", { name: "pkgsilo" })).toBeInTheDocument();
  });

  it("welcomes in English by default", () => {
    render(<App />);

    expect(screen.getByText("Welcome to pkgsilo.")).toBeInTheDocument();
  });

  it("welcomes in German when the browser prefers it", () => {
    vi.spyOn(navigator, "languages", "get").mockReturnValue(["de-DE", "en"]);

    render(<App />);

    expect(screen.getByText("Willkommen bei pkgsilo.")).toBeInTheDocument();
  });
});
