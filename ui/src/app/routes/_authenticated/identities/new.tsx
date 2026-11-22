import { createFileRoute } from "@tanstack/react-router";

import { identitiesCollection } from "@/features/identities/api/identities-collection.ts";
import { NewIdentityPage } from "@/features/identities/components/new-identity-page.tsx";

// A static segment wins over $id, so /identities/new is this page, not an identity's.
export const Route = createFileRoute("/_authenticated/identities/new")({
  loader: async ({ context }) => {
    await context.dbClient.collection(identitiesCollection).preload();
  },
  component: NewIdentityPage,
});
