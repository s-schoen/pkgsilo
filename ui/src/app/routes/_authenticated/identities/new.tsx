import { createFileRoute } from "@tanstack/react-router";

import { NotFound } from "@/components/errors/not-found.tsx";
import { identitiesCollection } from "@/features/identities/api/identities-collection.ts";
import { NewIdentityPage } from "@/features/identities/components/new-identity-page.tsx";
import { identityCreateActions } from "@/features/identities/hooks/use-identity-permissions.ts";
import { requirePermission } from "@/features/permissions/utils/require-permission.ts";

// A static segment wins over $id, so /identities/new is this page, not an identity's. Without the
// create of every field that the page sends, the rule of useIdentityPermissions' canCreate, it's a
// page that doesn't exist, which its own notFoundComponent renders in the app's frame.
export const Route = createFileRoute("/_authenticated/identities/new")({
  beforeLoad: async ({ context }) => {
    await requirePermission(context.queryClient, identityCreateActions);
  },
  loader: async ({ context }) => {
    await context.dbClient.collection(identitiesCollection).preload();
  },
  component: NewIdentityPage,
  notFoundComponent: NotFound,
});
