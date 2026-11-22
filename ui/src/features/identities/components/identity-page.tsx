import { eq, useLiveQuery } from "@tanstack/react-db";
import { getRouteApi } from "@tanstack/react-router";

import { NotFound } from "@/components/errors/not-found.tsx";
import { DetailLayout } from "@/components/layouts/detail-layout.tsx";
import { PageHeader } from "@/components/layouts/page-header.tsx";
import { identitiesCollection } from "@/features/identities/api/identities-collection.ts";
import { IdentityProperties } from "@/features/identities/components/identity-properties.tsx";
import { identityName } from "@/features/identities/utils/identity-name.ts";
import { m } from "@/paraglide/messages.js";

const route = getRouteApi("/_authenticated/identities/$id");

// An identity's page, at /identities/<id>: its name as the title and its properties in the
// sidebar, from identitiesCollection, which the route's loader has loaded. An identity that isn't
// there, since it doesn't exist or the actor can't read it, is a page that doesn't exist.
export function IdentityPage() {
  const { id } = route.useParams();
  const { data: identity } = useLiveQuery((q) =>
    q
      .from({ identity: identitiesCollection })
      .where(({ identity: row }) => eq(row.id, id))
      .findOne(),
  );

  if (identity === undefined) {
    return <NotFound />;
  }

  const name = identityName(identity);
  return (
    <>
      <PageHeader breadcrumbs={[{ label: m.identities(), href: "/identities" }, { label: name }]} />
      <DetailLayout title={<h1>{name}</h1>} sidebar={<IdentityProperties identity={identity} />} />
    </>
  );
}
