// The characters as a user sees them, so that an initial is never half of one: an emoji, a letter
// outside the Basic Multilingual Plane, or a letter with a combining accent spans several UTF-16 code
// units.
const graphemes = new Intl.Segmenter(undefined, { granularity: "grapheme" });

// The first characters of up to two words of name, as the avatar shows them: "Ada Lovelace" is AL,
// "admin" is A.
export function initials(name: string): string {
  return name
    .split(/[\s._-]+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((word) => (graphemes.segment(word).containing(0)?.segment ?? "").toLocaleUpperCase())
    .join("");
}
