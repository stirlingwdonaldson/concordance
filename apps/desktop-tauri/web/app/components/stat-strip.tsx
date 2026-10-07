"use client";

import { formatNumber, formatPercent } from "../lib/format";
import type { GlossaryKey } from "../lib/glossary";
import type { DocumentStats } from "../lib/types";
import { Term } from "./term";

function Stat({
  term,
  label,
  value,
  note,
}: Readonly<{
  term: GlossaryKey;
  label: string;
  value: string;
  note?: string;
}>) {
  return (
    <div className="stat">
      <p className="stat-label">
        <Term k={term}>{label}</Term>
      </p>
      <p className="stat-value">{value}</p>
      {note && <p className="stat-note">{note}</p>}
    </div>
  );
}

export function StatStrip({
  stats,
}: Readonly<{ stats: DocumentStats | undefined }>) {
  if (!stats) {
    return (
      <div className="stats">
        {[0, 1, 2, 3, 4].map((i) => (
          <div key={i} className="stat skeleton" style={{ height: 104 }} />
        ))}
      </div>
    );
  }

  const variety = stats.tokens > 0 ? stats.terms / stats.tokens : 0;
  const hapaxShare = stats.terms > 0 ? stats.hapax / stats.terms : 0;

  return (
    <div className="stats">
      <Stat
        term="token"
        label="Words"
        value={formatNumber(stats.tokens)}
        note={`in ${formatNumber(stats.sentences)} sentences`}
      />
      <Stat
        term="types"
        label="Unique terms"
        value={formatNumber(stats.terms)}
      />
      <Stat
        term="hapax"
        label="Used once"
        value={formatNumber(stats.hapax)}
        note={`${formatPercent(hapaxShare, 0)} of unique terms`}
      />
      <Stat
        term="ttr"
        label="Vocabulary variety"
        value={variety.toFixed(3)}
        note="unique terms ÷ words"
      />
      <Stat
        term="passage"
        label="Passages"
        value={formatNumber(stats.passages)}
      />
    </div>
  );
}
