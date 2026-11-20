import type { Config } from "stylelint";

// Stylelint checks the CSS against the rules of ui/docs/agents/design.md and components.md. Each rule
// that enforces one of them says so in its message, so that the report names the fix. oxfmt formats
// the CSS; stylelint only checks it, and sorts the properties with --fix.

// The prefixes of the tokens in src/styles/tokens.css, which a module reads but never sets.
const tokenPrefixes = [
  "border",
  "color",
  "control",
  "duration",
  "ease",
  "focus",
  "font",
  "line-height",
  "radius",
  "scale",
  "shadow",
  "size",
  "space",
];

const tokenOnly = "take it from a token of src/styles/tokens.css (ui/docs/agents/components.md)";

export default {
  extends: ["stylelint-config-standard", "stylelint-config-recess-order"],
  ignoreFiles: ["dist/**", "build/**", "node_modules/**"],
  // As with oxlint, a disable says why (/* stylelint-disable-next-line <rule> -- <why> */), and an
  // unused one is an error.
  reportDescriptionlessDisables: true,
  reportInvalidScopeDisables: true,
  reportNeedlessDisables: true,
  // The writing mode of the page, so that --fix can turn a physical property into its logical one.
  languageOptions: {
    directionality: { inline: "left-to-right", block: "top-to-bottom" },
  },
  rules: {
    // Logical properties (inline-size, padding-inline), which follow the writing mode.
    "property-layout-mappings": "flow-relative",
    // The modules import the breakpoints as "@/styles/media.css", which Vite resolves.
    "import-notation": "string",
    // Plane's values as they are: oklch(96.1% 0.0013 286).
    "hue-degree-notation": "number",
    // oxfmt owns the layout: blank lines group the tokens and the declarations, and a comment sits
    // right above what it explains.
    "comment-empty-line-before": null,
    "custom-property-empty-line-before": null,
    "declaration-empty-line-before": null,
    // Font names keep their case: "Menlo" in the font stacks of tokens.css.
    "value-keyword-case": ["lower", { ignoreProperties: ["/^--font-/"] }],
  },
  overrides: [
    {
      // Every color, space, size, radius, font and duration is a token. tokens.css, below, defines them.
      files: ["src/**/*.css"],
      rules: {
        "color-no-hex": [
          true,
          { message: (hex: string) => `Unexpected color ${hex}: ${tokenOnly}` },
        ],
        "color-named": [
          "never",
          { message: (name: string) => `Unexpected color ${name}: ${tokenOnly}` },
        ],
        "function-disallowed-list": [
          [
            "rgb",
            "rgba",
            "hsl",
            "hsla",
            "hwb",
            "lab",
            "lch",
            "oklab",
            "oklch",
            "color",
            "color-mix",
            "light-dark",
            "cubic-bezier",
            "steps",
            "linear",
          ],
          { message: (name: string) => `Unexpected ${name}(): ${tokenOnly}` },
        ],
        // em and % stay allowed: an icon is sized in em, so that it follows the text.
        "unit-disallowed-list": [
          ["px", "rem", "pt", "ms", "s"],
          { message: (unit: string) => `Unexpected literal in ${unit}: ${tokenOnly}` },
        ],
        "declaration-property-value-allowed-list": [
          {
            "font-size": ["/^var\\(--/", "inherit"],
            "font-weight": ["/^var\\(--/", "inherit"],
            "font-family": ["/^var\\(--/", "inherit"],
            "line-height": ["/^var\\(--/", "inherit"],
            "box-shadow": ["/^var\\(--/", "none"],
          },
          {
            message: (property: string, value: string) =>
              `Unexpected ${property}: ${value}: ${tokenOnly}`,
          },
        ],
      },
    },
    {
      files: ["src/**/*.module.css"],
      rules: {
        "selector-pseudo-class-disallowed-list": [
          [
            "hover",
            "active",
            "focus",
            "focus-visible",
            "focus-within",
            "disabled",
            "checked",
            "invalid",
            "required",
            "read-only",
            "placeholder-shown",
          ],
          {
            message: (selector: string) =>
              `Unexpected ${selector}: style the state through React Aria's data attribute, as &[data-hovered] (ui/docs/agents/components.md)`,
          },
        ],
        // A module's classes are read as styles.iconOnly.
        "selector-class-pattern": [
          "^[a-z][a-zA-Z0-9]*$",
          { message: (name: string) => `Expected class ${name} to be camelCase` },
        ],
        // The tokens and the color scheme apply to the whole page, never to a part of it.
        "selector-disallowed-list": [
          [":root", "html", "body", "/\\[data-color-scheme/"],
          {
            message: (selector: string) =>
              `Unexpected ${selector}: a module styles its own classes; the page's styles live in src/styles/`,
          },
        ],
        // A module names its own properties after itself (--button-bg) and never sets a token.
        "property-disallowed-list": [
          [`/^--(${tokenPrefixes.join("|")})-/`],
          {
            message: (name: string) =>
              `Unexpected ${name}: a module never sets a token; name its own properties after it, as --button-bg`,
          },
        ],
        "declaration-property-value-disallowed-list": [
          { "/.*/": ["/--color-canvas\\b/"] },
          {
            message: () =>
              "Unexpected --color-canvas: the canvas is the page itself, which global.css sets on <body> (ui/docs/agents/design.md)",
          },
        ],
        "media-feature-name-disallowed-list": [
          ["/width$/", "/height$/", "prefers-reduced-motion", "prefers-color-scheme"],
          {
            message: (name: string) =>
              `Unexpected media feature ${name}: take a breakpoint from src/styles/media.css (@media (--md)); the tokens handle reduced motion and the color scheme`,
          },
        ],
      },
    },
    {
      // The tokens hold the literal values. Colors are oklch(), so no hex or named colors here either.
      files: ["src/styles/tokens.css"],
      rules: {
        "function-disallowed-list": null,
        "unit-disallowed-list": null,
        "declaration-property-value-allowed-list": null,
      },
    },
  ],
} satisfies Config;
