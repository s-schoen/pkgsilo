import { createFileRoute } from "@tanstack/react-router";

import { NotFound } from "@/components/errors/not-found.tsx";

// Every path that no other route takes: the page that doesn't exist, in the app's frame, once the
// user is signed in.
export const Route = createFileRoute("/_authenticated/$")({
  component: NotFound,
});
