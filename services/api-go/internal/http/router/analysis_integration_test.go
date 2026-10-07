package router

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func getJSON(t *testing.T, target string, into any) int {
	t.Helper()

	resp, err := http.Get(target)
	if err != nil {
		t.Fatalf("GET %s: %v", target, err)
	}
	defer resp.Body.Close()
	if into != nil {
		if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
			t.Fatalf("decode %s: %v", target, err)
		}
	}

	return resp.StatusCode
}

func getText(t *testing.T, target string) (string, http.Header) {
	t.Helper()

	resp, err := http.Get(target)
	if err != nil {
		t.Fatalf("GET %s: %v", target, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d: %s", target, resp.StatusCode, body)
	}

	return string(body), resp.Header
}

func uploadReady(t *testing.T, base, projectID, name, text string) string {
	t.Helper()

	doc, code := uploadFile(t, base, projectID, name, []byte(text))
	if code != http.StatusAccepted {
		t.Fatalf("upload %s: status %d", name, code)
	}
	if status := waitForTerminal(t, base, doc.ID, 30*time.Second); status["status"] != "ready" {
		t.Fatalf("%s did not become ready: %v", name, status)
	}

	return doc.ID
}

func TestPartsOfSpeechProjectSearchComparisonAndExport(t *testing.T) {
	server := newIntegrationServer(t, &recordingNLPClient{})
	base := server.url
	projectID := createProject(t, base)

	a := uploadReady(t, base, projectID, "cats.txt",
		"The cat runs. The cat ran quickly. A cat sat. =HYPERLINK cat. cat cat cat.")
	b := uploadReady(t, base, projectID, "dogs.txt",
		"The dog runs. The dog sleeps. A dog sat. dog dog dog dog dog.")

	// --- parts of speech ---
	var pos struct {
		Items []struct {
			POS   string `json:"pos"`
			Count int    `json:"count"`
		} `json:"items"`
	}
	getJSON(t, base+"/api/documents/"+a+"/pos", &pos)
	counts := map[string]int{}
	for _, item := range pos.Items {
		counts[item.POS] = item.Count
	}
	if counts["VERB"] != 2 || counts["NOUN"] < 5 {
		t.Fatalf("unexpected tag counts: %v", counts)
	}

	var words struct {
		Items []struct {
			Lemma string `json:"lemma"`
		} `json:"items"`
		Total int `json:"total"`
	}
	getJSON(t, base+"/api/documents/"+a+"/concordance?pos=VERB", &words)
	if words.Total != 1 || words.Items[0].Lemma != "run" {
		t.Fatalf("pos filter on the word list: %+v", words)
	}
	getJSON(t, base+"/api/documents/"+a+"/concordance?pos=ADJ", &words)
	if words.Total != 0 {
		t.Fatalf("no adjectives expected, got %d", words.Total)
	}

	// --- KWIC: matches the printed word as well as the lemma, filters by tag, escapes wildcards ---
	var kwic kwicListResponse
	for query, want := range map[string]int{
		"lemma=ran":          1, // the printed form is searchable even though its lemma is "run"
		"lemma=run&pos=VERB": 2,
		"lemma=run&pos=NOUN": 0,
		"lemma=%25":          0, // a literal percent sign must not match everything
		"lemma=_":            0,
		"lemma=cat&pos=NOUN": 7,
		"lemma=ra":           0, // whole words only: "ra" is not in "ran"
		"lemma=ca*":          7, // a * widens the match
	} {
		getJSON(t, base+"/api/documents/"+a+"/kwic?limit=1&"+query, &kwic)
		if kwic.Total != want {
			t.Fatalf("%s: got=%d want=%d", query, kwic.Total, want)
		}
	}

	// --- project-wide search ---
	var across struct {
		Items []struct {
			DocumentName string `json:"documentName"`
			POS          string `json:"pos"`
		} `json:"items"`
		Total int `json:"total"`
	}
	getJSON(t, base+"/api/projects/"+projectID+"/kwic?lemma=sat", &across)
	if across.Total != 2 {
		t.Fatalf("project search should find one hit per file, got %d", across.Total)
	}
	names := map[string]bool{}
	for _, item := range across.Items {
		names[item.DocumentName] = true
	}
	if !names["cats.txt"] || !names["dogs.txt"] {
		t.Fatalf("results should say which file each hit is from: %v", names)
	}

	// --- comparison ---
	var compare struct {
		Items []struct {
			Lemma string  `json:"lemma"`
			Side  string  `json:"side"`
			Score float64 `json:"score"`
		} `json:"items"`
		Total     int    `json:"total"`
		DocumentA string `json:"documentA"`
	}
	if code := getJSON(t, base+"/api/documents/"+a+"/compare?against="+b+"&min=1", &compare); code != http.StatusOK {
		t.Fatalf("compare status %d", code)
	}
	sides := map[string]string{}
	for _, item := range compare.Items {
		sides[item.Lemma] = item.Side
		if item.Score < 0 {
			t.Fatalf("score must not be negative: %+v", item)
		}
	}
	if sides["cat"] != "a" || sides["dog"] != "b" || compare.DocumentA != "cats.txt" {
		t.Fatalf("keyness sides wrong: %v (%s)", sides, compare.DocumentA)
	}
	if len(compare.Items) > 1 && compare.Items[0].Score < compare.Items[1].Score {
		t.Fatalf("results must be ordered by score")
	}
	getJSON(t, base+"/api/documents/"+a+"/compare?against="+b+"&min=1&side=b", &compare)
	for _, item := range compare.Items {
		if item.Side != "b" {
			t.Fatalf("side filter leaked %+v", item)
		}
	}
	if code := getJSON(t, base+"/api/documents/"+a+"/compare?against="+a, nil); code != http.StatusBadRequest {
		t.Fatalf("comparing a file with itself: status %d", code)
	}
	if code := getJSON(t, base+"/api/documents/"+a+"/compare", nil); code != http.StatusBadRequest {
		t.Fatalf("missing reference: status %d", code)
	}

	// --- CSV exports ---
	csvText, header := getText(t, base+"/api/documents/"+a+"/concordance.csv")
	if !strings.HasPrefix(csvText, "\xef\xbb\xbfword,count,per_1000_words,appears_once") {
		t.Fatalf("csv should start with a BOM and header: %q", csvText[:40])
	}
	if !strings.Contains(header.Get("Content-Disposition"), "cats.txt-word-list.csv") {
		t.Fatalf("download name: %q", header.Get("Content-Disposition"))
	}
	if !strings.Contains(csvText, "'=hyperlink") {
		t.Fatalf("text starting with = must be neutralised so spreadsheets don't run it:\n%s", csvText)
	}
	if strings.Contains(csvText, "\n=hyperlink") {
		t.Fatalf("an unescaped formula reached the CSV")
	}

	kwicCSV, _ := getText(t, base+"/api/documents/"+a+"/kwic.csv?lemma="+url.QueryEscape("cat"))
	if lines := strings.Count(kwicCSV, "\n"); lines != 8 { // header + 7 matches
		t.Fatalf("kwic export: %d lines\n%s", lines, kwicCSV)
	}
	projectCSV, _ := getText(t, base+"/api/projects/"+projectID+"/kwic.csv?lemma=sat")
	if !strings.Contains(projectCSV, "dogs.txt") || !strings.Contains(projectCSV, "cats.txt") {
		t.Fatalf("project export should name the file for each line:\n%s", projectCSV)
	}
	compareCSV, _ := getText(t, base+"/api/documents/"+a+"/compare.csv?against="+b+"&min=1")
	if !strings.Contains(compareCSV, "cat,") || !strings.Contains(compareCSV, "dog,") {
		t.Fatalf("comparison export:\n%s", compareCSV)
	}

	// --- deleting the project removes its files' analysis ---
	req, _ := http.NewRequest(http.MethodDelete, base+"/api/projects/"+projectID, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete project: %v %v", err, resp)
	}
	if code := getJSON(t, base+"/api/documents/"+a+"/status", nil); code != http.StatusNotFound {
		t.Fatalf("documents should be gone with their project, status %d", code)
	}
}
