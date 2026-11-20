# Forms

How to build a form with TanStack Form and validate it with zod.

Build a form through `useAppForm` from `@/components/form/use-app-form.ts`, which binds the shared field and form components:

```tsx
const form = useAppForm({
  defaultValues: { name: "" },
  validators: { onChange: CreateGroupBody },
  onSubmit: async ({ value }) => {
    await createGroup.mutateAsync({ data: value });
  },
});

<form.AppForm>
  <form.Form>
    <form.AppField name="name">
      {(field) => <field.TextField label={m.groupName()} />}
    </form.AppField>
    <form.SubmitButton label={m.save()} />
  </form.Form>
</form.AppForm>;
```

- `form.Form` renders React Aria's `<Form>`, which submits through `form.handleSubmit()` (Enter in a field too) and stacks the fields; its `validationBehavior` is `"aria"`, so the browser's own validation never blocks a submission.
- The field components bind the fields of the UI library (`ui/docs/agents/components.md`): `TextField`, `TextArea` (strings), `Checkbox`, `Switch` (booleans), `Select` and `ComboBox`, whose field holds the id of the picked option, or `null` while there's none.
  `Select` and `ComboBox` take `items` and render each as a `ListBoxItem` (`@/components/ui/list-box`), as the UI library's do.

- Validate with the generated zod schema of the request (`CreateGroupBody` from `src/api/gen/zod/<tag>/<tag>.zod.ts`), extended with `.extend()` or `.refine()` for the rules that the schema doesn't state.
  The schemas check the types and the required members, not the rules that only a `jsonschema` description states.
- Submit with a mutation of the API's hooks (`ui/docs/agents/api-client.md`), or with an insert or update of a collection (`ui/docs/agents/client-store.md`).
- Import zod from `zod/v4` (and its locales from `zod/v4/locales`); `no-restricted-imports` rejects `zod`.
  The generated schemas import `zod`, the same module, so they share zod's settings.
- A field shows its errors once the user has left it or submitted the form, in a `role="alert"` element that describes the input: `useFieldErrors()` (`use-field-errors.ts`) gives a field component the `isInvalid` and `errorMessage` that the UI library's fields take.
- `SubmitButton` is pending, with a spinner, while the form submits, and disabled while the form can't submit.
  A disabled submit button also stops Enter from submitting, as the browser does.
- Register a new field or form component in `use-app-form.ts`; it reads its field with `useFieldContext<Value>()` or its form with `useFormContext()` from `form-contexts.ts`.
  Its text props, such as `label`, are messages: add a new one to `restrictedAttributes` of `react/jsx-no-literals` (`ui/docs/agents/translations.md`).
- zod's messages, such as a schema's validation error, follow the locale: `configureZodLocale()` (`src/lib/zod-locale.ts`) sets it, and its map names a zod locale for every locale of the UI.
  `main.tsx` calls it with the browser's locale, `src/test/setup.ts` with English, and `.storybook/preview.tsx` with the toolbar's.
- A test or story finds an input by its label, and an error through zod's message for the same value (`schema.safeParse(value).error?.issues[0]?.message`), never as a literal.
