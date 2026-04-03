"use client";

import { useEffect, useMemo, useState } from "react";
import type { ChangeEvent } from "react";

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
    } else {
      setPipelineJobs([]);
    }
  }, [selectedDocument]);

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
  }, [selectedDocument, selectedProject, wsBase]);

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

      {statusMessage ? (
        <section className="panel">{statusMessage}</section>
      ) : null}
    </main>
  );
}
