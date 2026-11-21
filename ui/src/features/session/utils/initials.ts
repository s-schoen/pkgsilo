// The first letters of up to two words of name, as the avatar shows them: "Ada Lovelace" is AL,
// "admin" is A.
export function initials(name: string): string {
  return name
    .split(/[\s._-]+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((word) => word.charAt(0).toLocaleUpperCase())
    .join("");
}
