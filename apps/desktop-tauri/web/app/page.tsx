"use client";

import { useEffect, useMemo, useState } from "react";
import type { ChangeEvent } from "react";
import { ConcordancePanel } from "./components/concordance-panel";
import { DocumentsPanel } from "./components/documents-panel";
import { HealthPanel } from "./components/health-panel";
import { HeroSection } from "./components/hero-section";
import { KWICPanel } from "./components/kwic-panel";
import { PipelinePanel } from "./components/pipeline-panel";
import { ProjectsPanel } from "./components/projects-panel";
import { StatusBar } from "./components/status-bar";
import { useJobStream } from "./hooks/use-job-stream";
import {
  apiBase,
  createProjectRequest,
  fetchConcordance,
  fetchDocuments,
  fetchHealth,
  fetchKWIC,
  fetchPipelineJobs,
  fetchProjects,
  retryDocumentRequest,
  uploadDocumentRequest,
} from "./lib/api";
import { toWebSocketBase } from "./lib/format";
import type {
  ConcordanceTerm,
  Document,
  Health,
  KWICOccurrence,
  PipelineJob,
  Project,
} from "./lib/types";

export default function HomePage() {
  const [health, setHealth] = useState<Health | null>(null);
  const [projects, setProjects] = useState<Project[]>([]);
  const [projectName, setProjectName] = useState("Sample Project");
  const [selectedProject, setSelectedProject] = useState("");
  const [documents, setDocuments] = useState<Document[]>([]);
  const [selectedDocument, setSelectedDocument] = useState("");
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
  const [statusMessage, setStatusMessage] = useState("");

  const wsBase = useMemo(() => toWebSocketBase(apiBase), []);
  const selectedProjectExists = useMemo(
    () => projects.some((p) => p.id === selectedProject),
    [projects, selectedProject],
  );

  function onError(context: string, error: unknown) {
    const detail = error instanceof Error ? error.message : "Unknown error";
    setStatusMessage(
      `${context} Unable to reach API at ${apiBase} (${detail}).`,
    );
  }

  // --- data refresh helpers ---

  async function refreshProjects() {
    const items = await fetchProjects(onError);
    if (items) {
      setProjects(items);
      if (items.length > 0) {
        setSelectedProject((cur) => cur || items[0].id);
      }
    }
  }

  async function refreshDocuments(projectId: string) {
    const items = await fetchDocuments(projectId, onError);
    if (!items) return;
    setDocuments(items);
    if (items.length === 0) {
      setSelectedDocument("");
      return;
    }
    setSelectedDocument((cur) => {
      const exists = items.some((d) => d.id === cur);
      return exists ? cur : items[0].id;
    });
  }

  async function refreshPipelineJobs(docId: string) {
    const items = await fetchPipelineJobs(docId, onError);
    if (items) setPipelineJobs(items);
  }

  async function refreshConcordanceNow() {
    if (selectedDocument === "") return;
    const items = await fetchConcordance(
      selectedDocument,
      concordanceLemma,
      concordancePOS,
      onError,
    );
    if (items) setConcordanceTerms(items);
  }

  async function refreshKWICNow(offset?: number) {
    if (selectedDocument === "") return;
    const payload = await fetchKWIC(
      selectedDocument,
      {
        lemma: kwicLemma,
        rawLimit: kwicLimit,
        sortBy: kwicSort,
        sortDir: kwicDir,
        offset: offset ?? kwicOffset,
      },
      onError,
    );
    if (payload) {
      setKwicRows(payload.items);
      setKWICTotal(payload.total);
      setKWICOffset(payload.offset);
    }
  }

  // --- effects ---

  // biome-ignore lint/correctness/useExhaustiveDependencies: mount-only effect
  useEffect(() => {
    void fetchHealth(onError).then((h) => h && setHealth(h));
    void refreshProjects();
  }, []);

  // biome-ignore lint/correctness/useExhaustiveDependencies: refreshDocuments is stable in intent
  useEffect(() => {
    if (selectedProject !== "") {
      void refreshDocuments(selectedProject);
    }
  }, [selectedProject]);

  // biome-ignore lint/correctness/useExhaustiveDependencies: refresh helpers read current state via closure
  useEffect(() => {
    if (selectedDocument !== "") {
      void refreshPipelineJobs(selectedDocument);
      void refreshConcordanceNow();
      void refreshKWICNow();
    } else {
      setPipelineJobs([]);
      setConcordanceTerms([]);
      setKwicRows([]);
      setKWICTotal(0);
      setKWICOffset(0);
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

  // --- WebSocket stream ---

  const { streamState, streamStatusLabel, lastEventLabel, isStreamStale } =
    useJobStream({
      documentId: selectedDocument,
      wsBase,
      onDocumentStatusUpdate(status, progress) {
        setDocuments((cur) =>
          cur.map((d) =>
            d.id === selectedDocument ? { ...d, status, progress } : d,
          ),
        );
      },
      onJobsSnapshot(jobs) {
        setPipelineJobs(jobs);
      },
      onStageChange() {
        void refreshPipelineJobs(selectedDocument);
        if (selectedProject !== "") {
          void refreshDocuments(selectedProject);
        }
        void refreshConcordanceNow();
        void refreshKWICNow();
      },
      onDocumentReady() {
        void refreshConcordanceNow();
        void refreshKWICNow();
      },
    });

  // --- actions ---

  async function handleCreateProject() {
    const ok = await createProjectRequest(projectName, onError);
    if (ok) {
      setStatusMessage("Project created.");
      await refreshProjects();
    }
  }

  async function handleUpload(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    if (!file || selectedProject === "") return;
    const ok = await uploadDocumentRequest(selectedProject, file, onError);
    if (ok) {
      setStatusMessage("Upload accepted. Refreshing status.");
      await refreshDocuments(selectedProject);
      window.setTimeout(() => {
        void refreshDocuments(selectedProject);
      }, 800);
    }
  }

  async function handleRetry() {
    if (selectedDocument === "") return;
    const ok = await retryDocumentRequest(selectedDocument, onError);
    if (ok) {
      setStatusMessage("Retry queued. Refreshing timeline.");
      if (selectedProject !== "") await refreshDocuments(selectedProject);
      await refreshPipelineJobs(selectedDocument);
      window.setTimeout(() => {
        if (selectedProject !== "") void refreshDocuments(selectedProject);
        void refreshPipelineJobs(selectedDocument);
      }, 800);
    }
  }

  // --- render ---

  return (
    <main className="page">
      <HeroSection />
      <HealthPanel health={health} />
      <ProjectsPanel
        projects={projects}
        selectedProject={selectedProject}
        projectName={projectName}
        onProjectNameChange={setProjectName}
        onSelectProject={setSelectedProject}
        onCreateProject={() => void handleCreateProject()}
        onRefresh={() => void refreshProjects()}
      />
      <DocumentsPanel
        documents={documents}
        selectedDocument={selectedDocument}
        selectedProjectExists={selectedProjectExists}
        onSelectDocument={setSelectedDocument}
        onUpload={(e) => void handleUpload(e)}
        onRefresh={() => {
          void refreshDocuments(selectedProject);
          if (selectedDocument !== "")
            void refreshPipelineJobs(selectedDocument);
        }}
      />
      <PipelinePanel
        pipelineJobs={pipelineJobs}
        selectedDocument={selectedDocument}
        streamStatusLabel={streamStatusLabel}
        lastEventLabel={lastEventLabel}
        isStreamStale={isStreamStale}
        streamState={streamState}
        onRetry={() => void handleRetry()}
      />
      <ConcordancePanel
        concordanceTerms={concordanceTerms}
        concordanceLemma={concordanceLemma}
        concordancePOS={concordancePOS}
        selectedDocument={selectedDocument}
        onLemmaChange={setConcordanceLemma}
        onPOSChange={setConcordancePOS}
        onApplyFilters={() => void refreshConcordanceNow()}
      />
      <KWICPanel
        selectedDocument={selectedDocument}
        kwicRows={kwicRows}
        kwicTotal={kwicTotal}
        kwicOffset={kwicOffset}
        kwicLemma={kwicLemma}
        kwicLimit={kwicLimit}
        kwicSort={kwicSort}
        kwicDir={kwicDir}
        onLemmaChange={setKWICLemma}
        onLimitChange={setKWICLimit}
        onSortChange={setKWICSort}
        onDirChange={setKWICDir}
        onQuery={(offset) => void refreshKWICNow(offset)}
      />
      <StatusBar message={statusMessage} />
    </main>
  );
}
