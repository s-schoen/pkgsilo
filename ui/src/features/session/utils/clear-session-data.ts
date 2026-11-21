import type { DbClient } from "@tanstack/react-db";
import type { QueryClient } from "@tanstack/react-query";

// Removes what the clients hold of a session that ended: every collection of the DbClient, which
// keeps its rows apart from the QueryClient, and every cached response. They belong to that session,
// and the next one, maybe another identity's, must not see them. Call it once the sign-in page
// shows, when no page reads them anymore; a collection that's read again loads anew.
export async function clearSessionData({
  queryClient,
  dbClient,
}: {
  queryClient: QueryClient;
  dbClient: DbClient;
}) {
  try {
    await dbClient.cleanup();
  } finally {
    queryClient.removeQueries();
  }
}
