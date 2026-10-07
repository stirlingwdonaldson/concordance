package ingest

import (
	"archive/zip"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func init() {
	tikaRetryBase = 0
	// Keep tests independent of whether poppler happens to be installed.
	pdfToTextBinary = "pdftotext-not-installed-for-tests"
}

func writeFile(t *testing.T, name string, content []byte) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}

	return path
}

func writeZip(t *testing.T, name string, files map[string]string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	out, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	defer out.Close()

	zw := zip.NewWriter(out)
	for entry, content := range files {
		w, err := zw.Create(entry)
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	return path
}

func deadTikaURL() string {
	server := httptest.NewServer(http.NotFoundHandler())
	url := server.URL
	server.Close()

	return url
}

func TestDecodeTextHandlesCommonEncodings(t *testing.T) {
	cases := map[string]struct {
		raw  []byte
		want string
	}{
		"utf8":        {[]byte("café “quoted”"), "café “quoted”"},
		"utf8 bom":    {append([]byte{0xEF, 0xBB, 0xBF}, []byte("hello")...), "hello"},
		"utf16le bom": {[]byte{0xFF, 0xFE, 'h', 0, 0xE9, 0, 'y', 0}, "héy"},
		"utf16be bom": {[]byte{0xFE, 0xFF, 0, 'h', 0, 0xE9, 0, 'y'}, "héy"},
		"windows1252": {[]byte("caf\xe9 \x93quoted\x94 \x97 dash"), "café “quoted” — dash"},
	}

	for name, tc := range cases {
		if got := DecodeText(tc.raw); got != tc.want {
			t.Errorf("%s: got=%q want=%q", name, got, tc.want)
		}
	}
}

func TestCleanTextRemovesCharactersThatBreakStorage(t *testing.T) {
	input := "a\x00b c­​d\x07e\f\u0085f\xffg"
	got := CleanText(input)

	want := "ab cde\n\nfg"
	if got != want {
		t.Fatalf("got=%q want=%q", got, want)
	}
}

func TestExtractTextReadsLegacyEncodedTextFile(t *testing.T) {
	path := writeFile(t, "old.txt", []byte("caf\xe9 society"))

	text, err := NewExtractor("").ExtractText(context.Background(), path, "txt")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if text != "café society" {
		t.Fatalf("unexpected text: %q", text)
	}
}

func TestExtractTextFailsClearlyWhenThereIsNoText(t *testing.T) {
	path := writeFile(t, "empty.txt", []byte("  \n\x00\n"))

	_, err := NewExtractor("").ExtractText(context.Background(), path, "txt")
	if err == nil || !strings.Contains(err.Error(), "no extractable text") {
		t.Fatalf("expected a no-text error, got %v", err)
	}
}

func TestExtractTextExplainsScannedPDFs(t *testing.T) {
	path := writeFile(t, "scan.pdf", []byte("%PDF-1.4"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("\n\n  \n"))
	}))
	defer server.Close()

	_, err := NewExtractor(server.URL).ExtractText(context.Background(), path, "pdf")
	if err == nil || !strings.Contains(err.Error(), "OCR") {
		t.Fatalf("expected an OCR hint, got %v", err)
	}
}

func TestTikaIsRetriedOnTransientFailures(t *testing.T) {
	path := writeFile(t, "book.pdf", []byte("%PDF-1.4 fake"))

	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("finally extracted"))
	}))
	defer server.Close()

	text, err := NewExtractor(server.URL).ExtractText(context.Background(), path, "pdf")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if text != "finally extracted" || atomic.LoadInt32(&calls) != 3 {
		t.Fatalf("text=%q calls=%d", text, calls)
	}
}

func TestTikaFileProblemsAreNotRetriedAndAreExplained(t *testing.T) {
	path := writeFile(t, "locked.pdf", []byte("%PDF-1.4 fake"))

	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte("encrypted"))
	}))
	defer server.Close()

	_, err := NewExtractor(server.URL).ExtractText(context.Background(), path, "pdf")
	if err == nil || !strings.Contains(err.Error(), "encrypted") {
		t.Fatalf("expected an explanatory error, got %v", err)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("expected a single attempt, got %d", calls)
	}
}

func TestUnreachableTikaTellsYouHowToStartIt(t *testing.T) {
	path := writeFile(t, "book.pdf", []byte("%PDF-1.4 fake"))

	_, err := NewExtractor(deadTikaURL()).ExtractText(context.Background(), path, "pdf")
	if err == nil || !strings.Contains(err.Error(), "make dev-db-up") {
		t.Fatalf("expected a how-to-fix hint, got %v", err)
	}
}

