// pkgsilo's own lint rules, an oxlint JS plugin that oxlint.config.ts loads (jsPlugins). Each rule
// has a test next to this file, run by Vitest's unit project.

import type { RuleTester } from "oxlint/plugins-dev";

type Rule = Parameters<RuleTester["run"]>[1];

// The functions that tell whether the signed-in identity may do an action: usePermissions' can and
// allows (ui/src/features/permissions).
const permissionChecks = new Set(["can", "allows"]);

// The JSX nodes whose expressions are JSX's: a child's or an attribute's {…}, and a spread.
const jsxExpressions = new Set(["JSXExpressionContainer", "JSXSpreadAttribute", "JSXSpreadChild"]);

// Keeps the permission checks out of JSX, so that it reads named booleans: a resource's
// permissions hook (use<Resource>Permissions) holds the action strings and the UI's own rules, and
// returns { canCreate, canDelete, … }, which the JSX passes to the components' props. It reports a
// call of can or allows, as can(…) or permissions.can(…), anywhere inside a JSX expression, a
// callback's body too.
const noPermissionChecksInJsx: Rule = {
  meta: {
    type: "suggestion",
    docs: { description: "Disallow calling can() or allows() inside JSX." },
    messages: {
      inJsx:
        "Don't call {{name}}() in JSX: return a named boolean, such as canDelete, from the resource's permissions hook (use<Resource>Permissions) and use that.",
    },
  },
  create(context) {
    return {
      CallExpression(node) {
        const { callee } = node;
        const name =
          callee.type === "Identifier"
            ? callee.name
            : callee.type === "MemberExpression" &&
                !callee.computed &&
                callee.property.type === "Identifier"
              ? callee.property.name
              : undefined;
        if (name === undefined || !permissionChecks.has(name)) {
          return;
        }
        let ancestor: { type: string; parent: unknown } | null = node.parent;
        while (ancestor !== null) {
          if (jsxExpressions.has(ancestor.type)) {
            context.report({ node, messageId: "inJsx", data: { name } });
            return;
          }
          ancestor = ancestor.parent as typeof ancestor;
        }
      },
    };
  },
};

export default {
  meta: { name: "pkgsilo" },
  rules: { "no-permission-checks-in-jsx": noPermissionChecksInJsx },
};
