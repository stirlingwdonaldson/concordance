"use client";

import { useEffect, useState } from "react";
import { useAsync, useDebounced } from "../hooks/use-async";
import { usePartsOfSpeech } from "../hooks/use-pos";
import { api, exportUrl } from "../lib/api";
import { posLabel } from "../lib/format";
import { EXAMPLE_SEARCHES } from "../lib/glossary";
import { ExportLink } from "./export-link";
import { Pagination } from "./pagination";
import { PosSelect } from "./pos-select";
import { Term } from "./term";

type Sort = "position-asc" | "keyword-asc" | "lemma-asc";
type Scope = "file" | "project";

const SORTS: { value: Sort; label: string }[] = [
  { value: "position-asc", label: "In order of appearance" },
  { value: "keyword-asc", label: "By spelling (A to Z)" },
  { value: "lemma-asc", label: "By dictionary form" },
];

export function KwicCard({
  projectId,
  documentId,
  ready,
  word,
  onWordChange,
  fileCount,
}: Readonly<{
  projectId: string;
  documentId: string;
  ready: boolean;
  word: string;
  onWordChange: (word: string) => void;
  fileCount: number;
}>) {
  const [scope, setScope] = useState<Scope>("file");
  const [pos, setPos] = useState("");
  const [sort, setSort] = useState<Sort>("position-asc");
  const [limit, setLimit] = useState(25);
  const [offset, setOffset] = useState(0);
  const debounced = useDebounced(word);
  const searching = debounced.trim() !== "";
  const across = scope === "project" && fileCount > 1;
  const posOptions = usePartsOfSpeech(documentId, ready);

  // biome-ignore lint/correctness/useExhaustiveDependencies: a new document or query restarts at page 1
  useEffect(
    () => setOffset(0),
    [documentId, debounced, sort, limit, pos, scope],
  );

  const [by, dir] = sort.split("-");
  const params = { lemma: debounced, pos, sort: by, dir, limit, offset };
  const { data, error, loading } = useAsync(
    (signal) =>
      across
        ? api.projectKwic(projectId, params, signal)
        : api.kwic(documentId, params, signal),
    [across, projectId, documentId, debounced, pos, by, dir, limit, offset],
    ready && searching,
  );

  const exportHref = across
    ? exportUrl(`/api/projects/${projectId}/kwic.csv`, {
        lemma: debounced,
        pos,
        sort: by,
        dir,
      })
    : exportUrl(`/api/documents/${documentId}/kwic.csv`, {
        lemma: debounced,
        pos,
        sort: by,
        dir,
      });

  return (
    <>
      <div className="toolbar">
        <label className="field field-grow">
          <span className="sr-only">Word to look up</span>
          <input
            type="search"
            value={word}
            placeholder="Type a word, e.g. learning"
            onChange={(event) => onWordChange(event.target.value)}
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

      {fileCount > 1 && (
        <fieldset className="segmented" aria-label="Where to search">
          <label className={scope === "file" ? "is-on" : undefined}>
            <input
              type="radio"
              name="kwic-scope"
              checked={scope === "file"}
              onChange={() => setScope("file")}
            />
            This file
          </label>
          <label className={scope === "project" ? "is-on" : undefined}>
            <input
              type="radio"
              name="kwic-scope"
              checked={scope === "project"}
              onChange={() => setScope("project")}
            />
            All {fileCount} files in this project
          </label>
        </fieldset>
      )}

      <p className="examples">
        {!searching ? (
          <>
            Try:{" "}
            {EXAMPLE_SEARCHES.map((example) => (
              <button
                key={example}
                type="button"
                className="chip"
                onClick={() => onWordChange(example)}
              >
                {example}
              </button>
            ))}
          </>
        ) : (
          <>
            Matches whole words, in any form. Add a <Term k="wildcard">*</Term>{" "}
            to match part of a word.{" "}
            <button
              type="button"
              className="link"
              onClick={() => onWordChange("")}
            >
              Clear
            </button>
          </>
        )}
      </p>

      {error && <p className="callout callout-bad">{error}</p>}

      <ol
        className={`kwic${loading ? " is-loading" : ""}`}
        aria-label="Word in context"
      >
        {data?.items.map((row) => (
          <li key={`${row.documentId ?? ""}-${row.id}`} className="kwic-row">
            {across && row.documentName && (
              <span className="kwic-file">{row.documentName}</span>
            )}
            <span className="kwic-left">{row.leftContext}</span>
            <mark
              className="kwic-key"
              title={row.pos ? posLabel(row.pos) : undefined}
            >
              {row.keyword}
            </mark>
            <span className="kwic-right">{row.rightContext}</span>
          </li>
        ))}
      </ol>

      {!searching && (
        <p className="empty">
          Pick a word from the list or chart, or type one above, to read every
          place it appears.
        </p>
      )}
      {searching && data && data.items.length === 0 && (
        <p className="empty">
          {pos !== ""
            ? `No “${debounced}” used as a ${posLabel(pos).toLowerCase()}. Files processed before this update need Reprocess before they can be filtered by type of word.`
            : `“${debounced}” doesn't appear${across ? " in this project" : " in this file"}. Check the spelling, or add * (for example ${debounced}*).`}
        </p>
      )}
      {searching && data && (
        <Pagination
          total={data.total}
          offset={offset}
          limit={limit}
          onChange={setOffset}
          onLimitChange={setLimit}
          noun="matches"
          extra={
            <ExportLink
              href={exportHref}
              title="Download these matches (up to 50,000) as a spreadsheet"
            />
          }
        />
      )}
      <p className="hint">
        Each line shows one <Term k="kwic" /> match with its{" "}
        <Term k="context" />. Hover a highlighted word to see its type.
      </p>
    </>
  );
}
