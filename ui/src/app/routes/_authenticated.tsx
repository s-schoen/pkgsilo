import { createFileRoute, redirect } from "@tanstack/react-router";

import { getGetIdentityQueryOptions } from "@/api/gen/client/identities/identities.ts";
import { getGetCurrentSessionQueryOptions } from "@/api/gen/client/sessions/sessions.ts";
import { AuthenticatedLayout } from "@/app/authenticated-layout.tsx";
import { isUnauthorized } from "@/features/session/utils/unauthorized.ts";

// The pages that need a session, in the app's frame: every route below routes/_authenticated/.
// Without a session, a page leads to the sign-in page, with its own URL as the redirect, so that a
// sign-in leads back to it. A session ending later on a page leads there too
// (redirectOnUnauthorized in router.tsx).
export const Route = createFileRoute("/_authenticated")({
  beforeLoad: async ({ context, location }) => {
    try {
      const session = await context.queryClient.ensureQueryData(getGetCurrentSessionQueryOptions());
      return { identityId: session.data.data.identityId };
    } catch (error) {
      if (isUnauthorized(error)) {
        throw redirect({ to: "/signin", search: { redirect: location.href }, replace: true });
      }
      throw error;
    }
  },
  // The account's menu shows the identity: it starts loading with the page. Policies may hide it
  // from itself, which the menu shows too, so its failure fails no page.
  loader: ({ context }) => {
    if (context.identityId !== undefined) {
      void context.queryClient.prefetchQuery(getGetIdentityQueryOptions(context.identityId));
    }
  },
  component: AuthenticatedLayout,
});
