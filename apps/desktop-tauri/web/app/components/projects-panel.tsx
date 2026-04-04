"use client";

import type { Project } from "../lib/types";

interface ProjectsPanelProps {
  projects: Project[];
  selectedProject: string;
  projectName: string;
  onProjectNameChange: (name: string) => void;
  onSelectProject: (id: string) => void;
  onCreateProject: () => void;
  onRefresh: () => void;
}

export function ProjectsPanel({
  projects,
  selectedProject,
  projectName,
  onProjectNameChange,
  onSelectProject,
  onCreateProject,
  onRefresh,
}: ProjectsPanelProps) {
  return (
    <section className="panel">
      <h2>Projects</h2>
      <div className="row">
        <input
          value={projectName}
          onChange={(event) => onProjectNameChange(event.target.value)}
          placeholder="Project name"
        />
        <button type="button" onClick={onCreateProject}>
          Create
        </button>
        <button type="button" onClick={onRefresh}>
          Refresh
        </button>
      </div>

      <select
        value={selectedProject}
        onChange={(event) => onSelectProject(event.target.value)}
      >
        {projects.map((project) => (
          <option key={project.id} value={project.id}>
            {project.name}
          </option>
        ))}
      </select>
    </section>
  );
}
