"use client";

import { useEffect, useRef, useState } from "react";
import type { FormEvent, ReactNode } from "react";

/** Native <dialog>: focus trap, Esc to close and a backdrop for free. */
export function Dialog({
  open,
  title,
  onClose,
  children,
}: Readonly<{
  open: boolean;
  title: string;
  onClose: () => void;
  children: ReactNode;
}>) {
  const ref = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    if (open && !dialog.open) dialog.showModal();
    if (!open && dialog.open) dialog.close();
  }, [open]);

  return (
    // biome-ignore lint/a11y/useKeyWithClickEvents: Esc already closes a native dialog; the backdrop click is a pointer shortcut
    <dialog
      ref={ref}
      className="dialog"
      aria-label={title}
      onClose={onClose}
      onClick={(event) => {
        if (event.target === ref.current) onClose();
      }}
    >
      {open && (
        <div className="dialog-body">
          <h2>{title}</h2>
          {children}
        </div>
      )}
    </dialog>
  );
}

export function ConfirmDialog({
  open,
  title,
  body,
  confirmLabel,
  danger = true,
  onConfirm,
  onClose,
}: Readonly<{
  open: boolean;
  title: string;
  body: ReactNode;
  confirmLabel: string;
  danger?: boolean;
  onConfirm: () => Promise<void> | void;
  onClose: () => void;
}>) {
  const [busy, setBusy] = useState(false);

  async function confirm() {
    setBusy(true);
    try {
      await onConfirm();
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open={open} title={title} onClose={onClose}>
      <div className="dialog-copy">{body}</div>
      <div className="dialog-actions">
        <button type="button" className="btn btn-quiet" onClick={onClose}>
          Cancel
        </button>
        <button
          type="button"
          className={`btn ${danger ? "btn-danger" : "btn-primary"}`}
          disabled={busy}
          onClick={confirm}
        >
          {confirmLabel}
        </button>
      </div>
    </Dialog>
  );
}

export function TextDialog({
  open,
  title,
  label,
  initial = "",
  placeholder,
  examples = [],
  submitLabel,
  onSubmit,
  onClose,
}: Readonly<{
  open: boolean;
  title: string;
  label: string;
  initial?: string;
  placeholder?: string;
  examples?: string[];
  submitLabel: string;
  onSubmit: (value: string) => Promise<void> | void;
  onClose: () => void;
}>) {
  const [value, setValue] = useState(initial);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (open) setValue(initial);
  }, [open, initial]);

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (value.trim() === "") return;
    setBusy(true);
    try {
      await onSubmit(value.trim());
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open={open} title={title} onClose={onClose}>
      <form onSubmit={submit} className="dialog-form">
        <label className="field">
          <span>{label}</span>
          <input
            // biome-ignore lint/a11y/noAutofocus: the dialog exists to fill in this field
            autoFocus
            value={value}
            placeholder={placeholder}
            onChange={(event) => setValue(event.target.value)}
          />
        </label>
        {examples.length > 0 && (
          <p className="examples">
            Examples:{" "}
            {examples.map((example) => (
              <button
                key={example}
                type="button"
                className="chip"
                onClick={() => setValue(example)}
              >
                {example}
              </button>
            ))}
          </p>
        )}
        <div className="dialog-actions">
          <button type="button" className="btn btn-quiet" onClick={onClose}>
            Cancel
          </button>
          <button
            type="submit"
            className="btn btn-primary"
            disabled={busy || value.trim() === ""}
          >
            {submitLabel}
          </button>
        </div>
      </form>
    </Dialog>
  );
}
