export type Health = {
  status: string;
  checks?: { database?: string; nlp_sidecar?: string };
};

export type Project = {
  id: string;
  name: string;
  description?: string;
  createdAt?: string;
};

export type Document = {
  id: string;
  projectId?: string;
  fileName: string;
  format?: string;
  status: string;
  progress: number;
  error?: string;
  createdAt?: string;
};

export type DocumentStats = {
  passages: number;
  sentences: number;
  tokens: number;
  terms: number;
  hapax: number;
};

export type PipelineJob = {
  id: string;
  stage: string;
  status: string;
  message: string;
  startedAt: string;
  finishedAt: string;
};

export type ConcordanceTerm = {
  id: number;
  lemma: string;
  normalizedForm: string;
  totalFreq: number;
  hapax: boolean;
};

export type KWICOccurrence = {
  id: number;
  termId: number;
  lemma: string;
  sentenceId: string;
  leftContext: string;
  keyword: string;
  rightContext: string;
  pos?: string;
  /** Only set when searching across every file in a project. */
  documentId?: string;
  documentName?: string;
};

export type POSCount = { pos: string; count: number };

export type KeyTerm = {
  lemma: string;
  freqA: number;
  freqB: number;
  perMillionA: number;
  perMillionB: number;
  score: number;
  side: "a" | "b";
};

export type ComparePage = Page<KeyTerm> & {
  documentA: string;
  documentB: string;
};

export type Page<T> = {
  items: T[];
  total: number;
  limit: number;
  offset: number;
};

export type JobStreamEvent = {
  version: string;
  eventId: string;
  eventType:
    | "snapshot"
    | "stage_started"
    | "stage_completed"
    | "stage_failed"
    | "document_status"
    | "heartbeat";
  occurredAt: string;
  documentId?: string;
  projectId?: string;
  payload: Record<string, unknown>;
};

export type StreamConnectionState =
  | "idle"
  | "connecting"
  | "connected"
  | "reconnecting";
