"use client";

import { useEffect, useMemo, useState } from "react";
import type { ChangeEvent, KeyboardEvent } from "react";

type Health = {
  status: string;
  checks?: {
    database?: string;
    nlp_sidecar?: string;
  };
};

type Project = {
  id: string;
  name: string;
};

type Document = {
  id: string;
  fileName: string;
  status: string;
  progress: number;
};

type PipelineJob = {
  id: string;
  stage: string;
  status: string;
  message: string;
  startedAt: string;
  finishedAt: string;
};

type ConcordanceTerm = {
  id: number;
  lemma: string;
  normalizedForm: string;
  totalFreq: number;
  hapax: boolean;
};

type KWICOccurrence = {
  id: number;
  termId: number;
  lemma: string;
  sentenceId: string;
  leftContext: string;
  keyword: string;
  rightContext: string;
};

type KWICListPayload = {
  items: KWICOccurrence[];
  total: number;
  limit: number;
  offset: number;
  sort: string;
  dir: string;
};

type JobStreamEvent = {
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

type StreamConnectionState =
  | "idle"
  | "connecting"
  | "connected"
  | "reconnecting";

function formatElapsed(ms: number): string {
  const seconds = Math.max(0, Math.floor(ms / 1000));
  if (seconds < 60) {
    return `${seconds}s ago`;
  }

  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) {
    return `${minutes}m ago`;
  }

  const hours = Math.floor(minutes / 60);
  return `${hours}h ago`;
}

const apiBase = process.env.NEXT_PUBLIC_API_BASE ?? "http://127.0.0.1:8080";

function toWebSocketBase(rawBase: string): string {
  const url = new URL(rawBase);
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  url.pathname = "";
  url.search = "";
  url.hash = "";
  return url.toString().replace(/\/$/, "");
}

