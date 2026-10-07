package ingest

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/net/html"
)

// maxEntryBytes bounds how much of any single archive entry is read, guarding
// against decompression bombs.
const maxEntryBytes = 512 * 1024 * 1024

var (
	htmlWhitespaceRun = regexp.MustCompile(`[ \t\r\n\f]+`)
	lineEdgeSpaces    = regexp.MustCompile(`[ \t]*\n[ \t]*`)
	manyBlankLines    = regexp.MustCompile(`\n{3,}`)
)

// nativeExtract reads formats we can parse without Tika. The bool reports
// whether the format is handled natively at all.
func nativeExtract(localPath, format string) (string, bool, error) {
	switch format {
	case "docx":
		text, err := extractDOCX(localPath)
		return text, true, err
	case "odt":
		text, err := extractODT(localPath)
		return text, true, err
	case "epub":
		text, err := extractEPUB(localPath)
		return text, true, err
	case "html", "htm":
		raw, err := os.ReadFile(localPath)
		if err != nil {
			return "", true, err
		}
		text, err := htmlToText(strings.NewReader(DecodeText(raw)))
		return text, true, err
	default:
		return "", false, nil
	}
}

// sniffZipFormat identifies Office/EPUB containers that arrived without a
// useful file extension.
func sniffZipFormat(localPath string) string {
	zr, err := zip.OpenReader(localPath)
	if err != nil {
		return ""
	}
	defer zr.Close()

	for _, f := range zr.File {
		switch f.Name {
		case "word/document.xml":
			return "docx"
		case "META-INF/container.xml":
			return "epub"
		case "content.xml":
			return "odt"
		}
	}

	return ""
}

func readZipEntry(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	return io.ReadAll(io.LimitReader(rc, maxEntryBytes))
}

func findZipEntry(zr *zip.ReadCloser, name string) *zip.File {
	for _, f := range zr.File {
		if f.Name == name {
			return f
		}
	}

	return nil
}

// extractDOCX pulls paragraph text out of word/document.xml.
func extractDOCX(localPath string) (string, error) {
	zr, err := zip.OpenReader(localPath)
	if err != nil {
		return "", fmt.Errorf("open docx: %w", err)
	}
	defer zr.Close()

	entry := findZipEntry(zr, "word/document.xml")
	if entry == nil {
		return "", errors.New("docx has no word/document.xml")
	}

	raw, err := readZipEntry(entry)
	if err != nil {
		return "", err
	}

	decoder := xml.NewDecoder(strings.NewReader(string(raw)))
	decoder.Strict = false

	var (
		b      strings.Builder
		inText bool
	)
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("parse docx: %w", err)
		}

		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t":
				inText = true
			case "tab":
				b.WriteByte('\t')
			case "br", "cr":
				b.WriteByte('\n')
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inText = false
			case "p":
				b.WriteString("\n\n")
			}
		case xml.CharData:
			if inText {
				b.Write(t)
			}
		}
	}

	return b.String(), nil
}

// extractODT pulls paragraph and heading text out of content.xml.
func extractODT(localPath string) (string, error) {
	zr, err := zip.OpenReader(localPath)
	if err != nil {
		return "", fmt.Errorf("open odt: %w", err)
	}
	defer zr.Close()

	entry := findZipEntry(zr, "content.xml")
	if entry == nil {
		return "", errors.New("odt has no content.xml")
	}

	raw, err := readZipEntry(entry)
	if err != nil {
		return "", err
	}

	decoder := xml.NewDecoder(strings.NewReader(string(raw)))
	decoder.Strict = false

	var (
		b     strings.Builder
		depth int
	)
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("parse odt: %w", err)
		}

		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p", "h":
				depth++
			case "tab":
				b.WriteByte('\t')
			case "line-break":
				b.WriteByte('\n')
			case "s":
				b.WriteByte(' ')
			}
		case xml.EndElement:
			if t.Name.Local == "p" || t.Name.Local == "h" {
				if depth > 0 {
					depth--
				}
				if depth == 0 {
					b.WriteString("\n\n")
				}
			}
		case xml.CharData:
			if depth > 0 {
				b.Write(t)
			}
		}
	}

	return b.String(), nil
}

type epubContainer struct {
	Rootfiles []struct {
		FullPath string `xml:"full-path,attr"`
	} `xml:"rootfiles>rootfile"`
}

type epubPackage struct {
	Manifest []struct {
		ID   string `xml:"id,attr"`
		Href string `xml:"href,attr"`
	} `xml:"manifest>item"`
	Spine []struct {
		IDRef string `xml:"idref,attr"`
	} `xml:"spine>itemref"`
}

