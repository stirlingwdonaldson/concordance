"use client";

import type { ChangeEvent } from "react";
import type { Document } from "../lib/types";

interface DocumentsPanelProps {
  documents: Document[];
  selectedDocument: string;
  selectedProjectExists: boolean;
  onSelectDocument: (id: string) => void;
  onUpload: (event: ChangeEvent<HTMLInputElement>) => void;
  onRefresh: () => void;
}

export function DocumentsPanel({
  documents,
  selectedDocument,
  selectedProjectExists,
  onSelectDocument,
  onUpload,
  onRefresh,
}: DocumentsPanelProps) {
  return (
    <section className="panel">
      <h2>Document uploads</h2>
      <input
        type="file"
        onChange={onUpload}
        disabled={!selectedProjectExists}
      />
      <button
        type="button"
        disabled={!selectedProjectExists}
        onClick={onRefresh}
      >
        Refresh status
      </button>

      <select
        value={selectedDocument}
        onChange={(event) => onSelectDocument(event.target.value)}
        disabled={documents.length === 0}
      >
        {documents.length === 0 ? (
          <option value="">No documents yet</option>
        ) : (
          documents.map((document) => (
            <option key={document.id} value={document.id}>
              {document.fileName}
            </option>
          ))
        )}
      </select>

      <ul>
        {documents.map((document) => (
          <li key={document.id}>
            {document.fileName} - {document.status} (
            {Math.round(document.progress * 100)}%)
          </li>
        ))}
      </ul>
    </section>
  );
}
