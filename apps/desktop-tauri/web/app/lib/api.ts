import type {
  ConcordanceTerm,
  Document,
  Health,
  KWICListPayload,
  PipelineJob,
  Project,
} from "./types";

export const apiBase =
  process.env.NEXT_PUBLIC_API_BASE ?? "http://127.0.0.1:8080";

type OnError = (context: string, error: unknown) => void;

export async function fetchHealth(onError: OnError): Promise<Health | null> {
  try {
    const response = await fetch(`${apiBase}/ready`);
    if (!response.ok) {
      return null;
    }
    return (await response.json()) as Health;
  } catch (error) {
    onError("Unable to load health state.", error);
    return null;
  }
}

export async function fetchProjects(
  onError: OnError,
): Promise<Project[] | null> {
  try {
    const response = await fetch(`${apiBase}/api/projects`);
    if (!response.ok) {
      return null;
    }
    const payload = (await response.json()) as { items: Project[] };
    return payload.items;
  } catch (error) {
    onError("Unable to load projects.", error);
    return null;
  }
}

export async function fetchDocuments(
  projectId: string,
  onError: OnError,
): Promise<Document[] | null> {
  try {
    const response = await fetch(
      `${apiBase}/api/projects/${projectId}/documents`,
    );
    if (!response.ok) {
      return null;
    }
    const payload = (await response.json()) as { items: Document[] };
    return payload.items;
  } catch (error) {
    onError("Unable to load documents.", error);
    return null;
  }
}

export async function fetchPipelineJobs(
  documentId: string,
  onError: OnError,
): Promise<PipelineJob[] | null> {
  try {
    const response = await fetch(
      `${apiBase}/api/documents/${documentId}/pipeline-jobs`,
    );
    if (!response.ok) {
      return null;
    }
    const payload = (await response.json()) as { items: PipelineJob[] };
    return payload.items;
  } catch (error) {
    onError("Unable to load pipeline jobs.", error);
    return null;
  }
}

export async function fetchConcordance(
  documentId: string,
  lemma: string,
  pos: string,
  onError: OnError,
): Promise<ConcordanceTerm[] | null> {
  const query = new URLSearchParams();
  if (lemma.trim() !== "") {
    query.set("lemma", lemma.trim());
  }
  if (pos.trim() !== "") {
    query.set("pos", pos.trim());
  }

  try {
    const response = await fetch(
      `${apiBase}/api/documents/${documentId}/concordance?${query.toString()}`,
    );
    if (!response.ok) {
      return null;
    }
    const payload = (await response.json()) as { items: ConcordanceTerm[] };
    return payload.items;
  } catch (error) {
    onError("Unable to load concordance.", error);
    return null;
  }
}

export interface KWICQueryParams {
  lemma: string;
  rawLimit: string;
  sortBy: string;
  sortDir: string;
  offset: number;
}

export async function fetchKWIC(
  documentId: string,
  params: KWICQueryParams,
  onError: OnError,
): Promise<KWICListPayload | null> {
  const query = new URLSearchParams();
  if (params.lemma.trim() !== "") {
    query.set("lemma", params.lemma.trim());
  }

  const limit = Number.parseInt(params.rawLimit, 10);
  if (!Number.isNaN(limit) && limit > 0) {
    query.set("limit", String(limit));
  }
  query.set("sort", params.sortBy);
  query.set("dir", params.sortDir);
  query.set("offset", String(Math.max(0, params.offset)));

  try {
    const response = await fetch(
      `${apiBase}/api/documents/${documentId}/kwic?${query.toString()}`,
    );
    if (!response.ok) {
      return null;
    }
    return (await response.json()) as KWICListPayload;
  } catch (error) {
    onError("Unable to load KWIC occurrences.", error);
    return null;
  }
}

export async function createProjectRequest(
  name: string,
  onError: OnError,
): Promise<boolean> {
  try {
    const response = await fetch(`${apiBase}/api/projects`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name }),
    });
    return response.ok;
  } catch (error) {
    onError("Unable to create project.", error);
    return false;
  }
}

export async function uploadDocumentRequest(
  projectId: string,
  file: File,
  onError: OnError,
): Promise<boolean> {
  const formData = new FormData();
  formData.append("file", file);

  try {
    const response = await fetch(
      `${apiBase}/api/projects/${projectId}/documents/upload`,
      { method: "POST", body: formData },
    );
    return response.ok;
  } catch (error) {
    onError("Upload failed.", error);
    return false;
  }
}

export async function retryDocumentRequest(
  documentId: string,
  onError: OnError,
): Promise<boolean> {
  try {
    const response = await fetch(
      `${apiBase}/api/documents/${documentId}/retry`,
      { method: "POST" },
    );
    return response.ok;
  } catch (error) {
    onError("Retry failed.", error);
    return false;
  }
}
