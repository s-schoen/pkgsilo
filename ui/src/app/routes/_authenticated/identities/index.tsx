import { createFileRoute } from "@tanstack/react-router";

import { dataTableSearchSchema } from "@/components/ui/data-table";
import { identitiesCollection } from "@/features/identities/api/identities-collection.ts";
import { IdentitiesPage } from "@/features/identities/components/identities-page.tsx";
import { IDENTITY_FILTERS } from "@/features/identities/components/identity-columns.tsx";

export const Route = createFileRoute("/_authenticated/identities/")({
  validateSearch: dataTableSearchSchema({ filters: IDENTITY_FILTERS }),
  loader: async ({ context }) => {
    await context.dbClient.collection(identitiesCollection).preload();
  },
  component: IdentitiesPage,
});
