export type Health = {
  status: string;
  checks?: {
    database?: string;
    nlp_sidecar?: string;
  };
};

export type Project = {
  id: string;
  name: string;
};

export type Document = {
  id: string;
  fileName: string;
  status: string;
  progress: number;
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
};

export type KWICListPayload = {
  items: KWICOccurrence[];
  total: number;
  limit: number;
  offset: number;
  sort: string;
  dir: string;
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
