"use client";

import { useEffect, useMemo, useState } from "react";
import { formatPercent } from "../lib/format";
import type { Document } from "../lib/types";
import { Pagination } from "./pagination";

const STATUS_LABEL: Record<string, string> = {
  queued: "Waiting",
  processing: "Processing",
  running: "Processing",
  ready: "Ready",
  failed: "Failed",
};

export function StatusPill({ status }: Readonly<{ status: string }>) {
  return (
    <span className={`pill pill-${status}`}>
      {STATUS_LABEL[status] ?? status}
    </span>
  );
}

export function LibraryCard({
  documents,
  selectedId,
  loading,
  onSelect,
  onRename,
  onRetry,
  onDelete,
}: Readonly<{
  documents: Document[];
  selectedId: string;
  loading: boolean;
  onSelect: (id: string) => void;
  onRename: (doc: Document) => void;
  onRetry: (doc: Document) => void;
  onDelete: (doc: Document) => void;
}>) {
  const [limit, setLimit] = useState(10);
  const [offset, setOffset] = useState(0);

  // Keep the selected file's page in view and never strand the pager past the end.
  useEffect(() => {
    const index = documents.findIndex((doc) => doc.id === selectedId);
    setOffset((current) => {
      if (current >= documents.length) {
        return Math.max(0, Math.floor((documents.length - 1) / limit) * limit);
      }
      return index >= 0 && (index < current || index >= current + limit)
        ? Math.floor(index / limit) * limit
        : current;
    });
  }, [documents, selectedId, limit]);

  const page = useMemo(
    () => documents.slice(offset, offset + limit),
    [documents, offset, limit],
  );

  if (documents.length === 0) {
    return (
      <p className="empty">
        {loading
          ? "Loading files…"
          : "No files yet. Add one on the right and it will appear here while it processes."}
      </p>
    );
  }

  return (
    <>
      <div className="table-wrap">
        <table className="table">
          <thead>
            <tr>
              <th scope="col">File</th>
              <th scope="col">Status</th>
              <th scope="col" className="col-actions">
                <span className="sr-only">Actions</span>
              </th>
            </tr>
          </thead>
          <tbody>
            {page.map((doc) => (
              <tr
                key={doc.id}
                className={doc.id === selectedId ? "is-selected" : undefined}
              >
                <td>
                  <button
                    type="button"
                    className="file-link"
                    aria-current={doc.id === selectedId ? "true" : undefined}
                    onClick={() => onSelect(doc.id)}
                  >
                    <span className="file-name">{doc.fileName}</span>
                    {doc.format && (
                      <span className="file-format">{doc.format}</span>
                    )}
                  </button>
                  {doc.status === "failed" && doc.error && (
                    <p className="file-error">{doc.error}</p>
                  )}
                </td>
                <td className="col-status">
                  <StatusPill status={doc.status} />
                  {doc.status !== "ready" && doc.status !== "failed" && (
                    <span className="mini-bar" aria-hidden="true">
                      <span style={{ width: formatPercent(doc.progress, 0) }} />
                    </span>
                  )}
                </td>
                <td className="col-actions">
                  <button
                    type="button"
                    className="btn btn-quiet btn-sm"
                    onClick={() => onRename(doc)}
                  >
                    Rename
                  </button>
                  {(doc.status === "failed" || doc.status === "ready") && (
                    <button
                      type="button"
                      className="btn btn-quiet btn-sm"
                      onClick={() => onRetry(doc)}
                    >
                      {doc.status === "failed" ? "Retry" : "Reprocess"}
                    </button>
                  )}
                  <button
                    type="button"
                    className="btn btn-quiet btn-sm btn-danger-text"
                    onClick={() => onDelete(doc)}
                  >
                    Delete
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <Pagination
        total={documents.length}
        offset={offset}
        limit={limit}
        onChange={setOffset}
        onLimitChange={(next) => {
          setLimit(next);
          setOffset(0);
        }}
        noun="files"
      />
    </>
  );
}
