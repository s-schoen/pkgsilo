import { onErrorResponse } from "@/api/fetch.ts";
import { permissionsQueryKeyPrefix } from "@/features/permissions/api/permissions-query.ts";

import type { QueryClient } from "@tanstack/react-query";

// A path segment that names one object: a UUID, or "me" for the signed-in identity.
const objectSegment = /^(?:me|[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})$/i;

// The path of the collection that the request to path is about: path itself, or without its last
// segment if that names one of the collection's objects. /api/v1/identities/<id> is about
// /api/v1/identities, /api/v1/identities/<id>/api-keys about itself.
export function collectionPath(path: string): string {
  const segments = path.split("/");
  return objectSegment.test(segments.at(-1) ?? "") ? segments.slice(0, -1).join("/") : path;
}

// Whether a query's key, whose first element is its path as the generated client builds it, is of
// the collection at path: the list or one of its objects.
function isOfCollection(queryKey: ReadonlyArray<unknown>, path: string): boolean {
  const [queryPath] = queryKey;
  if (typeof queryPath !== "string") {
    return false;
  }
  return (
    queryPath === path ||
    (queryPath.startsWith(`${path}/`) && !queryPath.slice(path.length + 1).includes("/"))
  );
}

// Refreshes what a denied request (403, fields-denied too) shows to be outdated, for every request
// of the generated client, whichever query, mutation or collection sent it: the signed-in identity's
// permissions, and the collection that the request was about, whose objects' allowedActions may
// have changed too. A collection of TanStack DB loads through its query, so it refetches too.
// Returns the function that stops it.
export function invalidateOnForbidden(queryClient: QueryClient): () => void {
  return onErrorResponse(({ status, path }) => {
    if (status !== 403) {
      return;
    }
    // Every list, even one that no component shows at the moment: a route's requirePermission
    // reads it from the cache.
    void queryClient.invalidateQueries({ queryKey: permissionsQueryKeyPrefix, refetchType: "all" });
    const collection = collectionPath(path);
    void queryClient.invalidateQueries({
      predicate: (query) => isOfCollection(query.queryKey, collection),
    });
  });
}
