"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Card } from "./components/card";
import { CompareCard } from "./components/compare-card";
import { ConfirmDialog, TextDialog } from "./components/dialogs";
import { GlossaryPanel } from "./components/glossary-panel";
import { KwicCard } from "./components/kwic-card";
import { LibraryCard } from "./components/library-card";
import { Sidebar } from "./components/sidebar";
import { StatStrip } from "./components/stat-strip";
import { StepsCard } from "./components/steps-card";
import { Term } from "./components/term";
import { ToastProvider, useToasts } from "./components/toasts";
import { TopWordsCard } from "./components/top-words-card";
import { UploadCard } from "./components/upload-card";
import { WordListCard } from "./components/word-list-card";
import { useAsync } from "./hooks/use-async";
import { useJobStream } from "./hooks/use-job-stream";
import { api, apiBase } from "./lib/api";
import { toWebSocketBase } from "./lib/format";
import { EXAMPLE_PROJECT_NAMES } from "./lib/glossary";
import type { Document, Health, Project } from "./lib/types";

type Dialogs =
  | { kind: "none" }
  | { kind: "create-project" }
  | { kind: "rename-project"; project: Project }
  | { kind: "delete-project"; project: Project }
  | { kind: "rename-doc"; doc: Document }
  | { kind: "delete-doc"; doc: Document }
  | { kind: "glossary" };

const IN_FLIGHT = new Set(["queued", "processing", "running"]);

function message(error: unknown): string {
  return error instanceof Error ? error.message : "Something went wrong.";
}

