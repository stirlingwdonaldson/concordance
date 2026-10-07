"use client";

import { STAGE_ORDER, formatPercent, stageLabel } from "../lib/format";
import type { Document, PipelineJob } from "../lib/types";
import type { StreamConnectionState } from "../lib/types";

function stepState(stage: string, jobs: PipelineJob[], doc: Document) {
  const job = [...jobs].reverse().find((j) => j.stage === stage);
  if (job) return { status: job.status, message: job.message };
  if (doc.status === "ready") return { status: "completed", message: "" };
  return { status: "pending", message: "" };
}

const STATE_LABEL: Record<string, string> = {
  completed: "Done",
  succeeded: "Done",
  running: "Working",
  started: "Working",
  failed: "Failed",
  pending: "Waiting",
};

export function StepsCard({
  doc,
  jobs,
  stream,
}: Readonly<{
  doc: Document;
  jobs: PipelineJob[];
  stream: StreamConnectionState;
}>) {
  return (
    <>
      <progress
        className="progress"
        max={100}
        value={Math.round(doc.progress * 100)}
        aria-label="Overall progress"
      />
      <p className="progress-text">
        {doc.status === "ready"
          ? "Finished. You can explore the results below."
          : doc.status === "failed"
            ? "Stopped before finishing."
            : `${formatPercent(doc.progress, 0)} complete${stream === "connected" ? " · live" : ""}`}
      </p>

      {doc.status === "failed" && doc.error && (
        <div className="callout callout-bad" role="alert">
          <strong>What went wrong</strong>
          <p>{doc.error}</p>
        </div>
      )}

      <ol className="steps">
        {STAGE_ORDER.map((stage) => {
          const state = stepState(stage, jobs, doc);
          return (
            <li key={stage} className={`step step-${state.status}`}>
              <span className="step-dot" aria-hidden="true" />
              <span className="step-name">{stageLabel(stage)}</span>
              <span className="step-state">
                {STATE_LABEL[state.status] ?? state.status}
              </span>
            </li>
          );
        })}
      </ol>
    </>
  );
}
