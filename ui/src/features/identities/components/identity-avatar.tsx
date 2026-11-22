import IconBox from "@tabler/icons-react/icons/IconBox";

import { initials } from "@/utils/initials.ts";

import styles from "@/features/identities/components/identity-avatar.module.css";

type IdentityAvatarProps = {
  // The identity's name, whose initials the avatar shows.
  name: string;
  // pkgsilo itself, for the nil UUID: the app's icon instead of initials.
  isPkgsilo?: boolean;
};

// An identity's initials in a circle, decorative: the name next to it names the identity.
export function IdentityAvatar({ name, isPkgsilo = false }: IdentityAvatarProps) {
  return (
    <span aria-hidden className={styles.avatar}>
      {isPkgsilo ? <IconBox className={styles.icon} /> : initials(name)}
    </span>
  );
}
