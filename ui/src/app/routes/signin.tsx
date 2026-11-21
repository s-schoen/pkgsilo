import { createFileRoute, redirect } from "@tanstack/react-router";
import { z } from "zod/v4";

import { getGetCurrentSessionQueryOptions } from "@/api/gen/client/sessions/sessions.ts";
import { SignInPage } from "@/features/session/components/sign-in-page.tsx";
import { safeRedirectTarget } from "@/features/session/utils/redirect-target.ts";

// The page to go to after the sign-in, which safeRedirectTarget checks; a malformed one is dropped.
const searchSchema = z.object({ redirect: z.string().optional().catch(undefined) });

// The sign-in page. A user who is signed in already goes on to the redirect at once. It always asks
// the API, not the cache: after a sign-out or an expiry, the cached session is the one that ended.
export const Route = createFileRoute("/signin")({
  validateSearch: searchSchema,
  beforeLoad: async ({ context, search }) => {
    try {
      await context.queryClient.fetchQuery(getGetCurrentSessionQueryOptions());
    } catch {
      // Not signed in (401), or the API can't tell: the form shows, and a sign-in tells the rest.
      return;
    }
    throw redirect({ href: safeRedirectTarget(search.redirect), replace: true });
  },
  component: SignInPage,
});
