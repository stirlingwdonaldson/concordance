"use client";

import type { Health, Project } from "../lib/types";

export function Sidebar({
  projects,
  selectedId,
  health,
  loading,
  onSelect,
  onCreate,
  onRename,
  onDelete,
  onOpenGlossary,
}: Readonly<{
  projects: Project[];
  selectedId: string;
  health: Health | null;
  loading: boolean;
  onSelect: (id: string) => void;
  onCreate: () => void;
  onRename: (project: Project) => void;
  onDelete: (project: Project) => void;
  onOpenGlossary: () => void;
}>) {
  const ready = health?.status === "ready";

  return (
    <aside className="sidebar">
      <div className="brand">
        <span className="brand-mark" aria-hidden="true">
          <i />
          <b />
          <i />
        </span>
        <span className="brand-name">Concordance</span>
      </div>

      <div className="sidebar-head">
        <h2>Projects</h2>
        <button
          type="button"
          className="btn btn-quiet btn-sm"
          onClick={onCreate}
        >
          New project
        </button>
      </div>

      <nav aria-label="Projects" className="project-list">
        {loading && projects.length === 0 && (
          <div className="skeleton" style={{ height: 120 }} />
        )}
        {!loading && projects.length === 0 && (
          <p className="sidebar-empty">
            Projects group related files, like one course or one report.
          </p>
        )}
        <ul>
          {projects.map((project) => (
            <li
              key={project.id}
              className={project.id === selectedId ? "is-selected" : undefined}
            >
              <button
                type="button"
                className="project-link"
                aria-current={project.id === selectedId ? "true" : undefined}
                onClick={() => onSelect(project.id)}
              >
                {project.name}
              </button>
              <span className="project-actions">
                <button
                  type="button"
                  className="icon-btn"
                  aria-label={`Rename ${project.name}`}
                  title="Rename"
                  onClick={() => onRename(project)}
                >
                  <svg viewBox="0 0 16 16" aria-hidden="true">
                    <path d="M11.2 2.3a1.6 1.6 0 0 1 2.3 2.3l-7.6 7.6-3 .7.7-3 7.6-7.6Z" />
                  </svg>
                </button>
                <button
                  type="button"
                  className="icon-btn icon-btn-danger"
                  aria-label={`Delete ${project.name}`}
                  title="Delete"
                  onClick={() => onDelete(project)}
                >
                  <svg viewBox="0 0 16 16" aria-hidden="true">
                    <path d="M3 4.5h10M6.5 4.5V3h3v1.5M5 4.5l.5 8h5l.5-8" />
                  </svg>
                </button>
              </span>
            </li>
          ))}
        </ul>
      </nav>

      <div className="sidebar-foot">
        <button
          type="button"
          className="btn btn-quiet btn-block"
          onClick={onOpenGlossary}
        >
          Glossary
        </button>
        <p
          className={`health ${ready ? "is-ok" : "is-bad"}`}
          title={
            health
              ? `Database: ${health.checks?.database ?? "unknown"} · Language engine: ${health.checks?.nlp_sidecar ?? "unknown"}`
              : "The API isn't answering."
          }
        >
          <span className="dot" aria-hidden="true" />
          {health === null
            ? "API offline"
            : ready
              ? "All systems ready"
              : "Starting up"}
        </p>
      </div>
    </aside>
  );
}
