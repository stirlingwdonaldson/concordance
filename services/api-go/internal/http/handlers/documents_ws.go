package handlers

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"concordance/services/api-go/internal/documents"
)

const websocketAcceptGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

func (h DocumentsHandler) StreamJobs(w http.ResponseWriter, r *http.Request) {
	if h.events == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "jobs stream unavailable"})
		return
	}

	filter := documents.JobEventFilter{
		DocumentID: strings.TrimSpace(r.URL.Query().Get("documentId")),
		ProjectID:  strings.TrimSpace(r.URL.Query().Get("projectId")),
	}

	if filter.DocumentID != "" {
		doc, ok := h.store.Get(r.Context(), filter.DocumentID)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "document not found"})
			return
		}
		if filter.ProjectID != "" && doc.ProjectID != filter.ProjectID {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "document does not belong to project"})
			return
		}
		filter.ProjectID = doc.ProjectID
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "websocket unsupported"})
		return
	}

	if err := validateWebSocketUpgrade(r); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	key := r.Header.Get("Sec-WebSocket-Key")
	accept := computeWebSocketAccept(key)

	conn, rw, err := hijacker.Hijack()
	if err != nil {
		return
	}
	defer conn.Close()

	if err := writeUpgradeResponse(rw, accept); err != nil {
		return
	}

	events, unsubscribe := h.events.Subscribe(filter)
	defer unsubscribe()

	if err := h.writeInitialSnapshot(r.Context(), conn, filter); err != nil {
		return
	}

	heartbeatTicker := time.NewTicker(25 * time.Second)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeatTicker.C:
			event := documents.NewJobEvent("heartbeat", filter.DocumentID, filter.ProjectID, map[string]any{})
			if err := writeWebSocketJSON(conn, event); err != nil {
				return
			}
		case event, ok := <-events:
			if !ok {
				return
			}

			if err := writeWebSocketJSON(conn, event); err != nil {
				return
			}
		}
	}
}

func (h DocumentsHandler) writeInitialSnapshot(ctx context.Context, conn net.Conn, filter documents.JobEventFilter) error {
	if filter.DocumentID == "" {
		return writeWebSocketJSON(conn, documents.NewJobEvent("snapshot", "", filter.ProjectID, map[string]any{
			"documents": []map[string]any{},
		}))
	}

	doc, ok := h.store.Get(ctx, filter.DocumentID)
	if !ok {
		return errors.New("document not found")
	}

	jobs, err := h.store.ListPipelineJobs(ctx, filter.DocumentID)
	if err != nil {
		return err
	}

	event := documents.NewJobEvent("snapshot", doc.ID, doc.ProjectID, map[string]any{
		"document": map[string]any{
			"status":    doc.Status,
			"progress":  doc.Progress,
			"updatedAt": doc.UpdatedAt,
		},
		"jobs": jobs,
	})

	return writeWebSocketJSON(conn, event)
}

func validateWebSocketUpgrade(r *http.Request) error {
	if r.Method != http.MethodGet {
		return errors.New("websocket requires GET")
	}

	connection := strings.ToLower(r.Header.Get("Connection"))
	if !strings.Contains(connection, "upgrade") {
		return errors.New("missing connection upgrade header")
	}

	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return errors.New("missing websocket upgrade header")
	}

	if r.Header.Get("Sec-WebSocket-Version") != "13" {
		return errors.New("unsupported websocket version")
	}

	if strings.TrimSpace(r.Header.Get("Sec-WebSocket-Key")) == "" {
		return errors.New("missing websocket key")
	}

	return nil
}

func computeWebSocketAccept(key string) string {
	hasher := sha1.New()
	_, _ = hasher.Write([]byte(key + websocketAcceptGUID))
	return base64.StdEncoding.EncodeToString(hasher.Sum(nil))
}

func writeUpgradeResponse(rw *bufio.ReadWriter, accept string) error {
	response := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n" +
		"\r\n"

	if _, err := rw.WriteString(response); err != nil {
		return err
	}

	return rw.Flush()
}

func writeWebSocketJSON(conn net.Conn, payload any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return writeWebSocketFrame(conn, 0x1, encoded)
}

func writeWebSocketFrame(conn net.Conn, opcode byte, payload []byte) error {
	header := []byte{0x80 | opcode}
	payloadLen := len(payload)

	switch {
	case payloadLen <= 125:
		header = append(header, byte(payloadLen))
	case payloadLen <= 65535:
		header = append(header, 126)
		ext := make([]byte, 2)
		binary.BigEndian.PutUint16(ext, uint16(payloadLen))
		header = append(header, ext...)
	default:
		header = append(header, 127)
		ext := make([]byte, 8)
		binary.BigEndian.PutUint64(ext, uint64(payloadLen))
		header = append(header, ext...)
	}

	if _, err := conn.Write(header); err != nil {
		return err
	}

	_, err := conn.Write(payload)
	return err
}
