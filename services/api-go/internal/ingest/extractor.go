package ingest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
)

const (
	tikaMinTimeout = 2 * time.Minute
	tikaMaxTimeout = 45 * time.Minute
	tikaPerMB      = 30 * time.Second
	tikaAttempts   = 3

	pdfToTextTimeout = 15 * time.Minute
)

// pdfToTextBinary is the poppler tool used for PDFs when it is installed. It is
// roughly 50x faster than Tika on large books and far lighter on memory. It is a
// variable so tests can point it at a stand-in.
var pdfToTextBinary = "pdftotext"

var hyphenatedLineBreak = regexp.MustCompile(`([a-z])-\n([a-z])`)

// tikaRetryBase is the base delay between Tika attempts (attempt number times
// this value). It is a variable so tests can run without real sleeping.
var tikaRetryBase = time.Second

type Extractor struct {
	tikaEndpoint string
	httpClient   *http.Client
}

func NewExtractor(tikaEndpoint string) *Extractor {
	return &Extractor{
		tikaEndpoint: strings.TrimSuffix(strings.TrimSpace(tikaEndpoint), "/"),
		// No client-wide timeout: each Tika request gets a deadline scaled to the
		// size of the file being extracted.
		httpClient: &http.Client{},
	}
}

// tikaUnreachableError means Tika could not be contacted at all (as opposed to
// Tika rejecting a particular file).
type tikaUnreachableError struct {
	endpoint string
	cause    error
}

func (e *tikaUnreachableError) Error() string {
	return fmt.Sprintf("tika is not reachable at %s (start it with `make dev-db-up`): %v", e.endpoint, e.cause)
}

func (e *tikaUnreachableError) Unwrap() error { return e.cause }

// ExtractText returns clean UTF-8 text for the file. It never returns an empty
// result without an error, so callers never end up with a "ready" document that
// contains nothing.
func (e *Extractor) ExtractText(ctx context.Context, localPath, format string) (string, error) {
	normalizedFormat := sniffFormat(localPath, strings.ToLower(strings.TrimSpace(format)))

	var (
		text string
		err  error
	)
	if isDirectTextFormat(normalizedFormat) {
		text, err = readTextFile(localPath)
	} else {
		text, err = e.extractBinary(ctx, localPath, normalizedFormat)
	}
	if err != nil {
		return "", err
	}

	text = CleanText(text)
	if strings.TrimSpace(text) == "" {
		if normalizedFormat == "pdf" {
			return "", errors.New("no extractable text found in this PDF; it is probably a scanned or image-only document and would need OCR")
		}

		return "", fmt.Errorf("no extractable text found in this %s file", normalizedFormat)
	}

	return text, nil
}

func (e *Extractor) extractBinary(ctx context.Context, localPath, format string) (string, error) {
	if e.tikaEndpoint == "" {
		if format == "pdf" {
			if text, available, err := extractPDFWithPoppler(ctx, localPath); available && err == nil {
				return text, nil
			}
		}

		if text, ok, err := nativeExtract(localPath, format); ok {
			return text, err
		}

		return "", fmt.Errorf("unsupported format %q without tika endpoint", format)
	}

	var popplerErr error
	if format == "pdf" {
		text, available, err := extractPDFWithPoppler(ctx, localPath)
		if available && err == nil {
			return text, nil
		}
		popplerErr = err
	}

	text, tikaErr := e.extractWithTika(ctx, localPath, format)
	if tikaErr == nil {
		if format == "pdf" {
			// Tika leaves "exam-\nple" style line-end hyphenation in place, which
			// would split one word into two tokens.
			text = hyphenatedLineBreak.ReplaceAllString(text, "$1$2")
		}

		return text, nil
	}

	if popplerErr != nil {
		return "", fmt.Errorf("%v; %w", popplerErr, tikaErr)
	}

	// Tika is the preferred extractor, but for formats we can read ourselves a
	// Tika outage or rejection should not fail the document.
	if native, ok, nativeErr := nativeExtract(localPath, format); ok {
		if nativeErr == nil {
			return native, nil
		}

		return "", fmt.Errorf("%v; built-in %s extraction also failed: %w", tikaErr, format, nativeErr)
	}

	return "", tikaErr
}

func (e *Extractor) extractWithTika(ctx context.Context, localPath, format string) (string, error) {
	content, err := os.ReadFile(localPath)
	if err != nil {
		return "", err
	}

	mimeType := tikaMIMEType(format, localPath)
	timeout := tikaTimeout(len(content))

	var lastErr error
	for attempt := 1; attempt <= tikaAttempts; attempt++ {
		text, retryable, err := e.tikaOnce(ctx, content, mimeType, timeout)
		if err == nil {
			return text, nil
		}

		lastErr = err
		if !retryable || attempt == tikaAttempts {
			break
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Duration(attempt) * tikaRetryBase):
		}
	}

	return "", lastErr
}

func (e *Extractor) tikaOnce(ctx context.Context, content []byte, mimeType string, timeout time.Duration) (string, bool, error) {
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPut, e.tikaEndpoint+"/tika", bytes.NewReader(content))
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Accept", "text/plain")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	req.Header.Set("Content-Type", mimeType)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		if errors.Is(reqCtx.Err(), context.DeadlineExceeded) {
			return "", false, fmt.Errorf("tika did not finish within %s", timeout.Round(time.Second))
		}

		return "", true, classifyTikaTransportError(e.tikaEndpoint, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", true, fmt.Errorf("read tika response: %w", err)
	}

	if resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(body))
		if len(message) > 200 {
			message = message[:200]
		}

		retryable := resp.StatusCode == http.StatusBadGateway ||
			resp.StatusCode == http.StatusServiceUnavailable ||
			resp.StatusCode == http.StatusGatewayTimeout

		hint := ""
		if resp.StatusCode == http.StatusUnprocessableEntity {
			hint = " (the file may be encrypted, password-protected, or corrupt)"
		}

		return "", retryable, fmt.Errorf("tika extraction failed (%d)%s: %s", resp.StatusCode, hint, message)
	}

	return string(body), false, nil
}

