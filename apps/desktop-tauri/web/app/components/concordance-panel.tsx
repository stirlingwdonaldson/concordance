import type { ConcordanceTerm } from "../lib/types";

interface ConcordancePanelProps {
  concordanceTerms: ConcordanceTerm[];
  concordanceLemma: string;
  concordancePOS: string;
  selectedDocument: string;
  onLemmaChange: (lemma: string) => void;
  onPOSChange: (pos: string) => void;
  onApplyFilters: () => void;
}

export function ConcordancePanel({
  concordanceTerms,
  concordanceLemma,
  concordancePOS,
  selectedDocument,
  onLemmaChange,
  onPOSChange,
  onApplyFilters,
}: ConcordancePanelProps) {
  return (
    <section className="panel">
      <h2>Concordance</h2>
      <div className="row">
        <input
          value={concordanceLemma}
          onChange={(event) => onLemmaChange(event.target.value)}
          placeholder="Filter by lemma"
          disabled={selectedDocument === ""}
        />
        <input
          value={concordancePOS}
          onChange={(event) => onPOSChange(event.target.value)}
          placeholder="Filter by POS"
          disabled={selectedDocument === ""}
        />
        <button
          type="button"
          disabled={selectedDocument === ""}
          onClick={onApplyFilters}
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
  );
}
