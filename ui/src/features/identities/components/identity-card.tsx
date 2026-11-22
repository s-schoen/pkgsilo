import { IdentityAvatar } from "@/features/identities/components/identity-avatar.tsx";
import { handle, identityName } from "@/features/identities/utils/identity-name.ts";
import { m } from "@/paraglide/messages.js";

import type { IdentityResponse } from "@/api/gen/client/model/identityResponse.ts";

import styles from "@/features/identities/components/identity-card.module.css";

type IdentityCardProps = {
  // The identity, or undefined for one that the actor can't read.
  identity: IdentityResponse | undefined;
  // pkgsilo itself, for the nil UUID.
  isPkgsilo?: boolean;
};

// An identity in a preview card: its avatar, its name and its username.
export function IdentityCard({ identity, isPkgsilo = false }: IdentityCardProps) {
  const name = isPkgsilo
    ? "pkgsilo"
    : identity === undefined
      ? m.identityUnknown()
      : identityName(identity);

  return (
    <span className={styles.card}>
      <IdentityAvatar name={name} isPkgsilo={isPkgsilo} />
      <span className={styles.names}>
        <span className={styles.name}>{name}</span>
        {identity?.username !== undefined && identity.username !== name && (
          <span className={styles.username}>{handle(identity.username)}</span>
        )}
      </span>
    </span>
  );
}