// classifyTikaTransportError separates "Tika is not running" (the connection
// could not be established) from "Tika dropped us mid-request", which is what a
// large upload looks like when Tika rejects it early or runs out of memory.
func classifyTikaTransportError(endpoint string, err error) error {
	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr.Op == "dial" {
		return &tikaUnreachableError{endpoint: endpoint, cause: err}
	}

	return fmt.Errorf("tika closed the connection while receiving the file (it may have rejected the request or run out of memory): %w", err)
}

// tikaTimeout scales the allowed extraction time with file size: two minutes
// plus 30 seconds per MB, capped at 45 minutes. (A 32 MB, 1,100-page textbook
// takes Tika around four and a half minutes.)
func tikaTimeout(sizeBytes int) time.Duration {
	timeout := tikaMinTimeout + time.Duration(sizeBytes/(1024*1024))*tikaPerMB
	if timeout > tikaMaxTimeout {
		return tikaMaxTimeout
	}

	return timeout
}

// extractPDFWithPoppler runs pdftotext when it is installed. The bool reports
// whether the tool is available at all, so callers can fall back to Tika.
func extractPDFWithPoppler(ctx context.Context, localPath string) (string, bool, error) {
	binary, err := exec.LookPath(pdfToTextBinary)
	if err != nil {
		return "", false, nil
	}

	runCtx, cancel := context.WithTimeout(ctx, pdfToTextTimeout)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(runCtx, binary, "-enc", "UTF-8", localPath, "-")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if len(message) > 200 {
			message = message[:200]
		}

		return "", true, fmt.Errorf("pdftotext failed: %v: %s", err, message)
	}

	return stdout.String(), true, nil
}

func readTextFile(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return DecodeText(raw), nil
}

// DecodeText converts raw file bytes to UTF-8, handling a UTF-8 BOM, UTF-16
// (with BOM), and legacy 8-bit text such as Windows-1252 exports from Word.
func DecodeText(raw []byte) string {
	switch {
	case bytes.HasPrefix(raw, []byte{0xEF, 0xBB, 0xBF}):
		raw = raw[3:]
	case bytes.HasPrefix(raw, []byte{0xFF, 0xFE}), bytes.HasPrefix(raw, []byte{0xFE, 0xFF}):
		decoded, err := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder().Bytes(raw)
		if err == nil {
			return string(decoded)
		}
	}

	if utf8.Valid(raw) {
		return string(raw)
	}

	decoded, err := charmap.Windows1252.NewDecoder().Bytes(raw)
	if err == nil {
		return string(decoded)
	}

	return strings.ToValidUTF8(string(raw), "")
}

// CleanText removes characters that databases and tokenizers choke on (NUL and
// other control characters, soft hyphens, zero-width marks) and normalizes page
// breaks and non-breaking spaces. Output is always valid UTF-8.
func CleanText(text string) string {
	text = strings.ToValidUTF8(text, "")

	var b strings.Builder
	b.Grow(len(text))
	for _, r := range text {
		switch {
		case r == '\n' || r == '\t' || r == '\r':
			b.WriteRune(r)
		case r == '\f':
			b.WriteString("\n\n")
		case r == '\u00a0':
			b.WriteByte(' ')
		case r < 0x20, r == 0x7f, r >= 0x80 && r <= 0x9f:
		case r == '\u00ad', r == '\u200b', r == '\ufeff':
		default:
			b.WriteRune(r)
		}
	}

	return b.String()
}

// sniffFormat corrects a declared format using the file's leading bytes, so a
// PDF or Office file with a missing or wrong extension is still handled.
func sniffFormat(path, declared string) string {
	if declared != "" && declared != "txt" && declared != "text" {
		return declared
	}

	file, err := os.Open(path)
	if err != nil {
		return declared
	}
	defer file.Close()

	header := make([]byte, 5)
	n, _ := io.ReadFull(file, header)
	header = header[:n]

	if bytes.HasPrefix(header, []byte("%PDF-")) {
		return "pdf"
	}

	if bytes.HasPrefix(header, []byte("PK\x03\x04")) {
		if detected := sniffZipFormat(path); detected != "" {
			return detected
		}
	}

	return declared
}

func isDirectTextFormat(format string) bool {
	switch format {
	case "txt", "text", "md", "markdown", "csv", "tsv", "log", "rst", "tex", "srt", "vtt":
		return true
	default:
		return false
	}
}

func tikaMIMEType(format, localPath string) string {
	switch format {
	case "pdf":
		return "application/pdf"
	case "doc":
		return "application/msword"
	case "docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case "ppt":
		return "application/vnd.ms-powerpoint"
	case "pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case "xls":
		return "application/vnd.ms-excel"
	case "xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case "rtf":
		return "application/rtf"
	case "odt":
		return "application/vnd.oasis.opendocument.text"
	case "html", "htm":
		return "text/html"
	case "epub":
		return "application/epub+zip"
	default:
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(localPath), "."))
		if ext == "" {
			return ""
		}
		if ext == "md" {
			return "text/markdown"
		}
		return "application/octet-stream"
	}
}
