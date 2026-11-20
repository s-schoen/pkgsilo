import { Input } from "react-aria-components/Input";
import { TextArea as AriaTextArea } from "react-aria-components/TextArea";
import { TextField as AriaTextField } from "react-aria-components/TextField";

import { Description, FieldError, Label, fieldClassName } from "@/components/ui/field";

import type { Ref } from "react";
import type { TextFieldProps as AriaTextFieldProps } from "react-aria-components/TextField";

import styles from "@/components/ui/text-field/text-field.module.css";

// The props that a text field and a text area share. validationBehavior is always "aria": the
// browser's own validation never blocks a form, whose schema (zod) validates it instead.
type FieldProps = Omit<
  AriaTextFieldProps,
  "children" | "className" | "style" | "validationBehavior" | "validate" | "type"
> & {
  // The name of the field, above the control: a message.
  label: string;
  // A hint below the control.
  description?: string;
  // The error below the control, shown while isInvalid is set.
  errorMessage?: string;
  // A sample of the value, shown while the field is empty: a message.
  placeholder?: string;
};

type TextFieldProps = FieldProps & {
  type?: "text" | "email" | "password" | "search" | "url" | "tel";
  // For a parent that focuses the input.
  ref?: Ref<HTMLInputElement>;
};

// A text input with its label, of React Aria Components, which takes its props, such as value,
// onChange, isRequired and isDisabled: <TextField label={m.name()} value={…} onChange={…} />.
// isInvalid with errorMessage shows the error below it. In a form of useAppForm, use the field's
// TextField (@/components/form), which binds it.
export function TextField({
  label,
  description,
  errorMessage,
  placeholder,
  ref,
  ...props
}: TextFieldProps) {
  return (
    <AriaTextField {...props} validationBehavior="aria" className={fieldClassName}>
      <Label>{label}</Label>
      <Input ref={ref} placeholder={placeholder} className={`${styles.control} ${styles.input}`} />
      {description && <Description>{description}</Description>}
      <FieldError>{errorMessage}</FieldError>
    </AriaTextField>
  );
}

type TextAreaProps = FieldProps & {
  // The height in lines of text, which the user can drag larger.
  rows?: number;
  // For a parent that focuses the text area.
  ref?: Ref<HTMLTextAreaElement>;
};

// A text input of several lines, with its label, such as a policy's source: the same props as
// TextField, and rows for its height.
export function TextArea({
  label,
  description,
  errorMessage,
  placeholder,
  rows = 4,
  ref,
  ...props
}: TextAreaProps) {
  return (
    <AriaTextField {...props} validationBehavior="aria" className={fieldClassName}>
      <Label>{label}</Label>
      <AriaTextArea
        ref={ref}
        rows={rows}
        placeholder={placeholder}
        className={`${styles.control} ${styles.textArea}`}
      />
      {description && <Description>{description}</Description>}
      <FieldError>{errorMessage}</FieldError>
    </AriaTextField>
  );
}
