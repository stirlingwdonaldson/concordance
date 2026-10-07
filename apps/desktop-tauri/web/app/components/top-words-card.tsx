"use client";

import { useMemo, useState } from "react";
import { useAsync } from "../hooks/use-async";
import { api } from "../lib/api";
import { formatNumber } from "../lib/format";
import { Term } from "./term";

// Function words that top every frequency list and tell you nothing about topic.
const FILLER = new Set(
  "the a an of and or but to in on at by for with from as is are was were be been being it its this that these those he she they we you i his her their our your not no if than then so such can could will would may might must shall should do does did done have has had having there which who whom whose what when where why how also into over under about between through during more most other some any each both all one two s t".split(
    /\s+/,
  ),
);

const BAR_COUNT = 12;

export function TopWordsCard({
  documentId,
  ready,
  onPick,
}: Readonly<{
  documentId: string;
  ready: boolean;
  onPick: (lemma: string) => void;
}>) {
  const [hideFiller, setHideFiller] = useState(true);
  const { data, error } = useAsync(
    (signal) =>
      api.concordance(
        documentId,
        { sort: "freq", dir: "desc", limit: 120, offset: 0 },
        signal,
      ),
    [documentId],
    ready,
  );

  const rows = useMemo(() => {
    const items = data?.items ?? [];
    const kept = hideFiller
      ? items.filter(
          (t) => !FILLER.has(t.lemma.toLowerCase()) && t.lemma.length > 1,
        )
      : items;
    return kept.slice(0, BAR_COUNT);
  }, [data, hideFiller]);

  const max = rows[0]?.totalFreq ?? 1;

  return (
    <>
      <label className="switch">
        <input
          type="checkbox"
          checked={hideFiller}
          onChange={(event) => setHideFiller(event.target.checked)}
        />
        <span>Hide filler words like “the”, “of”, “and”</span>
      </label>
      {error && <p className="callout callout-bad">{error}</p>}
      {!data && !error && <div className="skeleton" style={{ height: 260 }} />}
      {data && (
        <ol className="bars" aria-label="Most frequent words">
          {rows.map((term) => (
            <li key={term.id} className="bar-row">
              <button
                type="button"
                className="bar-label"
                onClick={() => onPick(term.lemma)}
                title={`Show “${term.lemma}” in context`}
              >
                {term.lemma}
              </button>
              <span className="bar-track">
                <span
                  className="bar-fill"
                  style={{
                    width: `${Math.max(1.5, (term.totalFreq / max) * 100)}%`,
                  }}
                />
              </span>
              <span className="bar-value">{formatNumber(term.totalFreq)}</span>
            </li>
          ))}
        </ol>
      )}
      <p className="hint">
        Select a word to see it in context. Counts group word forms by their{" "}
        <Term k="lemma" />.
      </p>
    </>
  );
}
