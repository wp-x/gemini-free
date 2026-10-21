package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"gemini-free/internal/generation"
)

type Options struct {
	Generator    generation.Generator
	Models       []generation.Model
	APIKeys      []string
	DefaultModel string
}

type server struct {
	options Options
}

func New(options Options) http.Handler {
	options.Models = append([]generation.Model(nil), options.Models...)
	options.APIKeys = append([]string(nil), options.APIKeys...)
	return &server{options: options}
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/health" {
		if r.Method != http.MethodGet {
			writeError(w, r, apiError{http.StatusMethodNotAllowed, "method_not_allowed", "GET required"})
			return
		}
		writeJSON(w, http.StatusOK, object{"status": "ok"})
		return
	}
	if !s.authorized(r) {
		writeError(w, r, apiError{http.StatusUnauthorized, "authentication_error", "invalid API key"})
		return
	}
	switch r.URL.Path {
	case "/v1/models":
		s.models(w, r)
	case "/v1/chat/completions":
		s.openAI(w, r)
	case "/v1/messages":
		s.anthropic(w, r)
	case "/v1/messages/count_tokens":
		s.countTokens(w, r)
	default:
		writeError(w, r, apiError{http.StatusNotFound, "not_found_error", "unknown endpoint"})
	}
}

func (s *server) authorized(r *http.Request) bool {
	if len(s.options.APIKeys) == 0 {
		return true
	}
	key := r.Header.Get("X-Api-Key")
	if bearer, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
		key = bearer
	}
	for _, expected := range s.options.APIKeys {
		if subtle.ConstantTimeCompare([]byte(key), []byte(expected)) == 1 {
			return true
		}
	}
	return false
}

func (s *server) models(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, r, apiError{http.StatusMethodNotAllowed, "method_not_allowed", "GET required"})
		return
	}
	items := make([]object, 0, len(s.options.Models))
	for _, model := range s.options.Models {
		items = append(items, object{"id": model.ID, "object": "model", "created": 0,
			"owned_by": "gemini-free", "requires_account": model.Account,
			"description": model.Description, "verification": model.Verification})
	}
	writeJSON(w, http.StatusOK, object{"object": "list", "data": items})
}

func (s *server) resolveModel(name string) (string, error) {
	if name == "" {
		name = s.options.DefaultModel
	}
	if name == "" && len(s.options.Models) > 0 {
		name = s.options.Models[0].ID
	}
	for _, model := range s.options.Models {
		if model.ID == name {
			return name, nil
		}
	}
	return "", fmt.Errorf("unknown model %q", name)
}

type object = map[string]any

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

type apiError struct {
	status  int
	kind    string
	message string
}

func writeError(w http.ResponseWriter, r *http.Request, failure apiError) {
	payload := object{"error": object{"type": failure.kind, "message": failure.message}}
	if strings.HasPrefix(r.URL.Path, "/v1/messages") {
		payload["type"] = "error"
	}
	writeJSON(w, failure.status, payload)
}

func decodeRequest(r *http.Request) (request, error) {
	if r.Method != http.MethodPost {
		return request{}, fmt.Errorf("POST required")
	}
	var input request
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&input); err != nil {
		return request{}, fmt.Errorf("invalid JSON: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return request{}, fmt.Errorf("request must contain exactly one JSON object")
	}
	if input.Messages == nil {
		return request{}, fmt.Errorf("messages must be a non-empty array")
	}
	return input, nil
}

func (s *server) prepare(w http.ResponseWriter, r *http.Request, anthropic bool) (chatContext, bool) {
	input, err := decodeRequest(r)
	if err != nil {
		requestError(w, r, err)
		return chatContext{}, false
	}
	chat, err := s.context(input, anthropic)
	if err != nil {
		requestError(w, r, err)
		return chatContext{}, false
	}
	if s.options.Generator == nil {
		writeError(w, r, apiError{http.StatusServiceUnavailable, "api_error", "generator unavailable"})
		return chatContext{}, false
	}
	w.Header().Set("X-Gemini-Token-Usage", "estimated; utf8-bytes/4")
	if len(chat.unsupported) > 0 {
		w.Header().Set("X-Gemini-Unsupported-Parameters", strings.Join(chat.unsupported, ", "))
	}
	return chat, true
}

func requestError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusBadRequest
	if r.Method != http.MethodPost {
		status = http.StatusMethodNotAllowed
	}
	writeError(w, r, apiError{status, "invalid_request_error", err.Error()})
}

func upstreamError(w http.ResponseWriter, r *http.Request, err error) {
	writeError(w, r, apiError{http.StatusBadGateway, "upstream_error", err.Error()})
}

func newID(prefix string) string {
	return prefix + rand.Text()
}
