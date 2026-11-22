import IconAt from "@tabler/icons-react/icons/IconAt";
import { useNavigate } from "@tanstack/react-router";

import { useAppForm } from "@/components/form/use-app-form.ts";
import { DetailLayout } from "@/components/layouts/detail-layout.tsx";
import { PageHeader } from "@/components/layouts/page-header.tsx";
import { Button } from "@/components/ui/button";
import { PropertyItem, PropertyList } from "@/components/ui/property-list";
import { showToast } from "@/components/ui/toast";
import { useCreateIdentity } from "@/features/identities/hooks/use-create-identity.ts";
import { identityName } from "@/features/identities/utils/identity-name.ts";
import { createIdentitySchema } from "@/features/identities/utils/identity-schema.ts";
import { m } from "@/paraglide/messages.js";

import styles from "@/features/identities/components/new-identity-page.module.css";

// The page that creates an identity, at /identities/new: it looks as the identity's page will, with
// the display name as the large title and the username in the sidebar, always editing. Create, or
// Enter in a field, submits; a taken username shows on its field, and any other failure keeps the
// input after its toast. Once created, the identity's page replaces this one.
export function NewIdentityPage() {
  const navigate = useNavigate();
  const create = useCreateIdentity();
  const form = useAppForm({
    defaultValues: { displayName: "", username: "" },
    validators: { onChange: createIdentitySchema },
    onSubmit: async ({ value, formApi }) => {
      const result = await create(value);
      if (result.status === "username-taken") {
        // The field's next valid change clears it.
        formApi.setErrorMap({
          onSubmit: { fields: { username: m.identityUsernameTaken() } },
        });
        return;
      }
      if (result.status === "created") {
        showToast({
          title: m.identityCreated({ name: identityName(result.identity) }),
          variant: "success",
        });
        await navigate({
          to: "/identities/$id",
          params: { id: result.identity.id },
          replace: true,
        });
      }
    },
  });

  return (
    <form.AppForm>
      <form.Form layout="page">
        <PageHeader
          breadcrumbs={[{ label: m.identities(), href: "/identities" }, { label: m.identityNew() }]}
          actions={
            <div className={styles.actions}>
              <Button
                variant="secondary"
                onPress={() => {
                  void navigate({ to: "/identities" });
                }}
              >
                {m.cancel()}
              </Button>
              <form.SubmitButton label={m.identityCreate()} />
            </div>
          }
        />
        <DetailLayout
          title={
            <>
              <h1 className={styles.heading}>{m.identityNew()}</h1>
              <form.AppField name="displayName">
                {(field) => (
                  <field.TextField
                    label={m.identityDisplayName()}
                    placeholder={m.identityDisplayName()}
                    variant="title"
                    isLabelHidden
                  />
                )}
              </form.AppField>
            </>
          }
          sidebar={
            <PropertyList>
              <PropertyItem icon={IconAt} label={m.username()}>
                <form.AppField name="username">
                  {(field) => <field.TextField label={m.username()} isLabelHidden />}
                </form.AppField>
              </PropertyItem>
            </PropertyList>
          }
        />
      </form.Form>
    </form.AppForm>
  );
}
