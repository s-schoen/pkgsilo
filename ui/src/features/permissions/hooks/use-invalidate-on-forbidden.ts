import { useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";

import { invalidateOnForbidden } from "@/features/permissions/utils/invalidate-on-forbidden.ts";

// Refreshes the permissions and the affected collection after any denied request, while the
// component that calls it is mounted (invalidateOnForbidden). The authenticated layout calls it, so
// it covers every page that needs a session.
export function useInvalidateOnForbidden() {
  const queryClient = useQueryClient();
  useEffect(() => invalidateOnForbidden(queryClient), [queryClient]);
}
