export function formatElapsed(ms: number): string {
  const seconds = Math.max(0, Math.floor(ms / 1000));
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  return `${Math.floor(minutes / 60)}h ago`;
}

export function toWebSocketBase(rawBase: string): string {
  const url = new URL(rawBase);
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  url.pathname = "";
  url.search = "";
  url.hash = "";
  return url.toString().replace(/\/$/, "");
}

const number = new Intl.NumberFormat("en-AU");

export function formatNumber(value: number): string {
  return number.format(value);
}

/** 1_240_000 -> "1.24M", 18_300 -> "18.3k" for tight spaces. */
export function formatCompact(value: number): string {
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(2)}M`;
  if (value >= 10_000) return `${(value / 1_000).toFixed(1)}k`;
  return formatNumber(value);
}

export function formatPercent(fraction: number, digits = 1): string {
  return `${(fraction * 100).toFixed(digits)}%`;
}

const STAGE_LABELS: Record<string, string> = {
  ingest_parse: "Read the file",
  segment_structure: "Split into sentences",
  nlp_analyze: "Tag every word",
  concordance_aggregate: "Build the word list",
};

export function stageLabel(stage: string): string {
  return STAGE_LABELS[stage] ?? stage;
}

export const STAGE_ORDER = [
  "ingest_parse",
  "segment_structure",
  "nlp_analyze",
  "concordance_aggregate",
];

const POS_LABELS: Record<string, string> = {
  NOUN: "Noun",
  PROPN: "Name",
  VERB: "Verb",
  AUX: "Helper verb",
  ADJ: "Adjective",
  ADV: "Adverb",
  ADP: "Preposition",
  DET: "Article",
  PRON: "Pronoun",
  NUM: "Number",
  CCONJ: "Conjunction",
  SCONJ: "Conjunction",
  PART: "Particle",
  INTJ: "Interjection",
  SYM: "Symbol",
  X: "Other",
  UNK: "Unknown",
};

/** "NOUN" -> "Noun". Unknown tags are shown as they are. */
export function posLabel(tag: string): string {
  return POS_LABELS[tag] ?? tag;
}

/** Log-likelihood evidence bands (p < 0.0001 and p < 0.01). */
export function evidenceLabel(score: number): "Strong" | "Moderate" | "Weak" {
  if (score >= 15.13) return "Strong";
  if (score >= 6.63) return "Moderate";
  return "Weak";
}
