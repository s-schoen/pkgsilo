import { createFormHook } from "@tanstack/react-form";

import { Checkbox } from "@/components/form/checkbox.tsx";
import { ComboBox } from "@/components/form/combo-box.tsx";
import { fieldContext, formContext } from "@/components/form/form-contexts.ts";
import { Form } from "@/components/form/form.tsx";
import { Select } from "@/components/form/select.tsx";
import { SubmitButton } from "@/components/form/submit-button.tsx";
import { Switch } from "@/components/form/switch.tsx";
import { TextArea } from "@/components/form/text-area.tsx";
import { TextField } from "@/components/form/text-field.tsx";

// The form hook of the UI: useAppForm takes the options of TanStack Form's useForm, and its form
// renders the components below bound to a field (form.AppField) or to the form (form.AppForm).
// withForm splits a form into components. Register a new field or form component here.
export const { useAppForm, withForm } = createFormHook({
  fieldContext,
  formContext,
  fieldComponents: { TextField, TextArea, Checkbox, Switch, Select, ComboBox },
  formComponents: { Form, SubmitButton },
});
