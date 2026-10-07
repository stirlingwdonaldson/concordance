"use client";

import type { ReactNode } from "react";
import { formatNumber } from "../lib/format";

export const PAGE_SIZES = [10, 25, 50, 100];

function pageWindow(current: number, last: number): (number | "gap")[] {
  const pages = new Set([1, last, current - 1, current, current + 1]);
  const sorted = [...pages]
    .filter((p) => p >= 1 && p <= last)
    .sort((a, b) => a - b);
  const out: (number | "gap")[] = [];
  for (const page of sorted) {
    const previous = out[out.length - 1];
    if (typeof previous === "number" && page - previous > 1) out.push("gap");
    out.push(page);
  }
  return out;
}

export function Pagination({
  total,
  offset,
  limit,
  onChange,
  onLimitChange,
  noun = "results",
  extra,
}: Readonly<{
  total: number;
  offset: number;
  limit: number;
  onChange: (offset: number) => void;
  onLimitChange?: (limit: number) => void;
  noun?: string;
  extra?: ReactNode;
}>) {
  const last = Math.max(1, Math.ceil(total / limit));
  const current = Math.min(last, Math.floor(offset / limit) + 1);
  const from = total === 0 ? 0 : offset + 1;
  const to = Math.min(total, offset + limit);

  return (
    <nav className="pager" aria-label={`${noun} pages`}>
      <p className="pager-count">
        {total === 0
          ? `No ${noun}`
          : `${formatNumber(from)}–${formatNumber(to)} of ${formatNumber(total)} ${noun}`}
      </p>
      <div className="pager-controls">
        {extra}
        {onLimitChange && (
          <label className="pager-size">
            <span className="sr-only">Rows per page</span>
            <select
              value={limit}
              onChange={(event) => onLimitChange(Number(event.target.value))}
            >
              {PAGE_SIZES.map((size) => (
                <option key={size} value={size}>
                  {size} per page
                </option>
              ))}
            </select>
          </label>
        )}
        <button
          type="button"
          className="btn btn-quiet btn-sm"
          disabled={current <= 1}
          onClick={() => onChange((current - 2) * limit)}
        >
          Previous
        </button>
        <ol className="pager-pages">
          {pageWindow(current, last).map((page, index) =>
            page === "gap" ? (
              // biome-ignore lint/suspicious/noArrayIndexKey: gaps have no identity
              <li key={`gap-${index}`} aria-hidden="true">
                …
              </li>
            ) : (
              <li key={page}>
                <button
                  type="button"
                  className={`pager-page${page === current ? " is-current" : ""}`}
                  aria-current={page === current ? "page" : undefined}
                  onClick={() => onChange((page - 1) * limit)}
                >
                  {formatNumber(page)}
                </button>
              </li>
            ),
          )}
        </ol>
        <button
          type="button"
          className="btn btn-quiet btn-sm"
          disabled={current >= last}
          onClick={() => onChange(current * limit)}
        >
          Next
        </button>
      </div>
    </nav>
  );
}
