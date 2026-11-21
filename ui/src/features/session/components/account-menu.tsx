import IconLogout from "@tabler/icons-react/icons/IconLogout";
import IconSelector from "@tabler/icons-react/icons/IconSelector";
import IconUser from "@tabler/icons-react/icons/IconUser";
import { Button } from "react-aria-components/Button";

import { useGetIdentity } from "@/api/gen/client/identities/identities.ts";
import {
  useDeleteCurrentSession,
  useGetCurrentSession,
} from "@/api/gen/client/sessions/sessions.ts";
import { Menu, MenuItem, MenuSection, MenuTrigger } from "@/components/ui/menu";
import { showToast } from "@/components/ui/toast";
import { initials } from "@/features/session/utils/initials.ts";
import { m } from "@/paraglide/messages.js";

import styles from "@/features/session/components/account-menu.module.css";

// The row at the bottom of the navigation that shows who is signed in: the identity's initials and
// display name, or its username, or "Signed in" while policies hide the identity from itself. It
// opens a menu with the username and the sign-out. onSignedOut follows a sign-out, also when the
// session had already ended; a sign-out that fails keeps the user signed in and says so in a toast.
export function AccountMenu({ onSignedOut }: { onSignedOut: () => void }) {
  const session = useGetCurrentSession();
  const identityId = session.data?.data.data.identityId;
  const identity = useGetIdentity(identityId ?? "", {
    query: { enabled: identityId !== undefined },
  });
  const deleteSession = useDeleteCurrentSession();

  const username = identity.data?.data.data.username;
  const name = identity.data?.data.data.displayName ?? username;

  function signOut() {
    deleteSession.mutate(undefined, {
      onSuccess: onSignedOut,
      onError: (error) => {
        // 401: the session had ended; 404: the request had none. Either way, none is left.
        if (error.status === 401 || error.status === 404) {
          onSignedOut();
          return;
        }
        showToast({ title: m.signOutFailed(), variant: "danger" });
      },
    });
  }

  return (
    <MenuTrigger>
      <Button className={styles.trigger}>
        {name === undefined ? (
          <IconUser aria-hidden className={styles.icon} />
        ) : (
          <span aria-hidden className={styles.avatar}>
            {initials(name)}
          </span>
        )}
        <span className={styles.name}>{name ?? m.signedIn()}</span>
        <IconSelector aria-hidden className={styles.icon} />
      </Button>
      <Menu
        aria-label={m.account()}
        placement="top start"
        onAction={(key) => {
          if (key === "signOut") {
            signOut();
          }
        }}
      >
        <MenuSection title={username === undefined ? undefined : m.signedInAs({ username })}>
          <MenuItem id="signOut" icon={IconLogout}>
            {m.signOut()}
          </MenuItem>
        </MenuSection>
      </Menu>
    </MenuTrigger>
  );
}
