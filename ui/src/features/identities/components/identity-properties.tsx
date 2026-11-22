import IconAt from "@tabler/icons-react/icons/IconAt";
import IconCalendarPlus from "@tabler/icons-react/icons/IconCalendarPlus";
import IconClock from "@tabler/icons-react/icons/IconClock";
import IconUserEdit from "@tabler/icons-react/icons/IconUserEdit";
import IconUserPlus from "@tabler/icons-react/icons/IconUserPlus";

import { PropertyItem, PropertyList } from "@/components/ui/property-list";
import { RelativeTime } from "@/components/ui/relative-time";
import { IdentityReference } from "@/features/identities/components/identity-reference.tsx";
import { m } from "@/paraglide/messages.js";

import type { IdentityResponse } from "@/api/gen/client/model/identityResponse.ts";

type IdentityPropertiesProps = {
  identity: IdentityResponse;
};

// An identity's properties in its page's sidebar: its username, who created and last changed it,
// and when. A field that the actor can't read is absent, and its row with it.
export function IdentityProperties({ identity }: IdentityPropertiesProps) {
  const { username, createdBy, createdAt, updatedBy, updatedAt } = identity;

  return (
    <PropertyList>
      {username !== undefined && (
        <PropertyItem icon={IconAt} label={m.username()}>
          {username}
        </PropertyItem>
      )}
      {createdBy !== undefined && (
        <PropertyItem icon={IconUserPlus} label={m.identityCreatedBy()}>
          <IdentityReference id={createdBy} />
        </PropertyItem>
      )}
      {createdAt !== undefined && (
        <PropertyItem icon={IconCalendarPlus} label={m.identityCreatedAt()}>
          <RelativeTime date={createdAt} />
        </PropertyItem>
      )}
      {updatedBy !== undefined && (
        <PropertyItem icon={IconUserEdit} label={m.identityUpdatedBy()}>
          <IdentityReference id={updatedBy} />
        </PropertyItem>
      )}
      {updatedAt !== undefined && (
        <PropertyItem icon={IconClock} label={m.identityUpdatedAt()}>
          <RelativeTime date={updatedAt} />
        </PropertyItem>
      )}
    </PropertyList>
  );
}
