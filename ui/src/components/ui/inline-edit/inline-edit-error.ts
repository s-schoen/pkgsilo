// The rejection of onSave that keeps the edit mode and shows its message on the field, such as
// "This username is taken.": throw new InlineEditError(m.usernameTaken()). Any other rejection
// reverts the value.
export class InlineEditError extends Error {
  override name = "InlineEditError";
}
