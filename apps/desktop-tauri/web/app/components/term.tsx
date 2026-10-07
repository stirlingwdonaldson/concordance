"use client";

import { useId, useState } from "react";
import type { ReactNode } from "react";
import { createPortal } from "react-dom";
import { GLOSSARY, type GlossaryKey } from "../lib/glossary";

type Placement = { left: number; top: number; above: boolean };

/**
 * Jargon with a plain-language explanation on hover or keyboard focus.
 * The bubble is portalled and fixed-positioned so cards can't clip it.
 */
export function Term({
  k,
  children,
}: Readonly<{ k: GlossaryKey; children?: ReactNode }>) {
  const entry = GLOSSARY[k] as {
    term: string;
    short: string;
    example?: string;
  };
  const id = useId();
  const [placement, setPlacement] = useState<Placement | null>(null);

  function show(element: HTMLElement) {
    const rect = element.getBoundingClientRect();
    const width = 280;
    const left = Math.min(
      Math.max(12, rect.left + rect.width / 2 - width / 2),
      window.innerWidth - width - 12,
    );
    const above = rect.top > 170;
    setPlacement({
      left,
      top: above ? rect.top - 10 : rect.bottom + 10,
      above,
    });
  }

  return (
    <>
      <button
        type="button"
        className="term"
        aria-describedby={placement ? id : undefined}
        onMouseEnter={(event) => show(event.currentTarget)}
        onFocus={(event) => show(event.currentTarget)}
        onMouseLeave={() => setPlacement(null)}
        onBlur={() => setPlacement(null)}
        onKeyDown={(event) => {
          if (event.key === "Escape") setPlacement(null);
        }}
      >
        {children ?? entry.term}
      </button>
      {placement &&
        createPortal(
          <div
            id={id}
            role="tooltip"
            className={`tip ${placement.above ? "tip-above" : "tip-below"}`}
            style={{ left: placement.left, top: placement.top }}
          >
            <strong>{entry.term}</strong>
            <span>{entry.short}</span>
            {entry.example && <em>{entry.example}</em>}
          </div>,
          document.body,
        )}
    </>
  );
}
