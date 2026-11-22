import { eq, useLiveQuery } from "@tanstack/react-db";

import { Link } from "@/components/ui/link";
import { identitiesCollection } from "@/features/identities/api/identities-collection.ts";
import { IdentityAvatar } from "@/features/identities/components/identity-avatar.tsx";
import { PKGSILO_ID, identityName } from "@/features/identities/utils/identity-name.ts";
import { useRoutePreload } from "@/hooks/use-route-preload.ts";
import { m } from "@/paraglide/messages.js";
import { routeHref } from "@/utils/route-href.ts";

import styles from "@/features/identities/components/identity-reference.module.css";

type IdentityReferenceProps = {
  // The id that a createdBy or updatedBy holds.
  id: string;
};

// The identity that a createdBy or updatedBy names, as its avatar and its name: a link to its page
// for an identity in identitiesCollection, "pkgsilo" for the nil UUID, and "Unknown identity" for
// one that the actor can't read, neither of which links.
export function IdentityReference({ id }: IdentityReferenceProps) {
  const { data: identity } = useLiveQuery((q) =>
    q
      .from({ identity: identitiesCollection })
      .where(({ identity: row }) => eq(row.id, id))
      .findOne(),
  );

  if (id === PKGSILO_ID) {
    return <Reference name="pkgsilo" isPkgsilo />;
  }
  if (identity === undefined) {
    return <Reference name={m.identityUnknown()} />;
  }
  return <ReferenceLink id={id} name={identityName(identity)} />;
}

function Reference({ name, isPkgsilo = false }: { name: string; isPkgsilo?: boolean }) {
  return (
    <span className={styles.reference}>
      <IdentityAvatar name={name} isPkgsilo={isPkgsilo} />
      <span className={styles.name}>{name}</span>
    </span>
  );
}

// A reference to an identity's page, which preloads it on the user's intent.
function ReferenceLink({ id, name }: { id: string; name: string }) {
  const href = routeHref("/identities/$id", { id });
  const preloadProps = useRoutePreload(href);

  return (
    <span className={styles.reference}>
      <IdentityAvatar name={name} />
      <Link variant="secondary" href={href} {...preloadProps}>
        {name}
      </Link>
    </span>
  );
}
