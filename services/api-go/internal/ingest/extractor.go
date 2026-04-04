package ingest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Extractor struct {
	tikaEndpoint string
	httpClient   *http.Client
}

func NewExtractor(tikaEndpoint string) *Extractor {
	return &Extractor{
		tikaEndpoint: strings.TrimSuffix(strings.TrimSpace(tikaEndpoint), "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (e *Extractor) ExtractText(ctx context.Context, localPath, format string) (string, error) {
	normalizedFormat := strings.ToLower(strings.TrimSpace(format))
	if isDirectTextFormat(normalizedFormat) {
		content, err := os.ReadFile(localPath)
		if err != nil {
			return "", err
		}

		return string(content), nil
	}

	if e.tikaEndpoint == "" {
		return "", fmt.Errorf("unsupported format %q without tika endpoint", normalizedFormat)
	}

	content, err := os.ReadFile(localPath)
	if err != nil {
		return "", err
	}

	endpoint := e.tikaEndpoint + "/tika/text"
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(content))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "text/plain")
	if mimeType := tikaMIMEType(normalizedFormat, localPath); mimeType != "" {
		req.Header.Set("Content-Type", mimeType)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(body))
		if len(message) > 200 {
			message = message[:200]
		}

		return "", fmt.Errorf("tika extraction failed (%d): %s", resp.StatusCode, message)
	}

	return string(body), nil
}

func isDirectTextFormat(format string) bool {
	switch format {
	case "txt", "text", "md", "markdown", "csv", "tsv", "log":
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
