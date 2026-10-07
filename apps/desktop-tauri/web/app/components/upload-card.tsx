"use client";

import { useRef, useState } from "react";
import type { DragEvent } from "react";
import { SUPPORTED_FORMATS } from "../lib/glossary";

export function UploadCard({
  disabled,
  onFiles,
}: Readonly<{ disabled: boolean; onFiles: (files: File[]) => Promise<void> }>) {
  const input = useRef<HTMLInputElement>(null);
  const [dragging, setDragging] = useState(false);
  const [busy, setBusy] = useState(false);

  async function handle(files: File[]) {
    if (files.length === 0 || disabled) return;
    setBusy(true);
    try {
      await onFiles(files);
    } finally {
      setBusy(false);
      if (input.current) input.current.value = "";
    }
  }

  function onDrop(event: DragEvent) {
    event.preventDefault();
    setDragging(false);
    void handle([...event.dataTransfer.files]);
  }

  return (
    <div
      className={`drop${dragging ? " is-over" : ""}${disabled ? " is-disabled" : ""}`}
      onDragOver={(event) => {
        event.preventDefault();
        if (!disabled) setDragging(true);
      }}
      onDragLeave={() => setDragging(false)}
      onDrop={onDrop}
    >
      <input
        ref={input}
        type="file"
        multiple
        hidden
        accept=".pdf,.docx,.epub,.odt,.html,.htm,.txt,.md,.markdown,.csv,.rtf"
        onChange={(event) => void handle([...(event.target.files ?? [])])}
      />
      <p className="drop-title">{busy ? "Uploading…" : "Drop files here"}</p>
      <p className="drop-sub">
        Textbooks, papers, reports. You can add several at once.
      </p>
      <button
        type="button"
        className="btn btn-primary"
        disabled={disabled || busy}
        onClick={() => input.current?.click()}
      >
        Choose files
      </button>
      <ul className="formats" aria-label="Supported formats">
        {SUPPORTED_FORMATS.map((format) => (
          <li key={format}>{format}</li>
        ))}
      </ul>
      <p className="drop-note">
        Scanned PDFs (photos of pages) have no text to read, so they can't be
        processed yet.
      </p>
    </div>
  );
}
