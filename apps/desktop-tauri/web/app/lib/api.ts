import type {
  ComparePage,
  ConcordanceTerm,
  Document,
  DocumentStats,
  Health,
  KWICOccurrence,
  POSCount,
  Page,
  PipelineJob,
  Project,
} from "./types";

export const apiBase =
  process.env.NEXT_PUBLIC_API_BASE ?? "http://127.0.0.1:8080";

/** An error with a message that is safe to show to the person using the app. */
export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message);
  }
}

async function request<T>(
  path: string,
  init?: RequestInit,
  signal?: AbortSignal,
): Promise<T> {
  let response: Response;
  try {
    response = await fetch(`${apiBase}${path}`, { ...init, signal });
  } catch (error) {
    if (error instanceof DOMException && error.name === "AbortError") {
      throw error;
    }
    throw new ApiError(
      `Can't reach the Concordance API at ${apiBase}. Is it running?`,
      0,
    );
  }

  if (!response.ok) {
    let message = `Request failed (${response.status}).`;
    try {
      const body = (await response.json()) as { error?: string };
      if (body.error) {
        message = body.error.charAt(0).toUpperCase() + body.error.slice(1);
      }
    } catch {
      // keep the generic message
    }
    throw new ApiError(message, response.status);
  }

  if (response.status === 204) {
    return undefined as T;
  }
  return (await response.json()) as T;
}

const json = (method: string, body: unknown): RequestInit => ({
  method,
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify(body),
});

function query(params: Record<string, string | number | undefined>): string {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== "") {
      search.set(key, String(value));
    }
  }
  const text = search.toString();
  return text === "" ? "" : `?${text}`;
}

/** A URL the browser can open directly, used for CSV downloads. */
export function exportUrl(
  path: string,
  params: Record<string, string | number | undefined> = {},
): string {
  return `${apiBase}${path}${query(params)}`;
}

export type KwicParams = {
  lemma?: string;
  pos?: string;
  sort?: string;
  dir?: string;
  limit: number;
  offset: number;
};

export type CompareParams = {
  against: string;
  side?: string;
  pos?: string;
  lemma?: string;
  min?: number;
  limit: number;
  offset: number;
};

export const api = {
  health: (signal?: AbortSignal) =>
    request<Health>("/ready", undefined, signal),

  listProjects: (signal?: AbortSignal) =>
    request<{ items: Project[] }>("/api/projects", undefined, signal).then(
      (r) => r.items,
    ),
  createProject: (name: string, description = "") =>
    request<Project>("/api/projects", json("POST", { name, description })),
  updateProject: (id: string, name: string, description = "") =>
    request<Project>(
      `/api/projects/${id}`,
      json("PATCH", { name, description }),
    ),
  deleteProject: (id: string) =>
    request<void>(`/api/projects/${id}`, { method: "DELETE" }),

  listDocuments: (projectId: string, signal?: AbortSignal) =>
    request<{ items: Document[] }>(
      `/api/projects/${projectId}/documents`,
      undefined,
      signal,
    ).then((r) => r.items),
  uploadDocument: (projectId: string, file: File) => {
    const form = new FormData();
    form.append("file", file);
    return request<Document>(`/api/projects/${projectId}/documents/upload`, {
      method: "POST",
      body: form,
    });
  },
  renameDocument: (id: string, fileName: string) =>
    request<Document>(`/api/documents/${id}`, json("PATCH", { fileName })),
  deleteDocument: (id: string) =>
    request<void>(`/api/documents/${id}`, { method: "DELETE" }),
  retryDocument: (id: string) =>
    request<Document>(`/api/documents/${id}/retry`, { method: "POST" }),
  documentStatus: (id: string, signal?: AbortSignal) =>
    request<Document>(`/api/documents/${id}/status`, undefined, signal),
  documentStats: (id: string, signal?: AbortSignal) =>
    request<DocumentStats>(`/api/documents/${id}/stats`, undefined, signal),
  pipelineJobs: (id: string, signal?: AbortSignal) =>
    request<{ items: PipelineJob[] }>(
      `/api/documents/${id}/pipeline-jobs`,
      undefined,
      signal,
    ).then((r) => r.items),

  partsOfSpeech: (id: string, signal?: AbortSignal) =>
    request<{ items: POSCount[] }>(
      `/api/documents/${id}/pos`,
      undefined,
      signal,
    ).then((r) => r.items),
  compare: (id: string, params: CompareParams, signal?: AbortSignal) =>
    request<ComparePage>(
      `/api/documents/${id}/compare${query(params)}`,
      undefined,
      signal,
    ),
  projectKwic: (projectId: string, params: KwicParams, signal?: AbortSignal) =>
    request<Page<KWICOccurrence>>(
      `/api/projects/${projectId}/kwic${query(params)}`,
      undefined,
      signal,
    ),
  concordance: (
    id: string,
    params: {
      lemma?: string;
      pos?: string;
      sort?: string;
      dir?: string;
      limit: number;
      offset: number;
    },
    signal?: AbortSignal,
  ) =>
    request<Page<ConcordanceTerm>>(
      `/api/documents/${id}/concordance${query(params)}`,
      undefined,
      signal,
    ),
  kwic: (id: string, params: KwicParams, signal?: AbortSignal) =>
    request<Page<KWICOccurrence>>(
      `/api/documents/${id}/kwic${query(params)}`,
      undefined,
      signal,
    ),
};
