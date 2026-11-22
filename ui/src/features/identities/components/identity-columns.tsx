import IconAt from "@tabler/icons-react/icons/IconAt";
import IconBox from "@tabler/icons-react/icons/IconBox";
import IconClock from "@tabler/icons-react/icons/IconClock";
import IconUser from "@tabler/icons-react/icons/IconUser";

import { Badge } from "@/components/ui/badge";
import { formatDateTime, formatRelativeTime } from "@/components/ui/relative-time";
import { IdentityAvatar } from "@/features/identities/components/identity-avatar.tsx";
import { IdentityCard } from "@/features/identities/components/identity-card.tsx";
import { IdentityTitle } from "@/features/identities/components/identity-title.tsx";
import { PKGSILO_ID, identityName } from "@/features/identities/utils/identity-name.ts";
import { m } from "@/paraglide/messages.js";
import { getLocale } from "@/paraglide/runtime.js";
import { routeHref } from "@/utils/route-href.ts";

import type { IdentityResponse } from "@/api/gen/client/model/identityResponse.ts";
import type {
  DataTableColumnDef,
  DataTableGroupingOption,
  DataTableOption,
} from "@/components/ui/data-table";

// The select filters that the list keeps in the URL, by their columns' ids: ?createdBy=<id>,<id>.
export const IDENTITY_FILTERS = ["createdBy"] as const;

// The identities of the list by their ids, which resolve a createdBy to its identity.
type IdentitiesById = ReadonlyMap<string, IdentityResponse>;

// The name of a createdBy or updatedBy: "pkgsilo" for the nil UUID, the identity's name, or
// "Unknown identity" for one that the actor can't read.
function referenceName(id: string, byId: IdentitiesById): string {
  if (id === PKGSILO_ID) {
    return "pkgsilo";
  }
  const identity = byId.get(id);
  return identity === undefined ? m.identityUnknown() : identityName(identity);
}

// The creator that the list groups an identity by: pkgsilo's or a readable identity's id, or
// undefined for a creator that the actor can't know, whether it can't read the creator or the
// identity's createdBy. Those form a single group, "Unknown identity".
function creatorGroup(identity: IdentityResponse, byId: IdentitiesById): string | undefined {
  const { createdBy } = identity;
  return createdBy !== undefined && (createdBy === PKGSILO_ID || byId.has(createdBy))
    ? createdBy
    : undefined;
}

// The name of a creator's group: undefined is the unknown one's.
function creatorGroupName(value: unknown, byId: IdentitiesById): string {
  return typeof value === "string" ? referenceName(value, byId) : m.identityUnknown();
}

// Orders names in the locale's collation, ignoring case and accents.
function compareNames(left: string, right: string): number {
  return left.localeCompare(right, getLocale(), { sensitivity: "base" });
}

// The columns of the list: the avatar, the name as the title (with a badge on the signed-in
// identity's), and chips for the username, the creator and the last change. Created only orders
// the rows. The creator's chip previews the creator; the change's previews its date and time.
export function identityColumns({
  byId,
  currentId,
}: {
  byId: IdentitiesById;
  currentId: string | undefined;
}): Array<DataTableColumnDef<IdentityResponse>> {
  return [
    {
      id: "avatar",
      meta: { placement: "leading" },
      cell: ({ row }) => <IdentityAvatar name={identityName(row.original)} />,
    },
    {
      id: "displayName",
      accessorFn: identityName,
      meta: { label: m.identityDisplayName, placement: "title" },
      sortFn: (left, right, columnId) =>
        compareNames(String(left.getValue(columnId)), String(right.getValue(columnId))),
      cell: ({ row }) => (
        <IdentityTitle name={identityName(row.original)} isYou={row.original.id === currentId} />
      ),
    },
    {
      id: "username",
      accessorKey: "username",
      meta: { label: m.username, placement: "property" },
      sortUndefined: "last",
      cell: ({ row }) =>
        row.original.username !== undefined && (
          <Badge variant="property" icon={IconAt}>
            {row.original.username}
          </Badge>
        ),
    },
    {
      id: "createdBy",
      accessorKey: "createdBy",
      meta: {
        label: m.identityCreatedBy,
        placement: "property",
        filterVariant: "select",
        options: creatorOptions(byId),
      },
      // The value is an id, which neither the search nor the order makes sense of.
      enableGlobalFilter: false,
      enableSorting: false,
      getGroupingValue: (identity) => creatorGroup(identity, byId),
      cell: ({ row }) => {
        const id = row.original.createdBy;
        return (
          id !== undefined && (
            <Badge variant="property" icon={id === PKGSILO_ID ? IconBox : IconUser}>
              {referenceName(id, byId)}
            </Badge>
          )
        );
      },
      preview: {
        content: ({ createdBy }) =>
          createdBy !== undefined && (
            <IdentityCard identity={byId.get(createdBy)} isPkgsilo={createdBy === PKGSILO_ID} />
          ),
        // To the creator's page: not for pkgsilo or a creator the actor can't read, whose chip
        // opens the row instead.
        href: ({ createdBy }) =>
          createdBy !== undefined && byId.has(createdBy)
            ? routeHref("/identities/$id", { id: createdBy })
            : undefined,
      },
    },
    {
      id: "createdAt",
      accessorKey: "createdAt",
      meta: { label: m.identityCreatedAt },
      enableGlobalFilter: false,
      sortUndefined: "last",
    },
    {
      id: "updatedAt",
      accessorKey: "updatedAt",
      meta: { label: m.identityUpdatedAt, placement: "property" },
      enableGlobalFilter: false,
      sortUndefined: "last",
      cell: ({ row }) =>
        row.original.updatedAt !== undefined && (
          <Badge variant="property" icon={IconClock}>
            {formatRelativeTime(new Date(row.original.updatedAt), getLocale())}
          </Badge>
        ),
      // The chip can't hold RelativeTime's tooltip, since it's a control itself: its card shows
      // the date and the time instead.
      preview: {
        content: ({ updatedAt }) =>
          updatedAt !== undefined && (
            <time dateTime={updatedAt}>{formatDateTime(new Date(updatedAt), getLocale())}</time>
          ),
      },
    },
  ];
}

// The creators that the Filters popover offers: those of the identities in the list, by name,
// with pkgsilo first.
function creatorOptions(byId: IdentitiesById): Array<DataTableOption> {
  const ids = new Set<string>();
  for (const identity of byId.values()) {
    if (identity.createdBy !== undefined) {
      ids.add(identity.createdBy);
    }
  }
  return [...ids]
    .map((id) => ({
      value: id,
      label: referenceName(id, byId),
      icon: <IdentityAvatar name={referenceName(id, byId)} isPkgsilo={id === PKGSILO_ID} />,
    }))
    .toSorted((left, right) =>
      left.value === PKGSILO_ID
        ? -1
        : right.value === PKGSILO_ID
          ? 1
          : compareNames(left.label, right.label),
    );
}

// What the Display popover's Group by offers: the creator, whose groups show their name and avatar.
export function identityGroupingOptions(byId: IdentitiesById): Array<DataTableGroupingOption> {
  return [
    {
      id: "createdBy",
      label: m.identityCreatedBy,
      formatValue: (value) => creatorGroupName(value, byId),
      renderIcon: (value) => (
        <IdentityAvatar name={creatorGroupName(value, byId)} isPkgsilo={value === PKGSILO_ID} />
      ),
    },
  ];
}
