"use client";

import { useState } from "react";
import type { KeyboardEvent } from "react";
import type { KWICOccurrence } from "../lib/types";

interface KWICPanelProps {
  selectedDocument: string;
  kwicRows: KWICOccurrence[];
  kwicTotal: number;
  kwicOffset: number;
  kwicLemma: string;
  kwicLimit: string;
  kwicSort: string;
  kwicDir: string;
  onLemmaChange: (lemma: string) => void;
  onLimitChange: (limit: string) => void;
  onSortChange: (sort: string) => void;
  onDirChange: (dir: string) => void;
  onQuery: (offset: number) => void;
}

export function KWICPanel({
  selectedDocument,
  kwicRows,
  kwicTotal,
  kwicOffset,
  kwicLemma,
  kwicLimit,
  kwicSort,
  kwicDir,
  onLemmaChange,
  onLimitChange,
  onSortChange,
  onDirChange,
  onQuery,
}: KWICPanelProps) {
  const [pageInput, setPageInput] = useState("1");

  const pageSize = Math.max(1, Number.parseInt(kwicLimit, 10) || 50);
  const from = kwicRows.length === 0 ? 0 : kwicOffset + 1;
  const to = kwicOffset + kwicRows.length;
  const canGoPrev = kwicOffset > 0;
  const canGoNext = kwicOffset + kwicRows.length < kwicTotal;
  const totalPages = Math.max(1, Math.ceil(kwicTotal / pageSize));

  function runQuery() {
    if (selectedDocument === "") {
      return;
    }
    setPageInput("1");
    onQuery(0);
  }

  function goToPage() {
    if (selectedDocument === "") {
      return;
    }
    const page = Number.parseInt(pageInput, 10);
    if (Number.isNaN(page) || page <= 0) {
      return;
    }
    const clampedPage = Math.min(page, totalPages);
    onQuery((clampedPage - 1) * pageSize);
  }

  function handleQueryEnter(
    event: KeyboardEvent<HTMLInputElement | HTMLSelectElement>,
  ) {
    if (event.key === "Enter") {
      event.preventDefault();
      runQuery();
    }
  }

  function handlePageEnter(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === "Enter") {
      event.preventDefault();
      goToPage();
    }
  }

  return (
    <section className="panel">
      <h2>KWIC explorer</h2>
      <div className="row">
        <input
          value={kwicLemma}
          onChange={(event) => onLemmaChange(event.target.value)}
          onKeyDown={handleQueryEnter}
          placeholder="Filter by lemma"
          disabled={selectedDocument === ""}
        />
        <input
          value={kwicLimit}
          onChange={(event) => onLimitChange(event.target.value)}
          onKeyDown={handleQueryEnter}
          placeholder="Limit"
          inputMode="numeric"
          disabled={selectedDocument === ""}
        />
        <select
          value={kwicSort}
          onChange={(event) => onSortChange(event.target.value)}
          onKeyDown={handleQueryEnter}
          disabled={selectedDocument === ""}
        >
          <option value="position">Sort: Position</option>
          <option value="lemma">Sort: Lemma</option>
          <option value="keyword">Sort: Keyword</option>
        </select>
        <select
          value={kwicDir}
          onChange={(event) => onDirChange(event.target.value)}
          onKeyDown={handleQueryEnter}
          disabled={selectedDocument === ""}
        >
          <option value="asc">Asc</option>
          <option value="desc">Desc</option>
        </select>
        <button
          type="button"
          disabled={selectedDocument === ""}
          onClick={runQuery}
        >
          Run query
        </button>
        <button
          type="button"
          disabled={selectedDocument === "" || !canGoPrev}
          onClick={() => onQuery(Math.max(0, kwicOffset - pageSize))}
        >
          Previous
        </button>
        <button
          type="button"
          disabled={selectedDocument === "" || !canGoNext}
          onClick={() => onQuery(kwicOffset + pageSize)}
        >
          Next
        </button>
        <input
          value={pageInput}
          onChange={(event) => setPageInput(event.target.value)}
          onKeyDown={handlePageEnter}
          placeholder="Page"
          inputMode="numeric"
          disabled={selectedDocument === ""}
        />
        <button
          type="button"
          disabled={selectedDocument === ""}
          onClick={goToPage}
        >
          Go to page
        </button>
      </div>
      <p className="kwic-meta">
        Showing {from}-{to} of {kwicTotal} (page{" "}
        {Math.floor(kwicOffset / pageSize) + 1} / {totalPages})
      </p>
      {kwicRows.length === 0 ? (
        <p>No KWIC rows yet.</p>
      ) : (
        <ul className="kwic-list">
          {kwicRows.map((row) => (
            <li key={row.id}>
              <span className="kwic-left">{row.leftContext}</span>
              <strong className="kwic-keyword">{row.keyword}</strong>
              <span className="kwic-right">{row.rightContext}</span>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
