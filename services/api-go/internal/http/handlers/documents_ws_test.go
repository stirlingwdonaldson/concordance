package handlers

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"concordance/services/api-go/internal/documents"
)

func TestStreamJobsWebSocketHandshakeAndEvents(t *testing.T) {
	b := documents.NewJobEventBroker()
	store := documents.NewMemoryStore(b)
	doc, err := store.Create(context.Background(), documents.CreateInput{
		ProjectID:  "p1",
		FileName:   "sample.txt",
		LocalPath:  "/tmp/sample.txt",
		SourceHash: "hash",
		Format:     "txt",
	})
	if err != nil {
		t.Fatalf("create doc: %v", err)
	}

	h := NewDocumentsHandler(store, t.TempDir(), b)
	server := httptest.NewServer(http.HandlerFunc(h.StreamJobs))
	defer server.Close()

	conn, reader := dialWebSocket(t, server.URL+"?documentId="+url.QueryEscape(doc.ID))
	defer conn.Close()

	snapshot := readEventFrame(t, reader)
	if snapshot.EventType != "snapshot" {
		t.Fatalf("expected first frame snapshot, got=%s", snapshot.EventType)
	}

	b.Publish(documents.NewJobEvent("stage_started", doc.ID, doc.ProjectID, map[string]any{"stage": "ingest_parse"}))

	event := readEventFrame(t, reader)
	if event.EventType != "stage_started" {
		t.Fatalf("expected stage_started event, got=%s", event.EventType)
	}
}

func dialWebSocket(t *testing.T, rawURL string) (net.Conn, *bufio.Reader) {
	t.Helper()

	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	conn, err := net.Dial("tcp", u.Host)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	keyBytes := make([]byte, 16)
	_, _ = rand.Read(keyBytes)
	key := base64.StdEncoding.EncodeToString(keyBytes)

	request := strings.Join([]string{
		"GET " + u.RequestURI() + " HTTP/1.1",
		"Host: " + u.Host,
		"Connection: Upgrade",
		"Upgrade: websocket",
		"Sec-WebSocket-Version: 13",
		"Sec-WebSocket-Key: " + key,
		"",
		"",
	}, "\r\n")

	if _, err := conn.Write([]byte(request)); err != nil {
		t.Fatalf("write request: %v", err)
	}

	reader := bufio.NewReader(conn)
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read status line: %v", err)
	}
	if !strings.Contains(statusLine, "101") {
		t.Fatalf("expected 101 status, got=%s", strings.TrimSpace(statusLine))
	}

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read header line: %v", err)
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}

	return conn, reader
}

func readEventFrame(t *testing.T, reader *bufio.Reader) documents.JobEvent {
	t.Helper()

	first, err := reader.ReadByte()
	if err != nil {
		t.Fatalf("read frame first byte: %v", err)
	}

	opcode := first & 0x0F
	if opcode != 0x1 {
		t.Fatalf("expected text frame opcode, got=%d", opcode)
	}

	second, err := reader.ReadByte()
	if err != nil {
		t.Fatalf("read frame second byte: %v", err)
	}
	masked := second&0x80 != 0
	if masked {
		t.Fatalf("expected server frame to be unmasked")
	}

	lengthCode := int(second & 0x7F)
	payloadLen := 0
	switch lengthCode {
	case 126:
		ext := make([]byte, 2)
		if _, err := io.ReadFull(reader, ext); err != nil {
			t.Fatalf("read extended length: %v", err)
		}
		payloadLen = int(binary.BigEndian.Uint16(ext))
	case 127:
		ext := make([]byte, 8)
		if _, err := io.ReadFull(reader, ext); err != nil {
			t.Fatalf("read extended length: %v", err)
		}
		payloadLen = int(binary.BigEndian.Uint64(ext))
	default:
		payloadLen = lengthCode
	}

	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(reader, payload); err != nil {
		t.Fatalf("read payload: %v", err)
	}

	var event documents.JobEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		t.Fatalf("decode event payload: %v", err)
	}

	return event
}
