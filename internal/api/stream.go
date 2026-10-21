package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type eventStream struct {
	writer  http.ResponseWriter
	context context.Context
}

func startStream(w http.ResponseWriter, r *http.Request) *eventStream {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	return &eventStream{writer: w, context: r.Context()}
}

func (s *eventStream) send(event string, payload any) error {
	if err := s.context.Err(); err != nil {
		return err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if event != "" {
		if _, err := fmt.Fprintf(s.writer, "event: %s\n", event); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(s.writer, "data: %s\n\n", data); err != nil {
		return err
	}
	return http.NewResponseController(s.writer).Flush()
}

func (s *eventStream) done() error {
	if err := s.context.Err(); err != nil {
		return err
	}
	if _, err := fmt.Fprint(s.writer, "data: [DONE]\n\n"); err != nil {
		return err
	}
	return http.NewResponseController(s.writer).Flush()
}

func (s *eventStream) failure(err error, anthropic bool) {
	if s.context.Err() != nil {
		return
	}
	payload := object{"error": object{"type": "upstream_error", "message": err.Error()}}
	if anthropic {
		payload["type"] = "error"
		_ = s.send("error", payload)
		return
	}
	_ = s.send("", payload)
	_ = s.done()
}
