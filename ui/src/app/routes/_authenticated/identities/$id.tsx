import { createFileRoute } from "@tanstack/react-router";

import { identitiesCollection } from "@/features/identities/api/identities-collection.ts";
import { IdentityPage } from "@/features/identities/components/identity-page.tsx";

export const Route = createFileRoute("/_authenticated/identities/$id")({
  loader: async ({ context }) => {
    await context.dbClient.collection(identitiesCollection).preload();
  },
  component: IdentityPage,
});