export default function HomePage() {
  const [health, setHealth] = useState<Health | null>(null);
  const [projects, setProjects] = useState<Project[]>([]);
  const [projectName, setProjectName] = useState("Sample Project");
  const [selectedProject, setSelectedProject] = useState<string>("");
  const [documents, setDocuments] = useState<Document[]>([]);
  const [selectedDocument, setSelectedDocument] = useState<string>("");
  const [pipelineJobs, setPipelineJobs] = useState<PipelineJob[]>([]);
  const [concordanceTerms, setConcordanceTerms] = useState<ConcordanceTerm[]>(
    [],
  );
  const [kwicRows, setKwicRows] = useState<KWICOccurrence[]>([]);
  const [kwicTotal, setKWICTotal] = useState(0);
  const [kwicOffset, setKWICOffset] = useState(0);
  const [concordanceLemma, setConcordanceLemma] = useState("");
  const [concordancePOS, setConcordancePOS] = useState("");
  const [kwicLemma, setKWICLemma] = useState("");
  const [kwicLimit, setKWICLimit] = useState("50");
  const [kwicSort, setKWICSort] = useState("position");
  const [kwicDir, setKWICDir] = useState("asc");
  const [kwicPageInput, setKWICPageInput] = useState("1");
  const [statusMessage, setStatusMessage] = useState("");
  const [streamState, setStreamState] = useState<StreamConnectionState>("idle");
  const [lastStreamEventAt, setLastStreamEventAt] = useState<number | null>(
    null,
  );
  const [nowMs, setNowMs] = useState(() => Date.now());
  const wsBase = useMemo(() => toWebSocketBase(apiBase), []);

  const selectedProjectExists = useMemo(
    () => projects.some((project) => project.id === selectedProject),
    [projects, selectedProject],
  );

  useEffect(() => {
    void refreshHealth();
    void refreshProjects();
  }, []);

  function handleRequestError(context: string, error: unknown) {
    const detail = error instanceof Error ? error.message : "Unknown error";
    setStatusMessage(
      `${context} Unable to reach API at ${apiBase} (${detail}).`,
    );
  }

  useEffect(() => {
    if (selectedProject !== "") {
      void refreshDocuments(selectedProject);
    }
  }, [selectedProject]);

  useEffect(() => {
    if (selectedDocument !== "") {
      void refreshPipelineJobs(selectedDocument);
      void refreshConcordance(
        selectedDocument,
        concordanceLemma,
        concordancePOS,
      );
      void refreshKWIC(
        selectedDocument,
        kwicLemma,
        kwicLimit,
        kwicSort,
        kwicDir,
      );
    } else {
      setPipelineJobs([]);
      setConcordanceTerms([]);
      setKwicRows([]);
      setKWICTotal(0);
      setKWICOffset(0);
      setKWICPageInput("1");
    }
  }, [
    concordanceLemma,
    concordancePOS,
    kwicLemma,
    kwicLimit,
    kwicSort,
    kwicDir,
    selectedDocument,
  ]);

  useEffect(() => {
    if (lastStreamEventAt === null) {
      return;
    }

    const intervalId = window.setInterval(() => {
      setNowMs(Date.now());
    }, 1000);

    return () => {
      window.clearInterval(intervalId);
    };
  }, [lastStreamEventAt]);

  useEffect(() => {
    if (selectedDocument === "") {
      setStreamState("idle");
      setLastStreamEventAt(null);
      return;
    }

    const documentId = selectedDocument;
    const projectId = selectedProject;
    let closed = false;
    let socket: WebSocket | null = null;
    let retryDelayMs = 500;
    let retryTimeout: ReturnType<typeof setTimeout> | null = null;
    let hasConnected = false;

    function updateDocumentStatus(status: string, progress: number) {
      setDocuments((current) =>
        current.map((doc) =>
          doc.id === documentId ? { ...doc, status, progress } : doc,
        ),
      );
    }

    function handleStreamMessage(rawData: string) {
      let event: JobStreamEvent;
      try {
        event = JSON.parse(rawData) as JobStreamEvent;
      } catch {
        return;
      }

      setLastStreamEventAt(Date.now());

      if (event.eventType === "snapshot") {
        const payloadDocument = event.payload.document as
          | { status?: string; progress?: number }
          | undefined;
        if (
          payloadDocument?.status !== undefined &&
          payloadDocument?.progress !== undefined
        ) {
          updateDocumentStatus(
            payloadDocument.status,
            payloadDocument.progress,
          );
        }

        const payloadJobs = event.payload.jobs;
        if (Array.isArray(payloadJobs)) {
          setPipelineJobs(payloadJobs as PipelineJob[]);
        }

        return;
      }

      if (event.eventType === "document_status") {
        const status = event.payload.status;
        const progress = event.payload.progress;
        if (typeof status === "string" && typeof progress === "number") {
          updateDocumentStatus(status, progress);
          if (status === "ready") {
            void refreshConcordance(
              documentId,
              concordanceLemma,
              concordancePOS,
            );
            void refreshKWIC(
              documentId,
              kwicLemma,
              kwicLimit,
              kwicSort,
              kwicDir,
            );
          }
        }
        return;
      }

      if (
        event.eventType === "stage_started" ||
        event.eventType === "stage_completed" ||
        event.eventType === "stage_failed"
      ) {
        void refreshPipelineJobs(documentId);
        if (projectId !== "") {
          void refreshDocuments(projectId);
        }
        void refreshConcordance(documentId, concordanceLemma, concordancePOS);
        void refreshKWIC(documentId, kwicLemma, kwicLimit, kwicSort, kwicDir);
      }
    }

    function connect() {
      if (closed) {
        return;
      }

      setStreamState(hasConnected ? "reconnecting" : "connecting");
      const socketUrl = `${wsBase}/ws/jobs?documentId=${encodeURIComponent(documentId)}`;
      socket = new WebSocket(socketUrl);

      socket.onopen = () => {
        hasConnected = true;
        retryDelayMs = 500;
        setStreamState("connected");
      };

      socket.onmessage = (message) => {
        if (typeof message.data !== "string") {
          return;
        }

        handleStreamMessage(message.data);
      };

      socket.onerror = () => {
        socket?.close();
      };

      socket.onclose = () => {
        if (closed) {
          return;
        }

        setStreamState("reconnecting");
        retryTimeout = setTimeout(() => {
          connect();
        }, retryDelayMs);
        retryDelayMs = Math.min(retryDelayMs * 2, 10000);
      };
    }

    connect();

    return () => {
      closed = true;
      if (retryTimeout !== null) {
        clearTimeout(retryTimeout);
      }
      socket?.close();
    };
  }, [
    concordanceLemma,
    concordancePOS,
    kwicLemma,
    kwicLimit,
    kwicSort,
    kwicDir,
    selectedDocument,
    selectedProject,
    wsBase,
  ]);

  const streamStatusLabel =
    streamState === "idle"
      ? "Idle"
      : streamState === "connecting"
        ? "Connecting"
        : streamState === "connected"
          ? "Live"
          : "Reconnecting";

  const lastEventLabel =
    lastStreamEventAt === null
      ? "No events yet"
      : `${new Date(lastStreamEventAt).toLocaleTimeString()} (${formatElapsed(nowMs - lastStreamEventAt)})`;

  const isStreamStale =
    streamState === "connected" &&
    lastStreamEventAt !== null &&
    nowMs - lastStreamEventAt > 30000;

  async function refreshHealth() {
    try {
      const response = await fetch(`${apiBase}/ready`);
      if (!response.ok) {
        setStatusMessage("Unable to load health state.");
        return;
      }

      const payload = (await response.json()) as Health;
      setHealth(payload);
    } catch (error) {
      handleRequestError("Unable to load health state.", error);
    }
  }

  async function refreshProjects() {
    try {
      const response = await fetch(`${apiBase}/api/projects`);
      if (!response.ok) {
        setStatusMessage("Unable to load projects.");
        return;
      }

      const payload = (await response.json()) as { items: Project[] };
      setProjects(payload.items);

      if (payload.items.length > 0) {
        setSelectedProject((current) => current || payload.items[0].id);
      }
    } catch (error) {
      handleRequestError("Unable to load projects.", error);
    }
  }

  async function refreshDocuments(projectId: string) {
    try {
      const response = await fetch(
        `${apiBase}/api/projects/${projectId}/documents`,
      );
      if (!response.ok) {
        setStatusMessage("Unable to load documents.");
        return;
      }

      const payload = (await response.json()) as { items: Document[] };
      setDocuments(payload.items);

      if (payload.items.length === 0) {
        setSelectedDocument("");
        return;
      }

      setSelectedDocument((current) => {
        const exists = payload.items.some(
          (document) => document.id === current,
        );
        if (exists) {
          return current;
        }

        return payload.items[0].id;
      });
    } catch (error) {
      handleRequestError("Unable to load documents.", error);
    }
  }

  async function refreshPipelineJobs(documentId: string) {
    try {
      const response = await fetch(
        `${apiBase}/api/documents/${documentId}/pipeline-jobs`,
      );
      if (!response.ok) {
        setStatusMessage("Unable to load pipeline jobs.");
        return;
      }

      const payload = (await response.json()) as { items: PipelineJob[] };
      setPipelineJobs(payload.items);
    } catch (error) {
      handleRequestError("Unable to load pipeline jobs.", error);
    }
  }

  async function refreshConcordance(
    documentId: string,
    lemma: string,
    pos: string,
  ) {
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
        setStatusMessage("Unable to load concordance.");
        return;
      }

      const payload = (await response.json()) as { items: ConcordanceTerm[] };
      setConcordanceTerms(payload.items);
    } catch (error) {
      handleRequestError("Unable to load concordance.", error);
    }
  }

  async function refreshKWIC(
    documentId: string,
    lemma: string,
    rawLimit: string,
    sortBy: string,
    sortDir: string,
    rawOffset?: number,
  ) {
    const query = new URLSearchParams();
    if (lemma.trim() !== "") {
      query.set("lemma", lemma.trim());
    }

    const limit = Number.parseInt(rawLimit, 10);
    if (!Number.isNaN(limit) && limit > 0) {
      query.set("limit", String(limit));
    }
    query.set("sort", sortBy);
    query.set("dir", sortDir);

    const offset = rawOffset ?? kwicOffset;
    query.set("offset", String(Math.max(0, offset)));

    try {
      const response = await fetch(
        `${apiBase}/api/documents/${documentId}/kwic?${query.toString()}`,
      );
      if (!response.ok) {
        setStatusMessage("Unable to load KWIC occurrences.");
        return;
      }

      const payload = (await response.json()) as KWICListPayload;
      setKwicRows(payload.items);
      setKWICTotal(payload.total);
      setKWICOffset(payload.offset);
      setKWICPageInput(String(Math.floor(payload.offset / payload.limit) + 1));
    } catch (error) {
      handleRequestError("Unable to load KWIC occurrences.", error);
    }
  }

  function runKWICQuery() {
    if (selectedDocument === "") {
      return;
    }

    setKWICOffset(0);
    setKWICPageInput("1");
    void refreshKWIC(
      selectedDocument,
      kwicLemma,
      kwicLimit,
      kwicSort,
      kwicDir,
      0,
    );
  }

  function goToKWICPage() {
    if (selectedDocument === "") {
      return;
    }

    const page = Number.parseInt(kwicPageInput, 10);
    if (Number.isNaN(page) || page <= 0) {
      return;
    }

    const clampedPage = Math.min(page, kwicTotalPages);
    const nextOffset = (clampedPage - 1) * kwicPageSize;
    void refreshKWIC(
      selectedDocument,
      kwicLemma,
      kwicLimit,
      kwicSort,
      kwicDir,
      nextOffset,
    );
  }

  function handleKWICQueryEnter(
    event: KeyboardEvent<HTMLInputElement | HTMLSelectElement>,
  ) {
    if (event.key !== "Enter") {
      return;
    }

    event.preventDefault();
    runKWICQuery();
  }

  function handleKWICPageEnter(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key !== "Enter") {
      return;
    }

    event.preventDefault();
    goToKWICPage();
  }

  const kwicPageSize = Math.max(1, Number.parseInt(kwicLimit, 10) || 50);
  const kwicFrom = kwicRows.length === 0 ? 0 : kwicOffset + 1;
  const kwicTo = kwicOffset + kwicRows.length;
  const canGoPrev = kwicOffset > 0;
  const canGoNext = kwicOffset + kwicRows.length < kwicTotal;
  const kwicTotalPages = Math.max(1, Math.ceil(kwicTotal / kwicPageSize));

  async function createProject() {
    try {
      const response = await fetch(`${apiBase}/api/projects`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: projectName }),
      });

      if (!response.ok) {
        setStatusMessage("Unable to create project.");
        return;
      }

      setStatusMessage("Project created.");
      await refreshProjects();
    } catch (error) {
      handleRequestError("Unable to create project.", error);
    }
  }

  async function onUpload(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    if (!file || selectedProject === "") {
      return;
    }

    const formData = new FormData();
    formData.append("file", file);

    try {
      const response = await fetch(
        `${apiBase}/api/projects/${selectedProject}/documents/upload`,
        {
          method: "POST",
          body: formData,
        },
      );

      if (!response.ok) {
        setStatusMessage("Upload failed.");
        return;
      }

      setStatusMessage("Upload accepted. Refreshing status.");
      await refreshDocuments(selectedProject);
      window.setTimeout(() => {
        void refreshDocuments(selectedProject);
      }, 800);
    } catch (error) {
      handleRequestError("Upload failed.", error);
    }
  }

  async function retrySelectedDocument() {
    if (selectedDocument === "") {
      return;
    }

    try {
      const response = await fetch(
        `${apiBase}/api/documents/${selectedDocument}/retry`,
        { method: "POST" },
      );

      if (!response.ok) {
        setStatusMessage("Retry failed.");
        return;
      }

      setStatusMessage("Retry queued. Refreshing timeline.");
      if (selectedProject !== "") {
        await refreshDocuments(selectedProject);
      }
      await refreshPipelineJobs(selectedDocument);
      window.setTimeout(() => {
        if (selectedProject !== "") {
          void refreshDocuments(selectedProject);
        }
        void refreshPipelineJobs(selectedDocument);
      }, 800);
    } catch (error) {
      handleRequestError("Retry failed.", error);
    }
  }

  return (
    <main className="page">
      <section className="hero">
        <p className="eyebrow">Concordance Desktop</p>
        <h1>Literary analysis workspace</h1>
        <p className="subtitle">
          API-connected foundation with project creation and upload pipeline
          status.
        </p>
      </section>

      <section className="panel">
        <h2>System readiness</h2>
        <p>
          Overall: <strong>{health?.status ?? "loading"}</strong>
        </p>
        <p>Database: {health?.checks?.database ?? "unknown"}</p>
        <p>NLP sidecar: {health?.checks?.nlp_sidecar ?? "unknown"}</p>
      </section>

      <section className="panel">
        <h2>Projects</h2>
        <div className="row">
          <input
            value={projectName}
            onChange={(event) => setProjectName(event.target.value)}
            placeholder="Project name"
          />
          <button type="button" onClick={createProject}>
            Create
          </button>
          <button type="button" onClick={() => void refreshProjects()}>
            Refresh
          </button>
        </div>

        <select
          value={selectedProject}
          onChange={(event) => setSelectedProject(event.target.value)}
        >
          {projects.map((project) => (
            <option key={project.id} value={project.id}>
              {project.name}
            </option>
          ))}
        </select>
      </section>

      <section className="panel">
        <h2>Document uploads</h2>
        <input
          type="file"
          onChange={(event) => void onUpload(event)}
          disabled={!selectedProjectExists}
        />
        <button
          type="button"
          disabled={!selectedProjectExists}
          onClick={() => {
            void refreshDocuments(selectedProject);
            if (selectedDocument !== "") {
              void refreshPipelineJobs(selectedDocument);
            }
          }}
        >
          Refresh status
        </button>

        <select
          value={selectedDocument}
          onChange={(event) => setSelectedDocument(event.target.value)}
          disabled={documents.length === 0}
        >
          {documents.length === 0 ? (
            <option value="">No documents yet</option>
          ) : (
            documents.map((document) => (
              <option key={document.id} value={document.id}>
                {document.fileName}
              </option>
            ))
          )}
        </select>

        <ul>
          {documents.map((document) => (
            <li key={document.id}>
              {document.fileName} - {document.status} (
              {Math.round(document.progress * 100)}%)
            </li>
          ))}
        </ul>
      </section>

      <section className="panel">
        <h2>Pipeline timeline</h2>
        <p
          className={`stream-state ${streamState} ${isStreamStale ? "stale" : ""}`}
        >
          Stream: <strong>{streamStatusLabel}</strong>
          <span className="stream-last-event">
            Last event: {lastEventLabel}
          </span>
        </p>
        <button
          type="button"
          disabled={selectedDocument === ""}
          onClick={() => void retrySelectedDocument()}
        >
          Retry ingestion
        </button>
        {pipelineJobs.length === 0 ? (
          <p>No pipeline jobs recorded for the selected document yet.</p>
        ) : (
          <ul className="timeline">
            {pipelineJobs.map((job) => (
              <li
                key={job.id}
                className={
                  job.status === "failed"
                    ? "timeline-item failed"
                    : "timeline-item"
                }
              >
                <strong>{job.stage}</strong> - {job.status} - {job.message}
              </li>
            ))}
          </ul>
        )}
      </section>

      <section className="panel">
        <h2>Concordance</h2>
        <div className="row">
          <input
            value={concordanceLemma}
            onChange={(event) => setConcordanceLemma(event.target.value)}
            placeholder="Filter by lemma"
            disabled={selectedDocument === ""}
          />
          <input
            value={concordancePOS}
            onChange={(event) => setConcordancePOS(event.target.value)}
            placeholder="Filter by POS"
            disabled={selectedDocument === ""}
          />
          <button
            type="button"
            disabled={selectedDocument === ""}
            onClick={() => {
              if (selectedDocument !== "") {
                void refreshConcordance(
                  selectedDocument,
                  concordanceLemma,
                  concordancePOS,
                );
              }
            }}
          >
            Apply filters
          </button>
        </div>
        {concordanceTerms.length === 0 ? (
          <p>No concordance terms yet.</p>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Lemma</th>
                  <th>Normalized</th>
                  <th>Freq</th>
                  <th>Hapax</th>
                </tr>
              </thead>
              <tbody>
                {concordanceTerms.map((term) => (
                  <tr key={term.id}>
                    <td>{term.lemma}</td>
                    <td>{term.normalizedForm}</td>
                    <td>{term.totalFreq}</td>
                    <td>{term.hapax ? "Yes" : "No"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <section className="panel">
        <h2>KWIC explorer</h2>
        <div className="row">
          <input
            value={kwicLemma}
            onChange={(event) => setKWICLemma(event.target.value)}
            onKeyDown={handleKWICQueryEnter}
            placeholder="Filter by lemma"
            disabled={selectedDocument === ""}
          />
          <input
            value={kwicLimit}
            onChange={(event) => setKWICLimit(event.target.value)}
            onKeyDown={handleKWICQueryEnter}
            placeholder="Limit"
            inputMode="numeric"
            disabled={selectedDocument === ""}
          />
          <select
            value={kwicSort}
            onChange={(event) => setKWICSort(event.target.value)}
            onKeyDown={handleKWICQueryEnter}
            disabled={selectedDocument === ""}
          >
            <option value="position">Sort: Position</option>
            <option value="lemma">Sort: Lemma</option>
            <option value="keyword">Sort: Keyword</option>
          </select>
          <select
            value={kwicDir}
            onChange={(event) => setKWICDir(event.target.value)}
            onKeyDown={handleKWICQueryEnter}
            disabled={selectedDocument === ""}
          >
            <option value="asc">Asc</option>
            <option value="desc">Desc</option>
          </select>
          <button
            type="button"
            disabled={selectedDocument === ""}
            onClick={runKWICQuery}
          >
            Run query
          </button>
          <button
            type="button"
            disabled={selectedDocument === "" || !canGoPrev}
            onClick={() => {
              if (selectedDocument !== "") {
                const nextOffset = Math.max(0, kwicOffset - kwicPageSize);
                void refreshKWIC(
                  selectedDocument,
                  kwicLemma,
                  kwicLimit,
                  kwicSort,
                  kwicDir,
                  nextOffset,
                );
              }
            }}
          >
            Previous
          </button>
          <button
            type="button"
            disabled={selectedDocument === "" || !canGoNext}
            onClick={() => {
              if (selectedDocument !== "") {
                const nextOffset = kwicOffset + kwicPageSize;
                void refreshKWIC(
                  selectedDocument,
                  kwicLemma,
                  kwicLimit,
                  kwicSort,
                  kwicDir,
                  nextOffset,
                );
              }
            }}
          >
            Next
          </button>
          <input
            value={kwicPageInput}
            onChange={(event) => setKWICPageInput(event.target.value)}
            onKeyDown={handleKWICPageEnter}
            placeholder="Page"
            inputMode="numeric"
            disabled={selectedDocument === ""}
          />
          <button
            type="button"
            disabled={selectedDocument === ""}
            onClick={goToKWICPage}
          >
            Go to page
          </button>
        </div>
        <p className="kwic-meta">
          Showing {kwicFrom}-{kwicTo} of {kwicTotal} (page{" "}
          {Math.floor(kwicOffset / kwicPageSize) + 1} / {kwicTotalPages})
        </p>
        {kwicRows.length === 0 ? (
          <p>No KWIC rows yet.</p>
        ) : (
          <ul className="kwic-list">
            {kwicRows.map((row) => (
              <li key={row.id}>
                <span className="kwic-left">{row.leftContext}</span>
                <strong className="kwic-keyword">{row.keyword}</strong>
                <span className="kwic-right">{row.rightContext}</span>
              </li>
            ))}
          </ul>
        )}
      </section>

      {statusMessage ? (
        <section className="panel">{statusMessage}</section>
      ) : null}
    </main>
  );
}