func TestPDFWithWrongExtensionIsStillSentToTika(t *testing.T) {
	path := writeFile(t, "chapter.txt", []byte("%PDF-1.7 pretend pdf bytes"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Content-Type"); got != "application/pdf" {
			t.Errorf("content type: got=%s", got)
		}
		_, _ = w.Write([]byte("real text"))
	}))
	defer server.Close()

	text, err := NewExtractor(server.URL).ExtractText(context.Background(), path, "txt")
	if err != nil || text != "real text" {
		t.Fatalf("text=%q err=%v", text, err)
	}
}

const docxBody = `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>
<w:p><w:r><w:t>First paragraph</w:t></w:r><w:r><w:t xml:space="preserve"> continues.</w:t></w:r></w:p>
<w:p><w:r><w:t>Second</w:t></w:r><w:r><w:tab/><w:t>paragraph.</w:t></w:r></w:p>
</w:body></w:document>`

func TestDocxIsReadWithoutTika(t *testing.T) {
	path := writeZip(t, "notes.docx", map[string]string{"word/document.xml": docxBody})

	for name, endpoint := range map[string]string{"no endpoint": "", "tika down": deadTikaURL()} {
		text, err := NewExtractor(endpoint).ExtractText(context.Background(), path, "docx")
		if err != nil {
			t.Fatalf("%s: extract: %v", name, err)
		}
		if !strings.Contains(text, "First paragraph continues.") || !strings.Contains(text, "Second\tparagraph.") {
			t.Fatalf("%s: unexpected text %q", name, text)
		}
		if !strings.Contains(text, "continues.\n\nSecond") {
			t.Fatalf("%s: paragraphs should be separated by a blank line: %q", name, text)
		}
	}
}

func TestDocxWithoutExtensionIsDetectedFromContents(t *testing.T) {
	path := writeZip(t, "mystery", map[string]string{"word/document.xml": docxBody})

	text, err := NewExtractor("").ExtractText(context.Background(), path, "txt")
	if err != nil || !strings.Contains(text, "First paragraph") {
		t.Fatalf("text=%q err=%v", text, err)
	}
}

func TestEpubChaptersFollowSpineOrder(t *testing.T) {
	path := writeZip(t, "book.epub", map[string]string{
		"META-INF/container.xml": `<container><rootfiles><rootfile full-path="OEBPS/content.opf"/></rootfiles></container>`,
		"OEBPS/content.opf": `<package><manifest>
<item id="b" href="chapter%20two.xhtml"/><item id="a" href="one.xhtml"/></manifest>
<spine><itemref idref="a"/><itemref idref="b"/></spine></package>`,
		"OEBPS/one.xhtml":         `<html><head><title>ignore me</title></head><body><h1>One</h1><p>first chapter</p></body></html>`,
		"OEBPS/chapter two.xhtml": `<html><body><p>second chapter</p></body></html>`,
	})

	text, err := NewExtractor(deadTikaURL()).ExtractText(context.Background(), path, "epub")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	first, second := strings.Index(text, "first chapter"), strings.Index(text, "second chapter")
	if first < 0 || second < 0 || first > second {
		t.Fatalf("chapters missing or out of order: %q", text)
	}
	if strings.Contains(text, "ignore me") {
		t.Fatalf("head content leaked into text: %q", text)
	}
}

func TestHTMLDropsScriptsAndKeepsParagraphs(t *testing.T) {
	path := writeFile(t, "page.html", []byte(`<html><head><style>p{}</style></head><body>
<p>Hello
   there &amp; welcome.</p><script>var x = "nope";</script><p>Second.</p></body></html>`))

	text, err := NewExtractor("").ExtractText(context.Background(), path, "html")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if text != "Hello there & welcome.\n\nSecond." {
		t.Fatalf("unexpected text: %q", text)
	}
}

func TestTikaTimeoutScalesWithFileSizeAndIsCapped(t *testing.T) {
	if got := tikaTimeout(0); got != 2*time.Minute {
		t.Fatalf("small file: %v", got)
	}
	// A 32 MB textbook took Tika 4.5 minutes; the allowance must clear that easily.
	if got := tikaTimeout(32 * 1024 * 1024); got < 15*time.Minute {
		t.Fatalf("32MB file gets too little time: %v", got)
	}
	if got := tikaTimeout(5 * 1024 * 1024 * 1024); got != 45*time.Minute {
		t.Fatalf("huge file should be capped: %v", got)
	}
}

