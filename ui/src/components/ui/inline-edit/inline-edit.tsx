import { useEffect, useRef, useState } from "react";
import { Button } from "react-aria-components/Button";

import { InlineEditError } from "@/components/ui/inline-edit/inline-edit-error.ts";
import { m } from "@/paraglide/messages.js";

import type { FocusEvent, KeyboardEvent, ReactNode } from "react";

import styles from "@/components/ui/inline-edit/inline-edit.module.css";

// What renderEdit gets to render the editor.
export type InlineEditRenderProps<T> = {
  value: T;
  onChange: (value: T) => void;
  // The error to show below the editor: the value's own, once the user tried to save it, or the one
  // of InlineEditError.
  isInvalid: boolean;
  errorMessage: string | undefined;
  // While onSave runs: the editor shows it and takes no input.
  isPending: boolean;
};

export type InlineEditProps<T> = {
  // The field's name, a message: the view's button is "Edit <label>", and the editor's name.
  label: string;
  // The value, controlled: the editor changes it through onChange, and the shell restores the value
  // it had before the edit when the user cancels or the save fails.
  value: T;
  onChange: (value: T) => void;
  // The value's error, a message, while it's invalid: the shell keeps the edit mode and shows it
  // once the user tries to save. As a field's errorMessage, its validation (a form's) is the
  // caller's.
  errorMessage?: string;
  // The value outside the edit mode.
  renderView: (value: T) => ReactNode;
  // The editor, which takes the focus when it renders, such as a text field with autoFocus.
  renderEdit: (edit: InlineEditRenderProps<T>) => ReactNode;
  // Saves a changed value. Resolve to leave the edit mode; reject with InlineEditError to stay in it
  // with that message; any other rejection reverts the value and leaves, so show its toast here
  // before rejecting.
  onSave: (value: T) => Promise<void>;
  // Called when the edit mode starts and ends, after a cancel or a failed save has restored the
  // value: a caller whose value can change meanwhile, such as from another tab, takes the latest
  // one while the user isn't editing.
  onEditingChange?: (isEditing: boolean) => void;
  // Renders the value as plain text, without the edit mode, as for a value the actor can't change.
  isDisabled?: boolean;
  // The size: default for a sidebar's value, title for a page's large title.
  variant?: "default" | "title";
};

// A value that the user edits in place, as Plane's properties: the view looks like text, with a fill
// on hover, and a press (or Enter) on it shows the editor. Enter or leaving the editor saves, Escape
// cancels; both return the focus to the view. An invalid value stays in the edit mode with its error.
// InlineTextField is the shell with a text input; a form binds that one (@/components/form).
//
// The view is a button under the value rather than around it, so that the value may be a heading
// whose name is the value itself, not the button's "Edit <label>".
export function InlineEdit<T>({
  label,
  value,
  onChange,
  errorMessage,
  renderView,
  renderEdit,
  onSave,
  onEditingChange,
  isDisabled = false,
  variant = "default",
}: InlineEditProps<T>) {
  const [isEditing, setIsEditing] = useState(false);
  const [isPending, setIsPending] = useState(false);
  const [hasTriedSave, setHasTriedSave] = useState(false);
  const [saveError, setSaveError] = useState<string>();
  // The value before the edit, which cancelling or a failed save restores.
  const valueBefore = useRef(value);
  // The handlers of a render may run after the edit has ended or while a save runs (a blur after
  // Enter): these refs tell them, which the state of their render can't.
  const editing = useRef(false);
  const saving = useRef(false);
  const viewRef = useRef<HTMLButtonElement>(null);
  const editorRef = useRef<HTMLDivElement>(null);
  const focusViewOnLeave = useRef(false);

  useEffect(() => {
    if (!isEditing && focusViewOnLeave.current) {
      focusViewOnLeave.current = false;
      viewRef.current?.focus();
    }
  }, [isEditing]);

  const rootClassName = `${styles.inlineEdit} ${variant === "title" ? styles.title : ""}`;

  if (isDisabled) {
    return (
      <div className={rootClassName}>
        <div className={styles.value}>{renderView(value)}</div>
      </div>
    );
  }

  function edit() {
    valueBefore.current = value;
    editing.current = true;
    setIsEditing(true);
    onEditingChange?.(true);
  }

  // Leaves the edit mode; focusView returns the focus to the view, unless it has moved on: the user
  // may have moved it to another field while the save ran.
  function leave(focusView: boolean) {
    editing.current = false;
    const focused = document.activeElement;
    const isFocusInEditor = focused instanceof HTMLElement && editorRef.current?.contains(focused);
    // The editor gives up the focus before React removes it: the browser's blur of a removed element
    // would reach React Aria's handlers while React commits, which React rejects.
    if (isFocusInEditor) {
      focused.blur();
    }
    focusViewOnLeave.current =
      focusView && (isFocusInEditor || focused === null || focused === document.body);
    setIsEditing(false);
    setHasTriedSave(false);
    setSaveError(undefined);
    onEditingChange?.(false);
  }

  async function save(focusView: boolean) {
    if (!editing.current || saving.current) {
      return;
    }
    if (errorMessage !== undefined) {
      setHasTriedSave(true);
      return;
    }
    if (Object.is(value, valueBefore.current)) {
      leave(focusView);
      return;
    }

    saving.current = true;
    setIsPending(true);
    try {
      await onSave(value);
      leave(focusView);
    } catch (error) {
      if (error instanceof InlineEditError) {
        setSaveError(error.message);
      } else {
        onChange(valueBefore.current);
        leave(focusView);
      }
    } finally {
      saving.current = false;
      setIsPending(false);
    }
  }

  function cancel() {
    if (saving.current) {
      return;
    }
    onChange(valueBefore.current);
    leave(true);
  }

  function handleKeyDown(event: KeyboardEvent) {
    // The keys of an input method's composition, as the Enter that confirms Japanese characters,
    // are the composition's. Safari sends that Enter after the composition has ended, with the
    // keyCode 229 of a key that the input method took.
    if (event.nativeEvent.isComposing || event.keyCode === 229) {
      return;
    }
    if (event.key === "Enter") {
      event.preventDefault();
      void save(true);
    } else if (event.key === "Escape") {
      event.preventDefault();
      cancel();
    }
  }

  // Leaving the editor saves, but not moving within it, nor leaving the window, as for another tab:
  // the editor has the focus again when the user comes back.
  function handleBlur(event: FocusEvent) {
    if (!document.hasFocus()) {
      return;
    }
    if (!event.currentTarget.contains(event.relatedTarget)) {
      void save(false);
    }
  }

  if (!isEditing) {
    return (
      <div className={`${rootClassName} ${styles.view}`}>
        <Button
          ref={viewRef}
          aria-label={m.inlineEditLabel({ label })}
          onPress={edit}
          className={styles.trigger}
        />
        <div className={styles.value}>{renderView(value)}</div>
      </div>
    );
  }

  const shownError = saveError ?? (hasTriedSave ? errorMessage : undefined);
  return (
    // The editor's own keys and its focus bubble to the shell, which needs no role of its own.
    // oxlint-disable-next-line jsx-a11y/no-static-element-interactions -- the editor inside is the control; the shell only listens to its keys
    <div ref={editorRef} className={rootClassName} onKeyDown={handleKeyDown} onBlur={handleBlur}>
      {renderEdit({
        value,
        onChange: (next) => {
          setSaveError(undefined);
          onChange(next);
        },
        isInvalid: shownError !== undefined,
        errorMessage: shownError,
        isPending,
      })}
    </div>
  );
}