function Dashboard() {
  const toast = useToasts();
  const wsBase = useMemo(() => toWebSocketBase(apiBase), []);
  const [dialog, setDialog] = useState<Dialogs>({ kind: "none" });
  const [projectId, setProjectId] = useState("");
  const [docId, setDocId] = useState("");
  const [word, setWord] = useState("");
  const contextRef = useRef<HTMLDivElement>(null);
  // A project we just created and selected, before the list has reloaded to include it.
  const pendingProject = useRef("");

  // --- health ---
  const [health, setHealth] = useState<Health | null>(null);
  useEffect(() => {
    let cancelled = false;
    const check = () =>
      api
        .health()
        .then((h) => !cancelled && setHealth(h))
        .catch(() => !cancelled && setHealth(null));
    void check();
    const id = window.setInterval(check, 15000);
    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
  }, []);

  // --- projects ---
  const projects = useAsync((signal) => api.listProjects(signal), []);
  const projectList = projects.data ?? [];
  useEffect(() => {
    if (!projects.data) return;
    if (projects.data.some((p) => p.id === projectId)) {
      if (pendingProject.current === projectId) pendingProject.current = "";
      return;
    }
    if (projectId !== "" && pendingProject.current === projectId) return;
    setProjectId(projects.data[0]?.id ?? "");
  }, [projects.data, projectId]);
  const project = projectList.find((p) => p.id === projectId);

  // --- documents (polled while anything is still processing) ---
  const documents = useAsync(
    (signal) => api.listDocuments(projectId, signal),
    [projectId],
    projectId !== "",
  );
  const docList = documents.data ?? [];
  const reloadDocuments = documents.reload;
  const anyInFlight = docList.some((d) => IN_FLIGHT.has(d.status));
  useEffect(() => {
    if (!anyInFlight) return;
    const id = window.setInterval(reloadDocuments, 2000);
    return () => window.clearInterval(id);
  }, [anyInFlight, reloadDocuments]);

  useEffect(() => {
    if (!documents.data) return;
    if (!documents.data.some((d) => d.id === docId)) {
      setDocId(documents.data[0]?.id ?? "");
    }
  }, [documents.data, docId]);
  const doc = docList.find((d) => d.id === docId);
  const ready = doc?.status === "ready";
  const readyDocs = docList.filter((d) => d.status === "ready");

  // --- per-document data ---
  const stats = useAsync(
    (signal) => api.documentStats(docId, signal),
    [docId, doc?.status],
    ready,
  );
  const jobs = useAsync(
    (signal) => api.pipelineJobs(docId, signal),
    [docId, doc?.status, Math.round((doc?.progress ?? 0) * 20)],
    doc !== undefined,
  );
  const reloadJobs = jobs.reload;

  const { streamState } = useJobStream({
    documentId: docId,
    wsBase,
    onDocumentStatusUpdate: reloadDocuments,
    onJobsSnapshot: () => {},
    onStageChange: useCallback(() => {
      reloadJobs();
      reloadDocuments();
    }, [reloadJobs, reloadDocuments]),
    onDocumentReady: reloadDocuments,
  });

  // --- actions ---
  const close = () => setDialog({ kind: "none" });

  async function run<T>(action: () => Promise<T>, success?: string) {
    try {
      const result = await action();
      if (success) toast.ok(success);
      return result;
    } catch (error) {
      toast.error(message(error));
      return undefined;
    }
  }

  async function createProject(name: string) {
    const created = await run(
      () => api.createProject(name),
      `Created “${name}”.`,
    );
    if (created) {
      close();
      pendingProject.current = created.id;
      setProjectId(created.id);
      setDocId("");
      setWord("");
      projects.reload();
    }
  }

  async function renameProject(target: Project, name: string) {
    const updated = await run(
      () => api.updateProject(target.id, name, target.description ?? ""),
      "Project renamed.",
    );
    if (updated) {
      close();
      projects.reload();
    }
  }

  async function deleteProject(target: Project) {
    const done = await run(async () => {
      await api.deleteProject(target.id);
      return true;
    }, `Deleted “${target.name}”.`);
    if (done) {
      close();
      if (target.id === projectId) {
        setProjectId("");
        setDocId("");
      }
      projects.reload();
    }
  }

  async function upload(files: File[]) {
    let uploaded = 0;
    for (const file of files) {
      const created = await run(() => api.uploadDocument(projectId, file));
      if (created) {
        uploaded += 1;
        setDocId((current) => current || created.id);
      }
    }
    if (uploaded > 0) {
      toast.ok(
        uploaded === 1
          ? "File added. Processing has started."
          : `${uploaded} files added. Processing has started.`,
      );
      reloadDocuments();
    }
  }

  async function renameDoc(target: Document, fileName: string) {
    const updated = await run(
      () => api.renameDocument(target.id, fileName),
      "File renamed.",
    );
    if (updated) {
      close();
      reloadDocuments();
    }
  }

  async function deleteDoc(target: Document) {
    const done = await run(async () => {
      await api.deleteDocument(target.id);
      return true;
    }, `Deleted “${target.fileName}”.`);
    if (done) {
      close();
      if (target.id === docId) setDocId("");
      reloadDocuments();
    }
  }

  async function retry(target: Document) {
    const queued = await run(
      () => api.retryDocument(target.id),
      "Processing again.",
    );
    if (queued) {
      setDocId(target.id);
      reloadDocuments();
    }
  }

  function showInContext(lemma: string) {
    setWord(lemma);
    contextRef.current?.scrollIntoView({ behavior: "smooth", block: "start" });
  }

  const noProjects = !projects.loading && projectList.length === 0;

  return (
    <div className="shell">
      <Sidebar
        projects={projectList}
        selectedId={projectId}
        health={health}
        loading={projects.loading}
        onSelect={(id) => {
          setProjectId(id);
          setDocId("");
          setWord("");
        }}
        onCreate={() => setDialog({ kind: "create-project" })}
        onRename={(p) => setDialog({ kind: "rename-project", project: p })}
        onDelete={(p) => setDialog({ kind: "delete-project", project: p })}
        onOpenGlossary={() => setDialog({ kind: "glossary" })}
      />

      <main className="main">
        {projects.error && (
          <div className="callout callout-bad" role="alert">
            <strong>Can't load projects</strong>
            <p>{projects.error}</p>
            <button
              type="button"
              className="btn btn-quiet btn-sm"
              onClick={projects.reload}
            >
              Try again
            </button>
          </div>
        )}

        {noProjects && !projects.error && (
          <section className="welcome">
            <h1>See how words are used.</h1>
            <p>
              Add a textbook, paper or report and Concordance indexes every
              word, so you can search it and read each use in context.
            </p>
            <button
              type="button"
              className="btn btn-primary btn-lg"
              onClick={() => setDialog({ kind: "create-project" })}
            >
              Create your first project
            </button>
            <p className="examples">
              Ideas:{" "}
              {EXAMPLE_PROJECT_NAMES.map((n) => (
                <span key={n} className="chip chip-static">
                  {n}
                </span>
              ))}
            </p>
          </section>
        )}

        {project && (
          <>
            <header className="page-head">
              <div>
                <h1>{project.name}</h1>
                <p className="page-sub">
                  {docList.length === 0
                    ? "No files yet."
                    : `${docList.length} ${docList.length === 1 ? "file" : "files"}`}
                  {doc && (
                    <>
                      {" "}
                      · exploring <strong>{doc.fileName}</strong>
                    </>
                  )}
                </p>
              </div>
            </header>

            <div className="grid">
              <Card
                className="span-8"
                title="Files"
                hint="Select a file to explore it. New files appear here while they process."
              >
                <LibraryCard
                  documents={docList}
                  selectedId={docId}
                  loading={documents.loading}
                  onSelect={setDocId}
                  onRename={(d) => setDialog({ kind: "rename-doc", doc: d })}
                  onRetry={retry}
                  onDelete={(d) => setDialog({ kind: "delete-doc", doc: d })}
                />
              </Card>
              <Card className="span-4" title="Add files">
                <UploadCard disabled={projectId === ""} onFiles={upload} />
              </Card>

              {doc && (
                <>
                  {ready ? (
                    <div className="span-12">
                      <StatStrip stats={stats.data} />
                    </div>
                  ) : null}

                  <Card
                    className={ready ? "span-4" : "span-12"}
                    title={ready ? "Processing" : `Processing ${doc.fileName}`}
                    hint={
                      <>
                        Each file goes through a few{" "}
                        <Term k="pipeline">steps</Term> first. Large books take
                        a minute or two.
                      </>
                    }
                  >
                    <StepsCard
                      doc={doc}
                      jobs={jobs.data ?? []}
                      stream={streamState}
                    />
                  </Card>

                  {ready && (
                    <>
                      <Card
                        className="span-8"
                        title="Most used words"
                        hint="The words that carry the topic of this file."
                      >
                        <TopWordsCard
                          documentId={doc.id}
                          ready={ready}
                          onPick={showInContext}
                        />
                      </Card>

                      <Card
                        className="span-5"
                        title={<Term k="concordance">Word list</Term>}
                        hint="Every word in the file and how often it appears."
                      >
                        <WordListCard
                          documentId={doc.id}
                          ready={ready}
                          selectedWord={word}
                          onPick={showInContext}
                        />
                      </Card>

                      <div className="span-7 anchor" ref={contextRef}>
                        <Card
                          className="card-fill"
                          title="In context"
                          hint="Read each use of a word with the text around it."
                        >
                          <KwicCard
                            projectId={projectId}
                            fileCount={readyDocs.length}
                            documentId={doc.id}
                            ready={ready}
                            word={word}
                            onWordChange={setWord}
                          />
                        </Card>
                      </div>

                      <Card
                        className="span-12"
                        title={
                          <Term k="keyness">Compare with another file</Term>
                        }
                        hint="Find the words that set this file apart from another one in the project."
                      >
                        <CompareCard
                          doc={doc}
                          others={readyDocs.filter((d) => d.id !== doc.id)}
                          onPick={showInContext}
                        />
                      </Card>
                    </>
                  )}
                </>
              )}
            </div>
          </>
        )}
      </main>

      <TextDialog
        open={dialog.kind === "create-project"}
        title="New project"
        label="Project name"
        placeholder="e.g. Linguistics 101"
        examples={EXAMPLE_PROJECT_NAMES}
        submitLabel="Create project"
        onSubmit={createProject}
        onClose={close}
      />
      <TextDialog
        open={dialog.kind === "rename-project"}
        title="Rename project"
        label="Project name"
        initial={dialog.kind === "rename-project" ? dialog.project.name : ""}
        submitLabel="Save name"
        onSubmit={(name) =>
          dialog.kind === "rename-project"
            ? renameProject(dialog.project, name)
            : undefined
        }
        onClose={close}
      />
      <TextDialog
        open={dialog.kind === "rename-doc"}
        title="Rename file"
        label="File name"
        initial={dialog.kind === "rename-doc" ? dialog.doc.fileName : ""}
        submitLabel="Save name"
        onSubmit={(name) =>
          dialog.kind === "rename-doc" ? renameDoc(dialog.doc, name) : undefined
        }
        onClose={close}
      />
      <ConfirmDialog
        open={dialog.kind === "delete-project"}
        title="Delete this project?"
        body={
          <p>
            “{dialog.kind === "delete-project" ? dialog.project.name : ""}” and
            all of its files and analysis will be removed. This can't be undone.
          </p>
        }
        confirmLabel="Delete project"
        onConfirm={() =>
          dialog.kind === "delete-project"
            ? deleteProject(dialog.project)
            : undefined
        }
        onClose={close}
      />
      <ConfirmDialog
        open={dialog.kind === "delete-doc"}
        title="Delete this file?"
        body={
          <p>
            “{dialog.kind === "delete-doc" ? dialog.doc.fileName : ""}” and its
            analysis will be removed. This can't be undone.
          </p>
        }
        confirmLabel="Delete file"
        onConfirm={() =>
          dialog.kind === "delete-doc" ? deleteDoc(dialog.doc) : undefined
        }
        onClose={close}
      />
      <GlossaryPanel open={dialog.kind === "glossary"} onClose={close} />
    </div>
  );
}

export default function HomePage() {
  return (
    <ToastProvider>
      <Dashboard />
    </ToastProvider>
  );
}
