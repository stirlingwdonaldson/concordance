"use client";

import { GLOSSARY } from "../lib/glossary";
import { Dialog } from "./dialogs";

export function GlossaryPanel({
  open,
  onClose,
}: Readonly<{ open: boolean; onClose: () => void }>) {
  const entries = Object.values(GLOSSARY) as {
    term: string;
    short: string;
    example?: string;
  }[];

  return (
    <Dialog open={open} title="Glossary" onClose={onClose}>
      <p className="dialog-copy">
        Plain-language meanings for the terms used here. Hover or focus any
        underlined term to see these in place.
      </p>
      <dl className="glossary">
        {entries
          .sort((a, b) => a.term.localeCompare(b.term))
          .map((entry) => (
            <div key={entry.term}>
              <dt>{entry.term}</dt>
              <dd>
                {entry.short}
                {entry.example && <em> {entry.example}</em>}
              </dd>
            </div>
          ))}
      </dl>
      <div className="dialog-actions">
        <button type="button" className="btn btn-primary" onClick={onClose}>
          Done
        </button>
      </div>
    </Dialog>
  );
}
