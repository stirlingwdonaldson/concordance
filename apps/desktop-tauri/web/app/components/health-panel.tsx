import type { Health } from "../lib/types";

export function HealthPanel({ health }: { health: Health | null }) {
  return (
    <section className="panel">
      <h2>System readiness</h2>
      <p>
        Overall: <strong>{health?.status ?? "loading"}</strong>
      </p>
      <p>Database: {health?.checks?.database ?? "unknown"}</p>
      <p>NLP sidecar: {health?.checks?.nlp_sidecar ?? "unknown"}</p>
    </section>
  );
}
