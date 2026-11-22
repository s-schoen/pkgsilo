import { Badge } from "@/components/ui/badge";
import { m } from "@/paraglide/messages.js";

import styles from "@/features/identities/components/identity-title.module.css";

type IdentityTitleProps = {
  name: string;
  // The signed-in identity, which a badge marks.
  isYou: boolean;
};

// An identity's name in its row of the list, which ends in an ellipsis when it's too long, and the
// "You" badge after it on the signed-in identity's.
export function IdentityTitle({ name, isYou }: IdentityTitleProps) {
  return (
    <span className={styles.title}>
      <span className={styles.name}>{name}</span>
      {isYou && <Badge variant="accent">{m.identityYou()}</Badge>}
    </span>
  );
}
