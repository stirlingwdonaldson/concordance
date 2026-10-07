"use client";

import { api } from "../lib/api";
import type { POSCount } from "../lib/types";
import { useAsync } from "./use-async";

/** Part-of-speech tags present in a file, most common first. */
export function usePartsOfSpeech(
  documentId: string,
  enabled: boolean,
): POSCount[] {
  const { data } = useAsync(
    (signal) => api.partsOfSpeech(documentId, signal),
    [documentId],
    enabled,
  );
  return (data ?? []).filter(
    (item) => item.pos !== "PUNCT" && item.pos !== "SPACE",
  );
}
