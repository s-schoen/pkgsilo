import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { App } from "@/App.tsx";
import { getLocale } from "@/paraglide/runtime.js";

const root = document.getElementById("root");
if (!root) {
  throw new Error("index.html has no #root element");
}

// index.html says en, which is right only for English.
document.documentElement.lang = getLocale();

createRoot(root).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
