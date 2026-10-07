"use client";

import { formatNumber, posLabel } from "../lib/format";
import type { POSCount } from "../lib/types";

export function PosSelect({
  options,
  value,
  onChange,
}: Readonly<{
  options: POSCount[];
  value: string;
  onChange: (value: string) => void;
}>) {
  return (
    <label className="field">
      <span className="sr-only">Type of word</span>
      <select value={value} onChange={(event) => onChange(event.target.value)}>
        <option value="">Any type of word</option>
        {options.map((option) => (
          <option key={option.pos} value={option.pos}>
            {posLabel(option.pos)} ({formatNumber(option.count)})
          </option>
        ))}
      </select>
    </label>
  );
}
