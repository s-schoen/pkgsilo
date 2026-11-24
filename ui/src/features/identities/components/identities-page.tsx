import IconPlus from "@tabler/icons-react/icons/IconPlus";
import IconUsers from "@tabler/icons-react/icons/IconUsers";
import { useLiveQuery } from "@tanstack/react-db";
import { getRouteApi, useNavigate } from "@tanstack/react-router";
import { useMemo } from "react";

import { useGetCurrentSession } from "@/api/gen/client/sessions/sessions.ts";
import { PageHeader } from "@/components/layouts/page-header.tsx";
import { Button } from "@/components/ui/button";
import { DataTable, DataTableEmptyState, useDataTableSearch } from "@/components/ui/data-table";
import { identitiesCollection } from "@/features/identities/api/identities-collection.ts";
import {
  IDENTITY_FILTERS,
  identityColumns,
  identityGroupingOptions,
} from "@/features/identities/components/identity-columns.tsx";
import { useDeleteIdentities } from "@/features/identities/hooks/use-delete-identities.ts";
import { m } from "@/paraglide/messages.js";
import { routeHref } from "@/utils/route-href.ts";

import type { DataTableSearchDefaults } from "@/components/ui/data-table";

const route = getRouteApi("/_authenticated/identities/");

// Ordered by display name, until the user picks another order.
const defaults: DataTableSearchDefaults = { sorting: [{ id: "displayName", desc: false }] };

// The list of identities, at /identities: every identity the actor may read, which the route's
// loader has loaded into identitiesCollection. Its search, filter, order and grouping live in the
// URL. The user deletes the selected identities, except the signed-in one.
export function IdentitiesPage() {
  const { data: identities, isLoading } = useLiveQuery((q) =>
    q.from({ identity: identitiesCollection }),
  );
  const session = useGetCurrentSession();
  const currentId = session.data?.data.data.identityId;
  const search = useDataTableSearch(route, { filters: IDENTITY_FILTERS, defaults });
  const navigate = useNavigate();
  const deleteIdentities = useDeleteIdentities();
  const newIdentity = () => {
    void navigate({ to: "/identities/new" });
  };

  const byId = useMemo(
    () => new Map(identities.map((identity) => [identity.id, identity])),
    [identities],
  );
  const columns = useMemo(() => identityColumns({ byId, currentId }), [byId, currentId]);
  const groupingOptions = useMemo(() => identityGroupingOptions(byId), [byId]);

  return (
    <>
      <PageHeader
        title={m.identities()}
        actions={[
          {
            id: "new",
            element: (
              <Button icon={IconPlus} onPress={newIdentity}>
                {m.identityNew()}
              </Button>
            ),
          },
        ]}
      />
      <DataTable
        label={m.identities()}
        columns={columns}
        rows={identities}
        isLoading={isLoading}
        getRowId={(identity) => identity.id}
        getRowHref={(identity) => routeHref("/identities/$id", { id: identity.id })}
        groupingOptions={groupingOptions}
        onDelete={async (selected) => {
          await deleteIdentities(selected);
        }}
        // The signed-in identity can't delete itself; nothing is deleted until the session tells
        // which one that is.
        canDelete={(identity) => currentId !== undefined && identity.id !== currentId}
        emptyState={
          <DataTableEmptyState icon={IconUsers} message={m.identitiesEmpty()}>
            <Button variant="secondary" onPress={newIdentity}>
              {m.identityNew()}
            </Button>
          </DataTableEmptyState>
        }
        {...search}
      />
    </>
  );
}
