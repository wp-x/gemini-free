package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"gemini-free/internal/generation"
)

func (c *Control) probe(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Prompt          string `json:"prompt"`
		Model           string `json:"model"`
		ReasoningEffort string `json:"reasoning_effort"`
	}
	if err := decodeBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(input.Prompt) == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("请输入问题"))
		return
	}
	request := generation.Request{Prompt: input.Prompt, Model: input.Model, ReasoningEffort: input.ReasoningEffort}
	if _, err := generation.Resolve(request); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("X-Accel-Buffering", "no")
	result, err := c.Controller.Generate(r.Context(), request, func(event generation.Event) error {
		return sendEvent(w, "delta", event)
	})
	if err != nil {
		if writeErr := sendEvent(w, "error", map[string]string{"error": err.Error()}); writeErr != nil {
			c.Controller.AddLog("error", "无法写入问答错误响应："+writeErr.Error())
		}
		return
	}
	if err := sendEvent(w, "result", result); err != nil {
		c.Controller.AddLog("error", "无法写入问答结果："+err.Error())
	}
}

func sendEvent(w http.ResponseWriter, name string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, data); err != nil {
		return err
	}
	return http.NewResponseController(w).Flush()
}
