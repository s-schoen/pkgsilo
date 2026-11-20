import { readdirSync } from "node:fs";

import { defineConfig } from "oxlint";

// The imports that every file of src/ may not use. no-restricted-imports takes one set of options per
// file, and an override replaces it, so each override below builds its own from these parts.
const restrictedPaths = [
  { name: "zod", message: "Import zod from zod/v4." },
  { name: "zod/locales", message: "Import zod's locales from zod/v4/locales." },
  {
    name: "@tabler/icons-react",
    message:
      "Import each icon from @tabler/icons-react/icons/<IconName>: the package root makes the tools compile every icon.",
    allowTypeImports: true,
  },
];

const relativeImports = {
  group: ["./*", "../*"],
  message: "Import from src/ with the @/ alias.",
};

const featuresOrApp = {
  group: ["@/features/**", "@/app/**"],
  message: "Shared code doesn't import features or the app.",
};

const app = {
  group: ["@/app/**"],
  message: "A feature doesn't import the app.",
};

// A component of the UI library is imported by its folder, whose index.ts is its public API. The files
// of one component's folder import each other by their path, so ownComponent excludes that folder.
function uiLibraryInternals(ownComponent?: string) {
  return {
    group: [
      "@/components/ui/*/*",
      ...(ownComponent === undefined ? [] : [`!@/components/ui/${ownComponent}/*`]),
    ],
    message: "Import a component of the UI library by its folder: @/components/ui/<name>.",
  };
}

type RestrictedPattern = { group: Array<string>; message: string };

function restrictImports(
  ...patterns: Array<RestrictedPattern>
): ["error", { paths: typeof restrictedPaths; patterns: Array<RestrictedPattern> }] {
  return ["error", { paths: restrictedPaths, patterns }];
}

// The components of the UI library, each a folder of src/components/ui/.
const uiComponents = readdirSync(new URL("./src/components/ui", import.meta.url), {
  withFileTypes: true,
})
  .filter((entry) => entry.isDirectory())
  .map((entry) => entry.name);

