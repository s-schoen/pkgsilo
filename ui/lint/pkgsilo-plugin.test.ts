import { RuleTester } from "oxlint/plugins-dev";
import { describe, it } from "vitest";

import plugin from "./pkgsilo-plugin.ts";

RuleTester.describe = describe;
RuleTester.it = it;

const tester = new RuleTester({
  languageOptions: { sourceType: "module", parserOptions: { lang: "tsx" } },
});

tester.run("no-permission-checks-in-jsx", plugin.rules["no-permission-checks-in-jsx"], {
  valid: [
    // In a hook, which names the decision.
    `function useIdentityPermissions(identity) {
       const { can } = usePermissions();
       return { canCreate: can("identity:create:username"), canDelete: allows(identity, "identity:delete") };
     }`,
    // The named boolean in JSX.
    `function Page() {
       const { canDelete } = useIdentityPermissions(identity);
       return <Menu isVisible={canDelete} />;
     }`,
    // Another function of that name outside JSX, and other calls in JSX.
    `const ok = can("x"); const view = <p>{format(ok)}</p>;`,
  ],
  invalid: [
    {
      code: `const view = <>{can("identity:create:username") && <Button />}</>;`,
      errors: [{ messageId: "inJsx", data: { name: "can" } }],
    },
    {
      code: `const view = <DataTable canDelete={allows(row, "identity:delete")} />;`,
      errors: [{ messageId: "inJsx", data: { name: "allows" } }],
    },
    {
      code: `const view = <Menu items={[{ isVisible: permissions.can("identity:delete") }]} />;`,
      errors: [{ messageId: "inJsx", data: { name: "can" } }],
    },
    {
      code: `const view = <Button onPress={() => { if (can("identity:delete")) remove(); }} />;`,
      errors: [{ messageId: "inJsx", data: { name: "can" } }],
    },
  ],
});
