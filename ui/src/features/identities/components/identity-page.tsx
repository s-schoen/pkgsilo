import { eq, useLiveQuery } from "@tanstack/react-db";
import { getRouteApi } from "@tanstack/react-router";

import { NotFound } from "@/components/errors/not-found.tsx";
import { DetailLayout } from "@/components/layouts/detail-layout.tsx";
import { PageHeader } from "@/components/layouts/page-header.tsx";
import { identitiesCollection } from "@/features/identities/api/identities-collection.ts";
import { IdentityProperties } from "@/features/identities/components/identity-properties.tsx";
import { IdentityTextField } from "@/features/identities/components/identity-text-field.tsx";
import { useSaveIdentityField } from "@/features/identities/hooks/use-save-identity-field.ts";
import { identityName } from "@/features/identities/utils/identity-name.ts";
import { displayNameSchema } from "@/features/identities/utils/identity-schema.ts";
import { m } from "@/paraglide/messages.js";

const route = getRouteApi("/_authenticated/identities/$id");

// An identity's page, at /identities/<id>: its name as the title and its properties in the
// sidebar, from identitiesCollection, which the route's loader has loaded. The user edits the
// display name and the username in place; each save patches that one field. An identity that isn't
// there, since it doesn't exist or the actor can't read it, is a page that doesn't exist.
export function IdentityPage() {
  const { id } = route.useParams();
  const { data: identity } = useLiveQuery((q) =>
    q
      .from({ identity: identitiesCollection })
      .where(({ identity: row }) => eq(row.id, id))
      .findOne(),
  );
  const save = useSaveIdentityField(id);

  if (identity === undefined) {
    return <NotFound />;
  }

  const name = identityName(identity);
  return (
    <>
      <PageHeader breadcrumbs={[{ label: m.identities(), href: "/identities" }, { label: name }]} />
      <DetailLayout
        title={
          // Without a readable display name, the title falls back to the username or the id,
          // which isn't the display name to edit.
          identity.displayName === undefined ? (
            <h1>{name}</h1>
          ) : (
            <IdentityTextField
              value={identity.displayName}
              schema={displayNameSchema}
              label={m.identityDisplayName()}
              placeholder={m.identityDisplayName()}
              variant="title"
              onSave={async (value) => save("displayName", value)}
            />
          )
        }
        sidebar={
          <IdentityProperties
            identity={identity}
            onSaveUsername={async (value) => save("username", value)}
          />
        }
      />
    </>
  );
}
