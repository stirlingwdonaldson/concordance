"use client";

import { useEffect, useState } from "react";
import { useAsync, useDebounced } from "../hooks/use-async";
import { usePartsOfSpeech } from "../hooks/use-pos";
import { api, exportUrl } from "../lib/api";
import { formatNumber } from "../lib/format";
import { EXAMPLE_SEARCHES } from "../lib/glossary";
import { ExportLink } from "./export-link";
import { Pagination } from "./pagination";
import { PosSelect } from "./pos-select";
import { Term } from "./term";

type Sort = "freq-desc" | "freq-asc" | "lemma-asc" | "lemma-desc";

const SORTS: { value: Sort; label: string }[] = [
  { value: "freq-desc", label: "Most frequent first" },
  { value: "freq-asc", label: "Least frequent first" },
  { value: "lemma-asc", label: "A to Z" },
  { value: "lemma-desc", label: "Z to A" },
];

export function WordListCard({
  documentId,
  ready,
  selectedWord,
  onPick,
}: Readonly<{
  documentId: string;
  ready: boolean;
  selectedWord: string;
  onPick: (lemma: string) => void;
}>) {
  const [search, setSearch] = useState("");
  const [sort, setSort] = useState<Sort>("freq-desc");
  const [pos, setPos] = useState("");
  const [limit, setLimit] = useState(25);
  const [offset, setOffset] = useState(0);
  const debounced = useDebounced(search);

  // biome-ignore lint/correctness/useExhaustiveDependencies: a new document or query restarts at page 1
  useEffect(() => setOffset(0), [documentId, debounced, sort, limit, pos]);

  const posOptions = usePartsOfSpeech(documentId, ready);
  const [by, dir] = sort.split("-");
  const { data, error, loading } = useAsync(
    (signal) =>
      api.concordance(
        documentId,
        { lemma: debounced, sort: by, dir, limit, offset },
        signal,
      ),
    [documentId, debounced, by, dir, limit, offset],
    ready,
  );

  return (
    <>
      <div className="toolbar">
        <label className="field field-grow">
          <span className="sr-only">Search words</span>
          <input
            type="search"
            value={search}
            placeholder="Search words"
            onChange={(event) => setSearch(event.target.value)}
          />
        </label>
        <PosSelect options={posOptions} value={pos} onChange={setPos} />
        <label className="field">
          <span className="sr-only">Sort</span>
          <select
            value={sort}
            onChange={(event) => setSort(event.target.value as Sort)}
          >
            {SORTS.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        </label>
      </div>
      {search === "" && (
        <p className="examples">
          Try:{" "}
          {EXAMPLE_SEARCHES.map((example) => (
            <button
              key={example}
              type="button"
              className="chip"
              onClick={() => setSearch(example)}
            >
              {example}
            </button>
          ))}
        </p>
      )}

      {error && <p className="callout callout-bad">{error}</p>}

      <div className={`table-wrap${loading ? " is-loading" : ""}`}>
        <table className="table table-compact">
          <thead>
            <tr>
              <th scope="col">
                <Term k="lemma">Word</Term>
              </th>
              <th scope="col" className="num">
                <Term k="frequency">Count</Term>
              </th>
              <th scope="col" className="col-actions">
                <span className="sr-only">Actions</span>
              </th>
            </tr>
          </thead>
          <tbody>
            {data?.items.map((term) => (
              <tr
                key={term.id}
                className={
                  term.lemma === selectedWord ? "is-selected" : undefined
                }
              >
                <td>
                  {term.lemma}
                  {term.hapax && (
                    <span className="badge" title="Appears exactly once">
                      once
                    </span>
                  )}
                </td>
                <td className="num">{formatNumber(term.totalFreq)}</td>
                <td className="col-actions">
                  <button
                    type="button"
                    className="btn btn-quiet btn-sm"
                    onClick={() => onPick(term.lemma)}
                  >
                    See in context
                  </button>
                </td>
              </tr>
            ))}
            {data && data.items.length === 0 && (
              <tr>
                <td colSpan={3} className="empty-cell">
                  No words match “{debounced}”. Try fewer letters.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
      {data && (
        <Pagination
          total={data.total}
          offset={offset}
          limit={limit}
          onChange={setOffset}
          onLimitChange={setLimit}
          noun="words"
          extra={
            <ExportLink
              href={exportUrl(`/api/documents/${documentId}/concordance.csv`, {
                lemma: debounced,
                pos,
                sort: by,
                dir,
              })}
              title="Download this word list, with your search and filters, as a spreadsheet"
            />
          }
        />
      )}
    </>
  );
}