export default defineConfig({
  plugins: ["eslint", "import", "jsx-a11y", "oxc", "react", "typescript", "unicorn", "vitest"],
  jsPlugins: [
    "eslint-plugin-testing-library",
    { name: "paraglide", specifier: "@inlang/paraglide-js/eslint" },
    "eslint-plugin-storybook",
  ],
  categories: {
    correctness: "error",
  },
  env: {
    es2023: true,
  },
  ignorePatterns: [
    "build",
    "dist",
    "node_modules",
    ".tanstack",
    "src/api/gen",
    "src/app/routeTree.gen.ts",
    "src/paraglide",
  ],
  options: {
    maxWarnings: 0,
    reportUnusedDisableDirectives: "error",
    respectEslintDisableDirectives: false,
    typeAware: true,
    typeCheck: true,
  },
  rules: {
    "eslint/no-async-promise-executor": "error",
    "eslint/no-case-declarations": "error",
    "eslint/no-class-assign": "error",
    "eslint/no-compare-neg-zero": "error",
    "eslint/no-constant-binary-expression": "error",
    "eslint/no-constant-condition": "error",
    "eslint/no-control-regex": "error",
    "eslint/no-debugger": "error",
    "eslint/no-delete-var": "error",
    "eslint/no-dupe-else-if": "error",
    "eslint/no-duplicate-case": "error",
    "eslint/no-empty-character-class": "error",
    "eslint/no-empty-pattern": "error",
    "eslint/no-empty-static-block": "error",
    "eslint/no-ex-assign": "error",
    "eslint/no-extra-boolean-cast": "error",
    "eslint/no-fallthrough": "error",
    "eslint/no-global-assign": "error",
    "eslint/no-invalid-regexp": "error",
    "eslint/no-irregular-whitespace": "error",
    "eslint/no-loss-of-precision": "error",
    "eslint/no-misleading-character-class": "error",
    "eslint/no-nonoctal-decimal-escape": "error",
    "eslint/no-prototype-builtins": "error",
    "eslint/no-regex-spaces": "error",
    "eslint/no-self-assign": "error",
    "eslint/no-shadow-restricted-names": "error",
    "eslint/no-sparse-arrays": "error",
    "eslint/no-unsafe-finally": "error",
    "eslint/no-unsafe-optional-chaining": "error",
    "eslint/no-unused-labels": "error",
    "eslint/no-unused-private-class-members": "error",
    "eslint/no-useless-backreference": "error",
    "eslint/no-useless-catch": "error",
    "eslint/no-useless-escape": "error",
    "eslint/no-var": "error",
    "eslint/no-with": "error",
    "eslint/prefer-const": "error",
    "eslint/require-yield": "error",
    "eslint/use-isnan": "error",
    "eslint/valid-typeof": "error",
    "react/exhaustive-deps": "error",
    "react/only-export-components": "error",
    "react/rules-of-hooks": "error",
    "typescript/await-thenable": "error",
    "typescript/ban-ts-comment": "error",
    "typescript/no-duplicate-enum-values": "error",
    "typescript/no-empty-object-type": "error",
    "typescript/no-explicit-any": "error",
    "typescript/no-extra-non-null-assertion": "error",
    "typescript/no-floating-promises": "error",
    "typescript/no-misused-new": "error",
    "typescript/no-misused-promises": "error",
    "typescript/no-namespace": "error",
    "typescript/no-non-null-asserted-optional-chain": "error",
    "typescript/no-require-imports": "error",
    "typescript/no-this-alias": "error",
    "typescript/no-unsafe-declaration-merging": "error",
    "typescript/no-unsafe-function-type": "error",
    "typescript/no-unused-expressions": "error",
    "typescript/no-unused-vars": "error",
    "typescript/no-wrapper-object-types": "error",
    "typescript/prefer-as-const": "error",
    "typescript/prefer-namespace-keyword": "error",
    "typescript/triple-slash-reference": "error",
    "unicorn/prefer-node-protocol": "error",
  },
  overrides: [
    {
      files: ["oxlint.config.ts", "orval.config.ts", "vite.config.ts", "vitest.config.ts"],
      env: {
        node: true,
      },
    },
    {
      files: ["src/**/*.{ts,tsx}"],
      env: {
        browser: true,
      },
      rules: {
        "eslint/no-shadow": "warn",
        "import/consistent-type-specifier-style": ["error", "prefer-top-level"],
        "import/first": "error",
        "import/newline-after-import": "error",
        "import/no-commonjs": "error",
        "import/no-cycle": ["error", { ignoreExternal: true, ignoreTypes: true }],
        "import/no-duplicates": "error",
        "import/no-self-import": "error",
        "paraglide/no-dynamic-message-access": "error",
        "react/jsx-no-literals": [
          "error",
          {
            noStrings: true,
            ignoreProps: true,
            allowedStrings: ["pkgsilo"],
            restrictedAttributes: [
              "alt",
              "aria-description",
              "aria-label",
              "aria-placeholder",
              "aria-roledescription",
              "aria-valuetext",
              "label",
              "placeholder",
              "title",
            ],
          },
        ],
        "typescript/array-type": ["error", { default: "generic", readonly: "generic" }],
        "unicorn/filename-case": ["error", { case: "kebabCase" }],
        "typescript/ban-ts-comment": [
          "error",
          { "ts-expect-error": false, "ts-ignore": "allow-with-description" },
        ],
        "typescript/consistent-type-imports": ["error", { prefer: "type-imports" }],
        "typescript/method-signature-style": ["error", "property"],
        "typescript/no-for-in-array": "error",
        "typescript/no-inferrable-types": ["error", { ignoreParameters: true }],
        "typescript/no-unnecessary-condition": "error",
        "typescript/no-unnecessary-type-assertion": "error",
        "eslint/no-restricted-imports": restrictImports(relativeImports, uiLibraryInternals()),
      },
    },
    {
      // The names of the route files are the URL's syntax, such as __root.tsx and $groupId.tsx.
      files: ["src/app/routes/**"],
      rules: {
        "unicorn/filename-case": "off",
      },
    },
    {
      // Imports flow one way: shared code -> features -> app.
      files: [
        "src/api/**/*.{ts,tsx}",
        "src/components/**/*.{ts,tsx}",
        "src/hooks/**/*.{ts,tsx}",
        "src/lib/**/*.{ts,tsx}",
        "src/utils/**/*.{ts,tsx}",
      ],
      rules: {
        "eslint/no-restricted-imports": restrictImports(
          relativeImports,
          featuresOrApp,
          uiLibraryInternals(),
        ),
      },
    },
    // The files of a component of the UI library import each other by their path, but not the files of
    // another component.
    ...uiComponents.map((component) => ({
      files: [`src/components/ui/${component}/**/*.{ts,tsx}`],
      rules: {
        "eslint/no-restricted-imports": restrictImports(
          relativeImports,
          featuresOrApp,
          uiLibraryInternals(component),
        ),
      },
    })),
    {
      // A component's stories import it by its folder, as other code does.
      files: ["src/components/ui/**/*.stories.tsx"],
      rules: {
        "eslint/no-restricted-imports": restrictImports(
          relativeImports,
          featuresOrApp,
          uiLibraryInternals(),
        ),
      },
    },
    {
      // Storybook's configuration: main.ts runs in Node, preview.tsx in the browser of the stories.
      files: [".storybook/**"],
      env: {
        node: true,
        browser: true,
      },
      rules: {
        // Fast Refresh doesn't reload the preview, which exports its configuration, not components.
        "react/only-export-components": "off",
        "storybook/no-uninstalled-addons": "error",
      },
    },
    {
      files: ["src/**/*.stories.tsx"],
      rules: {
        "storybook/await-interactions": "error",
        "storybook/context-in-play-function": "error",
        "storybook/no-redundant-story-name": "error",
        "storybook/no-renderer-packages": "error",
        "storybook/prefer-pascal-case": "error",
        "storybook/use-storybook-expect": "error",
      },
    },
    {
      files: ["src/features/**/*.{ts,tsx}"],
      rules: {
        "eslint/no-restricted-imports": restrictImports(relativeImports, app, uiLibraryInternals()),
      },
    },
    {
      files: ["**/*.test.{ts,tsx}", "src/test/**/*.{ts,tsx}"],
      rules: {
        "react/jsx-no-literals": "off",
        "testing-library/await-async-events": ["error", { eventModule: "userEvent" }],
        "testing-library/await-async-queries": "error",
        "testing-library/await-async-utils": "error",
        "testing-library/no-await-sync-events": ["error", { eventModules: ["fire-event"] }],
        "testing-library/no-await-sync-queries": "error",
        "testing-library/no-dom-import": ["error", "react"],
        "testing-library/no-global-regexp-flag-in-query": "error",
        "testing-library/no-promise-in-fire-event": "error",
        "testing-library/no-wait-for-multiple-assertions": "error",
        "testing-library/prefer-find-by": "error",
        "testing-library/prefer-presence-queries": "error",
        "testing-library/prefer-query-by-disappearance": "error",
      },
    },
  ],
});
