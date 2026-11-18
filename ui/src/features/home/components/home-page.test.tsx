import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { HomePage } from "@/features/home/components/home-page.tsx";
import { m } from "@/paraglide/messages.js";

describe("HomePage", () => {
  it("welcomes in English by default", () => {
    render(<HomePage />);

    expect(screen.getByText(m.welcome({}, { locale: "en" }))).toBeInTheDocument();
  });

  it("welcomes in German when the browser prefers it", () => {
    vi.spyOn(navigator, "languages", "get").mockReturnValue(["de-DE", "en"]);

    render(<HomePage />);

    expect(screen.getByText(m.welcome({}, { locale: "de" }))).toBeInTheDocument();
  });
});
