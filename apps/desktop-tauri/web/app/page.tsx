const checks = [
  "Go API health endpoint",
  "Project listing endpoint",
  "Upload pipeline status endpoint",
  "Python sidecar health endpoint",
];

export default function HomePage() {
  return (
    <main className="page">
      <section className="hero">
        <p className="eyebrow">Concordance Desktop</p>
        <h1>Literary analysis workspace</h1>
        <p className="subtitle">
          Foundation build with health checks and pipeline skeleton ready for
          iterative delivery.
        </p>
      </section>

      <section className="panel">
        <h2>Current implementation slice</h2>
        <ul>
          {checks.map((item) => (
            <li key={item}>{item}</li>
          ))}
        </ul>
      </section>
    </main>
  );
}
