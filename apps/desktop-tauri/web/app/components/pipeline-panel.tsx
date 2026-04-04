import type { PipelineJob } from "../lib/types";

interface PipelinePanelProps {
  pipelineJobs: PipelineJob[];
  selectedDocument: string;
  streamStatusLabel: string;
  lastEventLabel: string;
  isStreamStale: boolean;
  streamState: string;
  onRetry: () => void;
}

export function PipelinePanel({
  pipelineJobs,
  selectedDocument,
  streamStatusLabel,
  lastEventLabel,
  isStreamStale,
  streamState,
  onRetry,
}: PipelinePanelProps) {
  return (
    <section className="panel">
      <h2>Pipeline timeline</h2>
      <p
        className={`stream-state ${streamState} ${isStreamStale ? "stale" : ""}`}
      >
        Stream: <strong>{streamStatusLabel}</strong>
        <span className="stream-last-event">Last event: {lastEventLabel}</span>
      </p>
      <button
        type="button"
        disabled={selectedDocument === ""}
        onClick={onRetry}
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
  );
}
