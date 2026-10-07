"use client";

import { useEffect, useMemo, useState } from "react";
import { useAsync } from "../hooks/use-async";
import { usePartsOfSpeech } from "../hooks/use-pos";
import { api, exportUrl } from "../lib/api";
import { evidenceLabel, formatNumber } from "../lib/format";
import type { Document } from "../lib/types";
import { ExportLink } from "./export-link";
import { Pagination } from "./pagination";
import { PosSelect } from "./pos-select";
import { Term } from "./term";

type Side = "" | "a" | "b";

const MIN_COUNTS = [2, 5, 10, 25];

function perMillion(value: number): string {
  return value >= 10 ? formatNumber(Math.round(value)) : value.toFixed(1);
}

export function CompareCard({
  doc,
  others,
  onPick,
}: Readonly<{
  doc: Document;
  /** Other files in the project that have finished processing. */
  others: Document[];
  onPick: (lemma: string) => void;
}>) {
  const [againstId, setAgainstId] = useState("");
  const [side, setSide] = useState<Side>("");
  const [pos, setPos] = useState("");
  const [min, setMin] = useState(5);
  const [limit, setLimit] = useState(25);
  const [offset, setOffset] = useState(0);

  const against = others.find((d) => d.id === againstId) ?? others[0];
  const posOptions = usePartsOfSpeech(doc.id, true);

  // biome-ignore lint/correctness/useExhaustiveDependencies: changing the question restarts at page 1
  useEffect(() => setOffset(0), [doc.id, against?.id, side, pos, min, limit]);

  const { data, error, loading } = useAsync(
    (signal) =>
      api.compare(
        doc.id,
        { against: against?.id ?? "", side, pos, min, limit, offset },
        signal,
      ),
    [doc.id, against?.id, side, pos, min, limit, offset],
    against !== undefined,
  );

  const maxScore = useMemo(
    () => Math.max(1, ...(data?.items ?? []).map((item) => item.score)),
    [data],
  );

  if (!against) {
    return (
      <p className="empty">
        Add a second file to this project and finish processing it. You'll see
        which words set the two files apart.
      </p>
    );
  }

  const nameA = doc.fileName;
  const nameB = against.fileName;

  return (
    <>
      <div className="toolbar">
        <label className="field">
          <span>Compare with</span>
          <select
            value={against.id}
            onChange={(event) => setAgainstId(event.target.value)}
          >
            {others.map((other) => (
              <option key={other.id} value={other.id}>
                {other.fileName}
              </option>
            ))}
          </select>
        </label>
        <div className="field">
          <span>Show</span>
          <fieldset
            className="segmented"
            aria-label="Which file's words to show"
          >
            {(
              [
                ["", "Both"],
                ["a", "More in this file"],
                ["b", "More in the other"],
              ] as const
            ).map(([value, label]) => (
              <label
                key={value}
                className={side === value ? "is-on" : undefined}
              >
                <input
                  type="radio"
                  name="compare-side"
                  checked={side === value}
                  onChange={() => setSide(value)}
                />
                {label}
              </label>
            ))}
          </fieldset>
        </div>
        <div className="field">
          <span>Type of word</span>
          <PosSelect options={posOptions} value={pos} onChange={setPos} />
        </div>
        <label className="field">
          <span>Used at least</span>
          <select
            value={min}
            onChange={(event) => setMin(Number(event.target.value))}
          >
            {MIN_COUNTS.map((count) => (
              <option key={count} value={count}>
                {count} times in total
              </option>
            ))}
          </select>
        </label>
      </div>

      {error && <p className="callout callout-bad">{error}</p>}

      <div className={`table-wrap${loading ? " is-loading" : ""}`}>
        <table className="table table-compact">
          <thead>
            <tr>
              <th scope="col">Word</th>
              <th scope="col" className="num">
                <span className="swatch swatch-a" aria-hidden="true" />
                {nameA}
              </th>
              <th scope="col" className="num">
                <span className="swatch swatch-b" aria-hidden="true" />
                {nameB}
              </th>
              <th scope="col">
                <Term k="evidence" />
              </th>
            </tr>
          </thead>
          <tbody>
            {data?.items.map((item) => (
              <tr key={item.lemma}>
                <td>
                  <button
                    type="button"
                    className="word-link"
                    onClick={() => onPick(item.lemma)}
                    title="See in context"
                  >
                    {item.lemma}
                  </button>
                </td>
                <td className={`num${item.side === "a" ? " is-winner-a" : ""}`}>
                  {formatNumber(item.freqA)}
                  <span className="sub">
                    {perMillion(item.perMillionA)} per million
                  </span>
                </td>
                <td className={`num${item.side === "b" ? " is-winner-b" : ""}`}>
                  {formatNumber(item.freqB)}
                  <span className="sub">
                    {perMillion(item.perMillionB)} per million
                  </span>
                </td>
                <td>
                  <span className="evidence">
                    <span className="evidence-track" aria-hidden="true">
                      <span
                        className={`evidence-fill evidence-${item.side}`}
                        style={{
                          width: `${Math.max(4, (item.score / maxScore) * 100)}%`,
                        }}
                      />
                    </span>
                    {evidenceLabel(item.score)}
                  </span>
                </td>
              </tr>
            ))}
            {data && data.items.length === 0 && (
              <tr>
                <td colSpan={4} className="empty-cell">
                  Nothing stands out with these settings. Try a lower minimum
                  count.
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
              href={exportUrl(`/api/documents/${doc.id}/compare.csv`, {
                against: against.id,
                side,
                pos,
                min,
              })}
              title="Download this comparison as a spreadsheet"
            />
          }
        />
      )}
      <p className="hint">
        Words are ranked by how clearly they favour one file. Counts are shown
        with the rate <Term k="perMillion">per million words</Term>, so a short
        file isn't unfairly compared with a long one.
      </p>
    </>
  );
}
