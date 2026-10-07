package ingest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractTextReadsDirectTextFormats(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "sample.md")
	if err := os.WriteFile(path, []byte("# title\nhello"), 0o644); err != nil {
		t.Fatalf("write sample file: %v", err)
	}

	extractor := NewExtractor("")
	text, err := extractor.ExtractText(context.Background(), path, "md")
	if err != nil {
		t.Fatalf("extract text: %v", err)
	}

	if text != "# title\nhello" {
		t.Fatalf("unexpected extracted text: got=%q", text)
	}
}

func TestExtractTextUsesTikaForBinaryFormats(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "sample.pdf")
	if err := os.WriteFile(path, []byte("fake-pdf-content"), 0o644); err != nil {
		t.Fatalf("write sample file: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("unexpected method: got=%s", r.Method)
		}
		if r.URL.Path != "/tika" {
			t.Fatalf("unexpected path: got=%s", r.URL.Path)
		}
		if got := r.Header.Get("Content-Type"); got != "application/pdf" {
			t.Fatalf("unexpected content type: got=%s", got)
		}
		if got := r.Header.Get("Accept"); got != "text/plain" {
			t.Fatalf("tika answers 406 unless Accept is text/plain: got=%s", got)
		}
		_, _ = w.Write([]byte("extracted from tika"))
	}))
	defer server.Close()

	extractor := NewExtractor(server.URL)
	text, err := extractor.ExtractText(context.Background(), path, "pdf")
	if err != nil {
		t.Fatalf("extract text: %v", err)
	}

	if text != "extracted from tika" {
		t.Fatalf("unexpected extracted text: got=%q", text)
	}
}

func TestExtractTextErrorsWithoutTikaForBinaryFormats(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "sample.pdf")
	if err := os.WriteFile(path, []byte("fake-pdf-content"), 0o644); err != nil {
		t.Fatalf("write sample file: %v", err)
	}

	extractor := NewExtractor("")
	_, err := extractor.ExtractText(context.Background(), path, "pdf")
	if err == nil {
		t.Fatalf("expected extraction error")
	}

	if !strings.Contains(err.Error(), "without tika endpoint") {
		t.Fatalf("unexpected error: %v", err)
	}
}
