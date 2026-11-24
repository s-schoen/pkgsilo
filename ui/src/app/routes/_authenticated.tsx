import { createFileRoute, redirect } from "@tanstack/react-router";

import { getGetIdentityQueryOptions } from "@/api/gen/client/identities/identities.ts";
import { getGetCurrentSessionQueryOptions } from "@/api/gen/client/sessions/sessions.ts";
import { AuthenticatedLayout } from "@/app/authenticated-layout.tsx";
import { NotFound } from "@/components/errors/not-found.tsx";
import { permissionsQueryOptions } from "@/features/permissions/api/permissions-query.ts";
import { isUnauthorized } from "@/features/session/utils/unauthorized.ts";

// The pages that need a session, in the app's frame: every route below routes/_authenticated/.
// Without a session, a page leads to the sign-in page, with its own URL as the redirect, so that a
// sign-in leads back to it. A session ending later on a page leads there too
// (redirectOnUnauthorized in router.tsx). The check doesn't retry an error, which would keep the
// page blank for seconds before the error shows.
//
// The signed-in identity's permissions load together with the session, so that no page renders
// without them: the pages hide what the policies deny (usePermissions). A list that fails to load
// with a server error fails no page: it's unknown and hides nothing until it loads
// (permissionsQueryOptions); only its 401 leads to the sign-in page. A route that the identity
// may not use renders NotFound in the app's frame (requirePermission), with a notFoundComponent of
// its own: this route's would replace the frame.
export const Route = createFileRoute("/_authenticated")({
  beforeLoad: async ({ context, location }) => {
    try {
      const [session] = await Promise.all([
        context.queryClient.ensureQueryData({
          ...getGetCurrentSessionQueryOptions(),
          retry: false,
        }),
        context.queryClient.ensureQueryData({ ...permissionsQueryOptions(), retry: false }),
      ]);
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
  notFoundComponent: NotFound,
});
