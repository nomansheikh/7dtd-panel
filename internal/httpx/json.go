package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// maxJSONBody bounds request bodies. The panel's own endpoints take small
// payloads; anything larger is a mistake or an attack.
const maxJSONBody = 1 << 20

// ErrorBody is the panel's error shape. Error is always the real reason, never
// a placeholder: the brief is explicit that operators should see the actual
// error. Code is a stable identifier the UI can branch on, and for failures
// relayed from the game server it is that server's own errorCode.
type ErrorBody struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

// WriteJSON sends v as JSON with the given status.
//
// An encoding failure is unrecoverable here: the status line and part of the
// body are already written, so there is no way to tell the client anything
// different. Handlers should marshal anything fallible before calling.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError sends a JSON error.
func WriteError(w http.ResponseWriter, status int, message, code string) {
	WriteJSON(w, status, ErrorBody{Error: message, Code: code})
}

// DecodeJSON reads a JSON request body into v, rejecting unknown fields so a
// misspelled field name is an error rather than silently ignored.
func DecodeJSON(w http.ResponseWriter, r *http.Request, v any) error {
	if ct := r.Header.Get("Content-Type"); ct != "" && !isJSON(ct) {
		return fmt.Errorf("expected Content-Type application/json, got %q", ct)
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONBody))
	dec.DisallowUnknownFields()

	if err := dec.Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("request body is empty")
		}
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	// A second value means the client sent concatenated documents.
	if dec.More() {
		return errors.New("request body must contain exactly one JSON object")
	}
	return nil
}

func isJSON(contentType string) bool {
	mediaType, _, _ := strings.Cut(contentType, ";")
	switch strings.ToLower(strings.TrimSpace(mediaType)) {
	case "application/json", "text/json":
		return true
	}
	return false
}
