import { Input } from "react-aria-components/Input";
import { TextField } from "react-aria-components/TextField";

import { FieldError, fieldClassName } from "@/components/ui/field";
import { InlineEdit } from "@/components/ui/inline-edit/inline-edit.tsx";
import { Spinner } from "@/components/ui/spinner";
import { m } from "@/paraglide/messages.js";

import type { InlineEditProps } from "@/components/ui/inline-edit/inline-edit.tsx";

import styles from "@/components/ui/inline-edit/inline-edit.module.css";

type InlineTextFieldProps = Omit<InlineEditProps<string>, "renderView" | "renderEdit"> & {
  // Shown while the value is empty, in the view and the editor: a message.
  placeholder?: string;
};

// A text that the user edits in place (InlineEdit), such as a username in a details page's sidebar,
// or with variant="title" the page's title, which it renders as the page's <h1>. In a form of
// useAppForm, use the field's InlineTextField (@/components/form), which binds it.
export function InlineTextField({ label, placeholder, variant, ...props }: InlineTextFieldProps) {
  const isTitle = variant === "title";

  return (
    <InlineEdit
      {...props}
      label={label}
      variant={variant}
      renderView={(value) => {
        const text =
          value === "" ? <span className={styles.placeholder}>{placeholder}</span> : value;
        return isTitle ? <h1 className={styles.heading}>{text}</h1> : text;
      }}
      renderEdit={({ value, onChange, isInvalid, errorMessage, isPending }) => (
        <TextField
          aria-label={label}
          value={value}
          onChange={onChange}
          isInvalid={isInvalid}
          isReadOnly={isPending}
          validationBehavior="aria"
          // The editor replaces the view that the user just pressed, so it takes the focus.
          // oxlint-disable-next-line jsx-a11y/no-autofocus -- the user asked for the editor by pressing the view
          autoFocus
          className={fieldClassName}
        >
          <div className={styles.control}>
            <Input
              placeholder={placeholder}
              onFocus={(event) => {
                event.currentTarget.select();
              }}
              className={isPending ? `${styles.input} ${styles.inputPending}` : styles.input}
            />
            {isPending && (
              <span className={styles.pending}>
                <Spinner label={m.inlineEditSaving()} />
              </span>
            )}
          </div>
          <FieldError>{errorMessage}</FieldError>
        </TextField>
      )}
    />
  );
}