// extractEPUB reads chapters in spine (reading) order, falling back to archive
// order if the package metadata is unusable.
func extractEPUB(localPath string) (string, error) {
	zr, err := zip.OpenReader(localPath)
	if err != nil {
		return "", fmt.Errorf("open epub: %w", err)
	}
	defer zr.Close()

	entries := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		entries[f.Name] = f
	}

	ordered := epubSpinePaths(entries)
	if len(ordered) == 0 {
		for name := range entries {
			lower := strings.ToLower(name)
			if strings.HasSuffix(lower, ".xhtml") || strings.HasSuffix(lower, ".html") || strings.HasSuffix(lower, ".htm") {
				ordered = append(ordered, name)
			}
		}
		sort.Strings(ordered)
	}

	if len(ordered) == 0 {
		return "", errors.New("epub contains no readable chapters")
	}

	var b strings.Builder
	for _, name := range ordered {
		entry, ok := entries[name]
		if !ok {
			continue
		}

		raw, err := readZipEntry(entry)
		if err != nil {
			return "", err
		}

		chapter, err := htmlToText(strings.NewReader(string(raw)))
		if err != nil {
			return "", fmt.Errorf("parse epub chapter %s: %w", name, err)
		}

		b.WriteString(chapter)
		b.WriteString("\n\n")
	}

	return b.String(), nil
}

func epubSpinePaths(entries map[string]*zip.File) []string {
	containerEntry, ok := entries["META-INF/container.xml"]
	if !ok {
		return nil
	}

	containerRaw, err := readZipEntry(containerEntry)
	if err != nil {
		return nil
	}

	var container epubContainer
	if err := xml.Unmarshal(containerRaw, &container); err != nil || len(container.Rootfiles) == 0 {
		return nil
	}

	opfPath := container.Rootfiles[0].FullPath
	opfEntry, ok := entries[opfPath]
	if !ok {
		return nil
	}

	opfRaw, err := readZipEntry(opfEntry)
	if err != nil {
		return nil
	}

	var pkg epubPackage
	if err := xml.Unmarshal(opfRaw, &pkg); err != nil {
		return nil
	}

	hrefByID := make(map[string]string, len(pkg.Manifest))
	for _, item := range pkg.Manifest {
		hrefByID[item.ID] = item.Href
	}

	baseDir := path.Dir(opfPath)
	paths := make([]string, 0, len(pkg.Spine))
	for _, ref := range pkg.Spine {
		href, ok := hrefByID[ref.IDRef]
		if !ok {
			continue
		}

		if unescaped, err := url.PathUnescape(href); err == nil {
			href = unescaped
		}
		if cut := strings.IndexAny(href, "#?"); cut >= 0 {
			href = href[:cut]
		}

		paths = append(paths, path.Clean(path.Join(baseDir, href)))
	}

	return paths
}

func isHTMLSkipTag(name string) bool {
	switch name {
	case "script", "style", "head", "noscript", "svg":
		return true
	default:
		return false
	}
}

func htmlBreakAfter(name string) string {
	switch name {
	case "p", "div", "h1", "h2", "h3", "h4", "h5", "h6", "section", "article", "blockquote", "pre", "table", "ul", "ol":
		return "\n\n"
	case "br", "li", "tr":
		return "\n"
	default:
		return ""
	}
}

// htmlToText converts HTML to plain text with paragraph breaks preserved.
func htmlToText(r io.Reader) (string, error) {
	z := html.NewTokenizer(r)

	var (
		b    strings.Builder
		skip int
	)
	for {
		switch z.Next() {
		case html.ErrorToken:
			if err := z.Err(); !errors.Is(err, io.EOF) {
				return "", err
			}

			return tidyHTMLText(b.String()), nil
		case html.TextToken:
			if skip == 0 {
				b.WriteString(htmlWhitespaceRun.ReplaceAllString(string(z.Text()), " "))
			}
		case html.StartTagToken:
			name, _ := z.TagName()
			tag := string(name)
			if isHTMLSkipTag(tag) {
				skip++
			} else if tag == "br" {
				b.WriteString("\n")
			}
		case html.SelfClosingTagToken:
			name, _ := z.TagName()
			if string(name) == "br" {
				b.WriteString("\n")
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			tag := string(name)
			if isHTMLSkipTag(tag) {
				if skip > 0 {
					skip--
				}
			} else if skip == 0 {
				b.WriteString(htmlBreakAfter(tag))
			}
		}
	}
}

func tidyHTMLText(text string) string {
	text = lineEdgeSpaces.ReplaceAllString(text, "\n")
	text = manyBlankLines.ReplaceAllString(text, "\n\n")

	return strings.TrimSpace(text)
}
