import IconTrash from "@tabler/icons-react/icons/IconTrash";
import { eq, useLiveQuery } from "@tanstack/react-db";
import { getRouteApi, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { useGetCurrentSession } from "@/api/gen/client/sessions/sessions.ts";
import { NotFound } from "@/components/errors/not-found.tsx";
import { DetailLayout } from "@/components/layouts/detail-layout.tsx";
import { PageHeader } from "@/components/layouts/page-header.tsx";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { ActionMenu } from "@/components/ui/menu";
import { identitiesCollection } from "@/features/identities/api/identities-collection.ts";
import { IdentityProperties } from "@/features/identities/components/identity-properties.tsx";
import { IdentityTextField } from "@/features/identities/components/identity-text-field.tsx";
import { useDeleteIdentities } from "@/features/identities/hooks/use-delete-identities.ts";
import { useSaveIdentityField } from "@/features/identities/hooks/use-save-identity-field.ts";
import { identityName } from "@/features/identities/utils/identity-name.ts";
import { displayNameSchema } from "@/features/identities/utils/identity-schema.ts";
import { m } from "@/paraglide/messages.js";

import type { IdentityResponse } from "@/api/gen/client/model/identityResponse.ts";

const route = getRouteApi("/_authenticated/identities/$id");

// An identity's page, at /identities/<id>: its name as the title and its properties in the
// sidebar, from identitiesCollection, which the route's loader has loaded. The user edits the
// display name and the username in place; each save patches that one field. An identity that isn't
// there, since it doesn't exist or the actor can't read it, is a page that doesn't exist. The
// header's ⋯ menu deletes the identity, unless it's the signed-in one.
export function IdentityPage() {
  const { id } = route.useParams();
  const { data: live } = useLiveQuery((q) =>
    q
      .from({ identity: identitiesCollection })
      .where(({ identity: row }) => eq(row.id, id))
      .findOne(),
  );
  // The identity that's being deleted: the collection drops it at once, but the page shows it
  // until the API has answered, and then leaves, or keeps it if the delete failed.
  const [deleting, setDeleting] = useState<IdentityResponse>();
  const identity = live ?? deleting;
  const save = useSaveIdentityField(id);
  const session = useGetCurrentSession();
  const currentId = session.data?.data.data.identityId;

  if (identity === undefined) {
    return <NotFound />;
  }

  const name = identityName(identity);
  return (
    <>
      <PageHeader
        breadcrumbs={[{ label: m.identities(), href: "/identities" }, { label: name }]}
        actions={[
          {
            id: "menu",
            element: <IdentityActions identity={identity} onDeletingChange={setDeleting} />,
            // The signed-in identity can't delete itself; nothing is offered until the session
            // tells which one that is.
            isVisible: currentId !== undefined && currentId !== identity.id,
          },
        ]}
      />
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

type IdentityActionsProps = {
  identity: IdentityResponse;
  // Gets the identity while its delete runs, and undefined once it failed.
  onDeletingChange: (identity: IdentityResponse | undefined) => void;
};

// The ⋯ menu of an identity's page, with Delete, which asks first. Once the API has deleted it, the
// page goes to the list; if the delete fails, which a toast reports, the dialog and the page stay.
function IdentityActions({ identity, onDeletingChange }: IdentityActionsProps) {
  const [isDeleteOpen, setDeleteOpen] = useState(false);
  const deleteIdentities = useDeleteIdentities();
  const navigate = useNavigate();
  const name = identityName(identity);

  const confirmDelete = async () => {
    onDeletingChange(identity);
    const { deleted } = await deleteIdentities([identity]);
    if (deleted === 0) {
      onDeletingChange(undefined);
      throw new Error("the identity wasn't deleted");
    }
    await navigate({ to: "/identities" });
  };

  return (
    <>
      <ActionMenu
        aria-label={m.identityActions()}
        items={[
          {
            id: "delete",
            label: m.identityDelete(),
            icon: IconTrash,
            variant: "danger",
            onAction: () => {
              setDeleteOpen(true);
            },
          },
        ]}
      />
      <ConfirmDialog
        isOpen={isDeleteOpen}
        onOpenChange={setDeleteOpen}
        title={m.identityDeleteTitle({ name })}
        description={m.identityDeleteDescription()}
        confirmLabel={m.identityDelete()}
        onConfirm={confirmDelete}
      />
    </>
  );
}
