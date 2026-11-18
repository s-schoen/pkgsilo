import { createFileRoute } from "@tanstack/react-router";

import { HomePage } from "@/features/home/components/home-page.tsx";

export const Route = createFileRoute("/")({
  component: HomePage,
});
