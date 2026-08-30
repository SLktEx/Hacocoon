package interactionhttp

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/SLktEx/Hacocoon/pkg/interaction"
)

//go:embed web/*
var webFS embed.FS

type BatchReader interface {
	Batch(context.Context, int64, int) (interaction.Batch, error)
}

type publicError struct {
	Code       string                     `json:"code"`
	Line       int                        `json:"line,omitempty"`
	ByteOffset int64                      `json:"byte_offset,omitempty"`
	Kind       interaction.CorruptionKind `json:"kind,omitempty"`
}

type batchResponse struct {
	SchemaVersion int                 `json:"schema_version"`
	Events        []interaction.Event `json:"events"`
	NextOffset    int64               `json:"next_offset"`
	Error         *publicError        `json:"error,omitempty"`
}

type handler struct {
	reader BatchReader
	static http.Handler
}

func NewHandler(reader BatchReader) (http.Handler, error) {
	if reader == nil {
		return nil, interaction.ErrInvalidArgument
	}
	assets, err := fs.Sub(webFS, "web")
	if err != nil {
		return nil, err
	}
	return &handler{reader: reader, static: http.FileServer(http.FS(assets))}, nil
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")

	if r.URL.Path == "/api/v1/events" {
		h.events(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.static.ServeHTTP(w, r)
}

func (h *handler) events(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	offset, err := parseInt64Query(r, "offset", 0)
	if err != nil || offset < 0 {
		writeAPIError(w, http.StatusBadRequest, "invalid-argument")
		return
	}
	limit64, err := parseInt64Query(r, "limit", interaction.DefaultBatchSize)
	if err != nil || limit64 < 1 || limit64 > interaction.MaxBatchSize {
		writeAPIError(w, http.StatusBadRequest, "invalid-argument")
		return
	}

	batch, batchErr := h.reader.Batch(r.Context(), offset, int(limit64))
	response := batchResponse{
		SchemaVersion: batch.SchemaVersion,
		Events:        batch.Events,
		NextOffset:    batch.NextOffset,
	}
	if response.Events == nil {
		response.Events = []interaction.Event{}
	}

	if batchErr != nil {
		var corruption *interaction.CorruptionError
		switch {
		case errors.As(batchErr, &corruption):
			response.Error = &publicError{
				Code:       "source-corruption",
				Line:       corruption.Line,
				ByteOffset: corruption.ByteOffset,
				Kind:       corruption.Kind,
			}
		case errors.Is(batchErr, interaction.ErrInvalidArgument):
			writeAPIError(w, http.StatusBadRequest, "invalid-argument")
			return
		default:
			writeAPIError(w, http.StatusInternalServerError, "unavailable")
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func parseInt64Query(r *http.Request, key string, fallback int64) (int64, error) {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return fallback, nil
	}
	return strconv.ParseInt(value, 10, 64)
}

func writeAPIError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}

func IsLoopbackAddress(address string) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(address))
	if err != nil {
		return false
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