func TestTikaRequestShapeMatchesWhatRealTikaAccepts(t *testing.T) {
	// Real Tika answers 406 to "/tika/text" with "Accept: text/plain", and returns
	// nothing useful when no Content-Type is sent.
	path := writeFile(t, "mystery.xyz", []byte("some bytes"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tika" || r.Header.Get("Accept") != "text/plain" {
			w.WriteHeader(http.StatusNotAcceptable)
			return
		}
		if r.Header.Get("Content-Type") == "" {
			t.Errorf("a content type must always be sent")
		}
		_, _ = w.Write([]byte("ok text"))
	}))
	defer server.Close()

	text, err := NewExtractor(server.URL).ExtractText(context.Background(), path, "xyz")
	if err != nil || text != "ok text" {
		t.Fatalf("text=%q err=%v", text, err)
	}
}

func TestDroppedConnectionIsNotReportedAsTikaBeingDown(t *testing.T) {
	endpoint := "http://127.0.0.1:9998"

	dial := classifyTikaTransportError(endpoint, &net.OpError{Op: "dial", Err: errors.New("connection refused")})
	var unreachable *tikaUnreachableError
	if !errors.As(dial, &unreachable) || !strings.Contains(dial.Error(), "make dev-db-up") {
		t.Fatalf("a refused connection means Tika is not running: %v", dial)
	}

	// This is the failure seen with a 32 MB PDF: Tika replied early and closed.
	write := classifyTikaTransportError(endpoint, &net.OpError{Op: "write", Err: errors.New("broken pipe")})
	if errors.As(write, &unreachable) || strings.Contains(write.Error(), "make dev-db-up") {
		t.Fatalf("a mid-upload drop must not claim Tika is down: %v", write)
	}
	if !strings.Contains(write.Error(), "closed the connection") {
		t.Fatalf("unhelpful message: %v", write)
	}
}

func writeFakePDFToText(t *testing.T, script string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "pdftotext")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatalf("write fake pdftotext: %v", err)
	}

	previous := pdfToTextBinary
	pdfToTextBinary = path
	t.Cleanup(func() { pdfToTextBinary = previous })
}

func neverCalledTika(t *testing.T) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Errorf("tika must not be called when pdftotext succeeds")
	}))
	t.Cleanup(server.Close)

	return server
}

func TestPDFsPreferPopplerWhenInstalled(t *testing.T) {
	writeFakePDFToText(t, `echo "text from poppler"`)
	path := writeFile(t, "book.pdf", []byte("%PDF-1.4 fake"))

	text, err := NewExtractor(neverCalledTika(t).URL).ExtractText(context.Background(), path, "pdf")
	if err != nil || strings.TrimSpace(text) != "text from poppler" {
		t.Fatalf("text=%q err=%v", text, err)
	}

	// Also works with no Tika configured at all.
	text, err = NewExtractor("").ExtractText(context.Background(), path, "pdf")
	if err != nil || strings.TrimSpace(text) != "text from poppler" {
		t.Fatalf("without tika: text=%q err=%v", text, err)
	}
}

func TestPDFFallsBackToTikaWhenPopplerFailsAndJoinsHyphenatedWords(t *testing.T) {
	writeFakePDFToText(t, `echo "Syntax Error: Couldn't read xref" >&2; exit 1`)
	path := writeFile(t, "book.pdf", []byte("%PDF-1.4 fake"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("a good exam-\nple of Berke-\nley and well-known facts, first-class"))
	}))
	defer server.Close()

	text, err := NewExtractor(server.URL).ExtractText(context.Background(), path, "pdf")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if !strings.Contains(text, "example of Berkeley") || !strings.Contains(text, "well-known") {
		t.Fatalf("hyphenation handling wrong: %q", text)
	}
}

func TestPopplerAndTikaFailuresAreBothReported(t *testing.T) {
	writeFakePDFToText(t, `echo "Incorrect password" >&2; exit 1`)
	path := writeFile(t, "locked.pdf", []byte("%PDF-1.4 fake"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))
	defer server.Close()

	_, err := NewExtractor(server.URL).ExtractText(context.Background(), path, "pdf")
	if err == nil || !strings.Contains(err.Error(), "Incorrect password") || !strings.Contains(err.Error(), "422") {
		t.Fatalf("both reasons should be visible: %v", err)
	}
}

func TestScannedPDFWithPopplerGetsOCRHint(t *testing.T) {
	writeFakePDFToText(t, `printf '\f\f'`)
	path := writeFile(t, "scan.pdf", []byte("%PDF-1.4 fake"))

	_, err := NewExtractor("").ExtractText(context.Background(), path, "pdf")
	if err == nil || !strings.Contains(err.Error(), "OCR") {
		t.Fatalf("expected an OCR hint, got %v", err)
	}
}
