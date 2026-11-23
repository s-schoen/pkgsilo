// The objects of the API's resources, each with the actions that the signed-in identity may do to
// it, typed by its resource's enum.
type WithAllowedActions = { allowedActions?: ReadonlyArray<string> };

// The actions that row's type may list in allowedActions.
export type AllowedActionOf<Row extends WithAllowedActions> = NonNullable<
  Row["allowedActions"]
>[number];

// Whether the signed-in identity may do action to row, exactly, as the API decided when it answered
// with row: such as allows(identity, "identity:delete"). A row without allowedActions, as a new one
// that the identity may not read, allows nothing.
//
// Call it in a resource's permissions hook (use<Resource>Permissions), which gives the JSX named
// booleans; a lint rule keeps allows() out of JSX.
export function allows<Row extends WithAllowedActions>(
  row: Row,
  action: AllowedActionOf<Row>,
): boolean {
  return row.allowedActions?.includes(action) ?? false;
}
